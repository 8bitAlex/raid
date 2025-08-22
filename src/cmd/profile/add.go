package profile

import (
	"fmt"
	"os"

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
	},
}
