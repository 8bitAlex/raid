package profile

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// profileDraft is the minimal structure written to a new profile file.
type profileDraft struct {
	Name         string      `yaml:"name"`
	Repositories []repoDraft `yaml:"repositories,omitempty"`
}

// repoDraft holds the fields collected for each repository during the wizard.
type repoDraft struct {
	Name   string `yaml:"name"`
	Path   string `yaml:"path"`
	URL    string `yaml:"url"`
	Branch string `yaml:"branch,omitempty"`
}

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
		name = readLine(reader, "Profile name: ")
		if err := sys.ValidateFileName(name); err != nil {
			fmt.Printf("Invalid name: %v\n", err)
			continue
		}
		break
	}

	defaultPath := filepath.Join(sys.GetHomeDir(), name+".raid.yaml")
	rawPath := readLine(reader, fmt.Sprintf("Save path [%s]: ", defaultPath))
	savePath := defaultPath
	if rawPath != "" {
		savePath = sys.ExpandPath(rawPath)
	}

	repos := collectRepos(reader)

	draft := profileDraft{Name: name, Repositories: repos}
	if err := writeProfileFile(draft, savePath); err != nil {
		fmt.Printf("Failed to write profile: %v\n", err)
		os.Exit(1)
	}

	if err := registerProfile(name, savePath); err != nil {
		fmt.Printf("Failed to register profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nProfile '%s' created at %s\n", name, savePath)

	if len(repos) > 0 && readYesNo(reader, "\nCreate a raid.yaml config for each repository? [y/N]: ") {
		createRepoConfigs(repos)
	}
}

func collectRepos(reader *bufio.Reader) []repoDraft {
	var repos []repoDraft
	for {
		fmt.Println()
		if !readYesNo(reader, "Add a repository? [y/N]: ") {
			break
		}
		name := readLine(reader, "  Name: ")
		url := readLine(reader, "  URL: ")
		path := readLine(reader, "  Local path: ")
		defaultBranch := detectDefaultBranch(url)
		branchPrompt := "  Default branch: "
		if defaultBranch != "" {
			branchPrompt = fmt.Sprintf("  Default branch [%s]: ", defaultBranch)
		}
		branch := readLine(reader, branchPrompt)
		if branch == "" {
			branch = defaultBranch
		}
		if branch == "" {
			branch = "main"
		}
		repo := repoDraft{
			Name:   name,
			URL:    url,
			Path:   path,
			Branch: branch,
		}
		if repo.Name == "" || repo.URL == "" || repo.Path == "" {
			fmt.Println("  Name, URL, and path are all required. Skipping.")
			continue
		}
		repos = append(repos, repo)
	}
	return repos
}

func writeProfileFile(draft profileDraft, path string) error {
	data, err := yaml.Marshal(draft)
	if err != nil {
		return fmt.Errorf("serializing profile: %w", err)
	}

	content := "# yaml-language-server: $schema=https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-profile.schema.json\n\n" + string(data)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func registerProfile(name, path string) error {
	if err := pro.Validate(path); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	profiles, err := pro.Unmarshal(path)
	if err != nil {
		return fmt.Errorf("reading profile: %w", err)
	}

	if err := pro.AddAll(profiles); err != nil {
		return fmt.Errorf("saving profile: %w", err)
	}

	if pro.Get().IsZero() {
		if err := pro.Set(name); err != nil {
			return fmt.Errorf("setting active profile: %w", err)
		}
		fmt.Printf("Profile '%s' set as active.\n", name)
	}

	return nil
}

func createRepoConfigs(repos []repoDraft) {
	for _, repo := range repos {
		repoPath := sys.ExpandPath(repo.Path)
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			fmt.Printf("  Failed to create directory for '%s': %v\n", repo.Name, err)
			continue
		}

		configPath := filepath.Join(repoPath, "raid.yaml")
		if sys.FileExists(configPath) {
			fmt.Printf("  raid.yaml already exists at %s, skipping.\n", configPath)
			continue
		}

		content := "# yaml-language-server: $schema=https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-repo.schema.json\n\nname: " + repo.Name + "\n"
		if repo.Branch != "" {
			content += "branch: " + repo.Branch + "\n"
		}
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			fmt.Printf("  Failed to write repo config for '%s': %v\n", repo.Name, err)
			continue
		}
		fmt.Printf("  Created %s\n", configPath)
	}
}


// detectDefaultBranch queries the remote to find its default branch without cloning.
// Returns an empty string if the remote is unreachable or the branch cannot be determined.
func detectDefaultBranch(url string) string {
	out, err := exec.Command("git", "ls-remote", "--symref", url, "HEAD").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		// Format: "ref: refs/heads/<branch>\tHEAD"
		if strings.HasPrefix(line, "ref: refs/heads/") {
			return strings.TrimPrefix(strings.SplitN(line, "\t", 2)[0], "ref: refs/heads/")
		}
	}
	return ""
}

func readLine(reader *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func readYesNo(reader *bufio.Reader, prompt string) bool {
	answer := readLine(reader, prompt)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}
