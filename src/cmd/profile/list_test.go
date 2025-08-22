package profile

import (
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
)

func TestListProfileCmd(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test with no profiles
	profiles := data.GetProfilesMap()
	if len(profiles) != 0 {
		t.Errorf("Expected no profiles initially, got %d", len(profiles))
	}
}

func TestListProfileCmdWithProfiles(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Add some profiles
	data.AddProfile("profile1", "/path/to/profile1.yaml")
	data.AddProfile("profile2", "/path/to/profile2.yaml")

	// Verify profiles were added
	profiles := data.GetProfilesMap()
	if len(profiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(profiles))
	}

	// Check that both profiles exist
	if _, exists := profiles["profile1"]; !exists {
		t.Errorf("Expected profile 'profile1' to exist")
	}
	if _, exists := profiles["profile2"]; !exists {
		t.Errorf("Expected profile 'profile2' to exist")
	}

	// Check profile paths
	if profiles["profile1"].Path != "/path/to/profile1.yaml" {
		t.Errorf("Expected profile1 path to be '/path/to/profile1.yaml', got '%s'", profiles["profile1"].Path)
	}
	if profiles["profile2"].Path != "/path/to/profile2.yaml" {
		t.Errorf("Expected profile2 path to be '/path/to/profile2.yaml', got '%s'", profiles["profile2"].Path)
	}
}

func TestListProfileCmdWithActiveProfile(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Add profiles
	data.AddProfile("profile1", "/path/to/profile1.yaml")
	data.AddProfile("profile2", "/path/to/profile2.yaml")

	// Set active profile
	data.SetProfile("profile1")

	// Verify active profile is set
	activeProfile := data.GetProfile()
	if activeProfile != "profile1" {
		t.Errorf("Expected active profile to be 'profile1', got '%s'", activeProfile)
	}

	// Verify both profiles exist
	profiles := data.GetProfilesMap()
	if _, exists := profiles["profile1"]; !exists {
		t.Errorf("Expected profile 'profile1' to exist")
	}
	if _, exists := profiles["profile2"]; !exists {
		t.Errorf("Expected profile 'profile2' to exist")
	}
}

func TestListProfileCmdUse(t *testing.T) {
	cmd := ListProfileCmd
	if cmd.Use != "list" {
		t.Errorf("Expected Use to be 'list', got '%s'", cmd.Use)
	}
}

func TestListProfileCmdShort(t *testing.T) {
	cmd := ListProfileCmd
	if cmd.Short != "List profiles" {
		t.Errorf("Expected Short to be 'List profiles', got '%s'", cmd.Short)
	}
}
