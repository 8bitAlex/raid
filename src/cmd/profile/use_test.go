package profile

import (
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
)

func TestUseProfileCmd(t *testing.T) {
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

func TestUseProfileCmdNonExistentProfile(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test that non-existent profile doesn't exist
	profiles := data.GetProfilesMap()
	if _, exists := profiles["non-existent-profile"]; exists {
		t.Errorf("Expected profile 'non-existent-profile' to not exist")
	}
}

func TestUseProfileCmdArgs(t *testing.T) {
	cmd := UseProfileCmd

	// Test with no args (should fail)
	err := cmd.Args(cmd, []string{})
	if err == nil {
		t.Errorf("Expected error with no args, got nil")
	}

	// Test with one arg (should work)
	err = cmd.Args(cmd, []string{"profile-name"})
	if err != nil {
		t.Errorf("Expected no error with one arg, got %v", err)
	}

	// Test with multiple args (should fail)
	err = cmd.Args(cmd, []string{"profile1", "profile2"})
	if err == nil {
		t.Errorf("Expected error with multiple args, got nil")
	}
}

func TestUseProfileCmdUse(t *testing.T) {
	cmd := UseProfileCmd
	if cmd.Use != "use profile" {
		t.Errorf("Expected Use to be 'use profile', got '%s'", cmd.Use)
	}
}

func TestUseProfileCmdShort(t *testing.T) {
	cmd := UseProfileCmd
	if cmd.Short != "Use a specific profile" {
		t.Errorf("Expected Short to be 'Use a specific profile', got '%s'", cmd.Short)
	}
}

func TestUseProfileCmdSuggestFor(t *testing.T) {
	cmd := UseProfileCmd
	if len(cmd.SuggestFor) != 1 {
		t.Errorf("Expected 1 suggestion, got %d", len(cmd.SuggestFor))
	}
	if cmd.SuggestFor[0] != "set" {
		t.Errorf("Expected suggestion to be 'set', got '%s'", cmd.SuggestFor[0])
	}
}
