package profile

import (
	"fmt"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
)

var ListProfileCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles",
	Run: func(cmd *cobra.Command, args []string) {
		profiles := lib.GetProfilesMap()
		activeProfile := lib.GetProfile()

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
