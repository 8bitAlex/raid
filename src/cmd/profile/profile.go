package profile

import (
	"fmt"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(AddProfileCmd)
	Command.AddCommand(ListProfileCmd)
	Command.AddCommand(UseProfileCmd)
}

var Command = &cobra.Command{
	Use:   "profile",
	Short: "Manage raid profiles",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		profile := lib.GetProfile()
		if profile != "" {
			fmt.Println(profile)
		} else {
			fmt.Println("No active profile found. Use 'raid profile use <profile>' to set one.")
		}
	},
}
