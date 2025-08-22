package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/8bitalex/raid/src/internal/lib/data"
	"github.com/8bitalex/raid/src/internal/sys"
)

// CloneRepository clones a repository to the specified path
func CloneRepository(repo data.Repository) error {
	// Expand any environment variables or home directory references
	expandedPath, err := expandPath(repo.Path)
	if err != nil {
		return fmt.Errorf("failed to expand path '%s': %w", repo.Path, err)
	}

	// Check if the directory already exists
	if sys.FileExists(expandedPath) {
		// Check if it's a git repository
		if isGitRepository(expandedPath) {
			fmt.Printf("Repository '%s' already exists at %s, skipping\n", repo.Name, expandedPath)
			return nil
		} else {
			return fmt.Errorf("path '%s' exists but is not a git repository", expandedPath)
		}
	}

	// Create the parent directory if it doesn't exist
	parentDir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", parentDir, err)
	}

	// Clone the repository
	fmt.Printf("Cloning repository '%s' to %s...\n", repo.Name, expandedPath)
	cmd := exec.Command("git", "clone", repo.URL, expandedPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository '%s': %w", repo.Name, err)
	}

	fmt.Printf("Successfully cloned repository '%s' to %s\n", repo.Name, expandedPath)
	return nil
}

// InstallProfile installs all repositories in the active profile
func InstallProfile() error {
	// Get the active profile content
	profile, err := data.GetActiveProfileContent()
	if err != nil {
		return fmt.Errorf("failed to get active profile: %w", err)
	}

	fmt.Printf("Installing profile '%s' with %d repositories...\n", profile.Name, len(profile.Repositories))

	if len(profile.Repositories) == 0 {
		fmt.Println("No repositories to install.")
		return nil
	}

	// Clone each repository
	for _, repo := range profile.Repositories {
		if err := CloneRepository(repo); err != nil {
			return fmt.Errorf("failed to install repository '%s': %w", repo.Name, err)
		}
	}

	fmt.Printf("Successfully installed all repositories for profile '%s'\n", profile.Name)
	return nil
}

// expandPath expands environment variables and home directory references
func expandPath(path string) (string, error) {
	// Replace ~ with home directory
	if strings.HasPrefix(path, "~") {
		homeDir := sys.GetHomeDir()
		path = filepath.Join(homeDir, path[1:])
	}

	// Expand environment variables
	expandedPath := os.ExpandEnv(path)
	return expandedPath, nil
}

// isGitRepository checks if a directory is a git repository
func isGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	return sys.FileExists(gitDir)
}
