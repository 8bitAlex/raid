package profile

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var UseProfileCmd = &cobra.Command{
	Use:   "use profile",
	Short: "Use a specific profile",
	SuggestFor: []string{"set"},
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// todo validate profile name
		viper.Set(CurrentProfileKey, args[0])
		viper.WriteConfig()
	},
}