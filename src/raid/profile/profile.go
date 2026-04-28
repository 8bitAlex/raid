// Manage raid profiles.
package profile

import (
	"bufio"

	"github.com/8bitalex/raid/src/internal/lib"
)

type Profile = lib.Profile
type ProfileDraft = lib.ProfileDraft
type RepoDraft = lib.RepoDraft
type Repo = lib.Repo

// Describe parses the raid.yaml at path and returns the resulting Repo
// (environments, install steps, commands, etc.) without merging it into the
// active profile. Used by the MCP `raid_describe_repo` tool.
func Describe(path string) (Repo, error) {
	return lib.ExtractRepo(path)
}

// Returns the active profile
func Get() Profile {
	return lib.GetProfile()
}

// Returns a slice of all added profiles
func ListAll() []Profile {
	return lib.ListProfiles()
}

// Adds a profile to the available profile list
func Add(profile Profile) error {
	return lib.AddProfile(profile)
}

// Adds multiple profiles to the profile list
func AddAll(profiles []Profile) error {
	return lib.AddProfiles(profiles)
}

// Sets the active profile
func Set(name string) error {
	return lib.SetProfile(name)
}

// Removes a profile from the profile list
func Remove(name string) error {
	return lib.RemoveProfile(name)
}

// Extracts profiles from a file
func Unmarshal(path string) ([]Profile, error) {
	return lib.ExtractProfiles(path)
}

// Validates a profile file against the JSON schema
func Validate(path string) error {
	return lib.ValidateProfile(path)
}

// Checks if a profile exists
func Contains(name string) bool {
	return lib.ContainsProfile(name)
}

// WriteFile serializes draft to path as a YAML profile file.
func WriteFile(draft ProfileDraft, path string) error {
	return lib.WriteProfileFile(draft, path)
}

// CollectRepos runs an interactive prompt loop to collect repository details from reader.
func CollectRepos(reader *bufio.Reader) []RepoDraft {
	return lib.CollectRepos(reader)
}

// CreateRepoConfigs writes a raid.yaml stub into each repository's local directory.
func CreateRepoConfigs(repos []RepoDraft) {
	lib.CreateRepoConfigs(repos)
}
