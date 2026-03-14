package lib

import (
	"bufio"
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
	activeProfileKey  = "profile"
	allProfilesKey    = "profiles"
	profileSchemaPath = "raid-profile.schema.json"
)

// Profile represents a named collection of repositories, environments, and task groups.
type Profile struct {
	Name         string            `json:"name"`
	Path         string            `json:"path"`
	Repositories []Repo            `json:"repositories"`
	Environments []Env             `json:"environments"`
	Install      OnInstall         `json:"install"`
	Groups       map[string][]Task `json:"task_groups" yaml:"task_groups"`
	Commands     []Command         `json:"commands"`
}

// IsZero reports whether the profile is uninitialized.
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

// SetProfile sets the named profile as the active profile.
func SetProfile(name string) error {
	if !ContainsProfile(name) {
		return fmt.Errorf("profile '%s' not found", name)
	}
	return Set(activeProfileKey, name)
}

// GetProfile returns the currently active profile.
func GetProfile() Profile {
	if context != nil && !context.Profile.IsZero() {
		return context.Profile
	}

	name := viper.GetString(activeProfileKey)
	paths := getProfilePaths()
	return Profile{
		Name: name,
		Path: paths[name],
	}
}

// AddProfile registers a profile in the config store.
func AddProfile(profile Profile) error {
	profiles := viper.GetStringMapString(allProfilesKey)
	if profiles == nil {
		profiles = make(map[string]string)
	}
	profiles[profile.Name] = profile.Path
	return Set(allProfilesKey, profiles)
}

// AddProfiles registers multiple profiles in the config store.
func AddProfiles(profiles []Profile) error {
	for _, profile := range profiles {
		if err := AddProfile(profile); err != nil {
			return err
		}
	}
	return nil
}

// ListProfiles returns all registered profiles.
func ListProfiles() []Profile {
	profilesMap := getProfilePaths()
	results := make([]Profile, 0, len(profilesMap))
	for name, path := range profilesMap {
		results = append(results, Profile{Name: name, Path: path})
	}
	return results
}

func getProfilePaths() map[string]string {
	profiles := viper.GetStringMapString(allProfilesKey)
	if profiles == nil {
		return make(map[string]string)
	}
	return profiles
}

// RemoveProfile removes a registered profile by name.
func RemoveProfile(name string) error {
	profiles := viper.GetStringMapString(allProfilesKey)
	if profiles == nil {
		return fmt.Errorf("no profiles found")
	}
	if _, exists := profiles[name]; !exists {
		return fmt.Errorf("profile '%s' not found", name)
	}
	delete(profiles, name)
	return Set(allProfilesKey, profiles)
}

// ExtractProfile reads and returns a single named profile from the given file.
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

// ExtractProfiles reads all profiles from a YAML or JSON file.
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

	documents := strings.Split(string(data), yamlSep)

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
	var profile Profile
	if err := json.Unmarshal(data, &profile); err == nil {
		profile.Path = path
		return []Profile{profile}, nil
	}

	var profiles []Profile
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

// ContainsProfile reports whether a profile with the given name is registered.
func ContainsProfile(name string) bool {
	profiles := viper.GetStringMapString(allProfilesKey)
	if profiles == nil {
		return false
	}

	_, exists := profiles[name]
	return exists
}

// ValidateProfile validates the profile file at path against the profile JSON schema.
func ValidateProfile(path string) error {
	return validateWithEmbeddedSchema(path, profileSchemaPath)
}

const (
	profileSchemaURL = "https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-profile.schema.json"
	repoSchemaURL    = "https://raw.githubusercontent.com/8bitalex/raid/main/schemas/raid-repo.schema.json"
)

// ProfileDraft is the minimal structure written to a new profile file.
type ProfileDraft struct {
	Name         string      `yaml:"name"`
	Repositories []RepoDraft `yaml:"repositories,omitempty"`
}

// RepoDraft holds the fields collected for each repository during profile creation.
type RepoDraft struct {
	Name   string `yaml:"name"`
	Path   string `yaml:"path"`
	URL    string `yaml:"url"`
	Branch string `yaml:"branch,omitempty"`
}

// WriteProfileFile serializes draft as YAML and writes it to path, creating parent directories as needed.
func WriteProfileFile(draft ProfileDraft, path string) error {
	data, err := yaml.Marshal(draft)
	if err != nil {
		return fmt.Errorf("serializing profile: %w", err)
	}
	content := "# yaml-language-server: $schema=" + profileSchemaURL + "\n\n" + string(data)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// CollectRepos runs an interactive prompt loop to collect repository details from reader.
func CollectRepos(reader *bufio.Reader) []RepoDraft {
	var repos []RepoDraft
	for {
		fmt.Println()
		if !sys.ReadYesNo(reader, "Add a repository? [y/N]: ") {
			break
		}
		name := sys.ReadLine(reader, "  Name: ")
		url := sys.ReadLine(reader, "  URL: ")
		path := sys.ReadLine(reader, "  Local path: ")
		defaultBranch := sys.DetectGitDefaultBranch(url)
		branchPrompt := "  Default branch: "
		if defaultBranch != "" {
			branchPrompt = fmt.Sprintf("  Default branch [%s]: ", defaultBranch)
		}
		branch := sys.ReadLine(reader, branchPrompt)
		if branch == "" {
			branch = defaultBranch
		}
		repo := RepoDraft{Name: name, URL: url, Path: path, Branch: branch}
		if repo.Name == "" || repo.URL == "" || repo.Path == "" {
			fmt.Println("  Name, URL, and path are all required. Skipping.")
			continue
		}
		repos = append(repos, repo)
	}
	return repos
}

// CreateRepoConfigs writes a raid.yaml stub into each repository's local directory.
func CreateRepoConfigs(repos []RepoDraft) {
	for _, repo := range repos {
		repoPath := sys.ExpandPath(repo.Path)
		if err := os.MkdirAll(repoPath, 0755); err != nil {
			fmt.Printf("  Failed to create directory for '%s': %v\n", repo.Name, err)
			continue
		}
		configPath := filepath.Join(repoPath, "raid.yaml")
		if sys.FileExists(configPath) {
			fmt.Printf("  raid.yaml already exists at %s, skipping.\n", configPath)
			continue
		}
		content := "# yaml-language-server: $schema=" + repoSchemaURL + "\n\nname: " + repo.Name + "\n"
		if repo.Branch != "" {
			content += "branch: " + repo.Branch + "\n"
		}
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			fmt.Printf("  Failed to write repo config for '%s': %v\n", repo.Name, err)
			continue
		}
		fmt.Printf("  Created %s\n", configPath)
	}
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
