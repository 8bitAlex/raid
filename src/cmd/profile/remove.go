package profile

import (
	"fmt"

	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

var RemoveProfileCmd = &cobra.Command{
	Use:        "remove profile",
	Short:      "Remove profile(s)",
	SuggestFor: []string{"delete"},
	Args:       cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {

		for _, name := range args {
			if err := pro.Remove(name); err != nil {
				fmt.Printf("Profile '%s' not found. Use 'raid profile list' to see available profiles.\n", name)
			} else {
				fmt.Printf("Profile '%s' has been removed.\n", name)
			}
		}

		fmt.Print()
	},
}
