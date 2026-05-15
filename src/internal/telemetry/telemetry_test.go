package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// setupTestEnv hermetically isolates each test from on-disk state and
// global config. Resets viper, points the ID file at a tempdir,
// resets the in-process ID cache, restores APIKey + endpoint +
// sample rate + DO_NOT_TRACK to their defaults on cleanup. Tests
// should call this first so they don't leak state.
func setupTestEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	idPath := filepath.Join(dir, "telemetry-id")

	// Reset viper to an isolated config in the tempdir.
	prevConfig := viper.ConfigFileUsed()
	viper.Reset()
	viper.SetConfigFile(filepath.Join(dir, "config.toml"))
	if f, err := os.Create(filepath.Join(dir, "config.toml")); err == nil {
		f.Close()
	}

	// Capture and restore env / package vars.
	prevID := os.Getenv(IDFileEnv)
	prevDNT := os.Getenv(DoNotTrackEnvVar)
	prevAPIKey := APIKey
	prevEndpoint := CaptureEndpoint
	prevRate := TaskSampleRate
	prevRng := rngFn
	prevHomeDirFn := homeDirFn
	prevIsInteractive := isInteractiveFn
	prevPromptIn := promptInFn
	prevPromptOut := promptOutFn

	os.Setenv(IDFileEnv, idPath)
	os.Unsetenv(DoNotTrackEnvVar)
	resetIDCacheForTest()
	APIKey = "phc_test"
	CaptureEndpoint = "" // tests must set this before Capture if they expect sends
	isInteractiveFn = func() bool { return false }

	t.Cleanup(func() {
		os.Setenv(IDFileEnv, prevID)
		if prevDNT == "" {
			os.Unsetenv(DoNotTrackEnvVar)
		} else {
			os.Setenv(DoNotTrackEnvVar, prevDNT)
		}
		APIKey = prevAPIKey
		CaptureEndpoint = prevEndpoint
		TaskSampleRate = prevRate
		rngFn = prevRng
		homeDirFn = prevHomeDirFn
		isInteractiveFn = prevIsInteractive
		promptInFn = prevPromptIn
		promptOutFn = prevPromptOut
		resetIDCacheForTest()
		viper.Reset()
		_ = prevConfig
	})
}

// --- IsActive ---

func TestIsActive_offByDefault(t *testing.T) {
	setupTestEnv(t)
	if IsActive() {
		t.Error("IsActive should be false on a fresh config")
	}
}

func TestIsActive_offWhenAPIKeyEmpty(t *testing.T) {
	setupTestEnv(t)
	APIKey = ""
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if IsActive() {
		t.Error("IsActive should be false when APIKey is empty regardless of consent")
	}
}

func TestIsActive_offWhenDoNotTrack(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	os.Setenv(DoNotTrackEnvVar, "1")
	if IsActive() {
		t.Error("IsActive should be false when DO_NOT_TRACK=1")
	}
}

func TestIsActive_onWhenAllConditionsMet(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if !IsActive() {
		t.Error("IsActive should be true with API key + decided + enabled + no DO_NOT_TRACK")
	}
}

// --- Capture: no-network when opted out ---

func TestCapture_optedOutMakesZeroNetworkCalls(t *testing.T) {
	setupTestEnv(t)
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	// Default state: undecided, disabled — Capture must no-op.
	Capture(EventCommandExecuted, CommandExecutedProps("test", 1, []string{"shell"}, 10))
	Capture(EventTaskExecuted, TaskExecutedProps("shell", 5, true))
	Flush(500 * time.Millisecond)

	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("opted-out Capture made %d network calls, want 0", got)
	}
}

func TestCapture_doNotTrackMakesZeroNetworkCalls(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	os.Setenv(DoNotTrackEnvVar, "1")

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	Capture(EventCommandExecuted, CommandExecutedProps("test", 1, []string{"shell"}, 10))
	Flush(500 * time.Millisecond)

	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("DO_NOT_TRACK Capture made %d network calls, want 0", got)
	}
}

// --- Capture: sends + sanitization ---

