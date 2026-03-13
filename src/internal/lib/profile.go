package lib

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	ACTIVE_PROFILE_KEY  = "profile"
	ALL_PROFILES_KEY    = "profiles"
	PROFILE_SCHEMA_PATH = "schemas/raid-profile.schema.json"
)

type Profile struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Repositories []Repo    `json:"repositories"`
	Environments []Env     `json:"environments"`
	Install      OnInstall `json:"install"`
}

func (p Profile) IsZero() bool {
	return p.Name == "" || p.Path == ""
}

func (p Profile) getEnv(name string) Env {
	for _, env := range p.Environments {
		if env.Name == name {
			return env
		}
	}
	return Env{}
}

func SetProfile(name string) error {
	if !ContainsProfile(name) {
		return fmt.Errorf("profile '%s' not found", name)
	}
	Set(ACTIVE_PROFILE_KEY, name)
	return nil
}

func GetProfile() Profile {
	if context != nil && !context.Profile.IsZero() {
		return context.Profile
	}

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

func ListProfiles() []Profile {
	profilesMap := getProfilePaths()
	results := make([]Profile, 0, len(profilesMap))
	for name, path := range profilesMap {
		results = append(results, Profile{Name: name, Path: path})
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

func ExtractProfile(name, path string) (Profile, error) {
	profiles, err := ExtractProfiles(path)
	if err != nil {
		return Profile{}, err
	}
	for _, profile := range profiles {
		if strings.EqualFold(profile.Name, name) {
			return profile, nil
		}
	}
	return Profile{}, fmt.Errorf("profile '%s' not found in %s", name, path)
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
	return ValidateSchema(path, PROFILE_SCHEMA_PATH)
}

func buildProfile(profile Profile) (Profile, error) {
	if profile.IsZero() {
		return Profile{}, fmt.Errorf("invalid profile: %v", profile)
	}
	if !sys.FileExists(profile.Path) {
		return Profile{}, fmt.Errorf("profile file not found at %s", profile.Path)
	}
	if err := ValidateProfile(profile.Path); err != nil {
		return Profile{}, fmt.Errorf("invalid profile: %w", err)
	}
	profile, err := ExtractProfile(profile.Name, profile.Path)
	if err != nil {
		return Profile{}, fmt.Errorf("invalid profile: %w", err)
	}
	return profile, nil
}
