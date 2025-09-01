package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Profile represents a raid profile with its name and file path
type Profile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

const ACTIVE_PROFILE_KEY = "profile"
const ALL_PROFILES_KEY = "profiles"

func SetProfile(profileName string) {
	Set(ACTIVE_PROFILE_KEY, profileName)
}

func GetProfile() string {
	profile := Get(ACTIVE_PROFILE_KEY)
	if profile == nil {
		return ""
	}
	return profile.(string)
}

// AddProfile adds a profile with the given name and file path
func AddProfile(profileName, filePath string) {
	profiles := viper.GetStringMap(ALL_PROFILES_KEY)
	if profiles == nil {
		profiles = make(map[string]interface{})
	}

	profiles[profileName] = map[string]interface{}{
		"name": profileName,
		"path": filePath,
	}
	Set(ALL_PROFILES_KEY, profiles)
}

// GetProfilesMap returns a map of profile names to Profile structs
func GetProfilesMap() map[string]Profile {
	profiles := viper.GetStringMap(ALL_PROFILES_KEY)
	if profiles == nil {
		return make(map[string]Profile)
	}

	// Convert the map to our Profile struct
	profileMap := make(map[string]Profile)
	for name, value := range profiles {
		if profileData, ok := value.(map[string]interface{}); ok {
			profileMap[name] = Profile{
				Name: profileData["name"].(string),
				Path: profileData["path"].(string),
			}
		}
	}
	return profileMap
}

// GetProfiles returns a slice of profile names
func GetProfiles() []string {
	profiles := GetProfilesMap()
	names := make([]string, 0, len(profiles))
	for name := range profiles {
		names = append(names, name)
	}
	return names
}

// GetProfilePath returns the file path for a given profile name
func GetProfilePath(profileName string) (string, error) {
	profiles := GetProfilesMap()
	if profile, exists := profiles[profileName]; exists {
		return profile.Path, nil
	}
	return "", fmt.Errorf("profile '%s' not found", profileName)
}

// ExtractProfileName extracts the profile name from a profile file
func ExtractProfileName(filePath string) (string, error) {
	profiles, err := ExtractProfiles(filePath)
	if err != nil {
		return "", err
	}

	if len(profiles) == 0 {
		return "", fmt.Errorf("no profiles found in file")
	}

	if len(profiles) > 1 {
		return "", fmt.Errorf("file contains multiple profiles, use ExtractProfiles instead")
	}

	return profiles[0].Name, nil
}

// ProfileInfo represents a profile with its name and file path
type ProfileInfo struct {
	Name string
	Path string
}

// ExtractProfiles extracts all profiles from a file (supports YAML documents and JSON arrays)
func ExtractProfiles(filePath string) ([]ProfileInfo, error) {
	// Read the profile file
	profileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	// Check file extension to determine format
	ext := strings.ToLower(filepath.Ext(filePath))
	var profiles []ProfileInfo

	switch ext {
	case ".yaml", ".yml":
		profiles, err = extractProfilesFromYAML(profileData, filePath)
	case ".json":
		profiles, err = extractProfilesFromJSON(profileData, filePath)
	default:
		return nil, fmt.Errorf("unsupported file format: %s. Supported formats are .yaml, .yml, and .json", ext)
	}

	if err != nil {
		return nil, err
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles found in file")
	}

	return profiles, nil
}

// extractProfilesFromYAML extracts profiles from YAML content (supports multiple documents)
func extractProfilesFromYAML(data []byte, filePath string) ([]ProfileInfo, error) {
	var profiles []ProfileInfo

	// Split by YAML document separator
	documents := strings.Split(string(data), "---")

	for i, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var profile map[string]interface{}
		if err := yaml.Unmarshal([]byte(doc), &profile); err != nil {
			return nil, fmt.Errorf("invalid YAML document %d: %w", i+1, err)
		}

		// Extract the name field
		name, ok := profile["name"]
		if !ok {
			return nil, fmt.Errorf("profile document %d is missing required 'name' field", i+1)
		}

		nameStr, ok := name.(string)
		if !ok {
			return nil, fmt.Errorf("profile document %d 'name' field must be a string", i+1)
		}

		if nameStr == "" {
			return nil, fmt.Errorf("profile document %d 'name' field cannot be empty", i+1)
		}

		profiles = append(profiles, ProfileInfo{
			Name: nameStr,
			Path: filePath,
		})
	}

	return profiles, nil
}

