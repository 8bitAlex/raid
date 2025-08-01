package profile

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const CurrentProfileKey = "profile"
const ProfileListKey = "profiles"

var Profile = viper.GetString(CurrentProfileKey)

var ProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage raid profiles",
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if (Profile != "") {
			fmt.Print(Profile)
		} else {
			fmt.Print("No profile set")
		}
	},
}

func init() {
	ProfileCmd.AddCommand(AddProfileCmd)
	ProfileCmd.AddCommand(ListProfileCmd)
	ProfileCmd.AddCommand(UseProfileCmd)
}