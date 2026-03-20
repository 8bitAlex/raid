package install

import (
	"log"

	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

var maxThreads int

func init() {
	Command.Flags().IntVarP(&maxThreads, "threads", "t", 0, "Maximum number of concurrent clone threads (0 = unlimited)")
}

var Command = &cobra.Command{
	Use:   "install [repo]",
	Short: "Install the active profile",
	Long:  "Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped. Repositories are cloned concurrently for better performance. Pass a repository name to install only that repository.",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 1 {
			if err := raid.InstallRepo(args[0]); err != nil {
				log.Fatalf("Installation failed: %v", err)
			}
			return
		}
		if err := raid.Install(maxThreads); err != nil {
			log.Fatalf("Installation failed: %v", err)
		}
	},
}
