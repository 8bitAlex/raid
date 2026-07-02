package profile

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/errs"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

// Injectable for testing.
var (
	gitCloneFunc = func(repoURL, dir string) error {
		out, err := exec.Command("git", "clone", "--depth", "1", repoURL, dir).CombinedOutput()
		if err != nil {
			return cloneError(err, out)
		}
		return nil
	}
	httpGetFunc = func(rawURL string) ([]byte, error) {
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Get(rawURL) //nolint:gosec
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
		}
		const maxBytes = 10 * 1024 * 1024
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > maxBytes {
			return nil, fmt.Errorf("response from %s exceeds 10 MB limit", rawURL)
		}
		return data, nil
	}
	// detectGitURL is injectable so tests can skip the live git ls-remote probe.
	detectGitURL = isGitURL
	getHomeDir   = sys.GetHomeDir
)

// cloneError shapes a failed `git clone` into an error that carries git's
// own diagnostics instead of a bare "exit status 128". Output is trimmed and
// truncated so a pathological clone (huge remote banner) can't flood the
// error message.
func cloneError(err error, output []byte) error {
	const maxLen = 300
	msg := strings.TrimSpace(string(output))
	if len(msg) > maxLen {
		cut := maxLen
		// Back up to a rune boundary so the cut never splits a UTF-8 sequence.
		for cut > 0 && !utf8.RuneStart(msg[cut]) {
			cut--
		}
		msg = msg[:cut] + "…"
	}
	if msg == "" {
		return err
	}
	return fmt.Errorf("%w: %s", err, msg)
}

// isURL reports whether s is an HTTP, HTTPS, or git SSH URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@")
}

