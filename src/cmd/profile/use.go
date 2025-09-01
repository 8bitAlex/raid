package profile

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
)

var UseProfileCmd = &cobra.Command{
	Use:        "use profile",
	Short:      "Use a specific profile",
	SuggestFor: []string{"set"},
	Args:       cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		profileName := args[0]

		// Validate that the profile exists
		profiles := lib.GetProfilesMap()
		if _, exists := profiles[profileName]; !exists {
			fmt.Printf("Profile '%s' not found. Use 'raid profile list' to see available profiles.\n", profileName)
			os.Exit(1)
		}

		lib.SetProfile(profileName)
		fmt.Printf("Profile '%s' is now active.\n", profileName)
	},
}
