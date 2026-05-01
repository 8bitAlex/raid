package profile

import (
	"fmt"
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/raid"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

// Injectable profile-package functions for testing error paths.
var (
	proValidate  = pro.Validate
	proUnmarshal = pro.Unmarshal
	proContains  = pro.Contains
	proAddAll    = pro.AddAll
	proGet       = pro.Get
	proSet       = pro.Set
)

var AddProfileCmd = &cobra.Command{
	Use:   "add <path|url>",
	Short: "Add profile(s) from a local file or URL",
	Long: `Add one or more profiles from a local file, a git repository URL, or a raw file URL.

Local path: the file is validated and registered directly.

Git URL (git@ prefix, .git suffix, or any HTTP URL that responds to git ls-remote):
  raid shallow-clones the repo and imports *.raid.yaml, *.raid.yml, and profile.json
  files found at the root.

Raw file URL (HTTP/HTTPS URL ending in .yaml, .yml, or .json):
  the file is downloaded, validated, and registered.

Profiles from URLs are saved to ~/<name>.raid.yaml before registration.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		code := runAddProfile(args[0])
		if code != 0 {
			osExit(code)
		}
	},
}

// runAddProfile performs the add-profile flow and returns an exit code.
// Extracted from AddProfileCmd.Run so tests can observe the exit code
// without os.Exit terminating the test process.
func runAddProfile(path string) int {
	if isURL(path) {
		return runAddProfileFromURL(path)
	}
	path = sys.ExpandPath(path)

	if !sys.FileExists(path) {
		fmt.Printf("File '%s' does not exist\n", path)
		return 1
	}

	if err := proValidate(path); err != nil {
		fmt.Printf("Invalid Profile: %v\n", err)
		return 1
	}

	profiles, err := proUnmarshal(path)
	if err != nil {
		fmt.Printf("Failed to extract profiles: %v\n", err)
		return 1
	}

	var newProfiles []pro.Profile
	var existingNames []string
	for _, profile := range profiles {
		if exists := proContains(profile.Name); exists {
			existingNames = append(existingNames, profile.Name)
		} else {
			newProfiles = append(newProfiles, profile)
		}
	}

	if len(existingNames) > 0 {
		fmt.Printf("Profiles already exist with names:\n\t%s\n\n", strings.Join(existingNames, ",\n\t"))
	}

	if len(newProfiles) == 0 {
		fmt.Printf("No new profiles found in %s\n", path)
		return 0
	}

	writeErr := raid.WithMutationLock(func() error {
		if err := proAddAll(newProfiles); err != nil {
			return fmt.Errorf("save: %w", err)
		}
		if proGet().IsZero() {
			if err := proSet(newProfiles[0].Name); err != nil {
				return fmt.Errorf("set active: %w", err)
			}
			fmt.Printf("Profile '%s' set as active\n", newProfiles[0].Name)
		}
		return nil
	})
	if writeErr != nil {
		fmt.Printf("Failed to save profiles: %v\n", writeErr)
		return 1
	}

	if len(newProfiles) == 1 {
		fmt.Printf("Profile '%s' has been successfully added from %s\n", newProfiles[0].Name, path)
	} else {
		names := make([]string, 0, len(newProfiles))
		for _, profile := range newProfiles {
			names = append(names, profile.Name)
		}
		fmt.Printf("Profiles:\n\t%s\nhave been successfully added from %s\n", strings.Join(names, ",\n\t"), path)
	}
	return 0
}
