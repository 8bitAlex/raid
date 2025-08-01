package profile

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var ListProfileCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles",
	Run: func(cmd *cobra.Command, args []string) {
		profiles := viper.GetStringSlice(ProfileListKey)
		for _, profile := range profiles {
			fmt.Println(profile)
		}
	},
}