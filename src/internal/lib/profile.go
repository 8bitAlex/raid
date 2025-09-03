package lib

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	sys "github.com/8bitalex/raid/src/internal/sys"
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
	name := viper.GetString(ACTIVE_PROFILE_KEY)
	paths := getProfilePaths()
	return Profile{
		Name: name,
		Path: paths[name],
	}
}

func AddProfile(profile Profile) {
	profiles := viper.GetStringMapString(ALL_PROFILES_KEY)
	if profiles == nil {
		profiles = make(map[string]string)
	}

	profiles[profile.Name] = profile.Path
	Set(ALL_PROFILES_KEY, profiles)
}

func AddProfiles(profiles []Profile) {
	for _, profile := range profiles {
		AddProfile(profile)
	}
}

func GetProfiles() []Profile {
	profilesMap := getProfilePaths()
	results := make([]Profile, 0, len(profilesMap))
	for name, path := range profilesMap {
		results = append(results, Profile{name, path})
	}
	return results
}

func getProfilePaths() map[string]string {
	profiles := viper.GetStringMapString(ALL_PROFILES_KEY)
	if profiles == nil {
		return make(map[string]string)
	}
	return profiles
}

func RemoveProfile(name string) error {
	profiles := viper.GetStringMapString(ALL_PROFILES_KEY)
	if profiles == nil {
		return fmt.Errorf("no profiles found")
	}
	if _, exists := profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}
	delete(profiles, name)
	Set(ALL_PROFILES_KEY, profiles)
	return nil
}

func ExtractProfiles(path string) ([]Profile, error) {
	profileData, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile from file %s: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var profiles []Profile

	switch ext {
	case ".yaml", ".yml":
		profiles, err = extractProfilesFromYAML(profileData, path)
	case ".json":
		profiles, err = extractProfilesFromJSON(profileData, path)
	default:
		return nil, fmt.Errorf("unsupported file format: %s. Supported formats are .yaml, .yml, and .json", ext)
	}

	if err != nil {
		return nil, err
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles found in file %s", path)
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
			return nil, fmt.Errorf("invalid YAML document in %s: %w", path, err)
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
		return nil, fmt.Errorf("invalid JSON format in %s: %w", path, err)
	}

	results := make([]Profile, 0, len(profiles))
	for _, p := range profiles {
		p.Path = path
		results = append(results, p)
	}

	return results, nil
}

func ContainsProfile(name string) bool {
	profiles := viper.GetStringMapString(ALL_PROFILES_KEY)
	if profiles == nil {
		return false
	}

	_, exists := profiles[name]
	return exists
}

func ValidateProfile(path string) error {
	if !sys.FileExists(path) {
		return fmt.Errorf("file not found at %s", path)
	}

	c := jsonschema.NewCompiler()
	sch, err := c.Compile(SCHEMA_PATH)
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	profile := io.Reader(f)

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		data, err := yamlToJSON(f)
		if err != nil {
			return err
		}
		profile = bytes.NewReader(data)
	}

	json, err := jsonschema.UnmarshalJSON(profile)
	if err != nil {
		return err
	}

	err = sch.Validate(json)
	if err != nil {
		return fmt.Errorf("invalid profile format: %w", err)
	}
	return nil
}

func yamlToJSON(file io.Reader) ([]byte, error) {
	var data interface{}
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}
	return json.Marshal(data)
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
