package profile

import (
	"fmt"
	"os"

<<<<<<< HEAD
	"github.com/8bitalex/raid/src/internal/lib"
=======
	"github.com/8bitalex/raid/src/internal/lib/data"
>>>>>>> 6a5ca86 (add schema validation)
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
<<<<<<< HEAD
		profiles := lib.GetProfilesMap()
=======
		profiles := data.GetProfilesMap()
>>>>>>> 6a5ca86 (add schema validation)
		if _, exists := profiles[profileName]; !exists {
			fmt.Printf("Profile '%s' not found. Use 'raid profile list' to see available profiles.\n", profileName)
			os.Exit(1)
		}

<<<<<<< HEAD
		lib.SetProfile(profileName)
=======
		data.SetProfile(profileName)
>>>>>>> 6a5ca86 (add schema validation)
		fmt.Printf("Profile '%s' is now active.\n", profileName)
	},
}
