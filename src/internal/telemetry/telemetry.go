// Package telemetry implements raid's opt-in, anonymous CLI telemetry
// pipeline. Issue #80 spec'd the scope: measure adoption + usage, never
// capture user content, default off, easy to inspect and disable.
//
// Lifecycle:
//
//  1. raid invokes telemetry.Capture(name, props) at hook points
//     (ExecuteCommand, ExecuteTask, first-run prompt accepted, etc.).
//  2. If consent isn't on, Capture is a no-op. Same for missing API
//     key (dev builds) and DO_NOT_TRACK=1.
//  3. Otherwise the event is enriched with anonymous machine ID +
//     version/os/arch and posted asynchronously to PostHog. The HTTP
//     call never blocks the caller — failures drop silently so a
//     network blip can't break a raid command.
//  4. executeRoot calls Flush at the end of every invocation so
//     in-flight events finish (or get dropped on timeout).
//
// The package is read-only side-effect-free for users who never opt
// in via prompt or `raid telemetry on`: no goroutines spawned, no
// HTTP calls. Two narrow exceptions exist:
//   - MaybePromptForConsent persists a "decided=off" entry to viper
//     when the prompt is skipped (DO_NOT_TRACK, non-TTY, headless) so
//     we don't re-prompt on the next interactive run. No anonymous ID
//     or network traffic results.
//   - CaptureOptOutConsented, used only by the first-run follow-up
//     prompt when a declining user explicitly consents to record their
//     decision, creates an anonymous ID and sends exactly one
//     `raid_telemetry_opt_out` event before leaving telemetry off
//     forever. This is per-event consent, never state-level.
//
// Outside of those two paths, no anonymous ID is created or written
// until the user explicitly opts in.
package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// APIKey is the PostHog publishable project key. Empty by default;
// injected at release time via `-ldflags -X
// github.com/8bitalex/raid/src/internal/telemetry.APIKey=phc_...`.
// When empty, Capture is a no-op — that's how dev builds and `go run`
// stay silent.
var APIKey string

// CaptureEndpoint is the PostHog capture URL. Overridable for tests so
// they can point at a httptest.Server without hitting the real endpoint.
var CaptureEndpoint = "https://us.i.posthog.com/i/v0/e/"

// Event names. Stable contract — these are the labels that show up in
// PostHog and on the public telemetry-disclosure page.
const (
	EventFirstRun        = "raid_first_run"
	EventCommandExecuted = "raid_command_executed"
	EventCommandFailed   = "raid_command_failed"
	EventTaskExecuted    = "raid_task_executed"
	EventTelemetryOptOut = "raid_telemetry_opt_out"
)

// Event is what Capture queues for delivery. Properties must already
// be sanitized — telemetry doesn't re-scan them at send time.
type Event struct {
	Name       string         `json:"event"`
	Properties map[string]any `json:"properties"`
}

// httpClient is the package's outbound HTTP client. Short timeout so a
// hung network can't drag out raid shutdown; tests swap it for a
// recording transport.
var httpClient = &http.Client{Timeout: 2 * time.Second}

// inflight tracks fire-and-forget goroutines so Flush can wait on them
// at process exit. Without this, raid would exit before the POST
// returns and the event would be dropped.
var inflight sync.WaitGroup

// Capture is the public hook every event fires through. The network
// POST runs on a goroutine; Capture itself only blocks on
// loadOrCreateID(), which can touch the filesystem (home-dir
// resolution, mkdir, write) the first time an opted-in user fires an
// event. Subsequent calls hit the in-process cache and return without
// disk I/O. Capture is safe to call even when telemetry isn't active —
// it just no-ops.
//
// Callers must not pass sensitive content in properties. Sanitization
// is enforced upstream by the event builders, not here.
func Capture(name string, properties map[string]any) {
	if !IsActive() {
		return
	}
	id := loadOrCreateID()
	if id == "" {
		return
	}
	full := enrichProperties(id, properties)
	inflight.Add(1)
	go func() {
		defer inflight.Done()
		send(Event{Name: name, Properties: full})
	}()
}

