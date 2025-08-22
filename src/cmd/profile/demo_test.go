package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
)

// TestAutoActivationDemo demonstrates the auto-activation feature
func TestAutoActivationDemo(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Get the path to the example profile file
	exampleProfilePath := "../../docs/examples/example.raid.yaml"

	// Check if the example file exists
	if _, err := os.Stat(exampleProfilePath); os.IsNotExist(err) {
		t.Skip("Example profile file not found, skipping demo test")
	}

	// Verify no active profile initially
	activeProfile := data.GetProfile()
	if activeProfile != "" {
		t.Errorf("Expected no active profile initially, got '%s'", activeProfile)
	}

	// Extract profile name from the example file
	profileName, err := data.ExtractProfileName(exampleProfilePath)
	if err != nil {
		t.Fatalf("Failed to extract profile name from example file: %v", err)
	}

	// Verify the profile name is "raid" as expected
	if profileName != "raid" {
		t.Errorf("Expected profile name 'raid', got '%s'", profileName)
	}

	// Add the profile (this simulates what the add command does)
	data.AddProfile(profileName, exampleProfilePath)

	// Verify profile was added
	profiles := data.GetProfilesMap()
	if _, exists := profiles[profileName]; !exists {
		t.Errorf("Expected profile '%s' to exist", profileName)
	}

	// Simulate the auto-activation logic from the add command
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

	// Verify the profile path is correct
	profilePath, err := data.GetProfilePath(profileName)
	if err != nil {
		t.Errorf("Failed to get profile path: %v", err)
	}

	// Get absolute path for comparison
	absExamplePath, err := filepath.Abs(exampleProfilePath)
	if err != nil {
		t.Errorf("Failed to get absolute path: %v", err)
	}

	if profilePath != absExamplePath {
		t.Errorf("Expected profile path '%s', got '%s'", absExamplePath, profilePath)
	}
}
