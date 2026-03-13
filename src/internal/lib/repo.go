package lib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"gopkg.in/yaml.v3"
)

const repoSchemaPath = "raid-repo.schema.json"

// Repo represents a single repository entry in a profile.
type Repo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	URL          string    `json:"url"`
	Branch       string    `json:"branch"`
	Environments []Env     `json:"environments"`
	Install      OnInstall `json:"install"`
	Commands     []Command `json:"commands"`
}

// IsZero reports whether the repo is uninitialized.
func (r Repo) IsZero() bool {
	return r.Name == "" || r.Path == "" || r.URL == ""
}

func (r Repo) getEnv(name string) Env {
	for _, env := range r.Environments {
		if env.Name == name {
			return env
		}
	}
	return Env{}
}

func buildRepo(repo *Repo) error {
	if repo.IsZero() {
		return fmt.Errorf("invalid repository: %v", repo)
	}

	raidFile := filepath.Join(sys.ExpandPath(repo.Path), RaidConfigFileName)
	if !sys.FileExists(raidFile) {
		return nil
	}

	if err := ValidateRepo(raidFile); err != nil {
		return fmt.Errorf("invalid raid configuration for '%s': %w", repo.Name, err)
	}

	repoConfig, err := ExtractRepo(repo.Path)
	if err != nil {
		return fmt.Errorf("failed to read config for '%s': %w", repo.Name, err)
	}

	repo.Environments = append(repo.Environments, repoConfig.Environments...)
	repo.Install.Tasks = append(repo.Install.Tasks, repoConfig.Install.Tasks...)
	repo.Commands = append(repo.Commands, repoConfig.Commands...)

	return nil
}

// CloneRepository clones a repository to its configured path. Skips if it already exists.
func CloneRepository(repo Repo) error {
	path := sys.ExpandPath(repo.Path)

	if sys.FileExists(path) && isGitRepository(path) {
		fmt.Printf("Repository '%s' already exists at %s, skipping\n", repo.Name, path)
		return nil
	}

	if !isGitInstalled() {
		return fmt.Errorf("git is not installed or not in the PATH")
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", path, err)
	}

	if err := clone(path, repo.URL, repo.Branch); err != nil {
		return fmt.Errorf("failed to clone repository '%s': %w", repo.Name, err)
	}

	return nil
}

func isGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	return sys.FileExists(gitDir)
}

func isGitInstalled() bool {
	cmd := exec.Command("git", "--version")
	return cmd.Run() == nil
}

func clone(path string, url string, branch string) error {
	args := []string{"clone", url, path}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ValidateRepo validates the repo config file at path against the repo JSON schema.
func ValidateRepo(path string) error {
	return validateWithEmbeddedSchema(path, repoSchemaPath)
}

// ExtractRepo reads and parses the raid.yaml from the given repository directory.
func ExtractRepo(path string) (Repo, error) {
	filePath := filepath.Join(sys.ExpandPath(path), RaidConfigFileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Repo{}, fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	var repo Repo
	if err := yaml.Unmarshal(data, &repo); err != nil {
		return Repo{}, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}

	return repo, nil
}
