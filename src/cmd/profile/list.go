package profile

import (
	"fmt"

	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

var ListProfileCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles",
	Run: func(cmd *cobra.Command, args []string) {
		profiles := pro.GetAll()
		activeProfile := pro.Get()

		if len(profiles) == 0 {
			fmt.Println("No profiles found.")
			return
		}

		fmt.Println("Available profiles:")
		for _, profile := range profiles {
			activeIndicator := ""
			if profile.Name == activeProfile.Name {
				activeIndicator = " (active)"
			}
			fmt.Printf("  %s%s\n    File: %s\n", profile.Name, activeIndicator, profile.Path)
		}
	},
}
