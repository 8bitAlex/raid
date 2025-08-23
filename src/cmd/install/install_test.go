package install

import (
	"testing"
)

func TestInstallCmd(t *testing.T) {
	cmd := Command

	// Test command metadata
	if cmd.Use != "install" {
		t.Errorf("Expected Use to be 'install', got '%s'", cmd.Use)
	}

	if cmd.Short != "Install repositories from the active profile" {
		t.Errorf("Expected Short to be 'Install repositories from the active profile', got '%s'", cmd.Short)
	}

	expectedLong := "Clones all repositories defined in the active profile to their specified paths. If a repository already exists, it will be skipped. Repositories are cloned concurrently for better performance."
	if cmd.Long != expectedLong {
		t.Errorf("Expected Long to be '%s', got '%s'", expectedLong, cmd.Long)
	}
}

func TestInstallCmdArgs(t *testing.T) {
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

func TestInstallCmdFlags(t *testing.T) {
	cmd := Command

	// Test that the concurrency flag exists
	concurrencyFlag := cmd.Flags().Lookup("threads")
	if concurrencyFlag == nil {
		t.Errorf("Expected concurrency flag to exist")
	}

	if concurrencyFlag.Name != "threads" {
		t.Errorf("Expected flag name 'concurrency', got '%s'", concurrencyFlag.Name)
	}

	if concurrencyFlag.Shorthand != "t" {
		t.Errorf("Expected flag shorthand 'c', got '%s'", concurrencyFlag.Shorthand)
	}

	// Test that the default value is 0 (unlimited)
	if concurrencyFlag.DefValue != "0" {
		t.Errorf("Expected default value '0', got '%s'", concurrencyFlag.DefValue)
	}
}
