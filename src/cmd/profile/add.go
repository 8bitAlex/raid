package profile

import (
	"fmt"
	"os"
	"strings"

	"github.com/8bitalex/raid/src/internal/lib/data"
	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/spf13/cobra"
)

var AddProfileCmd = &cobra.Command{
	Use:   "add filepath",
	Short: "Add profile(s) from YAML or JSON file",
	Long:  "Add profile(s) from a YAML (.yaml, .yml) or JSON (.json) file. The file will be validated against the raid profile schema.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profilePath := args[0]

		if !sys.FileExists(profilePath) {
			fmt.Printf("File '%s' does not exist", profilePath)
			os.Exit(1)
		}

		// Validate the profile file against the schema
		if err := data.ValidateProfileFile(profilePath); err != nil {
			fmt.Printf("Invalid Profile: %v\n", err)
			os.Exit(1)
		}

		// Extract all profiles from the file
		profiles, err := data.ExtractProfiles(profilePath)
		if err != nil {
			fmt.Printf("Failed to extract profiles: %v\n", err)
			os.Exit(1)
		}

		// Check for existing profiles and collect new ones
		existingProfiles := data.GetProfilesMap()
		var newProfiles []data.ProfileInfo
		var existingNames []string

		for _, profile := range profiles {
			if _, exists := existingProfiles[profile.Name]; exists {
				existingNames = append(existingNames, profile.Name)
			} else {
				newProfiles = append(newProfiles, profile)
			}
		}

		// Report existing profiles
		if len(existingNames) > 0 {
			fmt.Printf("Profiles already exist: %s\n", strings.Join(existingNames, ", "))
			if len(newProfiles) == 0 {
				os.Exit(1)
			}
		}

		// Add all new profiles
		var addedNames []string
		for _, profile := range newProfiles {
			data.AddProfile(profile.Name, profile.Path)
			addedNames = append(addedNames, profile.Name)
		}

		// Check if there's an active profile, if not set the first new one as active
		activeProfile := data.GetProfile()
		if activeProfile == "" && len(newProfiles) > 0 {
			data.SetProfile(newProfiles[0].Name)
			if len(newProfiles) == 1 {
				fmt.Printf("Profile '%s' has been successfully added from %s and set as active", newProfiles[0].Name, profilePath)
			} else {
				fmt.Printf("Profiles %s have been successfully added from %s. Profile '%s' has been set as active", strings.Join(addedNames, ", "), profilePath, newProfiles[0].Name)
			}
		} else {
			if len(newProfiles) == 1 {
				fmt.Printf("Profile '%s' has been successfully added from %s", newProfiles[0].Name, profilePath)
			} else {
				fmt.Printf("Profiles %s have been successfully added from %s", strings.Join(addedNames, ", "), profilePath)
			}
		}
	},
}