// extractProfilesFromJSON extracts profiles from JSON content (supports single object or array)
func extractProfilesFromJSON(data []byte, filePath string) ([]ProfileInfo, error) {
	var profiles []ProfileInfo

	// Try to parse as a single profile object first
	var singleProfile map[string]interface{}
	if err := json.Unmarshal(data, &singleProfile); err == nil {
		// Check if it has a name field (indicating it's a profile)
		if name, ok := singleProfile["name"]; ok {
			if nameStr, ok := name.(string); ok && nameStr != "" {
				return []ProfileInfo{{
					Name: nameStr,
					Path: filePath,
				}}, nil
			}
		}
	}

	// Try to parse as an array of profiles
	var profileArray []map[string]interface{}
	if err := json.Unmarshal(data, &profileArray); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	for i, profile := range profileArray {
		// Extract the name field
		name, ok := profile["name"]
		if !ok {
			return nil, fmt.Errorf("profile %d is missing required 'name' field", i+1)
		}

		nameStr, ok := name.(string)
		if !ok {
			return nil, fmt.Errorf("profile %d 'name' field must be a string", i+1)
		}

		if nameStr == "" {
			return nil, fmt.Errorf("profile %d 'name' field cannot be empty", i+1)
		}

		profiles = append(profiles, ProfileInfo{
			Name: nameStr,
			Path: filePath,
		})
	}

	return profiles, nil
}

// ValidateProfileFile validates a profile file against the JSON schema
func ValidateProfileFile(filePath string) error {
	// Extract profiles to validate each one
	profiles, err := ExtractProfiles(filePath)
	if err != nil {
		return fmt.Errorf("failed to extract profiles: %w", err)
	}

	// Validate each profile individually
	for i := range profiles {
		if err := validateSingleProfile(filePath, i+1); err != nil {
			return fmt.Errorf("profile %d validation failed: %w", i+1, err)
		}
	}

	return nil
}

