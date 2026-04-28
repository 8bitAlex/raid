package profile

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/raid"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(AddProfileCmd)
	Command.AddCommand(CreateProfileCmd)
	Command.AddCommand(ListProfileCmd)
	Command.AddCommand(RemoveProfileCmd)
}

var Command = &cobra.Command{
	Use:     "profile",
	Aliases: []string{"p"},
	Short:   "Manage raid profiles",
	Args:    cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			profile := pro.Get()
			if !profile.IsZero() {
				fmt.Println(profile.Name)
			} else {
				fmt.Println("No active profile found. Use 'raid profile use <profile>' to set one.")
			}
		} else if len(args) == 1 {
			name := args[0]
			err := raid.WithMutationLock(func() error {
				return pro.Set(name)
			})
			if err != nil {
				fmt.Printf("Profile '%s' not found. Use 'raid profile list' to see available profiles.\n", name)
				os.Exit(1)
			}
			fmt.Printf("Profile '%s' is now active.\n", name)
		} else {
			cmd.PrintErrln("Invalid number of arguments.")
		}
	},
}
