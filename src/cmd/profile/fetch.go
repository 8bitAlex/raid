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

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/raid"
	pro "github.com/8bitalex/raid/src/raid/profile"
)

// Injectable for testing.
var (
	gitCloneFunc = func(repoURL, dir string) error {
		return exec.Command("git", "clone", "--depth", "1", repoURL, dir).Run()
	}
	httpGetFunc = func(rawURL string) ([]byte, error) {
		resp, err := http.Get(rawURL) //nolint:gosec
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, rawURL)
		}
		return io.ReadAll(resp.Body)
	}
	// detectGitURL is injectable so tests can skip the live git ls-remote probe.
	detectGitURL = isGitURL
	getHomeDir   = sys.GetHomeDir
)

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

// runAddProfileFromURL is the entry point when the add argument is a URL.
func runAddProfileFromURL(rawURL string) int {
	if detectGitURL(rawURL) {
		return addProfilesFromGitURL(rawURL)
	}
	return addProfilesFromHTTPURL(rawURL)
}

func addProfilesFromGitURL(repoURL string) int {
	tmpDir, err := os.MkdirTemp("", "raid-profile-*")
	if err != nil {
		fmt.Printf("Failed to create temp directory: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	fmt.Printf("Cloning %s...\n", repoURL)
	if err := gitCloneFunc(repoURL, tmpDir); err != nil {
		fmt.Printf("Failed to clone repository: %v\n", err)
		return 1
	}

	paths := findProfileFilesInDir(tmpDir)
	if len(paths) == 0 {
		fmt.Println("No profile files found in repository")
		return 1
	}

	return processProfileFiles(paths)
}

func addProfilesFromHTTPURL(rawURL string) int {
	data, err := httpGetFunc(rawURL)
	if err != nil {
		fmt.Printf("Failed to download profile: %v\n", err)
		return 1
	}

	u, _ := url.Parse(rawURL)
	ext := strings.ToLower(filepath.Ext(u.Path))
	if ext == "" {
		ext = ".yaml"
	}

	tmpFile, err := os.CreateTemp("", "raid-profile-*"+ext)
	if err != nil {
		fmt.Printf("Failed to create temp file: %v\n", err)
		return 1
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		fmt.Printf("Failed to write temp file: %v\n", err)
		return 1
	}
	tmpFile.Close()

	return processProfileFiles([]string{tmpPath})
}

// findProfileFilesInDir returns profile YAML/JSON files found at the root of dir.
// Priority: profile.raid.yaml/yml first, then any *.raid.yaml/yml, then profile.json.
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

	entries, _ := os.ReadDir(dir)
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

	return found
}

// processProfileFiles validates, saves, and registers profiles from the given local paths.
func processProfileFiles(paths []string) int {
	type pending struct {
		p       pro.Profile
		srcPath string
	}

	var queued []pending
	var existingNames []string

	for _, srcPath := range paths {
		if err := proValidate(srcPath); err != nil {
			fmt.Printf("Skipping %s: invalid profile (%v)\n", filepath.Base(srcPath), err)
			continue
		}
		profiles, err := proUnmarshal(srcPath)
		if err != nil {
			fmt.Printf("Skipping %s: could not read profiles (%v)\n", filepath.Base(srcPath), err)
			continue
		}
		for _, p := range profiles {
			if proContains(p.Name) {
				existingNames = append(existingNames, p.Name)
				continue
			}
			queued = append(queued, pending{p: p, srcPath: srcPath})
		}
	}

	if len(existingNames) > 0 {
		fmt.Printf("Profiles already exist with names:\n\t%s\n\n", strings.Join(existingNames, ",\n\t"))
	}

	if len(queued) == 0 {
		fmt.Println("No new profiles found")
		return 0
	}

	// Copy each profile to a stable home-dir path before registering.
	home := getHomeDir()
	var toRegister []pro.Profile
	var destPaths []string
	for _, q := range queued {
		destPath := filepath.Join(home, q.p.Name+".raid.yaml")
		if err := sys.CopyFile(q.srcPath, destPath); err != nil {
			fmt.Printf("Failed to save profile '%s': %v\n", q.p.Name, err)
			continue
		}
		q.p.Path = destPath
		toRegister = append(toRegister, q.p)
		destPaths = append(destPaths, destPath)
	}

	if len(toRegister) == 0 {
		fmt.Println("No new profiles found")
		return 0
	}

	writeErr := raid.WithMutationLock(func() error {
		if err := proAddAll(toRegister); err != nil {
			return fmt.Errorf("save: %w", err)
		}
		if proGet().IsZero() {
			if err := proSet(toRegister[0].Name); err != nil {
				return fmt.Errorf("set active: %w", err)
			}
			fmt.Printf("Profile '%s' set as active\n", toRegister[0].Name)
		}
		return nil
	})
	if writeErr != nil {
		fmt.Printf("Failed to save profiles: %v\n", writeErr)
		return 1
	}

	for i, p := range toRegister {
		fmt.Printf("Profile '%s' added from URL, saved to %s\n", p.Name, destPaths[i])
	}
	return 0
}
