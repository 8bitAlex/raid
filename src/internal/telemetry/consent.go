package telemetry

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Viper config keys. `decided` is the consent "shown the prompt and
// got an answer" marker. `enabled` is the actual on/off state. We
// keep them separate so the first-run prompt fires exactly once,
// even when the user opts out.
const (
	consentDecidedKey = "telemetry.decided"
	consentEnabledKey = "telemetry.enabled"
)

// DoNotTrackEnvVar is the standard cross-tool opt-out env var that
// raid honors as a hard off. See https://consoledonottrack.com/.
const DoNotTrackEnvVar = "DO_NOT_TRACK"

// State is the user-facing consent snapshot read by `raid telemetry
// status` and by IsActive. Decided distinguishes "user has been asked
// and chose off" from "user hasn't been asked yet" — the first-run
// prompt only fires when Decided is false.
type State struct {
	Decided bool
	Enabled bool
}

// LoadState reads consent from viper. Defaults: Decided=false,
// Enabled=false. Either default is safe — a fresh install or a config
// without these keys yields off until the user opts in.
func LoadState() State {
	return State{
		Decided: viper.GetBool(consentDecidedKey),
		Enabled: viper.GetBool(consentEnabledKey),
	}
}

// SetEnabled persists the user's consent choice. Always sets Decided
// so we don't re-prompt — a user who answered no should stay
// not-prompted until they explicitly run `raid telemetry on`.
func SetEnabled(enabled bool) error {
	viper.Set(consentDecidedKey, true)
	viper.Set(consentEnabledKey, enabled)
	return viper.WriteConfig()
}

// SetDecidedOff marks the user as having declined without ever being
// prompted. Used in non-interactive contexts (no TTY, --yes/--headless,
// DO_NOT_TRACK=1) so we don't keep trying to prompt later. Behaves
// identically to SetEnabled(false) but documents intent at call site.
func SetDecidedOff() error {
	return SetEnabled(false)
}

// DoNotTrackActive is the public surface for callers that need to
// surface the DO_NOT_TRACK state (e.g. `raid telemetry status`).
// Mirrors the internal check exactly so the printed status matches
// what IsActive() actually enforces.
func DoNotTrackActive() bool {
	return isDoNotTrack()
}

// HasAPIKey reports whether this binary was built with a PostHog API
// key injected. Used by status to tell users that a dev build will
// never emit events even when consent is on.
func HasAPIKey() bool {
	return APIKey != ""
}

// isDoNotTrack reports whether DO_NOT_TRACK is set to a truthy value.
// Honored as a hard off regardless of the persisted consent state —
// matches the published cross-tool contract.
func isDoNotTrack() bool {
	v := strings.TrimSpace(os.Getenv(DoNotTrackEnvVar))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}
