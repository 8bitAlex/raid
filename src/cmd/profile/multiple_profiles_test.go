package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
)

// TestMultipleProfilesYAML tests multiple profiles in a single YAML file using document separators
func TestMultipleProfilesYAML(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create a YAML file with multiple profiles using document separators
	multiProfileYAML := `name: profile1
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1
---
name: profile2
repositories:
  - name: repo2
    path: /path/to/repo2
    url: https://github.com/user/repo2
---
name: profile3
repositories:
  - name: repo3
    path: /path/to/repo3
    url: https://github.com/user/repo3`

	multiProfileFile := filepath.Join(tempDir, "multi-profiles.yaml")
	err := os.WriteFile(multiProfileFile, []byte(multiProfileYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create multi-profile YAML file: %v", err)
	}

	// Extract all profiles
	profiles, err := data.ExtractProfiles(multiProfileFile)
	if err != nil {
		t.Fatalf("Failed to extract profiles: %v", err)
	}

	// Verify we got 3 profiles
	if len(profiles) != 3 {
		t.Errorf("Expected 3 profiles, got %d", len(profiles))
	}

	// Verify profile names
	expectedNames := []string{"profile1", "profile2", "profile3"}
	for i, profile := range profiles {
		if profile.Name != expectedNames[i] {
			t.Errorf("Expected profile %d name '%s', got '%s'", i+1, expectedNames[i], profile.Name)
		}
		if profile.Path != multiProfileFile {
			t.Errorf("Expected profile %d path '%s', got '%s'", i+1, multiProfileFile, profile.Path)
		}
	}

	// Add all profiles
	for _, profile := range profiles {
		data.AddProfile(profile.Name, profile.Path)
	}

	// Verify all profiles were added
	existingProfiles := data.GetProfilesMap()
	if len(existingProfiles) != 3 {
		t.Errorf("Expected 3 profiles in storage, got %d", len(existingProfiles))
	}

	for _, expectedName := range expectedNames {
		if _, exists := existingProfiles[expectedName]; !exists {
			t.Errorf("Expected profile '%s' to exist in storage", expectedName)
		}
	}
}

// TestMultipleProfilesJSON tests multiple profiles in a single JSON file using arrays
func TestMultipleProfilesJSON(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create a JSON file with multiple profiles using arrays
	multiProfileJSON := `[
  {
    "name": "profile1",
    "repositories": [
      {
        "name": "repo1",
        "path": "/path/to/repo1",
        "url": "https://github.com/user/repo1"
      }
    ]
  },
  {
    "name": "profile2",
    "repositories": [
      {
        "name": "repo2",
        "path": "/path/to/repo2",
        "url": "https://github.com/user/repo2"
      }
    ]
  },
  {
    "name": "profile3",
    "repositories": [
      {
        "name": "repo3",
        "path": "/path/to/repo3",
        "url": "https://github.com/user/repo3"
      }
    ]
  }
]`

	multiProfileFile := filepath.Join(tempDir, "multi-profiles.json")
	err := os.WriteFile(multiProfileFile, []byte(multiProfileJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create multi-profile JSON file: %v", err)
	}

	// Extract all profiles
	profiles, err := data.ExtractProfiles(multiProfileFile)
	if err != nil {
		t.Fatalf("Failed to extract profiles: %v", err)
	}

	// Verify we got 3 profiles
	if len(profiles) != 3 {
		t.Errorf("Expected 3 profiles, got %d", len(profiles))
	}

	// Verify profile names
	expectedNames := []string{"profile1", "profile2", "profile3"}
	for i, profile := range profiles {
		if profile.Name != expectedNames[i] {
			t.Errorf("Expected profile %d name '%s', got '%s'", i+1, expectedNames[i], profile.Name)
		}
		if profile.Path != multiProfileFile {
			t.Errorf("Expected profile %d path '%s', got '%s'", i+1, multiProfileFile, profile.Path)
		}
	}

	// Add all profiles
	for _, profile := range profiles {
		data.AddProfile(profile.Name, profile.Path)
	}

	// Verify all profiles were added
	existingProfiles := data.GetProfilesMap()
	if len(existingProfiles) != 3 {
		t.Errorf("Expected 3 profiles in storage, got %d", len(existingProfiles))
	}

	for _, expectedName := range expectedNames {
		if _, exists := existingProfiles[expectedName]; !exists {
			t.Errorf("Expected profile '%s' to exist in storage", expectedName)
		}
	}
}

// TestSingleProfileJSON tests that single profile JSON still works
func TestSingleProfileJSON(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create a JSON file with a single profile
	singleProfileJSON := `{
  "name": "single-profile",
  "repositories": [
    {
      "name": "repo1",
      "path": "/path/to/repo1",
      "url": "https://github.com/user/repo1"
    }
  ]
}`

	singleProfileFile := filepath.Join(tempDir, "single-profile.json")
	err := os.WriteFile(singleProfileFile, []byte(singleProfileJSON), 0644)
	if err != nil {
		t.Fatalf("Failed to create single profile JSON file: %v", err)
	}

	// Extract profiles
	profiles, err := data.ExtractProfiles(singleProfileFile)
	if err != nil {
		t.Fatalf("Failed to extract profiles: %v", err)
	}

	// Verify we got 1 profile
	if len(profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(profiles))
	}

	// Verify profile name
	if profiles[0].Name != "single-profile" {
		t.Errorf("Expected profile name 'single-profile', got '%s'", profiles[0].Name)
	}
}

// TestYAMLWithEmptyDocuments tests YAML with empty documents (should be skipped)
func TestYAMLWithEmptyDocuments(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Create a YAML file with empty documents
	yamlWithEmptyDocs := `name: profile1
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1
---
---
name: profile2
repositories:
  - name: repo2
    path: /path/to/repo2
    url: https://github.com/user/repo2
---
`

	yamlFile := filepath.Join(tempDir, "with-empty-docs.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlWithEmptyDocs), 0644)
	if err != nil {
		t.Fatalf("Failed to create YAML file with empty docs: %v", err)
	}

	// Extract profiles
	profiles, err := data.ExtractProfiles(yamlFile)
	if err != nil {
		t.Fatalf("Failed to extract profiles: %v", err)
	}

	// Verify we got 2 profiles (empty docs should be skipped)
	if len(profiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(profiles))
	}

	// Verify profile names
	expectedNames := []string{"profile1", "profile2"}
	for i, profile := range profiles {
		if profile.Name != expectedNames[i] {
			t.Errorf("Expected profile %d name '%s', got '%s'", i+1, expectedNames[i], profile.Name)
		}
	}
}

// TestMultipleProfilesWithExisting tests adding multiple profiles when some already exist
func TestMultipleProfilesWithExisting(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Add an existing profile first
	data.AddProfile("existing-profile", "/path/to/existing.yaml")

	// Create a YAML file with multiple profiles, one of which already exists
	multiProfileYAML := `name: existing-profile
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1
---
name: new-profile1
repositories:
  - name: repo2
    path: /path/to/repo2
    url: https://github.com/user/repo2
---
name: new-profile2
repositories:
  - name: repo3
    path: /path/to/repo3
    url: https://github.com/user/repo3`

	multiProfileFile := filepath.Join(tempDir, "mixed-profiles.yaml")
	err := os.WriteFile(multiProfileFile, []byte(multiProfileYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create mixed profiles YAML file: %v", err)
	}

	// Extract all profiles
	profiles, err := data.ExtractProfiles(multiProfileFile)
	if err != nil {
		t.Fatalf("Failed to extract profiles: %v", err)
	}

	// Verify we got 3 profiles
	if len(profiles) != 3 {
		t.Errorf("Expected 3 profiles, got %d", len(profiles))
	}

	// Check for existing profiles and collect new ones
	existingProfiles := data.GetProfilesMap()
	var newProfiles []data.ProfileInfo
	var existingNames []string

	for _, profile := range profiles {
		if _, exists := existingProfiles[profile.Name]; exists {
			existingNames = append(existingNames, profile.Name)
		} else {
			newProfiles = append(newProfiles, profile)
		}
	}

	// Verify we found the existing profile
	if len(existingNames) != 1 || existingNames[0] != "existing-profile" {
		t.Errorf("Expected to find existing profile 'existing-profile', got %v", existingNames)
	}

	// Verify we have 2 new profiles
	if len(newProfiles) != 2 {
		t.Errorf("Expected 2 new profiles, got %d", len(newProfiles))
	}

	// Add new profiles
	for _, profile := range newProfiles {
		data.AddProfile(profile.Name, profile.Path)
	}

	// Verify all profiles exist now
	finalProfiles := data.GetProfilesMap()
	if len(finalProfiles) != 3 {
		t.Errorf("Expected 3 total profiles, got %d", len(finalProfiles))
	}

	expectedNames := []string{"existing-profile", "new-profile1", "new-profile2"}
	for _, expectedName := range expectedNames {
		if _, exists := finalProfiles[expectedName]; !exists {
			t.Errorf("Expected profile '%s' to exist in storage", expectedName)
		}
	}
}

// TestMultipleProfilesAutoActivation tests auto-activation with multiple profiles
func TestMultipleProfilesAutoActivation(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()
	tempDir := t.TempDir()

	// Verify no active profile initially
	activeProfile := data.GetProfile()
	if activeProfile != "" {
		t.Errorf("Expected no active profile initially, got '%s'", activeProfile)
	}

	// Create a YAML file with multiple profiles
	multiProfileYAML := `name: profile1
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1
---
name: profile2
repositories:
  - name: repo2
    path: /path/to/repo2
    url: https://github.com/user/repo2`

	multiProfileFile := filepath.Join(tempDir, "auto-activation.yaml")
	err := os.WriteFile(multiProfileFile, []byte(multiProfileYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create auto-activation YAML file: %v", err)
	}

	// Extract all profiles
	profiles, err := data.ExtractProfiles(multiProfileFile)
	if err != nil {
		t.Fatalf("Failed to extract profiles: %v", err)
	}

	// Add all profiles
	for _, profile := range profiles {
		data.AddProfile(profile.Name, profile.Path)
	}

	// Simulate auto-activation logic (first profile should be set as active)
	activeProfile = data.GetProfile()
	if activeProfile == "" {
		data.SetProfile(profiles[0].Name)
		activeProfile = data.GetProfile()
	}

	// Verify the first profile is active
	if activeProfile != "profile1" {
		t.Errorf("Expected active profile to be 'profile1', got '%s'", activeProfile)
	}
}
