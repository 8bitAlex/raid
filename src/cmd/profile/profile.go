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

func init() {
<<<<<<< HEAD
	Command.AddCommand(AddProfileCmd)
	Command.AddCommand(ListProfileCmd)
	Command.AddCommand(UseProfileCmd)
}

var Command = &cobra.Command{
=======
	ProfileCmd.AddCommand(AddProfileCmd)
	ProfileCmd.AddCommand(ListProfileCmd)
	ProfileCmd.AddCommand(UseProfileCmd)
}

var ProfileCmd = &cobra.Command{
>>>>>>> 6a5ca86 (add schema validation)
	Use:   "profile",
	Short: "Manage raid profiles",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
<<<<<<< HEAD
		profile := lib.GetProfile()
=======
		profile := data.GetProfile()
>>>>>>> 6a5ca86 (add schema validation)
		if profile != "" {
			fmt.Println(profile)
		} else {
			fmt.Println("No active profile found. Use 'raid profile use <profile>' to set one.")
		}
	},
}
