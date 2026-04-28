package profile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/raid"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

// CreateProfileCmd is the interactive wizard for creating a new profile.
var CreateProfileCmd = &cobra.Command{
	Use:   "create",
	Short: "Interactively create a new profile",
	Args:  cobra.NoArgs,
	Run:   runCreateWizard,
}

// osExit is injectable for testing.
var osExit = os.Exit

// Injectable profile-package functions for testing (shared across add.go).
var (
	proWriteFile         = pro.WriteFile
	proCollectRepos      = pro.CollectRepos
	proCreateRepoConfigs = pro.CreateRepoConfigs
)

func runCreateWizard(cmd *cobra.Command, args []string) {
	if code := runCreateWizardCore(os.Stdin); code != 0 {
		osExit(code)
	}
}

// runCreateWizardCore performs the create-profile wizard and returns an exit
// code. Extracted from runCreateWizard so tests can observe the exit code
// without os.Exit terminating the test process.
func runCreateWizardCore(input *os.File) int {
	reader := bufio.NewReader(input)

	var name string
	for {
		name = sys.ReadLine(reader, "Profile name: ")
		if err := sys.ValidateFileName(name); err != nil {
			fmt.Printf("Invalid name: %v\n", err)
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
		fmt.Printf("Failed to write profile: %v\n", err)
		return 1
	}

	if err := proValidate(savePath); err != nil {
		fmt.Printf("Failed to register profile: %v\n", err)
		return 1
	}
	profiles, err := proUnmarshal(savePath)
	if err != nil {
		fmt.Printf("Failed to register profile: %v\n", err)
		return 1
	}
	writeErr := raid.WithMutationLock(func() error {
		if err := proAddAll(profiles); err != nil {
			return fmt.Errorf("register: %w", err)
		}
		if proGet().IsZero() {
			if err := proSet(name); err != nil {
				return fmt.Errorf("set active: %w", err)
			}
			fmt.Printf("Profile '%s' set as active.\n", name)
		}
		return nil
	})
	if writeErr != nil {
		fmt.Printf("Failed to register profile: %v\n", writeErr)
		return 1
	}

	fmt.Printf("\nProfile '%s' created at %s\n", name, savePath)

	if len(repos) > 0 && sys.ReadYesNo(reader, "\nCreate a raid.yaml config for each repository? [y/N]: ") {
		proCreateRepoConfigs(repos)
	}
	return 0
}
