package profile

import (
	"fmt"
	"os"

	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

var UseProfileCmd = &cobra.Command{
	Use:        "use profile",
	Short:      "Use a specific profile",
	SuggestFor: []string{"set"},
	Args:       cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]

		if err := pro.Set(name); err != nil {
			fmt.Printf("Profile '%s' not found. Use 'raid profile list' to see available profiles.\n", name)
			os.Exit(1)
		}
		fmt.Printf("Profile '%s' is now active.\n", name)
	},
}
