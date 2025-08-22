package profile

import (
	"fmt"

<<<<<<< HEAD
	"github.com/8bitalex/raid/src/internal/lib"
=======
	"github.com/8bitalex/raid/src/internal/lib/data"
>>>>>>> 6a5ca86 (add schema validation)
	"github.com/spf13/cobra"
)

var ListProfileCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles",
	Run: func(cmd *cobra.Command, args []string) {
<<<<<<< HEAD
		profiles := lib.GetProfilesMap()
		activeProfile := lib.GetProfile()
=======
		profiles := data.GetProfilesMap()
		activeProfile := data.GetProfile()
>>>>>>> 6a5ca86 (add schema validation)

		if len(profiles) == 0 {
			fmt.Println("No profiles found.")
			return
		}

		fmt.Println("Available profiles:")
		for name, profile := range profiles {
			activeIndicator := ""
			if name == activeProfile {
				activeIndicator = " (active)"
			}
			fmt.Printf("  %s%s\n    File: %s\n", name, activeIndicator, profile.Path)
		}
	},
}
