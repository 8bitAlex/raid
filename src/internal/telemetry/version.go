package telemetry

import "github.com/8bitalex/raid/src/resources"

// raidVersionFromResources pulls the version from the embedded
// app.properties so every event reports the binary's actual version.
// Failures fall back to empty string — events still send, the
// `raid_version` field is just absent. Telemetry never blocks a
// command on a bad lookup.
func raidVersionFromResources() string {
	v, err := resources.GetProperty(resources.PropertyVersion)
	if err != nil {
		return ""
	}
	return v
}
