package install

import (
	"os"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib/data"
)

// TestInstallDemo demonstrates the complete install workflow
func TestInstallDemo(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Get the path to the example profile file
	exampleProfilePath := "../../docs/examples/install-demo.raid.yaml"

	// Check if the example file exists
	if _, err := os.Stat(exampleProfilePath); os.IsNotExist(err) {
		t.Skip("Example profile file not found, skipping install demo test")
	}

	// Test the complete workflow
	testInstallWorkflow(t, exampleProfilePath)
}

func testInstallWorkflow(t *testing.T, profilePath string) {
	// Step 1: Read and parse the profile file
	profile, err := data.ReadProfileFile(profilePath)
	if err != nil {
		t.Fatalf("Failed to read profile file: %v", err)
	}

	// Verify profile content
	if profile.Name != "install-demo" {
		t.Errorf("Expected profile name 'install-demo', got '%s'", profile.Name)
	}

	if len(profile.Repositories) != 3 {
		t.Errorf("Expected 3 repositories, got %d", len(profile.Repositories))
	}

	// Verify repository details
	expectedRepos := []struct {
		name string
		path string
		url  string
	}{
		{"frontend", "~/Developer/demo-frontend", "https://github.com/8bitAlex/raid.git"},
		{"backend", "~/Developer/demo-backend", "https://github.com/8bitAlex/raid.git"},
		{"shared-libs", "~/Developer/demo-shared", "https://github.com/8bitAlex/raid.git"},
	}

	for i, expected := range expectedRepos {
		repo := profile.Repositories[i]
		if repo.Name != expected.name {
			t.Errorf("Expected repository %d name '%s', got '%s'", i+1, expected.name, repo.Name)
		}
		if repo.Path != expected.path {
			t.Errorf("Expected repository %d path '%s', got '%s'", i+1, expected.path, repo.Path)
		}
		if repo.URL != expected.url {
			t.Errorf("Expected repository %d URL '%s', got '%s'", i+1, expected.url, repo.URL)
		}
	}

	// Step 2: Add the profile to the system
	data.AddProfile(profile.Name, profilePath)

	// Step 3: Set it as the active profile
	data.SetProfile(profile.Name)

	// Step 4: Verify the profile is active
	activeProfile := data.GetProfile()
	if activeProfile != profile.Name {
		t.Errorf("Expected active profile to be '%s', got '%s'", profile.Name, activeProfile)
	}

	// Step 5: Test getting active profile content
	activeContent, err := data.GetActiveProfileContent()
	if err != nil {
		t.Fatalf("Failed to get active profile content: %v", err)
	}

	if activeContent.Name != profile.Name {
		t.Errorf("Expected active content name '%s', got '%s'", profile.Name, activeContent.Name)
	}

	if len(activeContent.Repositories) != len(profile.Repositories) {
		t.Errorf("Expected %d repositories in active content, got %d", len(profile.Repositories), len(activeContent.Repositories))
	}

	// Step 6: Test path expansion for repositories
	for _, repo := range activeContent.Repositories {
		// Verify the path contains the expected directory structure
		if !strings.Contains(repo.Path, "demo-") {
			t.Errorf("Expected repository path to contain 'demo-', got '%s'", repo.Path)
		}
	}

	// Note: We don't actually clone repositories in tests to avoid side effects
	// In a real scenario, you would call repo.InstallProfile() here
}

// setupTest initializes a clean test environment
func setupTest(t *testing.T) func() {
	// Store original config path
	originalCfgPath := data.CfgPath

	// Create a temporary directory for test config
	tempDir := t.TempDir()
	data.CfgPath = tempDir + "/config.toml"

	// Reset viper and initialize with temp config
	data.Initialize()

	// Return cleanup function
	return func() {
		data.CfgPath = originalCfgPath
	}
}
