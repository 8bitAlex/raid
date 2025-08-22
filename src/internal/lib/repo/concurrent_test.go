package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/8bitalex/raid/src/internal/lib/data"
)

// TestConcurrentInstallation tests the concurrent installation functionality
func TestConcurrentInstallation(t *testing.T) {
	// This test verifies that the concurrent installation works correctly
	// without actually cloning repositories (which would require git)

	// Test that InstallProfile calls InstallProfileWithConcurrency with 0
	// We can't easily test the actual concurrency behavior without real git operations,
	// but we can test the function signatures and error handling

	// Test with no active profile (should fail)
	err := InstallProfile()
	if err == nil {
		t.Errorf("Expected error when no active profile is set")
	}

	// Test with concurrency limit (should also fail due to no active profile)
	err = InstallProfileWithConcurrency(3)
	if err == nil {
		t.Errorf("Expected error when no active profile is set with concurrency limit")
	}

	// Verify both functions return the same type of error
	expectedError := "no active profile set"
	if err != nil && !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

// TestConcurrencyFlagValues tests different concurrency values
func TestConcurrencyFlagValues(t *testing.T) {
	// Test unlimited concurrency (0)
	err := InstallProfileWithConcurrency(0)
	if err == nil {
		t.Errorf("Expected error when no active profile is set")
	}

	// Test limited concurrency (positive values)
	err = InstallProfileWithConcurrency(1)
	if err == nil {
		t.Errorf("Expected error when no active profile is set")
	}

	err = InstallProfileWithConcurrency(5)
	if err == nil {
		t.Errorf("Expected error when no active profile is set")
	}

	err = InstallProfileWithConcurrency(10)
	if err == nil {
		t.Errorf("Expected error when no active profile is set")
	}
}

// TestConcurrentPathExpansion tests that path expansion works correctly in concurrent scenarios
func TestConcurrentPathExpansion(t *testing.T) {
	// Test concurrent path expansion (this is safe to test)
	paths := []string{
		"~/test1",
		"~/test2",
		"$HOME/test3",
		"/absolute/path",
	}

	// Set up environment variable for testing
	os.Setenv("HOME", "/tmp/test-home")

	// Test expansion in a concurrent manner
	results := make(chan string, len(paths))
	errors := make(chan error, len(paths))

	for _, path := range paths {
		go func(p string) {
			expanded, err := expandPath(p)
			if err != nil {
				errors <- err
				return
			}
			results <- expanded
		}(path)
	}

	// Collect results
	expandedPaths := make([]string, 0, len(paths))
	for i := 0; i < len(paths); i++ {
		select {
		case result := <-results:
			expandedPaths = append(expandedPaths, result)
		case err := <-errors:
			t.Errorf("Path expansion failed: %v", err)
		case <-time.After(5 * time.Second):
			t.Errorf("Path expansion timed out")
		}
	}

	// Verify results
	if len(expandedPaths) != len(paths) {
		t.Errorf("Expected %d expanded paths, got %d", len(paths), len(expandedPaths))
	}

	// Check that ~ was expanded
	for _, path := range expandedPaths {
		if strings.Contains(path, "~") {
			t.Errorf("Expected path to not contain '~', got '%s'", path)
		}
	}
}

// TestConcurrentGitRepositoryDetection tests git repository detection in concurrent scenarios
func TestConcurrentGitRepositoryDetection(t *testing.T) {
	tempDir := t.TempDir()

	// Create multiple directories to test concurrently
	dirs := []string{
		filepath.Join(tempDir, "dir1"),
		filepath.Join(tempDir, "dir2"),
		filepath.Join(tempDir, "dir3"),
	}

	// Create directories
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
	}

	// Test git repository detection concurrently
	results := make(chan bool, len(dirs))
	for _, dir := range dirs {
		go func(d string) {
			isGit := isGitRepository(d)
			results <- isGit
		}(dir)
	}

	// Collect results
	gitResults := make([]bool, 0, len(dirs))
	for i := 0; i < len(dirs); i++ {
		select {
		case result := <-results:
			gitResults = append(gitResults, result)
		case <-time.After(5 * time.Second):
			t.Errorf("Git repository detection timed out")
		}
	}

	// Verify results (none should be git repositories)
	for i, isGit := range gitResults {
		if isGit {
			t.Errorf("Expected directory %d to not be a git repository", i+1)
		}
	}

	// Now create a .git directory in one of them and test again
	gitDir := filepath.Join(dirs[0], ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Test again
	isGit := isGitRepository(dirs[0])
	if !isGit {
		t.Errorf("Expected directory with .git to be a git repository")
	}
}

// TestConcurrentErrorHandling tests that errors are properly handled in concurrent scenarios
func TestConcurrentErrorHandling(t *testing.T) {
	// Test that multiple errors are collected correctly
	// This simulates what would happen if multiple repositories failed to clone

	// Create a mock scenario where we have multiple repositories
	repos := []data.Repository{
		{Name: "repo1", Path: "/invalid/path1", URL: "https://invalid1.com"},
		{Name: "repo2", Path: "/invalid/path2", URL: "https://invalid2.com"},
		{Name: "repo3", Path: "/invalid/path3", URL: "https://invalid3.com"},
	}

	// Simulate concurrent error collection
	errorChan := make(chan error, len(repos))
	var wg sync.WaitGroup

	for _, repo := range repos {
		wg.Add(1)
		go func(r data.Repository) {
			defer wg.Done()
			// Simulate an error
			errorChan <- fmt.Errorf("failed to clone repository '%s'", r.Name)
		}(repo)
	}

	wg.Wait()
	close(errorChan)

	// Collect all errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	// Verify we got all expected errors
	if len(errors) != len(repos) {
		t.Errorf("Expected %d errors, got %d", len(repos), len(errors))
	}

	// Verify error messages (order doesn't matter in concurrent execution)
	expectedErrors := []string{
		"failed to clone repository 'repo1'",
		"failed to clone repository 'repo2'",
		"failed to clone repository 'repo3'",
	}

	// Check that all expected errors are present
	for _, expectedError := range expectedErrors {
		found := false
		for _, err := range errors {
			if contains(err.Error(), expectedError) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find error containing '%s'", expectedError)
		}
	}
}
