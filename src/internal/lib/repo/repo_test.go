package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
	"github.com/8bitalex/raid/src/internal/sys"
)

// TestExpandPath tests path expansion functionality
func TestExpandPath(t *testing.T) {
	// Test home directory expansion
	homeDir := sys.GetHomeDir()

	// Test with ~
	expanded, err := expandPath("~/test")
	if err != nil {
		t.Errorf("Failed to expand path: %v", err)
	}
	expected := filepath.Join(homeDir, "test")
	if expanded != expected {
		t.Errorf("Expected '%s', got '%s'", expected, expanded)
	}

	// Test with environment variable
	os.Setenv("TEST_PATH", "/tmp/test")
	expanded, err = expandPath("$TEST_PATH/repo")
	if err != nil {
		t.Errorf("Failed to expand path: %v", err)
	}
	expected = "/tmp/test/repo"
	if expanded != expected {
		t.Errorf("Expected '%s', got '%s'", expected, expanded)
	}

	// Test with absolute path (no expansion needed)
	expanded, err = expandPath("/absolute/path")
	if err != nil {
		t.Errorf("Failed to expand path: %v", err)
	}
	if expanded != "/absolute/path" {
		t.Errorf("Expected '/absolute/path', got '%s'", expanded)
	}
}

// TestIsGitRepository tests git repository detection
func TestIsGitRepository(t *testing.T) {
	tempDir := t.TempDir()

	// Test non-existent directory
	if isGitRepository("/non/existent/path") {
		t.Errorf("Expected non-existent path to not be a git repository")
	}

	// Test directory without .git
	if isGitRepository(tempDir) {
		t.Errorf("Expected directory without .git to not be a git repository")
	}

	// Test directory with .git
	gitDir := filepath.Join(tempDir, ".git")
	err := os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	if !isGitRepository(tempDir) {
		t.Errorf("Expected directory with .git to be a git repository")
	}
}

// TestCloneRepository tests repository cloning (without actually cloning)
func TestCloneRepository(t *testing.T) {
	tempDir := t.TempDir()

	repo := data.Repository{
		Name: "test-repo",
		Path: tempDir,
		URL:  "https://github.com/test/repo.git",
	}

	// Test with non-existent path (should fail since we can't actually clone)
	err := CloneRepository(repo)
	if err == nil {
		t.Errorf("Expected error when trying to clone to non-existent git environment")
	}
}

// TestInstallProfile tests profile installation logic
func TestInstallProfile(t *testing.T) {
	// This test would require setting up a mock profile and git environment
	// For now, we'll test the error case when no active profile is set

	// Test with no active profile (should fail)
	err := InstallProfile()
	if err == nil {
		t.Errorf("Expected error when no active profile is set")
	}

	// Verify the error message
	expectedError := "no active profile set"
	if err != nil && !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

// TestInstallProfileWithConcurrency tests profile installation with concurrency control
func TestInstallProfileWithConcurrency(t *testing.T) {
	// Test with no active profile (should fail)
	err := InstallProfileWithConcurrency(5)
	if err == nil {
		t.Errorf("Expected error when no active profile is set")
	}

	// Verify the error message
	expectedError := "no active profile set"
	if err != nil && !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
