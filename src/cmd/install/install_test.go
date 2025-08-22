package install

import (
	"testing"
)

func TestInstallCmd(t *testing.T) {
	cmd := InstallCmd

	// Test command metadata
	if cmd.Use != "install" {
		t.Errorf("Expected Use to be 'install', got '%s'", cmd.Use)
	}

	if cmd.Short != "Install repositories from the active profile" {
		t.Errorf("Expected Short to be 'Install repositories from the active profile', got '%s'", cmd.Short)
	}

	expectedLong := "Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped."
	if cmd.Long != expectedLong {
		t.Errorf("Expected Long to be '%s', got '%s'", expectedLong, cmd.Long)
	}
}

func TestInstallCmdArgs(t *testing.T) {
	cmd := InstallCmd

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
