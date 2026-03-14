package profile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	"github.com/8bitalex/raid/src/internal/sys"
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

func runCreateWizard(cmd *cobra.Command, args []string) {
	reader := bufio.NewReader(os.Stdin)

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

	repos := pro.CollectRepos(reader)

	draft := pro.ProfileDraft{Name: name, Repositories: repos}
	if err := pro.WriteFile(draft, savePath); err != nil {
		fmt.Printf("Failed to write profile: %v\n", err)
		os.Exit(1)
	}

	if err := pro.Validate(savePath); err != nil {
		fmt.Printf("Failed to register profile: %v\n", err)
		os.Exit(1)
	}
	profiles, err := pro.Unmarshal(savePath)
	if err != nil {
		fmt.Printf("Failed to register profile: %v\n", err)
		os.Exit(1)
	}
	if err := pro.AddAll(profiles); err != nil {
		fmt.Printf("Failed to register profile: %v\n", err)
		os.Exit(1)
	}
	if pro.Get().IsZero() {
		if err := pro.Set(name); err != nil {
			fmt.Printf("Failed to set active profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Profile '%s' set as active.\n", name)
	}

	fmt.Printf("\nProfile '%s' created at %s\n", name, savePath)

	if len(repos) > 0 && sys.ReadYesNo(reader, "\nCreate a raid.yaml config for each repository? [y/N]: ") {
		pro.CreateRepoConfigs(repos)
	}
}
