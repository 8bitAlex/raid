package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

// setupTest initializes a clean test environment
func setupTest(t *testing.T) func() {
	// Store original config path
	originalCfgPath := CfgPath

	// Create a temporary directory for test config
	tempDir := t.TempDir()
	CfgPath = tempDir + "/config.toml"

	// Reset viper and initialize with temp config
	viper.Reset()
	Initialize()

	// Return cleanup function
	return func() {
		viper.Reset()
		CfgPath = originalCfgPath
	}
}

func TestProfileStruct(t *testing.T) {
	profile := Profile{
		Name: "test-profile",
		Path: "/path/to/profile.yaml",
	}

	if profile.Name != "test-profile" {
		t.Errorf("Expected profile name to be 'test-profile', got '%s'", profile.Name)
	}

	if profile.Path != "/path/to/profile.yaml" {
		t.Errorf("Expected profile path to be '/path/to/profile.yaml', got '%s'", profile.Path)
	}
}

func TestSetProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Test setting a profile
	SetProfile("test-profile")

	// Verify it was set
	profile := GetProfile()
	if profile != "test-profile" {
		t.Errorf("Expected profile to be 'test-profile', got '%s'", profile)
	}
}

func TestGetProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Test getting profile when none is set
	profile := GetProfile()
	if profile != "" {
		t.Errorf("Expected empty profile when none is set, got '%s'", profile)
	}

	// Test getting profile when one is set
	SetProfile("test-profile")
	profile = GetProfile()
	if profile != "test-profile" {
		t.Errorf("Expected profile to be 'test-profile', got '%s'", profile)
	}
}

func TestAddProfile(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Test adding a profile
	AddProfile("test-profile", "/path/to/profile.yaml")

	// Verify it was added
	profiles := GetProfilesMap()
	if len(profiles) != 1 {
		t.Errorf("Expected 1 profile, got %d", len(profiles))
	}

	profile, exists := profiles["test-profile"]
	if !exists {
		t.Errorf("Expected profile 'test-profile' to exist")
	}

	if profile.Name != "test-profile" {
		t.Errorf("Expected profile name to be 'test-profile', got '%s'", profile.Name)
	}

	if profile.Path != "/path/to/profile.yaml" {
		t.Errorf("Expected profile path to be '/path/to/profile.yaml', got '%s'", profile.Path)
	}
}

func TestGetProfilesMap(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Test empty profiles map
	profiles := GetProfilesMap()
	if len(profiles) != 0 {
		t.Errorf("Expected empty profiles map, got %d profiles", len(profiles))
	}

	// Test with profiles
	AddProfile("profile1", "/path/to/profile1.yaml")
	AddProfile("profile2", "/path/to/profile2.yaml")

	profiles = GetProfilesMap()
	if len(profiles) != 2 {
		t.Errorf("Expected 2 profiles, got %d", len(profiles))
	}

	// Verify both profiles exist
	if _, exists := profiles["profile1"]; !exists {
		t.Errorf("Expected profile 'profile1' to exist")
	}

	if _, exists := profiles["profile2"]; !exists {
		t.Errorf("Expected profile 'profile2' to exist")
	}
}

func TestGetProfiles(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Test empty profiles
	profiles := GetProfiles()
	if len(profiles) != 0 {
		t.Errorf("Expected empty profiles slice, got %d profiles", len(profiles))
	}

	// Test with profiles
	AddProfile("profile1", "/path/to/profile1.yaml")
	AddProfile("profile2", "/path/to/profile2.yaml")

	profileNames := GetProfiles()
	if len(profileNames) != 2 {
		t.Errorf("Expected 2 profile names, got %d", len(profileNames))
	}

	// Check that both names are present (order doesn't matter)
	found1, found2 := false, false
	for _, name := range profileNames {
		if name == "profile1" {
			found1 = true
		}
		if name == "profile2" {
			found2 = true
		}
	}

	if !found1 {
		t.Errorf("Expected to find 'profile1' in profile names")
	}
	if !found2 {
		t.Errorf("Expected to find 'profile2' in profile names")
	}
}