func TestCapture_sendsWhenActive(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}

	var (
		hits  int32
		body  []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		body, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	Capture(EventCommandExecuted, CommandExecutedProps("build", 2, []string{"shell", "print"}, 1234))
	Flush(2 * time.Second)

	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("Capture sent %d events, want 1", got)
	}
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if got, _ := parsed["event"].(string); got != EventCommandExecuted {
		t.Errorf("event = %q, want %q", got, EventCommandExecuted)
	}
	props, _ := parsed["properties"].(map[string]any)
	if props == nil {
		t.Fatal("properties missing")
	}
	for _, key := range []string{"distinct_id", "raid_version", "os", "arch", "command_name", "task_count", "task_types", "duration_ms"} {
		if _, ok := props[key]; !ok {
			t.Errorf("properties missing %q: %v", key, props)
		}
	}
}

// --- Event builders: no user content leaks ---

func TestEventBuilders_neverLeakUserContent(t *testing.T) {
	// Each builder must produce a property map that doesn't contain
	// the forbidden substrings — even if a future field accidentally
	// captures one. Builders take the call-site values; the sanitizer
	// is "don't put them in the map" rather than "scrub them out", so
	// a typo in the builder body would surface here.
	forbidden := []string{
		"rm -rf /",        // example cmd body
		"/Users/secret",   // example path
		"SECRET_TOKEN",    // example env name
		"sk-live-",        // example secret prefix
	}
	cases := []struct {
		name  string
		props map[string]any
	}{
		{"CommandExecuted", CommandExecutedProps("rm -rf /", 1, []string{"/Users/secret"}, 1)},
		{"CommandFailed", CommandFailedProps("rm -rf /", "SECRET_TOKEN", 1)},
		{"TaskExecuted", TaskExecutedProps("sk-live-shell", 1, false)},
		{"FirstRun", FirstRunProps("rm -rf /")},
		{"OptOut", OptOutProps("/Users/secret")},
	}
	// The above intentionally feeds the builders forbidden values
	// in their permitted slots (command_name, task_type, etc.) to
	// confirm we treat those as labels, not content. The assertion
	// flips: command_name + task_type are *expected* to round-trip
	// (they're project-author labels), but no extra fields should
	// appear that contain the forbidden substrings outside those
	// permitted slots.
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			permitted := map[string]bool{
				"command_name":   true,
				"task_type":      true,
				"task_types":     true,
				"error_code":     true,
				"install_method": true,
				"reason":         true,
			}
			for k, v := range c.props {
				if permitted[k] {
					continue
				}
				s, ok := v.(string)
				if !ok {
					continue
				}
				for _, f := range forbidden {
					if strings.Contains(s, f) {
						t.Errorf("property %q contains forbidden substring %q: %q", k, f, s)
					}
				}
			}
		})
	}
}

// --- Anonymous ID ---

func TestLoadOrCreateID_persistsAcrossCalls(t *testing.T) {
	setupTestEnv(t)
	first := loadOrCreateID()
	if first == "" {
		t.Fatal("first call returned empty ID")
	}
	resetIDCacheForTest()
	second := loadOrCreateID()
	if first != second {
		t.Errorf("ID changed across calls: %q vs %q", first, second)
	}
}

func TestLoadOrCreateID_formatIsUUIDv4(t *testing.T) {
	setupTestEnv(t)
	id := loadOrCreateID()
	if len(id) != 36 {
		t.Fatalf("ID length = %d, want 36 (UUID): %q", len(id), id)
	}
	// Version 4 nibble at index 14.
	if id[14] != '4' {
		t.Errorf("version nibble = %c, want 4: %s", id[14], id)
	}
	// Variant bits 10x at index 19 → first char of group 4 is 8/9/a/b.
	switch id[19] {
	case '8', '9', 'a', 'b', 'A', 'B':
	default:
		t.Errorf("variant nibble = %c, want one of 8/9/a/b: %s", id[19], id)
	}
}

func TestPurgeID_removesFile(t *testing.T) {
	setupTestEnv(t)
	id := loadOrCreateID()
	if id == "" {
		t.Fatal("ID not created")
	}
	if _, err := os.Stat(IDPath()); err != nil {
		t.Fatalf("ID file should exist: %v", err)
	}
	if err := PurgeID(); err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if _, err := os.Stat(IDPath()); !os.IsNotExist(err) {
		t.Errorf("ID file should be gone: %v", err)
	}
}

