package lib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
	sys "github.com/8bitalex/raid/src/internal/sys"
	"gopkg.in/yaml.v3"
)

const repoSchemaID = "https://raidcli.dev/schema/v1/raid-repo.schema.json"

// Repo represents a single repository entry in a profile.
type Repo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	URL          string    `json:"url"`
	Branch       string    `json:"branch"`
	Environments []Env     `json:"environments"`
	Install      OnInstall `json:"install"`
	Commands     []Command `json:"commands"`
	Verify       []Verify  `json:"verify,omitempty"`
}

// IsZero reports whether the repo is uninitialized. URL is intentionally
// excluded — local-only repos with no git remote leave it empty.
func (r Repo) IsZero() bool {
	return r.Name == "" || r.Path == ""
}

// IsLocalOnly reports whether the repo has no configured git remote.
// Local-only repos skip cloning; install tasks run directly against the
// existing path. The path must already exist on disk for install to work.
//
// Whitespace-only URLs (e.g. `url: " "` from a stray edit) are treated as
// local-only — the trimmed value is what would be handed to `git clone`,
// and an empty string there produces a confusing error rather than a
// useful one.
func (r Repo) IsLocalOnly() bool {
	return strings.TrimSpace(r.URL) == ""
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
		return liberrs.Newf(liberrs.CodeRepoInvalid, liberrs.CategoryConfig, "invalid repository: %v", *repo)
	}

	raidFile := filepath.Join(sys.ExpandPath(repo.Path), RaidConfigFileName)
	if !sys.FileExists(raidFile) {
		return nil
	}

	if err := ValidateRepo(raidFile); err != nil {
		return liberrs.Newf(liberrs.CodeRepoInvalid, liberrs.CategoryConfig, "invalid raid configuration for '%s': %v", repo.Name, err)
	}

	repoConfig, err := ExtractRepo(repo.Path)
	if err != nil {
		return liberrs.Newf(liberrs.CodeRepoInvalid, liberrs.CategoryConfig, "failed to read config for '%s': %v", repo.Name, err)
	}

	repo.Environments = append(repo.Environments, repoConfig.Environments...)
	repo.Install.Tasks = append(repo.Install.Tasks, repoConfig.Install.Tasks...)
	repo.Commands = append(repo.Commands, repoConfig.Commands...)
	repo.Verify = append(repo.Verify, repoConfig.Verify...)

	return nil
}

// CloneRepository clones a repository to its configured path. Skips if it
// already exists. Repos with no configured `url` (local-only) are never
// cloned — the path must already exist on disk; otherwise an error is
// returned so the user knows the local repo is missing rather than getting
// a confusing git error.
func CloneRepository(repo Repo) error {
	path := sys.ExpandPath(repo.Path)

	if repo.IsLocalOnly() {
		if !sys.FileExists(path) {
			return liberrs.Newf(liberrs.CodeRepoNotCloned, liberrs.CategoryNotFound,
				"repository '%s' has no url and path '%s' does not exist; create the directory or add a url to clone",
				repo.Name, path)
		}
		fmt.Fprintf(commandStdout, "Repository '%s' is local-only at %s, skipping clone\n", repo.Name, path)
		return nil
	}

	if sys.FileExists(path) && isGitRepository(path) {
		fmt.Fprintf(commandStdout, "Repository '%s' already exists at %s, skipping\n", repo.Name, path)
		return nil
	}

	if !isGitInstalled() {
		return liberrs.GitNotInstalled()
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		return liberrs.Newf(liberrs.CodeCloneFailed, liberrs.CategoryNetwork, "failed to create directory '%s': %v", path, err)
	}

	if err := clone(path, strings.TrimSpace(repo.URL), repo.Branch); err != nil {
		return liberrs.CloneFailed(repo.Name, strings.TrimSpace(repo.URL), err)
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
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, url, path)
	cmd := exec.Command("git", args...)
	cmd.Stdout = commandStdout
	cmd.Stderr = commandStderr
	return cmd.Run()
}

// ValidateRepo validates the repo config file at path against the repo JSON schema.
func ValidateRepo(path string) error {
	return validateWithEmbeddedSchema(path, repoSchemaID)
}

// ExtractRepo reads and parses the raid.yaml from the given repository directory.
func ExtractRepo(path string) (Repo, error) {
	filePath := filepath.Join(sys.ExpandPath(path), RaidConfigFileName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Repo{}, liberrs.Newf(liberrs.CodeRepoInvalid, liberrs.CategoryConfig, "failed to read %s: %v", filePath, err)
	}

	var repo Repo
	if err := yaml.Unmarshal(data, &repo); err != nil {
		return Repo{}, liberrs.Newf(liberrs.CodeRepoInvalid, liberrs.CategoryConfig, "failed to parse %s: %v", filePath, err)
	}

	return repo, nil
}