// validateSingleProfile validates a single profile from a file
func validateSingleProfile(filePath string, profileIndex int) error {
	// Read the profile file
	profileData, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read profile file: %w", err)
	}

	// Check file extension to determine format
	ext := strings.ToLower(filepath.Ext(filePath))
	var profile interface{}
	var jsonData []byte

	switch ext {
	case ".yaml", ".yml":
		// For YAML, we need to handle multiple documents
		documents := strings.Split(string(profileData), "---")
		if profileIndex <= len(documents) {
			doc := strings.TrimSpace(documents[profileIndex-1])
			if doc == "" {
				return fmt.Errorf("profile %d is empty", profileIndex)
			}

			// Parse YAML
			if err := yaml.Unmarshal([]byte(doc), &profile); err != nil {
				return fmt.Errorf("invalid YAML format: %w", err)
			}
			// Convert YAML to JSON for schema validation
			jsonData, err = json.Marshal(profile)
			if err != nil {
				return fmt.Errorf("failed to convert YAML to JSON: %w", err)
			}
		} else {
			return fmt.Errorf("profile %d not found in file", profileIndex)
		}
	case ".json":
		// For JSON, we need to handle arrays
		var jsonProfiles []interface{}
		if err := json.Unmarshal(profileData, &jsonProfiles); err == nil {
			// It's an array
			if profileIndex <= len(jsonProfiles) {
				profile = jsonProfiles[profileIndex-1]
				jsonData, err = json.Marshal(profile)
				if err != nil {
					return fmt.Errorf("failed to marshal profile %d: %w", profileIndex, err)
				}
			} else {
				return fmt.Errorf("profile %d not found in file", profileIndex)
			}
		} else {
			// Try as single object
			if profileIndex == 1 {
				if err := json.Unmarshal(profileData, &profile); err != nil {
					return fmt.Errorf("invalid JSON format: %w", err)
				}
				jsonData = profileData
			} else {
				return fmt.Errorf("profile %d not found in file", profileIndex)
			}
		}
	default:
		return fmt.Errorf("unsupported file format: %s. Supported formats are .yaml, .yml, and .json", ext)
	}

	// Get the schema file path relative to the project root
	schemaPath := "spec/raid-profile.schema.json"

	// Check if schema file exists
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		return fmt.Errorf("schema file not found at %s", schemaPath)
	}

	// Read the schema file
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Parse the schema data
	var schemaMap map[string]interface{}
	if err := json.Unmarshal(schemaData, &schemaMap); err != nil {
		return fmt.Errorf("failed to parse schema file: %w", err)
	}

	// Create a new JSON schema compiler
	compiler := jsonschema.NewCompiler()

	// Add the schema to the compiler
	if err := compiler.AddResource("schema.json", schemaMap); err != nil {
		return fmt.Errorf("failed to add schema resource: %w", err)
	}

	// Compile the schema
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	// Parse the JSON data for validation
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return fmt.Errorf("failed to parse JSON data: %w", err)
	}

	// Validate the profile data against the schema
	if err := schema.Validate(data); err != nil {
		return fmt.Errorf("profile validation failed: %w", err)
	}

	return nil
}

// ProfileContent represents the content of a profile file
type ProfileContent struct {
	Name         string        `json:"name" yaml:"name"`
	Repositories []Repository  `json:"repositories" yaml:"repositories"`
	Environments []Environment `json:"environments" yaml:"environments"`
}

// Repository represents a repository in a profile
type Repository struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path" yaml:"path"`
	URL  string `json:"url" yaml:"url"`
}

// Environment represents an environment configuration
type Environment struct {
	Name      string                `json:"name" yaml:"name"`
	Tasks     []Task                `json:"tasks" yaml:"tasks"`
	Variables []EnvironmentVariable `json:"variables" yaml:"variables"`
}

// Task represents a task to be executed
type Task struct {
	Type string `json:"type" yaml:"type"`
	Cmd  string `json:"cmd,omitempty" yaml:"cmd,omitempty"`
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

// EnvironmentVariable represents an environment variable
type EnvironmentVariable struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

// GetActiveProfileContent reads and parses the active profile file
func GetActiveProfileContent() (*ProfileContent, error) {
	activeProfile := GetProfile()
	if activeProfile == "" {
		return nil, fmt.Errorf("no active profile set. Use 'raid profile use <profile-name>' to set an active profile")
	}

	profilePath, err := GetProfilePath(activeProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to get profile path for '%s': %w", activeProfile, err)
	}

	return ReadProfileFile(profilePath)
}

// ReadProfileFile reads and parses a profile file
func ReadProfileFile(filePath string) (*ProfileContent, error) {
	// Read the profile file
	profileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file: %w", err)
	}

	// Check file extension to determine format
	ext := strings.ToLower(filepath.Ext(filePath))
	var profile ProfileContent

	switch ext {
	case ".yaml", ".yml":
		// Parse YAML
		if err := yaml.Unmarshal(profileData, &profile); err != nil {
			return nil, fmt.Errorf("invalid YAML format: %w", err)
		}
	case ".json":
		// Parse JSON
		if err := json.Unmarshal(profileData, &profile); err != nil {
			return nil, fmt.Errorf("invalid JSON format: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s. Supported formats are .yaml, .yml, and .json", ext)
	}

	// Validate required fields
	if profile.Name == "" {
		return nil, fmt.Errorf("profile file is missing required 'name' field")
	}

	return &profile, nil
}
