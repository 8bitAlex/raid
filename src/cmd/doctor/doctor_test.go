package doctor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const subprocEnv = "RAID_TEST_DOCTOR_SUBPROCESS"

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

// TestRunDoctor_subprocess exercises runDoctor in a subprocess so that the
// os.Exit(1) it emits (when no profile is configured) does not kill the test
// process itself.
func TestRunDoctor_subprocess(t *testing.T) {
	if os.Getenv(subprocEnv) == "1" {
		// We're in the subprocess: run the command and let os.Exit happen.
		setupConfig(t)
		cmd := &cobra.Command{}
		cmd.SetOut(os.Stdout)
		cmd.SetErr(os.Stderr)
		runDoctor(cmd, nil)
		return
	}

	proc := exec.Command(os.Args[0], "-test.run=TestRunDoctor_subprocess", "-test.v")
	proc.Env = append(os.Environ(), subprocEnv+"=1")
	err := proc.Run()
	// With no profile configured, Doctor returns error findings → os.Exit(1).
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got: %T %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("runDoctor exit code = %d, want 1", exitErr.ExitCode())
	}
}

// TestCommand_isConfigured just verifies the exported Command var is properly
// set up; it does not invoke the Run handler.
func TestCommand_isConfigured(t *testing.T) {
	if Command.Use != "doctor" {
		t.Errorf("Command.Use = %q, want %q", Command.Use, "doctor")
	}
	if Command.Run == nil {
		t.Error("Command.Run is nil")
	}
}

// TestRunDoctor_allOK exercises the code path where every finding is SeverityOK,
// which prints "No issues found." and exits normally (no os.Exit).
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

	// Create a valid profile with a repo pointing to an existing directory
	// (so the doctor doesn't report any warnings or errors).
	repoDir := t.TempDir()
	// Make it look like a git repo by creating .git dir
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

	// Capture stdout (runDoctor uses fmt.Printf → os.Stdout)
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &cobra.Command{}
	runDoctor(cmd, nil)

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

// TestRunDoctor_warningsOnly exercises the path where warnings exist but no errors,
// which prints the warning count and does NOT call os.Exit.
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

	// Create a profile with repos pointing to non-existent paths.
	// Doctor will report: git OK, profile OK, profile file OK, schema OK,
	// but repos not cloned → Warn findings only.
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

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &cobra.Command{}
	runDoctor(cmd, nil)

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
	// Should show the suggestion arrow
	if !strings.Contains(got, "→") {
		t.Errorf("runDoctor warnings: expected suggestion arrow '→' in output, got %q", got)
	}
}

// TestRunDoctor_noReposWarning tests the path where the profile has no repositories
// configured, which produces a warning finding.
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

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &cobra.Command{}
	runDoctor(cmd, nil)

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