// isGitURL reports whether rawURL points to a clonable git repository.
// git@ and .git-suffix URLs are always git. HTTP URLs with a recognised file
// extension (.yaml/.yml/.json) are raw files; all others are probed with ls-remote.
func isGitURL(rawURL string) bool {
	if strings.HasPrefix(rawURL, "git@") || strings.HasSuffix(rawURL, ".git") {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	switch strings.ToLower(filepath.Ext(u.Path)) {
	case ".yaml", ".yml", ".json":
		return false
	}
	return sys.DetectGitDefaultBranch(rawURL) != ""
}

// runAddProfileFromURLE is the entry point when the add argument is a URL.
// Returns a structured error on failure so the root handler can emit the
// right exit-code category and JSON envelope.
func runAddProfileFromURLE(cmd *cobra.Command, rawURL string) error {
	if detectGitURL(rawURL) {
		return addProfilesFromGitURLE(cmd, rawURL)
	}
	return addProfilesFromHTTPURLE(cmd, rawURL)
}

func addProfilesFromGitURLE(cmd *cobra.Command, repoURL string) error {
	tmpDir, err := os.MkdirTemp("", "raid-profile-*")
	if err != nil {
		return errs.Unknown(fmt.Errorf("failed to create temp directory: %v", err))
	}
	defer os.RemoveAll(tmpDir)

	if !jsonMode(cmd) {
		fmt.Fprintf(cmd.OutOrStdout(), "Cloning %s...\n", repoURL)
	}
	if err := gitCloneFunc(repoURL, tmpDir); err != nil {
		return errs.CloneFailed("url-profile", repoURL, err)
	}

	paths := findProfileFilesInDir(tmpDir)
	if len(paths) == 0 {
		return errs.ProfileInvalid(repoURL, fmt.Errorf("No profile files found in repository"))
	}

	return processProfileFilesE(cmd, repoURL, paths)
}

func addProfilesFromHTTPURLE(cmd *cobra.Command, rawURL string) error {
	data, err := httpGetFunc(rawURL)
	if err != nil {
		return errs.Newf(errs.CodeTaskHTTPFailed, errs.CategoryNetwork, "failed to download profile: %v", err)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return errs.ArgInvalid(fmt.Sprintf("invalid URL: %v", err))
	}
	ext := strings.ToLower(filepath.Ext(u.Path))
	if ext == "" {
		ext = ".yaml"
	}

	tmpFile, err := os.CreateTemp("", "raid-profile-*"+ext)
	if err != nil {
		return errs.Unknown(fmt.Errorf("failed to create temp file: %v", err))
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return errs.Unknown(fmt.Errorf("failed to write temp file: %v", err))
	}
	tmpFile.Close()

	return processProfileFilesE(cmd, rawURL, []string{tmpPath})
}

// processProfileFilesE validates, saves, and registers profiles from
// downloaded paths. Emits text-mode progress prose by default; under
// --json collapses to one addResult envelope at the end.
func processProfileFilesE(cmd *cobra.Command, source string, paths []string) error {
	type pending struct {
		p       pro.Profile
		srcPath string
	}

	out := cmd.OutOrStdout()
	json := jsonMode(cmd)

	var queued []pending
	var existingNames []string
	seenQueued := map[string]bool{}

	for _, srcPath := range paths {
		if err := proValidate(srcPath); err != nil {
			if !json {
				fmt.Fprintf(out, "Skipping %s: invalid profile (%v)\n", filepath.Base(srcPath), err)
			}
			continue
		}
		profiles, err := proUnmarshal(srcPath)
		if err != nil {
			if !json {
				fmt.Fprintf(out, "Skipping %s: could not read profiles (%v)\n", filepath.Base(srcPath), err)
			}
			continue
		}
		for _, p := range profiles {
			if err := sys.ValidateFileName(p.Name); err != nil {
				if !json {
					fmt.Fprintf(out, "Skipping profile with invalid name %q: %v\n", p.Name, err)
				}
				continue
			}
			if proContains(p.Name) {
				existingNames = append(existingNames, p.Name)
				continue
			}
			if seenQueued[p.Name] {
				if !json {
					fmt.Fprintf(out, "Skipping duplicate profile name %q\n", p.Name)
				}
				continue
			}
			seenQueued[p.Name] = true
			queued = append(queued, pending{p: p, srcPath: srcPath})
		}
	}

	if !json && len(existingNames) > 0 {
		fmt.Fprintf(out, "Profiles already exist with names:\n\t%s\n\n", strings.Join(existingNames, ",\n\t"))
	}

	if len(queued) == 0 {
		// Differentiate "all profiles already registered" (fine, exit 0)
		// from "we found candidate files but none validated as profiles"
		// (a real failure — exit code config so callers and CI catch it).
		if len(existingNames) == 0 {
			return errs.ProfileInvalid(source, fmt.Errorf("No valid profiles found"))
		}
		if json {
			return emitJSON(cmd, addResult{Action: "skipped", Path: source, Existing: existingNames})
		}
		fmt.Fprintln(out, "No new profiles found")
		return nil
	}

	// Copy each profile to a stable home-dir path before registering.
	home := getHomeDir()

	// Refuse to clobber an existing file that isn't in the registry (a
	// registered profile with the same name was already skipped above).
	// Checked before any copy so a multi-profile add never leaves earlier
	// copies behind after aborting.
	for _, q := range queued {
		destPath := filepath.Join(home, q.p.Name+".raid.yaml")
		if sys.FileExists(destPath) {
			return errs.Newf(errs.CodeProfileAlreadyExists, errs.CategoryConfig,
				"a file already exists at %s but is not a registered profile; remove or rename it, or register it directly with `raid profile add %s`",
				destPath, destPath)
		}
	}

	var toRegister []pro.Profile
	var destPaths []string
	for _, q := range queued {
		destPath := filepath.Join(home, q.p.Name+".raid.yaml")
		if err := sys.CopyFile(q.srcPath, destPath); err != nil {
			if !json {
				fmt.Fprintf(out, "Failed to save profile '%s': %v\n", q.p.Name, err)
			}
			continue
		}
		q.p.Path = destPath
		toRegister = append(toRegister, q.p)
		destPaths = append(destPaths, destPath)
	}

	if len(toRegister) == 0 {
		if json {
			return emitJSON(cmd, addResult{Action: "skipped", Path: source, Existing: existingNames})
		}
		fmt.Fprintln(out, "No new profiles found")
		return nil
	}

	var activeAfter string
	writeErr := raid.WithMutationLock(func() error {
		if err := proAddAll(toRegister); err != nil {
			return err
		}
		if proGet().IsZero() {
			if err := proSet(toRegister[0].Name); err != nil {
				return err
			}
			activeAfter = toRegister[0].Name
		} else {
			activeAfter = proGet().Name
		}
		return nil
	})
	if writeErr != nil {
		return errs.ConfigInvalid(writeErr)
	}

	addedNames := make([]string, 0, len(toRegister))
	for _, p := range toRegister {
		addedNames = append(addedNames, p.Name)
	}

	if json {
		return emitJSON(cmd, addResult{
			Action:   "added",
			Path:     source,
			Profiles: addedNames,
			Existing: existingNames,
			Active:   activeAfter,
		})
	}

	for i, p := range toRegister {
		fmt.Fprintf(out, "Profile '%s' added from URL, saved to %s\n", p.Name, destPaths[i])
	}
	if activeAfter != "" && activeAfter == toRegister[0].Name && len(existingNames) == 0 {
		fmt.Fprintf(out, "Profile '%s' set as active\n", activeAfter)
	}
	return nil
}

// findProfileFilesInDir returns profile YAML/JSON files found at the root of dir.
// Priority: profile.raid.yaml/yml first, then any *.raid.yaml/yml, then profile.json.
// Fallback for repositories that don't follow the *.raid.yaml convention
// (single-file gists, scratch repos): if none of the above match, accept any
// plain .yaml/.yml/.json at the root. processProfileFiles validates each
// candidate against the profile schema, so non-profile YAML lying around
// in a repo root produces a clear "Skipping … invalid profile" message
// rather than incorrect behavior.
func findProfileFilesInDir(dir string) []string {
	seen := map[string]bool{}
	var found []string

	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		if full := filepath.Join(dir, name); sys.FileExists(full) {
			found = append(found, full)
		}
	}

	add("profile.raid.yaml")
	add("profile.raid.yml")

	entries, rdErr := os.ReadDir(dir)
	if rdErr != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to read directory %q: %v\n", dir, rdErr)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		ext := filepath.Ext(lower)
		stem := strings.TrimSuffix(lower, ext)
		if (ext == ".yaml" || ext == ".yml") && strings.HasSuffix(stem, ".raid") {
			add(name)
		}
	}

	add("profile.json")

	if len(found) == 0 {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".yaml" || ext == ".yml" || ext == ".json" {
				add(e.Name())
			}
		}
	}

	return found
}