func TestPurgeID_idempotent(t *testing.T) {
	setupTestEnv(t)
	if err := PurgeID(); err != nil {
		t.Errorf("first Purge of non-existent file should not error: %v", err)
	}
	if err := PurgeID(); err != nil {
		t.Errorf("second Purge should be idempotent: %v", err)
	}
}

func TestLoadOrCreateID_reusesExistingFile(t *testing.T) {
	// Simulates a concurrent-process race: another raid invocation
	// already wrote an ID before this one calls loadOrCreateID. The
	// O_CREATE|O_EXCL path must observe the existing file and adopt
	// the same value rather than overwriting with a fresh UUID.
	setupTestEnv(t)
	path := IDPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	preset := "11111111-2222-4333-8444-555555555555"
	if err := os.WriteFile(path, []byte(preset+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got := loadOrCreateID()
	if got != preset {
		t.Errorf("loadOrCreateID = %q, want preset %q", got, preset)
	}
}

func TestLoadOrCreateID_emptyOnHomeDirError(t *testing.T) {
	setupTestEnv(t)
	// Force the home-dir resolver to fail AND clear the env override
	// so IDPath returns "". loadOrCreateID must fail closed (empty
	// string) rather than panic or write somewhere unexpected.
	os.Unsetenv(IDFileEnv)
	homeDirFn = func() (string, error) { return "", os.ErrPermission }
	if got := loadOrCreateID(); got != "" {
		t.Errorf("loadOrCreateID = %q, want \"\" on home-dir failure", got)
	}
}

func TestLoadIDIfExists_returnsEmptyWhenFileMissing(t *testing.T) {
	setupTestEnv(t)
	if got := loadIDIfExists(); got != "" {
		t.Errorf("loadIDIfExists on fresh env = %q, want empty", got)
	}
}

// --- CaptureSync ---

func TestCaptureSync_sendsImmediately(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	CaptureSync(EventTelemetryOptOut, OptOutProps("test"))
	// No Flush — CaptureSync blocks on the request itself.
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("CaptureSync delivered %d events, want 1", got)
	}
}

func TestCaptureSync_noopWhenInactive(t *testing.T) {
	setupTestEnv(t)
	// IsActive is false (no SetEnabled). CaptureSync must short-circuit.
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL
	CaptureSync(EventTelemetryOptOut, OptOutProps("ignored"))
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("inactive CaptureSync sent %d events, want 0", got)
	}
}

// --- send: silent on errors ---

func TestSend_swallowsNetworkErrors(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	// Point at a closed server — Dial will fail synchronously, but
	// send() must absorb the error without panicking and Flush must
	// still return cleanly.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	closedURL := srv.URL
	srv.Close()
	CaptureEndpoint = closedURL
	Capture(EventCommandExecuted, CommandExecutedProps("x", 0, nil, 0))
	Flush(1 * time.Second) // must not hang
}

// --- PreviewPayload ---

func TestPreviewPayload_placeholderWhenNoID(t *testing.T) {
	setupTestEnv(t)
	// No prior ID file → preview should still render (with a
	// placeholder in the distinct_id slot) rather than returning
	// empty. The comment contract was updated to match this.
	payload := PreviewPayload(EventCommandExecuted, CommandExecutedProps("build", 0, nil, 0))
	if payload == "" {
		t.Fatal("preview empty when no ID file exists")
	}
	if !strings.Contains(payload, "no-id-yet") {
		t.Errorf("preview missing placeholder marker: %s", payload)
	}
}

// --- Sampling ---

func TestSampled_rateZeroNeverFires(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	TaskSampleRate = 0
	for i := 0; i < 100; i++ {
		if Sampled() {
			t.Fatal("Sampled() returned true with rate=0")
		}
	}
}

func TestSampled_rateOneAlwaysFires(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	TaskSampleRate = 1
	for i := 0; i < 100; i++ {
		if !Sampled() {
			t.Fatal("Sampled() returned false with rate=1")
		}
	}
}

func TestSampled_inactiveTelemetryNeverFires(t *testing.T) {
	setupTestEnv(t)
	TaskSampleRate = 1
	// IsActive is false (no SetEnabled call), so even rate=1 must
	// short-circuit — opted-out users pay zero per-task RNG cost.
	for i := 0; i < 100; i++ {
		if Sampled() {
			t.Fatal("Sampled() returned true while telemetry inactive")
		}
	}
}

