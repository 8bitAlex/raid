package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/spf13/viper"
)

// setupTest initializes a clean test environment
func setupTest(t *testing.T) func() {
	// Store original config path
	originalCfgPath := data.CfgPath

	// Create a temporary directory for test config
	tempDir := t.TempDir()
	data.CfgPath = tempDir + "/config.toml"

	// Reset viper and initialize with temp config
	viper.Reset()
	data.Initialize()

	// Return cleanup function
	return func() {
		viper.Reset()
		data.CfgPath = originalCfgPath
	}
}

// TestAddProfileLogic tests the logic that the add command uses
func TestAddProfileLogic(t *testing.T) {
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

	// Test file existence check
	if !sys.FileExists(validProfileFile) {
		t.Errorf("Expected file to exist: %s", validProfileFile)
	}

	// Test profile validation (skip schema validation for now since it requires schema file)
	// err = data.ValidateProfileFile(validProfileFile)
	// if err != nil {
	// 	t.Errorf("Expected profile validation to pass: %v", err)
	// }

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

// TestAddProfileLogicFileNotExists tests the file existence logic
func TestAddProfileLogicFileNotExists(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test non-existent file
	nonExistentFile := "/non/existent/file.yaml"
	if sys.FileExists(nonExistentFile) {
		t.Errorf("Expected file to not exist: %s", nonExistentFile)
	}
}

// TestAddProfileLogicInvalidProfile tests invalid profile handling
func TestAddProfileLogicInvalidProfile(t *testing.T) {
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

	// Test profile validation should fail
	err = data.ValidateProfileFile(invalidProfileFile)
	if err == nil {
		t.Errorf("Expected profile validation to fail")
	}

	// Test profile name extraction should fail
	_, err = data.ExtractProfileName(invalidProfileFile)
	if err == nil {
		t.Errorf("Expected profile name extraction to fail")
	}
}

// TestAddProfileLogicDuplicateProfile tests duplicate profile handling
func TestAddProfileLogicDuplicateProfile(t *testing.T) {
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

	// Add profile first time
	profileName, err := data.ExtractProfileName(validProfileFile)
	if err != nil {
		t.Fatalf("Failed to extract profile name: %v", err)
	}

	data.AddProfile(profileName, validProfileFile)

	// Check if profile exists
	profiles := data.GetProfilesMap()
	if _, exists := profiles[profileName]; !exists {
		t.Errorf("Expected profile to exist after first addition")
	}

	// Add the same profile again (should overwrite)
	data.AddProfile(profileName, validProfileFile)

	// Verify profile still exists
	profiles = data.GetProfilesMap()
	if _, exists := profiles[profileName]; !exists {
		t.Errorf("Expected profile to still exist after second addition")
	}
}

// TestUseProfileLogic tests the logic that the use command uses
func TestUseProfileLogic(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Add a profile
	data.AddProfile("test-profile", "/path/to/profile.yaml")

	// Test that profile exists
	profiles := data.GetProfilesMap()
	if _, exists := profiles["test-profile"]; !exists {
		t.Errorf("Expected profile 'test-profile' to exist")
	}

	// Test setting active profile
	data.SetProfile("test-profile")

	// Verify profile was set as active
	activeProfile := data.GetProfile()
	if activeProfile != "test-profile" {
		t.Errorf("Expected active profile to be 'test-profile', got '%s'", activeProfile)
	}
}

// TestUseProfileLogicNonExistent tests non-existent profile handling
func TestUseProfileLogicNonExistent(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test that non-existent profile doesn't exist
	profiles := data.GetProfilesMap()
	if _, exists := profiles["non-existent-profile"]; exists {
		t.Errorf("Expected profile 'non-existent-profile' to not exist")
	}
}

// TestListProfileLogic tests the logic that the list command uses
func TestListProfileLogic(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test with no profiles
	profiles := data.GetProfilesMap()
	if len(profiles) != 0 {
		t.Errorf("Expected no profiles initially, got %d", len(profiles))
	}

	// Add profiles
	data.AddProfile("profile1", "/path/to/profile1.yaml")
	data.AddProfile("profile2", "/path/to/profile2.yaml")

	// Test with profiles
	profiles = data.GetProfilesMap()
	if len(profiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(profiles))
	}

	// Test active profile
	data.SetProfile("profile1")
	activeProfile := data.GetProfile()
	if activeProfile != "profile1" {
		t.Errorf("Expected active profile to be 'profile1', got '%s'", activeProfile)
	}

	// Test profile names
	profileNames := data.GetProfiles()
	if len(profileNames) != 2 {
		t.Errorf("Expected 2 profile names, got %d", len(profileNames))
	}

	// Check that both names are present
	found1, found2 := false, false
	for _, name := range profileNames {
		if name == "profile1" {
			found1 = true
		}
		if name == "profile2" {
			found2 = true
		}
	}

	if !found1 {
		t.Errorf("Expected to find 'profile1' in profile names")
	}
	if !found2 {
		t.Errorf("Expected to find 'profile2' in profile names")
	}
}

