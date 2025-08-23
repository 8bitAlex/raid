package profile

import (
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
)

func TestProfileCmd(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test with no active profile
	profile := data.GetProfile()
	if profile != "" {
		t.Errorf("Expected no active profile initially, got '%s'", profile)
	}

	// Test with active profile
	data.SetProfile("test-profile")

	profile = data.GetProfile()
	if profile != "test-profile" {
		t.Errorf("Expected active profile to be 'test-profile', got '%s'", profile)
	}
}

func TestProfileCmdArgs(t *testing.T) {
	// Test that the command requires no arguments
	cmd := Command

	// Test with no args (should work)
	err := cmd.Args(cmd, []string{})
	if err != nil {
		t.Errorf("Expected no error with no args, got %v", err)
	}

	// Test with args (should fail)
	err = cmd.Args(cmd, []string{"arg1"})
	if err == nil {
		t.Errorf("Expected error with args, got nil")
	}
}

func TestProfileCmdUse(t *testing.T) {
	cmd := Command
	if cmd.Use != "profile" {
		t.Errorf("Expected Use to be 'profile', got '%s'", cmd.Use)
	}
}

func TestProfileCmdShort(t *testing.T) {
	cmd := Command
	if cmd.Short != "Manage raid profiles" {
		t.Errorf("Expected Short to be 'Manage raid profiles', got '%s'", cmd.Short)
	}
}