func TestSampled_intermediateRateUsesRNG(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	TaskSampleRate = 0.5
	// Force-deterministic: alternating 0.0 / 0.9 means exactly half
	// the samples (the ones below 0.5) should fire.
	var i int
	rngFn = func() float64 {
		v := []float64{0.1, 0.9}[i%2]
		i++
		return v
	}
	hits := 0
	for j := 0; j < 10; j++ {
		if Sampled() {
			hits++
		}
	}
	if hits != 5 {
		t.Errorf("hits = %d, want 5 (alternating < 0.5)", hits)
	}
}

// --- Preview ---

func TestPreviewPayload_redactsAPIKey(t *testing.T) {
	setupTestEnv(t)
	APIKey = "phc_supersecretkey12345"
	payload := PreviewPayload(EventCommandExecuted, CommandExecutedProps("build", 1, []string{"shell"}, 10))
	if payload == "" {
		t.Fatal("preview empty")
	}
	if strings.Contains(payload, APIKey) {
		t.Errorf("preview leaked full API key: %s", payload)
	}
	if !strings.Contains(payload, "phc_") || !strings.Contains(payload, "2345") {
		t.Errorf("preview should show prefix + suffix of redacted key: %s", payload)
	}
}

// --- Prompt ---

func TestMaybePromptForConsent_skipsWhenAPIKeyEmpty(t *testing.T) {
	setupTestEnv(t)
	APIKey = ""
	got := MaybePromptForConsent(false, false)
	if got != PromptSkipped {
		t.Errorf("outcome = %v, want PromptSkipped", got)
	}
	if LoadState().Decided {
		t.Error("Decided should stay false when APIKey is empty (telemetry is dead code)")
	}
}

func TestMaybePromptForConsent_skipsAndPersistsOffOnNonTTY(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return false }
	got := MaybePromptForConsent(false, false)
	if got != PromptSkipped {
		t.Errorf("outcome = %v, want PromptSkipped", got)
	}
	if !LoadState().Decided {
		t.Error("non-TTY skip should persist Decided=true so we don't re-prompt")
	}
	if LoadState().Enabled {
		t.Error("non-TTY skip should leave Enabled=false")
	}
}

func TestMaybePromptForConsent_skipsAndPersistsOffOnHeadless(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	got := MaybePromptForConsent(true, false) // skipPersistent=true (e.g. --yes)
	if got != PromptSkipped {
		t.Errorf("outcome = %v, want PromptSkipped", got)
	}
	if !LoadState().Decided {
		t.Error("headless skip should persist Decided=true")
	}
}

// TestMaybePromptForConsent_transientSkipDoesNotPersist pins the two-tier
// skip contract. A transient skip signal (today: --json) must suppress
// the prompt for the current invocation without writing consent state,
// so a later interactive run without --json still gets prompted. The
// prior single-bool API conflated these and silently opted users out
// for life after one `raid context --json | jq` invocation.
func TestMaybePromptForConsent_transientSkipDoesNotPersist(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	got := MaybePromptForConsent(false, true) // skipTransient=true (e.g. --json)
	if got != PromptSkipped {
		t.Errorf("outcome = %v, want PromptSkipped", got)
	}
	if LoadState().Decided {
		t.Error("transient skip should NOT persist consent state — bug C1 regression")
	}
}

// TestMaybePromptForConsent_transientSkipDoesNotOverridePersistent
// confirms the ordering: when both signals are set, persistent wins
// (state persisted). Belt-and-suspenders against future callers that
// might pass both.
func TestMaybePromptForConsent_transientSkipDoesNotOverridePersistent(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	got := MaybePromptForConsent(false, true)
	if got != PromptSkipped {
		t.Fatalf("outcome = %v, want PromptSkipped", got)
	}
	if LoadState().Decided {
		t.Fatal("transient-only skip persisted state; aborting subsequent assertions")
	}

	// Now flip persistent on for the second call — must persist.
	got = MaybePromptForConsent(true, true)
	if got != PromptSkipped {
		t.Errorf("outcome = %v, want PromptSkipped", got)
	}
	if !LoadState().Decided {
		t.Error("persistent skip should persist even when transient also true")
	}
}

