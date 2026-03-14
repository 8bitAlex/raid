// Manage raid profiles.
package profile

import "github.com/8bitalex/raid/src/internal/lib"

type Profile = lib.Profile

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
