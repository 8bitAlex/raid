package doctor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/8bitalex/raid/src/raid/errs"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func setupConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	oldCfg := lib.CfgPath
	oldLock := lib.LockPathOverride
	oldRecent := lib.RecentPathOverride
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.LockPathOverride = oldLock
		lib.RecentPathOverride = oldRecent
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	// Redirect raid's home-dir state files so concurrent test runs and the
	// developer's real ~/.raid/ stay isolated.
	lib.LockPathOverride = filepath.Join(dir, ".lock")
	lib.RecentPathOverride = filepath.Join(dir, "recent.json")
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("setupConfig: %v", err)
	}
}

// newDoctorCmd returns a child cobra.Command wired into a parent root
// whose persistent --json flag is set as requested. runDoctor reads
// jsonMode via cmd.Root().PersistentFlags() so the parent setup is the
// load-bearing piece.
func newDoctorCmd(jsonMode bool) *cobra.Command {
	cmd := &cobra.Command{}
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", jsonMode, "")
	if jsonMode {
		_ = root.PersistentFlags().Set("json", "true")
	}
	root.AddCommand(cmd)
	return cmd
}

// TestRunDoctor_returnsConfigError covers the path where the doctor finds
// errors (e.g. no profile configured). It used to os.Exit(1); now it
// returns a structured error in CategoryConfig (exit code 2) so the
// central handler routes the categorical exit code without aborting from
// inside the subcommand.
func TestRunDoctor_returnsConfigError(t *testing.T) {
	setupConfig(t)
	cmd := newDoctorCmd(false)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := runDoctor(cmd, nil)
	if err == nil {
		t.Fatal("expected error from runDoctor with no profile configured")
	}
	rErr, ok := errs.AsError(err)
	if !ok {
		t.Fatalf("error not structured: %v", err)
	}
	if rErr.Category() != errs.CategoryConfig {
		t.Errorf("category = %v, want CategoryConfig", rErr.Category())
	}
	if errs.ExitCode(err) != 2 {
		t.Errorf("exit code = %d, want 2", errs.ExitCode(err))
	}
}

// failingWriter implements io.Writer and always returns an error, used to
// force enc.Encode to fail and exercise the broken-pipe branch.
type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) { return 0, fmt.Errorf("simulated write failure") }

// TestRunDoctor_jsonEncodeError covers the branch where json.Encode fails
// (e.g. broken pipe). The old behavior was os.Exit(1); now it returns an
// Unknown-wrapped error so the central handler can route it. The
// "simulated write failure" message comes through verbatim via Error().
func TestRunDoctor_jsonEncodeError(t *testing.T) {
	setupConfig(t)
	cmd := newDoctorCmd(true)
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})

	err := runDoctor(cmd, nil)
	if err == nil {
		t.Fatal("expected error from runDoctor when encode fails")
	}
	if !strings.Contains(err.Error(), "simulated write failure") {
		t.Errorf("error %q does not contain 'simulated write failure'", err.Error())
	}
}

// TestRunDoctor_jsonWithErrorFindings covers --json mode when error
// findings are present: still emits the JSON, returns a config-category
// error so the central handler exits non-zero.
func TestRunDoctor_jsonWithErrorFindings(t *testing.T) {
	setupConfig(t)
	var stdout bytes.Buffer
	cmd := newDoctorCmd(true)
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})

	err := runDoctor(cmd, nil)
	if err == nil {
		t.Fatal("expected error when doctor findings include errors")
	}
	if errs.ExitCode(err) != 2 {
		t.Errorf("exit code = %d, want 2", errs.ExitCode(err))
	}
	if !strings.Contains(stdout.String(), "\"errors\"") {
		t.Errorf("--json output missing 'errors' field: %q", stdout.String())
	}
}

// TestCommand_isConfigured verifies the exported Command var.
func TestCommand_isConfigured(t *testing.T) {
	if Command.Use != "doctor" {
		t.Errorf("Command.Use = %q, want %q", Command.Use, "doctor")
	}
	if Command.RunE == nil {
		t.Error("Command.RunE is nil")
	}
}

