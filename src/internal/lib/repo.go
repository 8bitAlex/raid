package lib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	sys "github.com/8bitalex/raid/src/internal/sys"
)

type Repo struct {
	Name string
	Path string
	URL  string
}

func BuildRepo(repo Repo) (Repo, error) {
	return repo, nil
}

func CloneRepository(repo Repo) error {
	path := os.ExpandEnv(repo.Path)

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

	if err := clone(path, repo.URL); err != nil {
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

func clone(path string, url string) error {
	cmd := exec.Command("git", "clone", url, path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
