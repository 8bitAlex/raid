package profile

import (
	"os"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
)

// TestMultipleProfilesDemo demonstrates the multiple profiles functionality
func TestMultipleProfilesDemo(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test YAML multiple profiles
	testMultipleProfilesYAMLDemo(t)

	// Test JSON multiple profiles
	testMultipleProfilesJSONDemo(t)
}

func testMultipleProfilesYAMLDemo(t *testing.T) {
	// Get the path to the example YAML file
	exampleYAMLPath := "../../docs/examples/multiple-profiles.yaml"

	// Check if the example file exists
	if _, err := os.Stat(exampleYAMLPath); os.IsNotExist(err) {
		t.Skip("Example YAML file not found, skipping YAML demo test")
	}

	// Extract all profiles from the YAML file
	profiles, err := data.ExtractProfiles(exampleYAMLPath)
	if err != nil {
		t.Fatalf("Failed to extract profiles from YAML file: %v", err)
	}

	// Verify we got 3 profiles
	if len(profiles) != 3 {
		t.Errorf("Expected 3 profiles from YAML file, got %d", len(profiles))
	}

	// Verify profile names
	expectedNames := []string{"development", "personal", "open-source"}
	for i, profile := range profiles {
		if profile.Name != expectedNames[i] {
			t.Errorf("Expected YAML profile %d name '%s', got '%s'", i+1, expectedNames[i], profile.Name)
		}
		if profile.Path != exampleYAMLPath {
			t.Errorf("Expected YAML profile %d path '%s', got '%s'", i+1, exampleYAMLPath, profile.Path)
		}
	}

	// Add all profiles
	for _, profile := range profiles {
		data.AddProfile(profile.Name, profile.Path)
	}

	// Verify all profiles were added
	existingProfiles := data.GetProfilesMap()
	if len(existingProfiles) != 3 {
		t.Errorf("Expected 3 profiles in storage after YAML, got %d", len(existingProfiles))
	}

	for _, expectedName := range expectedNames {
		if _, exists := existingProfiles[expectedName]; !exists {
			t.Errorf("Expected YAML profile '%s' to exist in storage", expectedName)
		}
	}
}

func testMultipleProfilesJSONDemo(t *testing.T) {
	// Get the path to the example JSON file
	exampleJSONPath := "../../docs/examples/multiple-profiles.json"

	// Check if the example file exists
	if _, err := os.Stat(exampleJSONPath); os.IsNotExist(err) {
		t.Skip("Example JSON file not found, skipping JSON demo test")
	}

	// Extract all profiles from the JSON file
	profiles, err := data.ExtractProfiles(exampleJSONPath)
	if err != nil {
		t.Fatalf("Failed to extract profiles from JSON file: %v", err)
	}

	// Verify we got 3 profiles
	if len(profiles) != 3 {
		t.Errorf("Expected 3 profiles from JSON file, got %d", len(profiles))
	}

	// Verify profile names
	expectedNames := []string{"development", "personal", "open-source"}
	for i, profile := range profiles {
		if profile.Name != expectedNames[i] {
			t.Errorf("Expected JSON profile %d name '%s', got '%s'", i+1, expectedNames[i], profile.Name)
		}
		if profile.Path != exampleJSONPath {
			t.Errorf("Expected JSON profile %d path '%s', got '%s'", i+1, exampleJSONPath, profile.Path)
		}
	}

	// Add all profiles
	for _, profile := range profiles {
		data.AddProfile(profile.Name, profile.Path)
	}

	// Verify all profiles were added
	existingProfiles := data.GetProfilesMap()
	if len(existingProfiles) != 6 { // 3 from YAML + 3 from JSON
		t.Errorf("Expected 6 profiles in storage after JSON, got %d", len(existingProfiles))
	}

	// Check that all expected profiles exist
	allExpectedNames := []string{"development", "personal", "open-source"}
	for _, expectedName := range allExpectedNames {
		if _, exists := existingProfiles[expectedName]; !exists {
			t.Errorf("Expected JSON profile '%s' to exist in storage", expectedName)
		}
	}
}