func TestRunDoctor_allOK(t *testing.T) {
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		lib.ResetContext()
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	profilePath := filepath.Join(dir, "ok.raid.yaml")
	content := fmt.Sprintf("name: ok\nrepositories:\n  - name: repo1\n    url: https://example.com/repo.git\n    path: %s\n", repoDir)
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := lib.AddProfile(lib.Profile{Name: "ok", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("ok"); err != nil {
		t.Fatal(err)
	}
	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}

	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := newDoctorCmd(false)
	if err := runDoctor(cmd, nil); err != nil {
		t.Errorf("runDoctor allOK returned error: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "[ok]") {
		t.Errorf("runDoctor allOK: expected '[ok]' in output, got %q", got)
	}
	if !strings.Contains(got, "No issues found") {
		t.Errorf("runDoctor allOK: expected 'No issues found', got %q", got)
	}
}

func TestRunDoctor_warningsOnly(t *testing.T) {
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		lib.ResetContext()
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	profilePath := filepath.Join(dir, "warn.raid.yaml")
	content := "name: warn\nrepositories:\n  - name: missing-repo\n    url: https://example.com/repo.git\n    path: /tmp/nonexistent-path-raid-test-12345\n"
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := lib.AddProfile(lib.Profile{Name: "warn", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("warn"); err != nil {
		t.Fatal(err)
	}
	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}

	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := newDoctorCmd(false)
	if err := runDoctor(cmd, nil); err != nil {
		t.Errorf("runDoctor warningsOnly returned unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "[warn]") {
		t.Errorf("runDoctor warnings: expected '[warn]' in output, got %q", got)
	}
	if !strings.Contains(got, "warning(s)") {
		t.Errorf("runDoctor warnings: expected 'warning(s)' in output, got %q", got)
	}
	if !strings.Contains(got, "→") {
		t.Errorf("runDoctor warnings: expected suggestion arrow '→' in output, got %q", got)
	}
}

func TestRunDoctor_jsonAllOK(t *testing.T) {
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		lib.ResetContext()
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	repoDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(dir, "ok.raid.yaml")
	content := fmt.Sprintf("name: ok\nrepositories:\n  - name: repo1\n    url: https://example.com/repo.git\n    path: %s\n", repoDir)
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "ok", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("ok"); err != nil {
		t.Fatal(err)
	}
	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}

	var buf bytes.Buffer
	cmd := newDoctorCmd(true)
	cmd.SetOut(&buf)
	if err := runDoctor(cmd, nil); err != nil {
		t.Errorf("runDoctor allOK --json returned error: %v", err)
	}

	var got doctorOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
	if len(got.Findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	if got.Summary.Errors != 0 {
		t.Errorf("Summary.Errors = %d, want 0", got.Summary.Errors)
	}
	if got.Summary.OK == 0 {
		t.Errorf("Summary.OK = 0, want >0")
	}
	for _, f := range got.Findings {
		switch f.Severity {
		case "ok", "warn", "error":
		default:
			t.Errorf("finding %+v has unexpected severity string %q", f, f.Severity)
		}
	}
}

func TestRunDoctor_jsonWarningSurfacesSuggestion(t *testing.T) {
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		lib.ResetContext()
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	profilePath := filepath.Join(dir, "warn.raid.yaml")
	content := "name: warn\nrepositories:\n  - name: missing-repo\n    url: https://example.com/repo.git\n    path: /tmp/nonexistent-path-raid-test-67890\n"
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "warn", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("warn"); err != nil {
		t.Fatal(err)
	}
	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}

	var buf bytes.Buffer
	cmd := newDoctorCmd(true)
	cmd.SetOut(&buf)
	if err := runDoctor(cmd, nil); err != nil {
		t.Errorf("runDoctor warning --json returned unexpected error: %v", err)
	}

	var got doctorOutput
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
	if got.Summary.Warnings == 0 {
		t.Fatalf("Summary.Warnings = 0, want >0; output: %q", buf.String())
	}
	hasSuggestion := false
	for _, f := range got.Findings {
		if f.Severity == "warn" && f.Suggestion != "" {
			hasSuggestion = true
			break
		}
	}
	if !hasSuggestion {
		t.Errorf("expected at least one warn finding with non-empty suggestion; got %+v", got.Findings)
	}
}

func TestSeverityString(t *testing.T) {
	cases := map[lib.Severity]string{
		lib.SeverityOK:    "ok",
		lib.SeverityWarn:  "warn",
		lib.SeverityError: "error",
		lib.Severity(99):  "unknown",
	}
	for in, want := range cases {
		if got := severityString(in); got != want {
			t.Errorf("severityString(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestRunDoctor_noReposWarning(t *testing.T) {
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		lib.ResetContext()
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	profilePath := filepath.Join(dir, "norepo.raid.yaml")
	if err := os.WriteFile(profilePath, []byte("name: norepo\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "norepo", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("norepo"); err != nil {
		t.Fatal(err)
	}
	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}

	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := newDoctorCmd(false)
	if err := runDoctor(cmd, nil); err != nil {
		t.Errorf("runDoctor noReposWarning returned unexpected error: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "[warn]") {
		t.Errorf("runDoctor no repos: expected '[warn]' in output, got %q", got)
	}
	if !strings.Contains(got, "none configured") {
		t.Errorf("runDoctor no repos: expected 'none configured' in output, got %q", got)
	}
}

// Belt-and-braces: errors.As goes through the interface and returns true.
func TestRunDoctor_errorTypeIntegratesWithErrorsAs(t *testing.T) {
	setupConfig(t)
	cmd := newDoctorCmd(false)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := runDoctor(cmd, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var rErr errs.Error
	if !errors.As(err, &rErr) {
		t.Fatal("errors.As should resolve to errs.Error")
	}
	if rErr.Code() == "" {
		t.Error("Code should be non-empty")
	}
}
