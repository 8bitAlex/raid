package install

import (
	"log"

	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

var (
	maxThreads int = 0
)

func init() {
	Command.Flags().IntVarP(&maxThreads, "threads", "t", 0, "Maximum number of concurrent threads (0 = unlimited)")
}

var Command = &cobra.Command{
	Use:   "install",
	Short: "Install the active profile",
	Long:  "Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped. Repositories are cloned concurrently for better performance.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := raid.Install(maxThreads); err != nil {
			log.Fatalf("Installation failed: %v\n", err)
		}
	},
}
