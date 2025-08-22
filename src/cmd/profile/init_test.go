package profile

import (
	"testing"
)

func TestInit(t *testing.T) {
	// Test that the init function runs without error
	// This test ensures that all subcommands are properly added to the main command

	// Check that subcommands are added
	if ProfileCmd.Commands() == nil {
		t.Errorf("Expected ProfileCmd to have subcommands")
	}

	// Check that all expected subcommands are present
	expectedSubcommands := []string{"add", "list", "use"}
	foundSubcommands := make(map[string]bool)

	for _, cmd := range ProfileCmd.Commands() {
		foundSubcommands[cmd.Name()] = true
	}

	for _, expected := range expectedSubcommands {
		if !foundSubcommands[expected] {
			t.Errorf("Expected subcommand '%s' to be present", expected)
		}
	}
}
