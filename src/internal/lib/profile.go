package lib

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/resources"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	activeProfileKey = "profile"
	allProfilesKey   = "profiles"
	profileSchemaID  = "https://raidcli.dev/schema/v1/raid-profile.schema.json"
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
	Verify       []Verify          `json:"verify,omitempty"`
}

// IsZero reports whether the profile is uninitialized.
func (p Profile) IsZero() bool {
	return p.Name == "" || p.Path == ""
}

// IsSingleRepo reports whether the profile is backed by a raid.yaml (repo
// config) rather than a profile YAML. Detected by the registered path's
// basename: when `raid profile add ./raid.yaml` records a path pointing at
// a repo config, raid synthesizes a single-repo profile around it instead
// of requiring a wrapping profile file. The buildProfile / doctor paths
// branch on this check to switch schemas.
func (p Profile) IsSingleRepo() bool {
	return filepath.Base(p.Path) == RaidConfigFileName
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
		return liberrs.ProfileNotFound(name)
	}
	return Set(activeProfileKey, name)
}

// GetProfile returns the currently active profile.
func GetProfile() Profile {
	if ctx := loadContext(); ctx != nil && !ctx.Profile.IsZero() {
		return ctx.Profile
	}

	name := viper.GetString(activeProfileKey)
	paths := getProfilePaths()
	// Lookup is case-insensitive: viper.GetStringMapString lowercases
	// keys, so a profile registered as `MyProfile` lives in `paths`
	// under `myprofile`. Without the fold, GetProfile().Path returns
	// "" right after a successful add.
	return Profile{
		Name: name,
		Path: paths[strings.ToLower(name)],
	}
}

// AddProfile registers a profile in the config store.
func AddProfile(profile Profile) error {
	profiles := viper.GetStringMapString(allProfilesKey)
	profiles[profile.Name] = profile.Path
	return Set(allProfilesKey, profiles)
}

