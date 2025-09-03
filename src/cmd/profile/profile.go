package profile

import (
	"fmt"

	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(AddProfileCmd)
	Command.AddCommand(ListProfileCmd)
	Command.AddCommand(UseProfileCmd)
	Command.AddCommand(RemoveProfileCmd)
}

var Command = &cobra.Command{
	Use:     "profile",
	Aliases: []string{"p"},
	Short:   "Manage raid profiles",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		profile := pro.Get()
		if profile != (pro.Profile{}) {
			fmt.Println(profile.Name)
		} else {
			fmt.Println("No active profile found. Use 'raid profile use <profile>' to set one.")
		}
	},
}