func TestGetProfilePath(t *testing.T) {
	cleanup := setupTest(t)
	defer cleanup()

	// Test getting path for non-existent profile
	path, err := GetProfilePath("non-existent")
	if err == nil {
		t.Errorf("Expected error for non-existent profile")
	}
	if path != "" {
		t.Errorf("Expected empty path for non-existent profile, got '%s'", path)
	}

	// Test getting path for existing profile
	AddProfile("test-profile", "/path/to/profile.yaml")
	path, err = GetProfilePath("test-profile")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if path != "/path/to/profile.yaml" {
		t.Errorf("Expected path '/path/to/profile.yaml', got '%s'", path)
	}
}

func TestExtractProfileName(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()

	// Test YAML file
	yamlContent := `name: test-profile
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1`

	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}

	name, err := ExtractProfileName(yamlFile)
	if err != nil {
		t.Errorf("Unexpected error extracting name from YAML: %v", err)
	}
	if name != "test-profile" {
		t.Errorf("Expected name 'test-profile', got '%s'", name)
	}

	// Test JSON file
	jsonContent := `{
		"name": "test-profile-json",
		"repositories": [
			{
				"name": "repo1",
				"path": "/path/to/repo1",
				"url": "https://github.com/user/repo1"
			}
		]
	}`

	jsonFile := filepath.Join(tempDir, "test.json")
	err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	name, err = ExtractProfileName(jsonFile)
	if err != nil {
		t.Errorf("Unexpected error extracting name from JSON: %v", err)
	}
	if name != "test-profile-json" {
		t.Errorf("Expected name 'test-profile-json', got '%s'", name)
	}

	// Test unsupported file format
	unsupportedFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(unsupportedFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ExtractProfileName(unsupportedFile)
	if err == nil {
		t.Errorf("Expected error for unsupported file format")
	}

	// Test non-existent file
	_, err = ExtractProfileName("/non/existent/file.yaml")
	if err == nil {
		t.Errorf("Expected error for non-existent file")
	}

	// Test invalid YAML
	invalidYamlFile := filepath.Join(tempDir, "invalid.yaml")
	err = os.WriteFile(invalidYamlFile, []byte("invalid: yaml: content:"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid YAML file: %v", err)
	}

	_, err = ExtractProfileName(invalidYamlFile)
	if err == nil {
		t.Errorf("Expected error for invalid YAML")
	}

	// Test invalid JSON
	invalidJsonFile := filepath.Join(tempDir, "invalid.json")
	err = os.WriteFile(invalidJsonFile, []byte("{invalid json}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid JSON file: %v", err)
	}

	_, err = ExtractProfileName(invalidJsonFile)
	if err == nil {
		t.Errorf("Expected error for invalid JSON")
	}

	// Test missing name field
	noNameYamlFile := filepath.Join(tempDir, "noname.yaml")
	err = os.WriteFile(noNameYamlFile, []byte("repositories: []"), 0644)
	if err != nil {
		t.Fatalf("Failed to create no-name YAML file: %v", err)
	}

	_, err = ExtractProfileName(noNameYamlFile)
	if err == nil {
		t.Errorf("Expected error for missing name field")
	}

	// Test non-string name field
	nonStringNameYamlFile := filepath.Join(tempDir, "nonstringname.yaml")
	err = os.WriteFile(nonStringNameYamlFile, []byte("name: 123"), 0644)
	if err != nil {
		t.Fatalf("Failed to create non-string name YAML file: %v", err)
	}

	_, err = ExtractProfileName(nonStringNameYamlFile)
	if err == nil {
		t.Errorf("Expected error for non-string name field")
	}

	// Test empty name field
	emptyNameYamlFile := filepath.Join(tempDir, "emptyname.yaml")
	err = os.WriteFile(emptyNameYamlFile, []byte("name: ''"), 0644)
	if err != nil {
		t.Fatalf("Failed to create empty name YAML file: %v", err)
	}

	_, err = ExtractProfileName(emptyNameYamlFile)
	if err == nil {
		t.Errorf("Expected error for empty name field")
	}
}

func TestValidateProfileFile(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()

	// Test valid YAML file
	validYamlContent := `name: test-profile
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1`

	validYamlFile := filepath.Join(tempDir, "valid.yaml")
	err := os.WriteFile(validYamlFile, []byte(validYamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid YAML file: %v", err)
	}

	// Note: Schema validation tests would require mocking the schema file path
	// This is a simplified test that focuses on file format validation

	// Test valid JSON file
	validJsonContent := `{
		"name": "test-profile-json",
		"repositories": [
			{
				"name": "repo1",
				"path": "/path/to/repo1",
				"url": "https://github.com/user/repo1"
			}
		]
	}`

	validJsonFile := filepath.Join(tempDir, "valid.json")
	err = os.WriteFile(validJsonFile, []byte(validJsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create valid JSON file: %v", err)
	}

	// Test unsupported file format
	unsupportedFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(unsupportedFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = ValidateProfileFile(unsupportedFile)
	if err == nil {
		t.Errorf("Expected error for unsupported file format")
	}

	// Test non-existent file
	err = ValidateProfileFile("/non/existent/file.yaml")
	if err == nil {
		t.Errorf("Expected error for non-existent file")
	}

	// Test invalid YAML
	invalidYamlFile := filepath.Join(tempDir, "invalid.yaml")
	err = os.WriteFile(invalidYamlFile, []byte("invalid: yaml: content:"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid YAML file: %v", err)
	}

	err = ValidateProfileFile(invalidYamlFile)
	if err == nil {
		t.Errorf("Expected error for invalid YAML")
	}

	// Test invalid JSON
	invalidJsonFile := filepath.Join(tempDir, "invalid.json")
	err = os.WriteFile(invalidJsonFile, []byte("{invalid json}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid JSON file: %v", err)
	}

	err = ValidateProfileFile(invalidJsonFile)
	if err == nil {
		t.Errorf("Expected error for invalid JSON")
	}
}

// Helper function to create a temporary schema file for testing
func createTempSchemaFile(t *testing.T, tempDir string) string {
	schemaContent := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"name": {
				"type": "string"
			},
			"repositories": {
				"type": "array",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string"},
						"path": {"type": "string"},
						"url": {"type": "string"}
					},
					"required": ["name", "path", "url"]
				}
			}
		},
		"required": ["name"]
	}`

	schemaFile := filepath.Join(tempDir, "raid-profile.schema.json")
	err := os.WriteFile(schemaFile, []byte(schemaContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create schema file: %v", err)
	}

	return schemaFile
}

// TestProfileContent tests the ProfileContent struct
func TestProfileContent(t *testing.T) {
	profile := ProfileContent{
		Name: "test-profile",
		Repositories: []Repository{
			{
				Name: "repo1",
				Path: "/path/to/repo1",
				URL:  "https://github.com/user/repo1",
			},
			{
				Name: "repo2",
				Path: "/path/to/repo2",
				URL:  "https://github.com/user/repo2",
			},
		},
	}

	if profile.Name != "test-profile" {
		t.Errorf("Expected profile name 'test-profile', got '%s'", profile.Name)
	}

	if len(profile.Repositories) != 2 {
		t.Errorf("Expected 2 repositories, got %d", len(profile.Repositories))
	}

	if profile.Repositories[0].Name != "repo1" {
		t.Errorf("Expected first repository name 'repo1', got '%s'", profile.Repositories[0].Name)
	}

	if profile.Repositories[1].URL != "https://github.com/user/repo2" {
		t.Errorf("Expected second repository URL 'https://github.com/user/repo2', got '%s'", profile.Repositories[1].URL)
	}
}

// TestRepository tests the Repository struct
func TestRepository(t *testing.T) {
	repo := Repository{
		Name: "test-repo",
		Path: "/path/to/repo",
		URL:  "https://github.com/user/repo",
	}

	if repo.Name != "test-repo" {
		t.Errorf("Expected repository name 'test-repo', got '%s'", repo.Name)
	}

	if repo.Path != "/path/to/repo" {
		t.Errorf("Expected repository path '/path/to/repo', got '%s'", repo.Path)
	}

	if repo.URL != "https://github.com/user/repo" {
		t.Errorf("Expected repository URL 'https://github.com/user/repo', got '%s'", repo.URL)
	}
}

// TestReadProfileFile tests reading profile files
func TestReadProfileFile(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()

	// Test YAML file
	yamlContent := `name: test-profile
repositories:
  - name: repo1
    path: /path/to/repo1
    url: https://github.com/user/repo1
  - name: repo2
    path: /path/to/repo2
    url: https://github.com/user/repo2`

	yamlFile := filepath.Join(tempDir, "test.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}

	profile, err := ReadProfileFile(yamlFile)
	if err != nil {
		t.Errorf("Unexpected error reading YAML file: %v", err)
	}

	if profile.Name != "test-profile" {
		t.Errorf("Expected profile name 'test-profile', got '%s'", profile.Name)
	}

	if len(profile.Repositories) != 2 {
		t.Errorf("Expected 2 repositories, got %d", len(profile.Repositories))
	}

	// Test JSON file
	jsonContent := `{
		"name": "test-profile-json",
		"repositories": [
			{
				"name": "repo1",
				"path": "/path/to/repo1",
				"url": "https://github.com/user/repo1"
			}
		]
	}`

	jsonFile := filepath.Join(tempDir, "test.json")
	err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}

	profile, err = ReadProfileFile(jsonFile)
	if err != nil {
		t.Errorf("Unexpected error reading JSON file: %v", err)
	}

	if profile.Name != "test-profile-json" {
		t.Errorf("Expected profile name 'test-profile-json', got '%s'", profile.Name)
	}

	if len(profile.Repositories) != 1 {
		t.Errorf("Expected 1 repository, got %d", len(profile.Repositories))
	}

	// Test unsupported file format
	unsupportedFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(unsupportedFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err = ReadProfileFile(unsupportedFile)
	if err == nil {
		t.Errorf("Expected error for unsupported file format")
	}

	// Test non-existent file
	_, err = ReadProfileFile("/non/existent/file.yaml")
	if err == nil {
		t.Errorf("Expected error for non-existent file")
	}

	// Test invalid YAML
	invalidYamlFile := filepath.Join(tempDir, "invalid.yaml")
	err = os.WriteFile(invalidYamlFile, []byte("invalid: yaml: content:"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid YAML file: %v", err)
	}

	_, err = ReadProfileFile(invalidYamlFile)
	if err == nil {
		t.Errorf("Expected error for invalid YAML")
	}

	// Test invalid JSON
	invalidJsonFile := filepath.Join(tempDir, "invalid.json")
	err = os.WriteFile(invalidJsonFile, []byte("{invalid json}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid JSON file: %v", err)
	}

	_, err = ReadProfileFile(invalidJsonFile)
	if err == nil {
		t.Errorf("Expected error for invalid JSON")
	}

	// Test missing name field
	noNameYamlFile := filepath.Join(tempDir, "noname.yaml")
	err = os.WriteFile(noNameYamlFile, []byte("repositories: []"), 0644)
	if err != nil {
		t.Fatalf("Failed to create no-name YAML file: %v", err)
	}

	_, err = ReadProfileFile(noNameYamlFile)
	if err == nil {
		t.Errorf("Expected error for missing name field")
	}
}

// TestGetActiveProfileContent tests getting active profile content
func TestGetActiveProfileContent(t *testing.T) {
	// Setup
	cleanup := setupTest(t)
	defer cleanup()

	// Test with no active profile (should fail)
	_, err := GetActiveProfileContent()
	if err == nil {
		t.Errorf("Expected error when no active profile is set")
	}

	// Verify the error message
	expectedError := "no active profile set"
	if err != nil && !contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', got '%s'", expectedError, err.Error())
	}

	// Test with active profile but no profile file
	SetProfile("test-profile")
	AddProfile("test-profile", "/non/existent/profile.yaml")

	_, err = GetActiveProfileContent()
	if err == nil {
		t.Errorf("Expected error when profile file doesn't exist")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
