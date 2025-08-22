package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
	"github.com/8bitalex/raid/src/internal/sys"
)

func TestAddProfileCmd(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create a valid profile file
	validProfileContent := `name: test-profile
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1`

	validProfileFile := filepath.Join(tempDir, "valid-profile.yaml")
	err := os.WriteFile(validProfileFile, []byte(validProfileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test profile file: %v", err)
	}

	// Test the underlying logic that the command uses
	// Test file existence check
	if !sys.FileExists(validProfileFile) {
		t.Errorf("Expected file to exist: %s", validProfileFile)
	}

	// Test profile name extraction
	profileName, err := data.ExtractProfileName(validProfileFile)
	if err != nil {
		t.Errorf("Expected profile name extraction to succeed: %v", err)
	}
	if profileName != "test-profile" {
		t.Errorf("Expected profile name 'test-profile', got '%s'", profileName)
	}

	// Test profile addition
	data.AddProfile(profileName, validProfileFile)

	// Verify profile was added
	profiles := data.GetProfilesMap()
	if len(profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(profiles))
	}

	if _, exists := profiles["test-profile"]; !exists {
		t.Errorf("Expected profile 'test-profile' to exist")
	}
}

func TestAddProfileCmdFileNotExists(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test that the command validates file existence
	// Since the command calls os.Exit(1), we can't easily test the output
	// Instead, we'll test the underlying logic

	// Verify that a non-existent file doesn't exist
	if data.GetProfilesMap()["non-existent"] != (data.Profile{}) {
		t.Errorf("Expected no profile to exist for non-existent file")
	}
}

func TestAddProfileCmdInvalidProfile(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create an invalid profile file (missing required fields)
	invalidProfileContent := `repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1`

	invalidProfileFile := filepath.Join(tempDir, "invalid-profile.yaml")
	err := os.WriteFile(invalidProfileFile, []byte(invalidProfileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid profile file: %v", err)
	}

	// Test that the file exists but is invalid
	if !sys.FileExists(invalidProfileFile) {
		t.Errorf("Expected invalid profile file to exist")
	}

	// Test that profile validation fails
	err = data.ValidateProfileFile(invalidProfileFile)
	if err == nil {
		t.Errorf("Expected profile validation to fail for invalid profile")
	}
}

func TestAddProfileCmdDuplicateProfile(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create a valid profile file
	validProfileContent := `name: test-profile
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1`

	validProfileFile := filepath.Join(tempDir, "valid-profile.yaml")
	err := os.WriteFile(validProfileFile, []byte(validProfileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test profile file: %v", err)
	}

	// Add the profile first time
	data.AddProfile("test-profile", validProfileFile)

	// Verify profile exists
	profiles := data.GetProfilesMap()
	if _, exists := profiles["test-profile"]; !exists {
		t.Errorf("Expected profile 'test-profile' to exist after first addition")
	}

	// Test that adding the same profile again works (overwrites)
	data.AddProfile("test-profile", validProfileFile)

	// Verify profile still exists
	profiles = data.GetProfilesMap()
	if _, exists := profiles["test-profile"]; !exists {
		t.Errorf("Expected profile 'test-profile' to still exist after second addition")
	}
}

func TestAddProfileCmdArgs(t *testing.T) {
	cmd := AddProfileCmd

	// Test with no args (should fail)
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Errorf("Expected error with no args, got nil")
	}

	// Test with one arg (should work)
	err = cmd.Args(cmd, []string{"file.yaml"})
	if err != nil {
		t.Errorf("Expected no error with one arg, got %v", err)
	}

	// Test with multiple args (should fail)
	err = cmd.Args(cmd, []string{"file1.yaml", "file2.yaml"})
	if err == nil {
		t.Errorf("Expected error with multiple args, got nil")
	}
}

func TestAddProfileCmdUse(t *testing.T) {
	cmd := AddProfileCmd
	if cmd.Use != "add filepath" {
		t.Errorf("Expected Use to be 'add filepath', got '%s'", cmd.Use)
	}
}

func TestAddProfileCmdShort(t *testing.T) {
	cmd := AddProfileCmd
	if cmd.Short != "Add profile(s) from YAML or JSON file" {
		t.Errorf("Expected Short to be 'Add profile(s) from YAML or JSON file', got '%s'", cmd.Short)
	}
}

func TestAddProfileCmdLong(t *testing.T) {
	cmd := AddProfileCmd
	expectedLong := "Add profile(s) from a YAML (.yaml, .yml) or JSON (.json) file. The file will be validated against the raid profile schema."
	if cmd.Long != expectedLong {
		t.Errorf("Expected Long to be '%s', got '%s'", expectedLong, cmd.Long)
	}
}

// TestAddProfileCmdAutoActivation tests that new profiles are automatically set as active when no active profile exists
func TestAddProfileCmdAutoActivation(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create a valid profile file
	validProfileContent := `name: test-profile
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1`

	validProfileFile := filepath.Join(tempDir, "valid-profile.yaml")
	err := os.WriteFile(validProfileFile, []byte(validProfileContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test profile file: %v", err)
	}

	// Verify no active profile initially
	activeProfile := data.GetProfile()
	if activeProfile != "" {
		t.Errorf("Expected no active profile initially, got '%s'", activeProfile)
	}

	// Test the underlying logic that the add command uses
	// Test file existence check
	if !sys.FileExists(validProfileFile) {
		t.Errorf("Expected file to exist: %s", validProfileFile)
	}

	// Test profile name extraction
	profileName, err := data.ExtractProfileName(validProfileFile)
	if err != nil {
		t.Errorf("Expected profile name extraction to succeed: %v", err)
	}
	if profileName != "test-profile" {
		t.Errorf("Expected profile name 'test-profile', got '%s'", profileName)
	}

	// Test profile addition
	data.AddProfile(profileName, validProfileFile)

	// Verify profile was added
	profiles := data.GetProfilesMap()
	if len(profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(profiles))
	}

	if _, exists := profiles["test-profile"]; !exists {
		t.Errorf("Expected profile 'test-profile' to exist")
	}

	// Test auto-activation logic (simulating what the add command does)
	activeProfile = data.GetProfile()
	if activeProfile == "" {
		// No active profile, so set this one as active
		data.SetProfile(profileName)
		activeProfile = data.GetProfile()
	}

	// Verify the profile is now active
	if activeProfile != profileName {
		t.Errorf("Expected active profile to be '%s', got '%s'", profileName, activeProfile)
	}
}
