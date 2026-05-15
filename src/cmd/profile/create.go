package profile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/errs"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

// CreateProfileCmd is the interactive wizard for creating a new profile.
var CreateProfileCmd = &cobra.Command{
	Use:   "create",
	Short: "Interactively create a new profile",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCreateWizardE(cmd, os.Stdin)
	},
}

// runCreateWizard is the legacy cobra-style entry kept as a shim so
// existing tests (which call it directly with a stubbed os.Stdin) keep
// working. New code should go through CreateProfileCmd or
// runCreateWizardE directly. Errors map to osExit(1) for back-compat
// with the pre-refactor wrapper contract.
func runCreateWizard(cmd *cobra.Command, _ []string) {
	if err := runCreateWizardE(cmd, os.Stdin); err != nil {
		fmt.Fprintln(cmd.OutOrStderr(), err)
		osExit(1)
	}
}

// runCreateWizardCore is the legacy entry-point shim used by tests
// that pass an input File directly. Returns exit code 1 on any error
// (matching the pre-refactor behaviour) so existing table-driven
// tests stay structurally unchanged. The real cobra entry
// (CreateProfileCmd.RunE) emits the proper category-correct exit
// code via the root error handler.
func runCreateWizardCore(input *os.File) int {
	if err := runCreateWizardE(CreateProfileCmd, input); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// osExit is injectable for testing the subprocess code paths that still
// need it. Kept for back-compat with existing tests.
var osExit = os.Exit

// Injectable profile-package functions for testing (shared across add.go).
var (
	proWriteFile         = pro.WriteFile
	proCollectRepos      = pro.CollectRepos
	proCreateRepoConfigs = pro.CreateRepoConfigs
)

// runCreateWizardE is the structured-errors variant of the wizard.
// `--json` isn't meaningful for an interactive flow — prompts go to
// stderr/stdin and the only structured output is the final success
// message — so we keep text-mode prose. Errors flow through cobra's
// root handler so the exit code is categorically correct.
func runCreateWizardE(cmd *cobra.Command, input *os.File) error {
	reader := bufio.NewReader(input)
	out := cmd.OutOrStdout()

	var name string
	for {
		name = sys.ReadLine(reader, "Profile name: ")
		if err := sys.ValidateFileName(name); err != nil {
			fmt.Fprintf(out, "Invalid name: %v\n", err)
			continue
		}
		break
	}

	defaultPath := filepath.Join(sys.GetHomeDir(), name+".raid.yaml")
	rawPath := sys.ReadLine(reader, fmt.Sprintf("Save path [%s]: ", defaultPath))
	savePath := defaultPath
	if rawPath != "" {
		savePath = sys.ExpandPath(rawPath)
	}

	repos := proCollectRepos(reader)

	draft := pro.ProfileDraft{Name: name, Repositories: repos}
	if err := proWriteFile(draft, savePath); err != nil {
		return errs.ProfileInvalid(savePath, fmt.Errorf("failed to write profile: %w", err))
	}

	if err := proValidate(savePath); err != nil {
		return errs.ProfileInvalid(savePath, err)
	}
	profiles, err := proUnmarshal(savePath)
	if err != nil {
		return errs.ProfileFileRead(savePath, err)
	}
	writeErr := raid.WithMutationLock(func() error {
		if err := proAddAll(profiles); err != nil {
			return err
		}
		if proGet().IsZero() {
			if err := proSet(name); err != nil {
				return err
			}
			fmt.Fprintf(out, "Profile '%s' set as active.\n", name)
		}
		return nil
	})
	if writeErr != nil {
		return errs.ConfigInvalid(writeErr)
	}

	fmt.Fprintf(out, "\nProfile '%s' created at %s\n", name, savePath)

	if len(repos) > 0 && sys.ReadYesNo(reader, "\nCreate a raid.yaml config for each repository? [y/N]: ") {
		proCreateRepoConfigs(repos)
	}
	return nil
}
