package profile

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var AddProfileCmd = &cobra.Command{
	Use:   "add filepath",
	Short: "Add profile(s) from filepath",
	Args: cobra.ExactArgs(1),
	Run: func (cmd *cobra.Command, args []string) {
		AddProfile(args[0])
	},
}

func AddProfile(profile string) {
	if !sys.FileExists(profile) {
		fmt.Printf("File '%s' does not exist", profile)
		os.Exit(1)
	}
	// todo validate schema
	// get profile(s) name from file and use as map key
	profiles := viper.GetStringSlice(ProfileListKey)

	viper.Set(ProfileListKey, append(profiles, profile))
	viper.WriteConfig()

	fmt.Printf("The file %s has been successfully added.", profile)
}