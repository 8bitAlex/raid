package install

import (
	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/errs"
	"github.com/spf13/cobra"
)

var maxThreads int

func init() {
	Command.Flags().IntVarP(&maxThreads, "threads", "t", 0, "Maximum number of concurrent clone threads for full profile installs (0 = unlimited); not valid with a repository argument")
}

var Command = &cobra.Command{
	Use:   "install [repo]",
	Short: "Install the active profile",
	Long:  "Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped. Repositories are cloned concurrently for better performance; --threads caps that concurrency. Pass a repository name to install only that repository (a single clone, so --threads does not apply).",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// --threads only shapes the concurrent full-profile clone; reject an
		// explicit value alongside a repo argument instead of silently
		// ignoring it. Changed() keeps the untouched default working.
		if len(args) == 1 && cmd.Flags().Changed("threads") {
			return errs.ArgInvalid("--threads only applies to full profile installs and cannot be combined with a repository argument")
		}
		// Drop the previous log.Fatalf in favour of returning the error to
		// the cobra root, which routes the categorical exit code via
		// errs.ExitCode and emits JSON when --json is set.
		err := raid.WithMutationLock(func() error {
			if len(args) == 1 {
				return raid.InstallRepo(args[0])
			}
			return raid.Install(maxThreads)
		})
		if err != nil {
			return errs.Wrap(err)
		}
		return nil
	},
}
