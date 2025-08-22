package profile

import (
	"fmt"
	"os"
	"strings"

<<<<<<< HEAD
	"github.com/8bitalex/raid/src/internal/lib"
=======
	"github.com/8bitalex/raid/src/internal/lib/data"
>>>>>>> 6a5ca86 (add schema validation)
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
<<<<<<< HEAD
		if err := lib.ValidateProfileFile(profilePath); err != nil {
=======
		if err := data.ValidateProfileFile(profilePath); err != nil {
>>>>>>> 6a5ca86 (add schema validation)
			fmt.Printf("Invalid Profile: %v\n", err)
			os.Exit(1)
		}

<<<<<<< HEAD
		// Extract all profiles from the file
		profiles, err := lib.ExtractProfiles(profilePath)
		if err != nil {
			fmt.Printf("Failed to extract profiles: %v\n", err)
			os.Exit(1)
		}

		// Check for existing profiles and collect new ones
		existingProfiles := lib.GetProfilesMap()
		var newProfiles []lib.ProfileInfo
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
			lib.AddProfile(profile.Name, profile.Path)
			addedNames = append(addedNames, profile.Name)
		}

		// Check if there's an active profile, if not set the first new one as active
		activeProfile := lib.GetProfile()
		if activeProfile == "" && len(newProfiles) > 0 {
			lib.SetProfile(newProfiles[0].Name)
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
=======
		// Extract the profile name from the file
		profileName, err := data.ExtractProfileName(profilePath)
		if err != nil {
			fmt.Printf("Failed to extract profile name: %v\n", err)
			os.Exit(1)
		}

		// Check if profile already exists
		profiles := data.GetProfilesMap()
		if _, exists := profiles[profileName]; exists {
			fmt.Printf("Profile '%s' already exists. Use a different name or remove the existing profile first.\n", profileName)
			os.Exit(1)
		}

		// Add the profile with name and file path
		data.AddProfile(profileName, profilePath)

		fmt.Printf("Profile '%s' has been successfully added from %s", profileName, profilePath)
>>>>>>> 6a5ca86 (add schema validation)
	},
}
