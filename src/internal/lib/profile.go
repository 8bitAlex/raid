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

type Profile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

const ACTIVE_PROFILE_KEY = "profile"
const ALL_PROFILES_KEY = "profiles"
const SCHEMA_PATH = "schemas/raid-profile.schema.json"

func SetProfile(name string) error {
	if !ContainsProfile(name) {
		return fmt.Errorf("profile '%s' not found", name)
	}
	Set(ACTIVE_PROFILE_KEY, name)
	return nil
}

func GetProfile() Profile {
	profile := Get(ACTIVE_PROFILE_KEY)
	if profile == nil {
		return Profile{}
	}
	return profile.(Profile)
}

func AddProfile(profile Profile) {
	profiles := viper.GetStringMap(ALL_PROFILES_KEY)
	if profiles == nil {
		profiles = make(map[string]interface{})
	}

	profiles[profile.Name] = profile
	Set(ALL_PROFILES_KEY, profiles)
}

func AddProfiles(profiles []Profile) {
	for _, profile := range profiles {
		AddProfile(profile)
	}
}

func GetProfiles() []Profile {
	profilesMap := getProfilesMap()
	results := make([]Profile, 0, len(profilesMap))
	for name, path := range profilesMap {
		results = append(results, Profile{name, path})
	}
	return results
}

func getProfilesMap() map[string]string {
	profiles := viper.GetStringMap(ALL_PROFILES_KEY)
	if profiles == nil {
		return make(map[string]string)
	}

	results := make(map[string]string)
	for name, value := range profiles {
		results[name] = value.(string)
	}
	return results
}

func ExtractProfiles(path string) ([]Profile, error) {
	profileData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to read profile from file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var profiles []Profile

	switch ext {
	case ".yaml", ".yml":
		profiles, err = extractProfilesFromYAML(profileData, path)
	case ".json":
		profiles, err = extractProfilesFromJSON(profileData, path)
	default:
		return nil, fmt.Errorf("Unsupported file format: %s. Supported formats are .yaml, .yml, and .json", ext)
	}

	if err != nil {
		return nil, err
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("No profiles found in file %s", path)
	}

	return profiles, nil
}

func extractProfilesFromYAML(data []byte, path string) ([]Profile, error) {
	var profiles []Profile

	documents := strings.Split(string(data), YAML_SEP)

	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		var profile Profile
		if err := yaml.Unmarshal([]byte(doc), &profile); err != nil {
			return nil, fmt.Errorf("Invalid YAML document in %s: %w", path, err)
		}
		profile.Path = path

		profiles = append(profiles, profile)
	}

	return profiles, nil
}

func extractProfilesFromJSON(data []byte, path string) ([]Profile, error) {
	var profiles []Profile

	var profile Profile
	if err := json.Unmarshal(data, &profile); err == nil {
		profile.Path = path
		return []Profile{profile}, nil
	}

	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("Invalid JSON format in %s: %w", path, err)
	}

	results := make([]Profile, 0, len(profiles))
	for _, p := range profiles {
		p.Path = path
		results = append(results, p)
	}

	return results, nil
}

func ContainsProfile(name string) bool {
	profiles := viper.GetStringMap(ALL_PROFILES_KEY)
	if profiles == nil {
		return false
	}

	_, exists := profiles[name]
	return exists
}

func ValidateProfile(path string) error {
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

// // ProfileContent represents the content of a profile file
// type ProfileContent struct {
// 	Name         string        `json:"name" yaml:"name"`
// 	Repositories []Repository  `json:"repositories" yaml:"repositories"`
// 	Environments []Environment `json:"environments" yaml:"environments"`
// }

// // Repository represents a repository in a profile
// type Repository struct {
// 	Name string `json:"name" yaml:"name"`
// 	Path string `json:"path" yaml:"path"`
// 	URL  string `json:"url" yaml:"url"`
// }

// // Environment represents an environment configuration
// type Environment struct {
// 	Name      string                `json:"name" yaml:"name"`
// 	Tasks     []Task                `json:"tasks" yaml:"tasks"`
// 	Variables []EnvironmentVariable `json:"variables" yaml:"variables"`
// }

// // Task represents a task to be executed
// type Task struct {
// 	Type string `json:"type" yaml:"type"`
// 	Cmd  string `json:"cmd,omitempty" yaml:"cmd,omitempty"`
// 	Path string `json:"path,omitempty" yaml:"path,omitempty"`
// }

// // EnvironmentVariable represents an environment variable
// type EnvironmentVariable struct {
// 	Name  string `json:"name" yaml:"name"`
// 	Value string `json:"value" yaml:"value"`
// }

// // GetActiveProfileContent reads and parses the active profile file
// func GetActiveProfileContent() (*ProfileContent, error) {
// 	activeProfile := GetProfile()
// 	if activeProfile == "" {
// 		return nil, fmt.Errorf("no active profile set. Use 'raid profile use <profile-name>' to set an active profile")
// 	}

// 	profilePath, err := GetProfilePath(activeProfile)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to get profile path for '%s': %w", activeProfile, err)
// 	}

// 	return ReadProfileFile(profilePath)
// }

// // ReadProfileFile reads and parses a profile file
// func ReadProfileFile(filePath string) (*ProfileContent, error) {
// 	// Read the profile file
// 	profileData, err := os.ReadFile(filePath)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to read profile file: %w", err)
// 	}

// 	// Check file extension to determine format
// 	ext := strings.ToLower(filepath.Ext(filePath))
// 	var profile ProfileContent

// 	switch ext {
// 	case ".yaml", ".yml":
// 		// Parse YAML
// 		if err := yaml.Unmarshal(profileData, &profile); err != nil {
// 			return nil, fmt.Errorf("invalid YAML format: %w", err)
// 		}
// 	case ".json":
// 		// Parse JSON
// 		if err := json.Unmarshal(profileData, &profile); err != nil {
// 			return nil, fmt.Errorf("invalid JSON format: %w", err)
// 		}
// 	default:
// 		return nil, fmt.Errorf("unsupported file format: %s. Supported formats are .yaml, .yml, and .json", ext)
// 	}

// 	// Validate required fields
// 	if profile.Name == "" {
// 		return nil, fmt.Errorf("profile file is missing required 'name' field")
// 	}

// 	return &profile, nil
// }
