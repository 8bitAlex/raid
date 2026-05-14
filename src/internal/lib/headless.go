package lib

import (
	"os"
	"strings"
)

// HeadlessEnvVar is the env var that toggles headless mode. The CLI's
// `-y` / `--yes` / `--headless` persistent flag sets this var via
// rootCmd's PersistentPreRunE so a single read site in lib serves both
// the flag-driven and env-driven entry points (the latter is how CI,
// scheduled runs, and agent hosts opt in without parsing flags).
const HeadlessEnvVar = "RAID_HEADLESS"

// headlessOverride lets tests force the headless flag deterministically
// without mutating os.Environ. nil means "fall through to the env var".
var headlessOverride *bool

// IsHeadless reports whether interactive prompts (Confirm / Prompt
// tasks) should be auto-resolved instead of blocking on stdin. True
// when RAID_HEADLESS is set to a truthy value ("1", "true", "yes",
// "y", "on" — case-insensitive). Anything else — including "0",
// "false", and the empty string — is treated as not-headless so a
// stray export doesn't silently change behavior.
func IsHeadless() bool {
	if headlessOverride != nil {
		return *headlessOverride
	}
	v := strings.TrimSpace(os.Getenv(HeadlessEnvVar))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

// SetHeadlessForTest forces the headless toggle for the duration of a
// test. Returns a restore function to defer. Use this in package-level
// tests that need deterministic headless behavior without racing on
// os.Environ.
func SetHeadlessForTest(v bool) func() {
	prev := headlessOverride
	headlessOverride = &v
	return func() { headlessOverride = prev }
}
