package profile

import (
	"fmt"
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

var AddProfileCmd = &cobra.Command{
	Use:   "add filepath",
	Short: "Add profile(s) from YAML or JSON file",
	Args:  cobra.ExactArgs(1),
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
	if !sys.FileExists(path) {
		fmt.Printf("File '%s' does not exist\n", path)
		return 1
	}

	if err := pro.Validate(path); err != nil {
		fmt.Printf("Invalid Profile: %v\n", err)
		return 1
	}

	profiles, err := pro.Unmarshal(path)
	if err != nil {
		fmt.Printf("Failed to extract profiles: %v\n", err)
		return 1
	}

	var newProfiles []pro.Profile
	var existingNames []string
	for _, profile := range profiles {
		if exists := pro.Contains(profile.Name); exists {
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

	if err := pro.AddAll(newProfiles); err != nil {
		fmt.Printf("Failed to save profiles: %v\n", err)
		return 1
	}

	if pro.Get().IsZero() {
		if err := pro.Set(newProfiles[0].Name); err != nil {
			fmt.Printf("Failed to set active profile: %v\n", err)
			return 1
		}
		fmt.Printf("Profile '%s' set as active\n", newProfiles[0].Name)
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
