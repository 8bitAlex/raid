package profile

import (
	"fmt"

	"github.com/8bitalex/raid/src/raid"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/8bitalex/raid/src/raid/errs"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			profile := pro.Get()
			if !profile.IsZero() {
				fmt.Println(profile.Name)
			} else {
				fmt.Println("No active profile found. Use 'raid profile use <profile>' to set one.")
			}
			return nil
		}
		name := args[0]
		err := raid.WithMutationLock(func() error {
			return pro.Set(name)
		})
		if err != nil {
			return errs.Wrap(err)
		}
		fmt.Printf("Profile '%s' is now active.\n", name)
		return nil
	},
}
