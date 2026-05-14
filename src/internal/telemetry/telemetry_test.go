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
	got := MaybePromptForConsent(false)
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
	got := MaybePromptForConsent(false)
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
	got := MaybePromptForConsent(true) // skipInteractive=true (e.g. --yes)
	if got != PromptSkipped {
		t.Errorf("outcome = %v, want PromptSkipped", got)
	}
	if !LoadState().Decided {
		t.Error("headless skip should persist Decided=true")
	}
}

func TestMaybePromptForConsent_skipsWhenDoNotTrack(t *testing.T) {
	setupTestEnv(t)
	isInteractiveFn = func() bool { return true }
	os.Setenv(DoNotTrackEnvVar, "1")
	got := MaybePromptForConsent(false)
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
	got := MaybePromptForConsent(false)
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
	got := MaybePromptForConsent(false)
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
	got := MaybePromptForConsent(false)
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
	got := MaybePromptForConsent(false)
	if got != PromptAccepted {
		t.Errorf("outcome = %v, want PromptAccepted after explainer", got)
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
