package install

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/internal/lib/repo"
	"github.com/spf13/cobra"
)

var InstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install repositories from the active profile",
	Long:  "Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := repo.InstallProfile(); err != nil {
			fmt.Printf("Installation failed: %v\n", err)
			os.Exit(1)
		}
	},
}
