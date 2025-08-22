package repo

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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
	cmd := exec.Command("git", "clone", repo.URL, expandedPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone repository '%s': %w", repo.Name, err)
	}

	return nil
}

// InstallProfile installs all repositories in the active profile concurrently
func InstallProfile() error {
	return InstallProfileWithConcurrency(0) // 0 means unlimited concurrency
}

// InstallProfileWithConcurrency installs all repositories with controlled concurrency
func InstallProfileWithConcurrency(maxConcurrency int) error {
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

	// Use a semaphore to limit concurrency if specified
	var semaphore chan struct{}
	if maxConcurrency > 0 {
		semaphore = make(chan struct{}, maxConcurrency)
		fmt.Printf("Using concurrency limit of %d\n", maxConcurrency)
	}

	// Use a WaitGroup to wait for all goroutines to complete
	var wg sync.WaitGroup
	// Use a channel to collect errors from goroutines
	errorChan := make(chan error, len(profile.Repositories))
	// Use a mutex to synchronize output
	var outputMutex sync.Mutex

	// Clone each repository concurrently
	for _, repo := range profile.Repositories {
		wg.Add(1)
		go func(repo data.Repository) {
			defer wg.Done()

			// Acquire semaphore if concurrency is limited
			if semaphore != nil {
				semaphore <- struct{}{}
				defer func() { <-semaphore }()
			}

			// Lock output to prevent interleaved messages
			outputMutex.Lock()
			fmt.Printf("Starting to clone repository '%s'...\n", repo.Name)
			outputMutex.Unlock()

			if err := CloneRepository(repo); err != nil {
				errorChan <- fmt.Errorf("failed to install repository '%s': %w", repo.Name, err)
			} else {
				// Lock output to prevent interleaved messages
				outputMutex.Lock()
				fmt.Printf("Successfully cloned repository '%s'\n", repo.Name)
				outputMutex.Unlock()
			}
		}(repo)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorChan)

	// Check for any errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	// If there were any errors, return the first one
	if len(errors) > 0 {
		return errors[0]
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