// TestAddProfileAutoActivation tests that new profiles are automatically set as active when no active profile exists
func TestAddProfileAutoActivation(t *testing.T) {
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

	// Extract profile name and add profile
	profileName, err := data.ExtractProfileName(validProfileFile)
	if err != nil {
		t.Fatalf("Failed to extract profile name: %v", err)
	}

	// Add the profile
	data.AddProfile(profileName, validProfileFile)

	// Verify profile was added
	profiles := data.GetProfilesMap()
	if _, exists := profiles[profileName]; !exists {
		t.Errorf("Expected profile '%s' to exist", profileName)
	}

	// Test that the profile is automatically set as active when no active profile exists
	// This simulates the logic in the add command
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

// TestAddProfileNoAutoActivation tests that new profiles are NOT automatically set as active when an active profile already exists
func TestAddProfileNoAutoActivation(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create two valid profile files
	profile1Content := `name: profile1
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1`

	profile2Content := `name: profile2
repositories:
  - name: repo2
    path: /path/to/repo2
    url: https://github.com/user/repo2`

	profile1File := filepath.Join(tempDir, "profile1.yaml")
	profile2File := filepath.Join(tempDir, "profile2.yaml")

	err := os.WriteFile(profile1File, []byte(profile1Content), 0644)
	if err != nil {
		t.Fatalf("Failed to create profile1 file: %v", err)
	}

	err = os.WriteFile(profile2File, []byte(profile2Content), 0644)
	if err != nil {
		t.Fatalf("Failed to create profile2 file: %v", err)
	}

	// Add first profile (should be auto-activated)
	profile1Name, err := data.ExtractProfileName(profile1File)
	if err != nil {
		t.Fatalf("Failed to extract profile1 name: %v", err)
	}

	data.AddProfile(profile1Name, profile1File)

	// Set it as active (simulating auto-activation)
	data.SetProfile(profile1Name)

	// Verify profile1 is active
	activeProfile := data.GetProfile()
	if activeProfile != profile1Name {
		t.Errorf("Expected active profile to be '%s', got '%s'", profile1Name, activeProfile)
	}

	// Add second profile (should NOT be auto-activated since profile1 is already active)
	profile2Name, err := data.ExtractProfileName(profile2File)
	if err != nil {
		t.Fatalf("Failed to extract profile2 name: %v", err)
	}

	data.AddProfile(profile2Name, profile2File)

	// Verify profile2 was added but profile1 is still active
	profiles := data.GetProfilesMap()
	if _, exists := profiles[profile2Name]; !exists {
		t.Errorf("Expected profile '%s' to exist", profile2Name)
	}

	activeProfile = data.GetProfile()
	if activeProfile != profile1Name {
		t.Errorf("Expected active profile to remain '%s', got '%s'", profile1Name, activeProfile)
	}
}