func TestMaybePromptForConsent_skipsWhenDoNotTrack(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	os.Setenv(DoNotTrackEnvVar, "1")
	got := MaybePromptForConsent(false, false)
	if got != PromptSkipped {
		t.Errorf("outcome = %v, want PromptSkipped", got)
	}
}

func TestMaybePromptForConsent_skipsWhenAlreadyDecided(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	isInteractiveFn = func() bool { return true }
	got := MaybePromptForConsent(false, false)
	if got != PromptSkipped {
		t.Errorf("outcome = %v, want PromptSkipped (already decided)", got)
	}
}

func TestMaybePromptForConsent_acceptsOnYes(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	r := strings.NewReader("y\n")
	promptInFn = func() io.Reader { return r }
	promptOutFn = func() io.Writer { return io.Discard }
	got := MaybePromptForConsent(false, false)
	if got != PromptAccepted {
		t.Errorf("outcome = %v, want PromptAccepted", got)
	}
	st := LoadState()
	if !st.Decided || !st.Enabled {
		t.Errorf("state = %+v, want both true", st)
	}
}

func TestMaybePromptForConsent_declinesOnEmpty(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	r := strings.NewReader("\n")
	promptInFn = func() io.Reader { return r }
	promptOutFn = func() io.Writer { return io.Discard }
	got := MaybePromptForConsent(false, false)
	if got != PromptDeclined {
		t.Errorf("outcome = %v, want PromptDeclined (capital-N default)", got)
	}
	st := LoadState()
	if !st.Decided {
		t.Error("decline should still persist Decided=true")
	}
	if st.Enabled {
		t.Error("decline should leave Enabled=false")
	}
}

func TestMaybePromptForConsent_explainerThenAccept(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	// "?" then "y" on the next round. Reader is captured once so the
	// recursion sees the remaining input after the first read drains "?".
	r := strings.NewReader("?\ny\n")
	promptInFn = func() io.Reader { return r }
	promptOutFn = func() io.Writer { return io.Discard }
	got := MaybePromptForConsent(false, false)
	if got != PromptAccepted {
		t.Errorf("outcome = %v, want PromptAccepted after explainer", got)
	}
}

// --- Follow-up opt-out event consent ---

// TestMaybePromptForConsent_declineThenConsentToOptOutEvent covers
// the happy path of the follow-up prompt: user declines telemetry
// generally, then opts into sending a single anonymous
// "denial recorded" event so the project can measure opt-out
// rates. The event must land via the bypass path
// (CaptureOptOutConsented) because the standard IsActive() gate is
// false at that moment.
func TestMaybePromptForConsent_declineThenConsentToOptOutEvent(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	// "n" declines the first prompt; "y" accepts the follow-up.
	r := strings.NewReader("n\ny\n")
	promptInFn = func() io.Reader { return r }
	promptOutFn = func() io.Writer { return io.Discard }

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	got := MaybePromptForConsent(false, false)
	if got != PromptDeclined {
		t.Errorf("outcome = %v, want PromptDeclined", got)
	}
	st := LoadState()
	if !st.Decided || st.Enabled {
		t.Errorf("state = %+v, want decided=true enabled=false after decline", st)
	}
	if h := atomic.LoadInt32(&hits); h != 1 {
		t.Errorf("opt-out event delivered %d times, want 1", h)
	}
}

// TestMaybePromptForConsent_declineRefuseOptOutEventSendsNothing
// covers the conservative default — declining the main prompt and
// then declining the follow-up must send zero events. This is the
// trust-preserving path: a user who chose "no" twice should leave
// the binary in a state that has touched the network zero times.
func TestMaybePromptForConsent_declineRefuseOptOutEventSendsNothing(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	r := strings.NewReader("n\nn\n")
	promptInFn = func() io.Reader { return r }
	promptOutFn = func() io.Writer { return io.Discard }

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	got := MaybePromptForConsent(false, false)
	if got != PromptDeclined {
		t.Errorf("outcome = %v, want PromptDeclined", got)
	}
	if h := atomic.LoadInt32(&hits); h != 0 {
		t.Errorf("refusing follow-up sent %d events, want 0", h)
	}
}

