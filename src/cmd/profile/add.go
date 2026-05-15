package profile

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/errs"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

// Injectable profile-package functions for testing error paths.
var (
	proValidate       = pro.Validate
	proSynthesizeRepo = pro.SynthesizeFromRepoConfig
	proUnmarshal      = pro.Unmarshal
	proContains       = pro.Contains
	proAddAll         = pro.AddAll
	proGet            = pro.Get
	proSet            = pro.Set
)

// addResult is the JSON shape emitted under --json for `raid profile add`.
// Stable contract; new fields ship additively. `Active` is the name of the
// profile that's now active (set to the first newly-added profile when no
// profile was previously active; otherwise unchanged from before the add).
type addResult struct {
	Action   string   `json:"action"` // "added" | "skipped"
	Path     string   `json:"path"`
	Profiles []string `json:"profiles,omitempty"`
	Existing []string `json:"existing,omitempty"`
	Active   string   `json:"active,omitempty"`
}

// runAddProfile is the legacy exit-code shim for tests that observe
// the int return and assert on stdout text. New tests should drive
// AddProfileCmd through cobra and assert on the returned structured
// error / JSON envelope.
//
// To keep the existing assertions stable, this shim mimics the old
// text-mode output: prints the error message to stdout and returns
// exit code 1. The cobra entry point (AddProfileCmd.RunE) is the
// real public surface and still emits category-correct exit codes +
// structured-error JSON; this shim only papers over the test-facing
// signature.
func runAddProfile(path string) int {
	if err := runAddProfileE(AddProfileCmd, path); err != nil {
		fmt.Println(legacyErrPrefix(err))
		return 1
	}
	return 0
}

// legacyErrPrefix converts a structured error to its old prose
// representation. Mirrors the strings the previous prose-printing
// flow emitted ("Failed to clone repository: …", "Invalid Profile:
// …", etc.) so tests asserting on exact phrasing keep working.
func legacyErrPrefix(err error) string {
	if rErr, ok := errs.AsError(err); ok {
		switch rErr.Code() {
		case errs.CodeCloneFailed:
			return "Failed to clone repository: " + rErr.Error()
		case errs.CodeProfileFileMissing:
			return "File does not exist: " + rErr.Error()
		case errs.CodeProfileInvalid:
			// raid.yaml repo-config failures get an "Invalid raid.yaml"
			// prefix; profile-schema failures stay generic "Invalid
			// Profile". The wrapping cause string contains the marker
			// when the failure originated from synthesizeRepo, so a
			// substring check is enough.
			msg := rErr.Error()
			if strings.Contains(msg, "invalid raid.yaml") {
				return "Invalid raid.yaml: " + msg
			}
			return "Invalid Profile: " + msg
		case errs.CodeProfileFileRead:
			return "Failed to extract profiles: " + rErr.Error()
		case errs.CodeTaskHTTPFailed:
			return "Failed to download profile: " + rErr.Error()
		case errs.CodeConfigInvalid:
			return "Failed to save profiles: " + rErr.Error()
		}
		return rErr.Error()
	}
	return err.Error()
}

