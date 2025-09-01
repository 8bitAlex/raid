package install

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

var (
	concurrency int
)

func init() {
	Command.Flags().IntVarP(&concurrency, "threads", "t", 0, "Maximum number of concurrent repository clones (0 = unlimited)")
}

var Command = &cobra.Command{
	Use:   "install",
	Short: "Install repositories from the active profile",
	Long:  "Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped. Repositories are cloned concurrently for better performance.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := raid.Install(); err != nil {
			fmt.Printf("Installation failed: %v\n", err)
			os.Exit(1)
		}
	},
}