// TestMaybePromptForConsent_declineEmptyFollowUpSendsNothing pins
// the capital-N default on the follow-up: hitting Enter at the
// second prompt is "no". A stray Enter (or a piped script that
// answers the main prompt but leaves stdin closed) must never
// flip into the "send event" path.
func TestMaybePromptForConsent_declineEmptyFollowUpSendsNothing(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	// "\n" declines the main prompt; the follow-up reads EOF.
	r := strings.NewReader("\n")
	promptInFn = func() io.Reader { return r }
	promptOutFn = func() io.Writer { return io.Discard }

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	if got := MaybePromptForConsent(false, false); got != PromptDeclined {
		t.Errorf("outcome = %v, want PromptDeclined", got)
	}
	if h := atomic.LoadInt32(&hits); h != 0 {
		t.Errorf("EOF follow-up sent %d events, want 0", h)
	}
}

// TestMaybePromptForConsent_acceptSkipsFollowUp asserts the
// follow-up prompt does not fire when the user opted in — there's
// no "denial" to record, and an extra prompt would be confusing.
func TestMaybePromptForConsent_acceptSkipsFollowUp(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	// "y" accepts. The assertion below pins behavior by checking the
	// prompt output never contains the follow-up question text — if
	// the follow-up wrongly fired, that string would appear in `out`.
	r := strings.NewReader("y\n")
	promptInFn = func() io.Reader { return r }
	out := &strings.Builder{}
	promptOutFn = func() io.Writer { return out }

	got := MaybePromptForConsent(false, false)
	if got != PromptAccepted {
		t.Errorf("outcome = %v, want PromptAccepted", got)
	}
	if strings.Contains(out.String(), "recording your decision") {
		t.Errorf("follow-up prompt text leaked into output: %q", out.String())
	}
}

// --- CaptureOptOutConsented ---

// TestCaptureOptOutConsented_bypassesInactive — the whole point of
// this entry point. Consent is undecided / disabled (IsActive ==
// false), but an explicit per-event consent path must still send.
func TestCaptureOptOutConsented_bypassesInactive(t *testing.T) {
	setupTestEnv(t)
	var hits int32
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		body, _ = io.ReadAll(r.Body)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	if IsActive() {
		t.Fatal("test precondition: IsActive should be false on fresh setup")
	}
	CaptureOptOutConsented("prompt-declined")
	if h := atomic.LoadInt32(&hits); h != 1 {
		t.Fatalf("delivered %d events, want 1", h)
	}
	// Confirm the event carries the opt-out shape — name + reason.
	if !strings.Contains(string(body), EventTelemetryOptOut) {
		t.Errorf("body missing event name %q: %s", EventTelemetryOptOut, body)
	}
	if !strings.Contains(string(body), `"reason":"prompt-declined"`) {
		t.Errorf("body missing reason: %s", body)
	}
}

// TestCaptureOptOutConsented_respectsAPIKeyEmpty — even with
// explicit consent, a build without an API key has no destination
// and must no-op. Otherwise dev builds would surface a confusing
// "no endpoint" path.
func TestCaptureOptOutConsented_respectsAPIKeyEmpty(t *testing.T) {
	setupTestEnv(t)
	APIKey = ""
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	CaptureOptOutConsented("prompt-declined")
	if h := atomic.LoadInt32(&hits); h != 0 {
		t.Errorf("delivered %d events with empty API key, want 0", h)
	}
}

// TestCaptureOptOutConsented_respectsDoNotTrack — the hard
// kill-switch must override the per-event consent path. A user
// who set DO_NOT_TRACK=1 has expressed a global "never" that
// trumps any one-off consent we might collect afterward.
func TestCaptureOptOutConsented_respectsDoNotTrack(t *testing.T) {
	setupTestEnv(t)
	os.Setenv(DoNotTrackEnvVar, "1")
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	CaptureEndpoint = srv.URL

	CaptureOptOutConsented("prompt-declined")
	if h := atomic.LoadInt32(&hits); h != 0 {
		t.Errorf("delivered %d events with DO_NOT_TRACK=1, want 0", h)
	}
}

// --- Consent state ---

func TestSetEnabled_persistsBothKeys(t *testing.T) {
	setupTestEnv(t)
	if err := SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	st := LoadState()
	if !st.Decided || !st.Enabled {
		t.Errorf("state = %+v, want both true", st)
	}
	if err := SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	st = LoadState()
	if !st.Decided || st.Enabled {
		t.Errorf("state after off = %+v, want decided=true enabled=false", st)
	}
}