var AddProfileCmd = &cobra.Command{
	Use:   "add <path|url>",
	Short: "Add profile(s) from a local file or URL",
	Long: `Add one or more profiles from a local file, a git repository URL, or a raw file URL.

Local path: the file is validated and registered directly. A repo config
(raid.yaml) is also accepted and registered as a single-repo profile named
after the raid.yaml's ` + "`name`" + ` field — handy for projects that ship
only a raid.yaml without a wrapping profile.

Git URL (git@ prefix, .git suffix, or any HTTP URL that responds to git ls-remote):
  raid shallow-clones the repo and imports *.raid.yaml, *.raid.yml, and profile.json
  files found at the root.

Raw file URL (HTTP/HTTPS URL ending in .yaml, .yml, or .json):
  the file is downloaded, validated, and registered.

Profiles from URLs are saved to ~/<name>.raid.yaml before registration.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runAddProfileE(cmd, args[0])
	},
}

// runAddProfileE is the structured-errors + JSON-aware add-profile flow.
// Every failure produces a structured errs.Error so the root handler can
// emit the right exit-code category and JSON envelope.
func runAddProfileE(cmd *cobra.Command, path string) error {
	if isURL(path) {
		return runAddProfileFromURLE(cmd, path)
	}
	path = sys.ExpandPath(path)

	if !sys.FileExists(path) {
		return errs.ProfileFileMissing(path)
	}

	profiles, err := extractProfilesFromLocalFile(path)
	if err != nil {
		return err
	}

	return registerAddedProfiles(cmd, path, profiles)
}

// extractProfilesFromLocalFile reads + validates the file at path and
// returns the parsed profiles. Splits the "is this a raid.yaml repo
// config or a profile YAML?" choice out of the main flow so the URL
// path can reuse the same logic for downloaded files.
func extractProfilesFromLocalFile(path string) ([]pro.Profile, error) {
	if filepath.Base(path) == raid.RaidConfigFileName {
		// Files named raid.yaml are repo configs by convention. Validate
		// against the repo schema directly so a missing `branch` etc.
		// surfaces as a repo-schema error rather than a misleading
		// "Invalid Profile" message.
		single, serr := proSynthesizeRepo(path)
		if serr != nil {
			return nil, errs.ProfileInvalid(path, fmt.Errorf("invalid raid.yaml: %v", serr))
		}
		return []pro.Profile{single}, nil
	}
	if err := proValidate(path); err != nil {
		// Non-raid.yaml file failed profile-schema validation. Try the
		// repo schema as a fallback so callers can still register a
		// renamed repo config; otherwise report the profile error.
		if single, serr := proSynthesizeRepo(path); serr == nil {
			return []pro.Profile{single}, nil
		}
		return nil, errs.ProfileInvalid(path, err)
	}
	extracted, err := proUnmarshal(path)
	if err != nil {
		return nil, errs.ProfileFileRead(path, err)
	}
	return extracted, nil
}

// registerAddedProfiles partitions parsed profiles into new vs. already-
// registered, writes the new ones under the mutation lock, and emits
// the user-facing result (text or JSON). Used by both the local-file
// and URL paths.
func registerAddedProfiles(cmd *cobra.Command, path string, profiles []pro.Profile) error {
	var newProfiles []pro.Profile
	var existingNames []string
	for _, profile := range profiles {
		if proContains(profile.Name) {
			existingNames = append(existingNames, profile.Name)
		} else {
			newProfiles = append(newProfiles, profile)
		}
	}

	out := cmd.OutOrStdout()
	json := jsonMode(cmd)

	if len(newProfiles) == 0 {
		// Nothing new to register — not an error, just a no-op.
		if json {
			return emitJSON(cmd, addResult{Action: "skipped", Path: path, Existing: existingNames})
		}
		if len(existingNames) > 0 {
			fmt.Fprintf(out, "Profiles already exist with names:\n\t%s\n\n", strings.Join(existingNames, ",\n\t"))
		}
		fmt.Fprintf(out, "No new profiles found in %s\n", path)
		return nil
	}

	var activeAfter string
	writeErr := raid.WithMutationLock(func() error {
		if err := proAddAll(newProfiles); err != nil {
			return err
		}
		if proGet().IsZero() {
			if err := proSet(newProfiles[0].Name); err != nil {
				return err
			}
			activeAfter = newProfiles[0].Name
		} else {
			activeAfter = proGet().Name
		}
		return nil
	})
	if writeErr != nil {
		return errs.ConfigInvalid(writeErr)
	}

	addedNames := make([]string, 0, len(newProfiles))
	for _, p := range newProfiles {
		addedNames = append(addedNames, p.Name)
	}

	if json {
		return emitJSON(cmd, addResult{
			Action:   "added",
			Path:     path,
			Profiles: addedNames,
			Existing: existingNames,
			Active:   activeAfter,
		})
	}

	if len(existingNames) > 0 {
		fmt.Fprintf(out, "Profiles already exist with names:\n\t%s\n\n", strings.Join(existingNames, ",\n\t"))
	}
	if activeAfter == newProfiles[0].Name && len(existingNames) == 0 {
		// Single newly-active profile path is the common case for first-time users.
		fmt.Fprintf(out, "Profile '%s' set as active\n", activeAfter)
	}
	if len(newProfiles) == 1 {
		fmt.Fprintf(out, "Profile '%s' has been successfully added from %s\n", newProfiles[0].Name, path)
	} else {
		fmt.Fprintf(out, "Profiles:\n\t%s\nhave been successfully added from %s\n", strings.Join(addedNames, ",\n\t"), path)
	}
	return nil
}
