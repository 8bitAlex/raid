package telemetry

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	libtelemetry "github.com/8bitalex/raid/src/internal/telemetry"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// setupCmdTestEnv mirrors src/internal/telemetry's setupTestEnv but
// also wires a root cobra command with the persistent --json flag so
// the subcommands' jsonMode helper has something to read.
func setupCmdTestEnv(t *testing.T) (*cobra.Command, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	idPath := filepath.Join(dir, "telemetry-id")

	viper.Reset()
	viper.SetConfigFile(filepath.Join(dir, "config.toml"))
	if f, err := os.Create(filepath.Join(dir, "config.toml")); err == nil {
		f.Close()
	}

	prevID := os.Getenv(libtelemetry.IDFileEnv)
	prevDNT := os.Getenv(libtelemetry.DoNotTrackEnvVar)
	os.Setenv(libtelemetry.IDFileEnv, idPath)
	os.Unsetenv(libtelemetry.DoNotTrackEnvVar)
	t.Cleanup(func() {
		os.Setenv(libtelemetry.IDFileEnv, prevID)
		if prevDNT == "" {
			os.Unsetenv(libtelemetry.DoNotTrackEnvVar)
		} else {
			os.Setenv(libtelemetry.DoNotTrackEnvVar, prevDNT)
		}
		viper.Reset()
	})

	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	// Detach the subcommand from the package-level Command so its
	// parent is this fresh root, not the real rootCmd. Cobra resolves
	// each subcommand's parent at AddCommand time; since onCmd et al.
	// are already attached to Command in init(), we re-add Command to
	// our test root.
	root.AddCommand(Command)
	// Cobra caches the merged flag set per command; calling Flags()
	// after a parent change forces it to rebuild so `--json` (on the
	// fresh root) is visible to the subcommand's flag parser instead
	// of the stale set inherited from a previous test's root.
	Command.ResetFlags()
	for _, sub := range Command.Commands() {
		sub.ResetFlags()
	}
	// Re-register the offCmd's --why flag since ResetFlags wiped it.
	for _, sub := range Command.Commands() {
		if sub.Use == "off" {
			sub.Flags().String("why", "", "Optional free-text reason recorded with the opt-out event")
		}
	}

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	return root, &out
}

func runCmd(t *testing.T, root *cobra.Command, args ...string) error {
	t.Helper()
	root.SetArgs(args)
	return root.Execute()
}

// --- on ---

func TestOnCmd_persistsEnabled(t *testing.T) {
	root, out := setupCmdTestEnv(t)
	if err := runCmd(t, root, "telemetry", "on"); err != nil {
		t.Fatalf("on: %v", err)
	}
	st := libtelemetry.LoadState()
	if !st.Decided || !st.Enabled {
		t.Errorf("state after `on` = %+v, want both true", st)
	}
	if !strings.Contains(out.String(), "Telemetry: on") {
		t.Errorf("output should confirm: %s", out.String())
	}
}

// --- off ---

func TestOffCmd_persistsDisabled(t *testing.T) {
	root, _ := setupCmdTestEnv(t)
	// First flip on so we have a real "previously enabled" state.
	if err := libtelemetry.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	if err := runCmd(t, root, "telemetry", "off"); err != nil {
		t.Fatalf("off: %v", err)
	}
	st := libtelemetry.LoadState()
	if !st.Decided || st.Enabled {
		t.Errorf("state after `off` = %+v, want decided=true enabled=false", st)
	}
}

func TestOffCmd_acceptsWhyFlag(t *testing.T) {
	root, _ := setupCmdTestEnv(t)
	if err := runCmd(t, root, "telemetry", "off", "--why", "ci runner"); err != nil {
		t.Fatalf("off --why: %v", err)
	}
	if libtelemetry.LoadState().Enabled {
		t.Error("--why must not change the off semantics")
	}
}

// --- status ---

func TestStatusCmd_textOutputShowsStateAndIDPath(t *testing.T) {
	root, out := setupCmdTestEnv(t)
	if err := runCmd(t, root, "telemetry", "status"); err != nil {
		t.Fatalf("status: %v", err)
	}
	got := out.String()
	for _, want := range []string{"State:", "ID file:"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q: %s", want, got)
		}
	}
}

func TestStatusCmd_jsonOutputIsParseable(t *testing.T) {
	root, out := setupCmdTestEnv(t)
	if err := runCmd(t, root, "--json", "telemetry", "status"); err != nil {
		t.Fatalf("status --json: %v", err)
	}
	var parsed statusEntry
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("JSON parse: %v\n%s", err, out.String())
	}
	if parsed.Enabled {
		t.Error("Enabled should default to false")
	}
	if parsed.Decided {
		t.Error("Decided should default to false")
	}
	if parsed.IDPath == "" {
		t.Error("IDPath should always be populated for the user to inspect")
	}
}

// --- purge ---

func TestPurgeCmd_removesIDFile(t *testing.T) {
	root, _ := setupCmdTestEnv(t)
	// Force an ID to exist by setting consent on + capturing — but
	// capture without an API key is a no-op, so use LoadIDIfExists's
	// sister directly via SetEnabled then loadOrCreateID. The cmd
	// surface doesn't expose loadOrCreateID, so we write the file
	// ourselves to simulate prior opt-in.
	path := libtelemetry.IDPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test-uuid\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := runCmd(t, root, "telemetry", "purge"); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("ID file should be gone after purge: %v", err)
	}
}

// --- preview ---

func TestPreviewCmd_rendersPayload(t *testing.T) {
	root, out := setupCmdTestEnv(t)
	// Inject a fake API key so PreviewPayload exercises the redaction
	// branch — without it, the preview would show "<not configured>".
	prev := libtelemetry.APIKey
	libtelemetry.APIKey = "phc_test_key_xyz"
	t.Cleanup(func() { libtelemetry.APIKey = prev })

	if err := runCmd(t, root, "telemetry", "preview"); err != nil {
		t.Fatalf("preview: %v", err)
	}
	got := out.String()
	for _, want := range []string{"event", "raid_command_executed", "command_name", "build", "redacted", "phc_"} {
		if want == "redacted" {
			// The redaction marker is the unicode ellipsis between
			// prefix and suffix; check the suffix slice instead.
			if !strings.Contains(got, "_xyz") {
				t.Errorf("preview missing redacted-key suffix: %s", got)
			}
			continue
		}
		if !strings.Contains(got, want) {
			t.Errorf("preview missing %q: %s", want, got)
		}
	}
	if strings.Contains(got, "phc_test_key_xyz") {
		t.Errorf("preview leaked full API key: %s", got)
	}
}