// AddProfiles registers multiple profiles in the config store. Because
// viper lowercases registry keys, two profiles in the same batch whose
// names differ only by case silently collapse to one registration —
// last one wins. That collision is surfaced as a warning (mirroring the
// setRepoVars sanitized-name warning) so the loss isn't invisible.
func AddProfiles(profiles []Profile) error {
	seen := make(map[string]string, len(profiles))
	for _, profile := range profiles {
		key := strings.ToLower(profile.Name)
		if prev, collided := seen[key]; collided && prev != profile.Name {
			fmt.Fprintf(os.Stderr,
				"raid: warning: profiles %q and %q differ only by case and share one registration; %q wins\n",
				prev, profile.Name, profile.Name)
		}
		seen[key] = profile.Name
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
	return viper.GetStringMapString(allProfilesKey)
}

// RemoveProfile removes a registered profile by name. Case-insensitive
// lookup mirrors ContainsProfile — viper lowercases keys internally,
// so a name typed as `MyProfile` is stored as `myprofile` and must be
// looked up the same way.
func RemoveProfile(name string) error {
	profiles := viper.GetStringMapString(allProfilesKey)
	key := strings.ToLower(name)
	if _, exists := profiles[key]; !exists {
		return liberrs.ProfileNotFound(name)
	}
	delete(profiles, key)
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
	return Profile{}, liberrs.Newf(liberrs.CodeProfileNotFound, liberrs.CategoryNotFound, "profile '%s' not found in %s", name, path)
}

// ExtractProfiles reads all profiles from a YAML or JSON file.
func ExtractProfiles(path string) ([]Profile, error) {
	profileData, err := os.ReadFile(path)
	if err != nil {
		return nil, liberrs.ProfileFileRead(path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var profiles []Profile

	switch ext {
	case ".yaml", ".yml":
		profiles, err = extractProfilesFromYAML(profileData, path)
	case ".json":
		profiles, err = extractProfilesFromJSON(profileData, path)
	default:
		return nil, liberrs.Newf(liberrs.CodeProfileInvalid, liberrs.CategoryConfig, "unsupported file format: %s. Supported formats are .yaml, .yml, and .json", ext)
	}

	if err != nil {
		return nil, err
	}

	if len(profiles) == 0 {
		return nil, liberrs.Newf(liberrs.CodeProfileNotFound, liberrs.CategoryNotFound, "no profiles found in file %s", path)
	}

	return profiles, nil
}

func extractProfilesFromYAML(data []byte, path string) ([]Profile, error) {
	// yaml.NewDecoder respects the multi-document stream marker
	// without doing a string-level split. The earlier strings.Split
	// on "---" corrupted legitimate content that happened to contain
	// the substring (e.g. `usage: "---- separator ----"` inside a
	// command description, or a commit-message-style usage field).
	// ValidateProfile already uses NewDecoder on its side, so the
	// validation path and the extraction path can't disagree anymore.
	var profiles []Profile
	dec := yaml.NewDecoder(bytes.NewReader(data))
	for {
		var profile Profile
		err := dec.Decode(&profile)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, liberrs.Newf(liberrs.CodeProfileInvalid, liberrs.CategoryConfig, "invalid YAML document in %s: %v", path, err)
		}
		// Skip explicitly-empty documents (e.g. trailing `---\n` with
		// no fields). Detected by an empty Name — Path hasn't been
		// stamped yet, so Profile.IsZero (which requires non-empty
		// Path) is too strict here.
		if profile.Name == "" {
			continue
		}
		profile.Path = path
		profiles = append(profiles, profile)
	}

	return profiles, nil
}

func extractProfilesFromJSON(data []byte, path string) ([]Profile, error) {
	// Only accept the single-object form when it produced a named
	// profile. A top-level `null` (or an object missing `name`)
	// unmarshals "successfully" into a zero Profile, which would slip
	// past the "no profiles found" guard and surface later as a
	// confusing not-found error instead of a validation error here.
	// A nameless object/null returns zero profiles (not the array-parse
	// error) so ExtractProfiles reports "no profiles found".
	var profile Profile
	if err := json.Unmarshal(data, &profile); err == nil {
		if profile.Name == "" {
			return nil, nil
		}
		profile.Path = path
		return []Profile{profile}, nil
	}

	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, liberrs.Newf(liberrs.CodeProfileInvalid, liberrs.CategoryConfig, "invalid JSON format in %s: %v", path, err)
	}

	results := make([]Profile, 0, len(profiles))
	for _, p := range profiles {
		// Skip unnamed entries, mirroring the YAML path's handling of
		// explicitly-empty documents.
		if p.Name == "" {
			continue
		}
		p.Path = path
		results = append(results, p)
	}

	return results, nil
}

// ContainsProfile reports whether a profile with the given name is registered.
// Case-insensitive: viper's GetStringMapString lowercases keys at storage
// time, so a name typed as `MyProfile` is stored under `myprofile`. Without
// the lowercase comparison here, `raid profile MyProfile` would fail
// even though `MyProfile` was just registered.
func ContainsProfile(name string) bool {
	_, exists := viper.GetStringMapString(allProfilesKey)[strings.ToLower(name)]
	return exists
}

// ValidateProfile validates the profile file at path against the profile JSON schema.
func ValidateProfile(path string) error {
	return validateWithEmbeddedSchema(path, profileSchemaID)
}

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
		return liberrs.Newf(liberrs.CodeProfileFileRead, liberrs.CategoryConfig, "serializing profile: %v", err)
	}
	content := resources.ProfileTemplate() + "\n" + string(data)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return liberrs.Newf(liberrs.CodeProfileFileRead, liberrs.CategoryConfig, "creating directory: %v", err)
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
		url := sys.ReadLine(reader, "  URL (leave empty for local-only): ")
		path := sys.ReadLine(reader, "  Local path: ")
		var defaultBranch string
		if url != "" {
			defaultBranch = sys.DetectGitDefaultBranch(url)
		}
		branchPrompt := "  Default branch: "
		if defaultBranch != "" {
			branchPrompt = fmt.Sprintf("  Default branch [%s]: ", defaultBranch)
		}
		branch := sys.ReadLine(reader, branchPrompt)
		if branch == "" {
			branch = defaultBranch
		}
		repo := RepoDraft{Name: name, URL: url, Path: path, Branch: branch}
		if repo.Name == "" || repo.Path == "" {
			fmt.Println("  Name and path are required. Skipping.")
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
		content := resources.RepoTemplate() + "\nname: " + repo.Name + "\n"
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
		return Profile{}, liberrs.Newf(liberrs.CodeProfileInvalid, liberrs.CategoryConfig, "invalid profile: %v", profile)
	}
	if !sys.FileExists(profile.Path) {
		return Profile{}, liberrs.ProfileFileMissing(profile.Path)
	}
	if profile.IsSingleRepo() {
		built, err := BuildSingleRepoProfile(profile.Path)
		if err != nil {
			return Profile{}, err
		}
		// The registered profile name is the lookup key used by
		// `raid profile <name>` and active-profile detection. If the
		// raid.yaml's name field has drifted since registration, refuse
		// to load rather than silently returning a profile whose Name
		// differs from the registered key. Case-insensitive to match
		// registration and activation: viper lowercases stored keys, so
		// a raid.yaml declaring `MyRepo` is registered (and activated)
		// as `myrepo` — that's not drift.
		if profile.Name != "" && !strings.EqualFold(built.Name, profile.Name) {
			return Profile{}, liberrs.Newf(liberrs.CodeProfileInvalid, liberrs.CategoryConfig,
				"raid.yaml at %s now declares name %q but was registered as %q; re-run `raid profile add %s` to update",
				profile.Path, built.Name, profile.Name, profile.Path,
			)
		}
		return built, nil
	}
	if err := ValidateProfile(profile.Path); err != nil {
		return Profile{}, liberrs.ProfileInvalid(profile.Path, err)
	}
	profile, err := ExtractProfile(profile.Name, profile.Path)
	if err != nil {
		return Profile{}, liberrs.ProfileInvalid(profile.Path, err)
	}
	return profile, nil
}

// BuildSingleRepoProfile validates the raid.yaml at path and returns a
// synthetic profile whose only repository points at the raid.yaml's
// directory. The profile's Path is the raid.yaml itself, so IsSingleRepo
// reports true on subsequent loads. The repository's full configuration
// (commands, environments, install tasks) is merged in by buildRepo later
// in the load pipeline.
func BuildSingleRepoProfile(path string) (Profile, error) {
	if filepath.Base(path) != RaidConfigFileName {
		return Profile{}, liberrs.Newf(liberrs.CodeRepoInvalid, liberrs.CategoryConfig, "single-repo profile path must end in %s, got %s", RaidConfigFileName, path)
	}
	if err := ValidateRepo(path); err != nil {
		return Profile{}, liberrs.Newf(liberrs.CodeRepoInvalid, liberrs.CategoryConfig, "invalid raid.yaml: %v", err)
	}
	repoDir := filepath.Dir(path)
	repo, err := ExtractRepo(repoDir)
	if err != nil {
		return Profile{}, err
	}
	if repo.Name == "" {
		return Profile{}, liberrs.Newf(liberrs.CodeRepoInvalid, liberrs.CategoryConfig, "raid.yaml at %s has missing or empty name field", path)
	}
	return Profile{
		Name: repo.Name,
		Path: path,
		Repositories: []Repo{{
			Name:   repo.Name,
			Path:   repoDir,
			Branch: repo.Branch,
		}},
	}, nil
}
