package telemetry

import "github.com/8bitalex/raid/src/resources"

// raidVersionFromResources pulls the version from the embedded
// app.properties so every event reports the binary's actual version.
// On lookup failure this returns the empty string; enrichProperties
// still writes it as `raid_version=""` rather than omitting the
// field — events always send, even if the version label is blank.
// Telemetry never blocks a command on a bad lookup.
func raidVersionFromResources() string {
	v, err := resources.GetProperty(resources.PropertyVersion)
	if err != nil {
		return ""
	}
	return v
}
