package profile

import (
	"fmt"

	"github.com/8bitalex/raid/src/internal/lib/data"
	"github.com/spf13/cobra"
)

func init() {
	ProfileCmd.AddCommand(AddProfileCmd)
	ProfileCmd.AddCommand(ListProfileCmd)
	ProfileCmd.AddCommand(UseProfileCmd)
}

var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage raid profiles",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		profile := data.GetProfile()
		if profile != "" {
			fmt.Println(profile)
		} else {
			fmt.Println("No active profile found. Use 'raid profile use <profile>' to set one.")
		}
	},
}