// CaptureOptOutConsented fires the opt-out event under explicit
// per-event consent — used by the first-run prompt when a user
// declines telemetry generally but agrees to send a single
// anonymous "denial recorded" event. Bypasses the standard
// IsActive() gate (which would short-circuit because consent has
// just been set to off, or is still undecided) but still respects
// the two hard kill-switches: a build with no APIKey and a
// DO_NOT_TRACK env var. Synchronous so the event has its best
// chance to land before raid exits.
//
// Callers MUST ensure the user has explicitly consented to this
// specific event — never call this for any other purpose, and
// never extend it to a general "bypass" capture path. The whole
// trust model of telemetry rests on consent being explicit at the
// event level when state-level consent isn't true.
func CaptureOptOutConsented(reason string) {
	if APIKey == "" {
		return
	}
	if isDoNotTrack() {
		return
	}
	id := loadOrCreateID()
	if id == "" {
		return
	}
	send(Event{Name: EventTelemetryOptOut, Properties: enrichProperties(id, OptOutProps(reason))})
}

// CaptureSync is Capture's blocking variant. Used by `raid telemetry
// off` so the opt-out event is attempted synchronously before the
// process exits — we want to give the event the best chance to land
// rather than rely on Flush's timeout. This is best-effort: send()
// silently drops network and non-2xx errors, so delivery isn't
// guaranteed, just synchronously attempted.
func CaptureSync(name string, properties map[string]any) {
	if !IsActive() {
		return
	}
	id := loadOrCreateID()
	if id == "" {
		return
	}
	send(Event{Name: name, Properties: enrichProperties(id, properties)})
}

// Flush waits up to timeout for in-flight events to finish. Called at
// the end of executeRoot so async events sent during a command run
// don't get dropped when raid exits.
func Flush(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// IsActive reports whether Capture will actually send. Combines the
// build-time API key, the consent state, and DO_NOT_TRACK.
//
// This is the gate every telemetry path checks before doing real
// work — keep the test surface narrow by going through here, not by
// reading individual flags.
func IsActive() bool {
	if APIKey == "" {
		return false
	}
	if isDoNotTrack() {
		return false
	}
	st := LoadState()
	return st.Decided && st.Enabled
}

// enrichProperties merges the common envelope (anonymous ID, raid
// version, os/arch) into the event's specific properties. PostHog
// expects $properties.distinct_id at the top level.
func enrichProperties(id string, properties map[string]any) map[string]any {
	out := make(map[string]any, len(properties)+6)
	for k, v := range properties {
		out[k] = v
	}
	out["distinct_id"] = id
	out["raid_version"] = raidVersion()
	out["os"] = runtime.GOOS
	out["arch"] = runtime.GOARCH
	return out
}

// raidVersionFn is the source of the raid version baked into every
// event. Override in tests to avoid pulling the resources package.
var raidVersionFn = func() string {
	return raidVersionFromResources()
}

func raidVersion() string {
	return raidVersionFn()
}

// send posts a single event to the PostHog capture endpoint. Errors
// are intentionally swallowed: telemetry must never break raid, so
// any network / encoding failure is a silent drop.
func send(evt Event) {
	body, err := json.Marshal(map[string]any{
		"api_key":    APIKey,
		"event":      evt.Name,
		"properties": evt.Properties,
		"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, CaptureEndpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// PreviewPayload returns the JSON body that would be sent for the
// given event, without sending it. Used by `raid telemetry preview`
// so users can see exactly what raid emits before opting in.
//
// Pretty-prints the JSON for human inspection. When no anonymous ID
// exists yet (the user hasn't opted in and we don't want to create
// the id file just for a preview), a placeholder string is shown in
// the distinct_id field so the preview still renders the full payload
// shape. Returns an empty string only on JSON marshaling failure.
func PreviewPayload(name string, properties map[string]any) string {
	id := loadIDIfExists()
	if id == "" {
		id = "<no-id-yet — generated on first opt-in>"
	}
	body, err := json.MarshalIndent(map[string]any{
		"api_key":    apiKeyForPreview(),
		"event":      name,
		"properties": enrichProperties(id, properties),
		"timestamp":  "<RFC3339 timestamp at send time>",
	}, "", "  ")
	if err != nil {
		return ""
	}
	return string(body)
}

// apiKeyForPreview masks the real key in preview output so a user
// running `raid telemetry preview` doesn't accidentally copy/paste
// the key when sharing the payload.
func apiKeyForPreview() string {
	if APIKey == "" {
		return "<not configured in this build>"
	}
	if len(APIKey) <= 8 {
		return "<redacted>"
	}
	return APIKey[:4] + "…" + APIKey[len(APIKey)-4:]
}
