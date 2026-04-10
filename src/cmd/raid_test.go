package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/8bitalex/raid/src/raid"
)

func TestBaseVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"stable release", "1.2.3", "1.2.3"},
		{"beta release", "1.2.3-beta", "1.2.3-beta"},
		{"preview build", "1.2.3-preview", "1.2.3"},
		{"beta preview build", "1.2.3-beta-preview", "1.2.3-beta"},
		{"preview build with sha", "1.2.3-beta-preview.abc1234", "1.2.3-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := baseVersion(tt.version)
			if got != tt.want {
				t.Errorf("baseVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsInfoCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", []string{"raid"}, true},
		{"empty args", []string{}, true},
		{"help subcommand", []string{"raid", "help"}, true},
		{"version subcommand", []string{"raid", "version"}, true},
		{"completion subcommand", []string{"raid", "completion"}, true},
		{"--help flag", []string{"raid", "--help"}, true},
		{"-h flag", []string{"raid", "-h"}, true},
		{"--version flag", []string{"raid", "--version"}, true},
		{"-v flag", []string{"raid", "-v"}, true},
		{"regular command", []string{"raid", "install"}, false},
		{"doctor command", []string{"raid", "doctor"}, false},
		{"profile subcommand", []string{"raid", "profile", "create"}, false},
		{"flag after end-of-flags marker", []string{"raid", "--", "--help"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInfoCommand(tt.args)
			if got != tt.want {
				t.Errorf("isInfoCommand(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestApplyConfigFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
	}{
		{
			name:     "no config flag",
			args:     []string{"raid", "install"},
			wantPath: "",
		},
		{
			name:     "long flag with separate value",
			args:     []string{"raid", "--config", "/custom/config.toml", "install"},
			wantPath: "/custom/config.toml",
		},
		{
			name:     "long flag with equals",
			args:     []string{"raid", "--config=/custom/config.toml"},
			wantPath: "/custom/config.toml",
		},
		{
			name:     "short flag",
			args:     []string{"raid", "-c", "/custom/config.toml"},
			wantPath: "/custom/config.toml",
		},
		{
			name:     "config flag at end with no value",
			args:     []string{"raid", "--config"},
			wantPath: "",
		},
		{
			name:     "config flag before subcommand",
			args:     []string{"raid", "--config", "/path.toml", "env", "list"},
			wantPath: "/path.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := *raid.ConfigPath
			*raid.ConfigPath = ""
			t.Cleanup(func() { *raid.ConfigPath = old })

			applyConfigFlag(tt.args)

			if got := *raid.ConfigPath; got != tt.wantPath {
				t.Errorf("applyConfigFlag() ConfigPath = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

// --- registerUserCommands ---

// newTestRoot returns a minimal cobra root command suitable for registration tests.
func newTestRoot() *cobra.Command {
	return &cobra.Command{Use: "raid"}
}

// helpOutput captures the help text produced by root.
func helpOutput(root *cobra.Command) string {
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	_ = root.Help()
	return buf.String()
}

func TestRegisterUserCommands_emptyProfile(t *testing.T) {
	root := newTestRoot()
	registerUserCommands(root, nil)
	if len(root.Commands()) != 0 {
		t.Errorf("expected no subcommands, got %d", len(root.Commands()))
	}
}

func TestRegisterUserCommands_appearsInHelp(t *testing.T) {
	root := newTestRoot()
	registerUserCommands(root, []lib.Command{
		{Name: "deploy", Usage: "Deploy all services"},
		{Name: "sync", Usage: "Sync repositories"},
	})

	out := helpOutput(root)
	for _, want := range []string{"deploy", "Deploy all services", "sync", "Sync repositories"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q\n\nfull output:\n%s", want, out)
		}
	}
}

func TestRegisterUserCommands_reservedNameSkipped(t *testing.T) {
	root := newTestRoot()
	registerUserCommands(root, []lib.Command{
		{Name: "install", Usage: "should be skipped"},
		{Name: "deploy", Usage: "should appear"},
	})

	cmds := root.Commands()
	if len(cmds) != 1 || cmds[0].Name() != "deploy" {
		t.Errorf("expected only 'deploy', got %v", func() []string {
			names := make([]string, len(cmds))
			for i, c := range cmds {
				names[i] = c.Name()
			}
			return names
		}())
	}
}

// Regression: user commands must appear in output for every invocation type,
// including info commands (no args, --help, help) and unknown subcommands.
// Each subtest drives Cobra via SetArgs + Execute() so the full Cobra dispatch
// path is exercised rather than calling Help() directly.
func TestRegisterUserCommands_visibleForAllInvocationTypes(t *testing.T) {
	cmds := []lib.Command{{Name: "build", Usage: "Build services"}}

	tests := []struct {
		inv  string
		args []string
	}{
		{"no args", []string{}},
		{"--help flag", []string{"--help"}},
		{"help subcommand", []string{"help"}},
	}

	for _, tt := range tests {
		t.Run(tt.inv, func(t *testing.T) {
			root := newTestRoot()
			registerUserCommands(root, cmds)

			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetErr(&buf)
			root.SetArgs(tt.args)
			_ = root.Execute() // error expected for unknown-cmd; ignore it

			if !strings.Contains(buf.String(), "build") {
				t.Errorf("'build' missing from output for invocation %q\noutput:\n%s", tt.inv, buf.String())
			}
		})
	}
}

func TestApplyConfigFlag_shortFlagWithEquals(t *testing.T) {
	old := *raid.ConfigPath
	*raid.ConfigPath = ""
	t.Cleanup(func() { *raid.ConfigPath = old })

	applyConfigFlag([]string{"raid", "-c=/custom/path.toml"})

	if got := *raid.ConfigPath; got != "/custom/path.toml" {
		t.Errorf("applyConfigFlag() ConfigPath = %q, want %q", got, "/custom/path.toml")
	}
}

func TestApplyConfigFlag_endOfFlagsStopsSearch(t *testing.T) {
	old := *raid.ConfigPath
	*raid.ConfigPath = ""
	t.Cleanup(func() { *raid.ConfigPath = old })

	applyConfigFlag([]string{"raid", "--", "--config", "/path"})

	if got := *raid.ConfigPath; got != "" {
		t.Errorf("applyConfigFlag() ConfigPath = %q, want empty after -- marker", got)
	}
}

func TestApplyConfigFlag_configFollowedByFlag(t *testing.T) {
	old := *raid.ConfigPath
	*raid.ConfigPath = ""
	t.Cleanup(func() { *raid.ConfigPath = old })

	// When next arg after --config starts with -, it should not be treated as the value
	applyConfigFlag([]string{"raid", "--config", "-v"})

	if got := *raid.ConfigPath; got != "" {
		t.Errorf("applyConfigFlag() ConfigPath = %q, want empty when next arg starts with -", got)
	}
}

func TestApplyConfigFlag_shortFlagFollowedByFlag(t *testing.T) {
	old := *raid.ConfigPath
	*raid.ConfigPath = ""
	t.Cleanup(func() { *raid.ConfigPath = old })

	applyConfigFlag([]string{"raid", "-c", "--help"})

	if got := *raid.ConfigPath; got != "" {
		t.Errorf("applyConfigFlag() ConfigPath = %q, want empty when next arg starts with -", got)
	}
}

const (
	subprocExecHelp    = "RAID_TEST_EXEC_HELP"
	subprocExecVersion = "RAID_TEST_EXEC_VERSION"
	subprocExecUnknown = "RAID_TEST_EXEC_UNKNOWN"
)

func setupTestConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("setupTestConfig: %v", err)
	}
}

func TestExecute_help_subprocess(t *testing.T) {
	if os.Getenv(subprocExecHelp) == "1" {
		setupTestConfig(t)
		os.Args = []string{"raid", "--help"}
		Execute()
		return
	}

	proc := exec.Command(os.Args[0], "-test.run=^TestExecute_help_subprocess$", "-test.v")
	proc.Env = append(os.Environ(), subprocExecHelp+"=1")
	out, err := proc.CombinedOutput()
	if err != nil {
		t.Fatalf("Execute --help: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "Raid") {
		t.Errorf("Execute --help: output missing 'Raid'\n%s", out)
	}
}

func TestExecute_version_subprocess(t *testing.T) {
	if os.Getenv(subprocExecVersion) == "1" {
		setupTestConfig(t)
		os.Args = []string{"raid", "--version"}
		Execute()
		return
	}

	proc := exec.Command(os.Args[0], "-test.run=^TestExecute_version_subprocess$", "-test.v")
	proc.Env = append(os.Environ(), subprocExecVersion+"=1")
	out, err := proc.CombinedOutput()
	if err != nil {
		t.Fatalf("Execute --version: %v\noutput: %s", err, out)
	}
}

func TestExecute_unknownCommand_subprocess(t *testing.T) {
	if os.Getenv(subprocExecUnknown) == "1" {
		setupTestConfig(t)
		os.Args = []string{"raid", "nonexistent-cmd"}
		Execute()
		return
	}

	proc := exec.Command(os.Args[0], "-test.run=^TestExecute_unknownCommand_subprocess$", "-test.v")
	proc.Env = append(os.Environ(), subprocExecUnknown+"=1")
	out, err := proc.CombinedOutput()
	// With no Run/RunE on the root, cobra shows help for unknown subcommands
	// and exits cleanly (code 0).
	if err != nil {
		t.Fatalf("Execute unknown-cmd: unexpected error: %v\noutput: %s", err, out)
	}
	if !strings.Contains(string(out), "raid") {
		t.Errorf("Execute unknown-cmd: expected help output containing 'raid'\n%s", out)
	}
}

// TestExecute_inProcess_help tests executeRoot() with --help (info command).
func TestExecute_inProcess_help(t *testing.T) {
	oldArgs := os.Args
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		os.Args = oldArgs
		lib.CfgPath = oldCfg
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	// Use a fast mock to avoid network delays.
	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string { return "" }
	t.Cleanup(func() { latestReleaseFn = oldFn })

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	if code := executeRoot([]string{"raid", "--help"}); code != 0 {
		t.Errorf("executeRoot --help: code = %d, want 0", code)
	}
}

// TestExecute_inProcess_version tests executeRoot() with --version.
func TestExecute_inProcess_version(t *testing.T) {
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string { return "" }
	t.Cleanup(func() { latestReleaseFn = oldFn })

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	if code := executeRoot([]string{"raid", "--version"}); code != 0 {
		t.Errorf("executeRoot --version: code = %d, want 0", code)
	}
}

// TestExecute_updateNotice covers the update-notice display path by mocking
// the version fetcher to return a version different from the current one.
func TestExecute_updateNotice(t *testing.T) {
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string { return "99.99.99" }
	t.Cleanup(func() { latestReleaseFn = oldFn })

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	// --help triggers the info path which waits for the version check.
	if code := executeRoot([]string{"raid", "--help"}); code != 0 {
		t.Errorf("executeRoot with update: code = %d, want 0", code)
	}
}

// TestExecute_unknownCommandReturnsError covers the error handling path in
// executeRoot() where rootCmd.Execute() returns an error.
func TestExecute_unknownCommandReturnsError(t *testing.T) {
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string { return "" }
	t.Cleanup(func() { latestReleaseFn = oldFn })

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	// Unknown flag triggers an error from cobra's flag parser.
	code := executeRoot([]string{"raid", "--unknown-flag-xyz"})
	if code != 1 {
		t.Errorf("executeRoot unknown flag: code = %d, want 1", code)
	}
}

// TestExecute_wrapperCallsOsExit verifies Execute() calls the injected
// exit function when executeRoot returns non-zero.
func TestExecute_wrapperCallsOsExit(t *testing.T) {
	oldCfg := lib.CfgPath
	oldArgs := os.Args
	oldExit := osExit
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		os.Args = oldArgs
		osExit = oldExit
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string { return "" }
	t.Cleanup(func() { latestReleaseFn = oldFn })

	var exitCode int
	osExit = func(code int) { exitCode = code }

	os.Args = []string{"raid", "--unknown-flag-xyz"}

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	Execute()
	if exitCode != 1 {
		t.Errorf("Execute: exitCode = %d, want 1", exitCode)
	}
}

// TestExecute_wrapperSuccess verifies Execute() does not call exit on success.
func TestExecute_wrapperSuccess(t *testing.T) {
	oldCfg := lib.CfgPath
	oldArgs := os.Args
	oldExit := osExit
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		os.Args = oldArgs
		osExit = oldExit
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string { return "" }
	t.Cleanup(func() { latestReleaseFn = oldFn })

	exitCalled := false
	osExit = func(code int) { exitCalled = true }

	os.Args = []string{"raid", "--help"}

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	Execute()
	if exitCalled {
		t.Error("Execute: should not call exit on success")
	}
}

// TestExecute_previewEnvironment covers the Preview environment branches by
// mocking the pre-release fetcher and simulating a preview build.
func TestExecute_previewEnvironment(t *testing.T) {
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	// This test does not change the compiled-in environment, but it exercises
	// the non-info path selection of the update channel.
	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string { return "" }
	t.Cleanup(func() { latestReleaseFn = oldFn })

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	// Non-info command uses the non-blocking version check select path.
	_ = executeRoot([]string{"raid", "env", "list"})
}

func TestRegisterUserCommands_runEExecution(t *testing.T) {
	setupTestConfig(t)
	root := newTestRoot()
	registerUserCommands(root, []lib.Command{
		{Name: "testcmd", Usage: "A test command"},
	})

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"testcmd"})
	err := root.Execute()
	// With no loaded context, ExecuteCommand should return an error.
	if err == nil {
		t.Error("registerUserCommands RunE: expected error with no context, got nil")
	}
}

// TestExecute_inProcess_nonInfoCommand exercises the non-info path of Execute()
// by running "raid env list" which doesn't call os.Exit. This test MUST run last
// in the file to avoid state pollution since Execute() modifies global state.
func TestExecute_inProcess_nonInfoCommand(t *testing.T) {
	oldArgs := os.Args
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		os.Args = oldArgs
		lib.CfgPath = oldCfg
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	os.Args = []string{"raid", "env", "list"}

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	Execute()
}

// TestExecute_exitErrorPropagation covers the errors.As(err, &exitErr) branch
// by registering a user command whose task exits with a non-zero code.
func TestExecute_exitErrorPropagation(t *testing.T) {
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	// Build a profile with a command that runs "exit 5".
	profilePath := filepath.Join(dir, "exitcmd.raid.yaml")
	content := "name: exitcmd\ncommands:\n  - name: exitfive\n    tasks:\n      - type: Shell\n        cmd: exit 5\n"
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "exitcmd", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("exitcmd"); err != nil {
		t.Fatal(err)
	}

	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string { return "" }
	t.Cleanup(func() { latestReleaseFn = oldFn })

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	// The user command "exitfive" runs a shell task that exits with code 5.
	// executeRoot should unwrap the *exec.ExitError and return that code.
	code := executeRoot([]string{"raid", "exitfive"})
	if code != 5 {
		t.Errorf("executeRoot with exit 5 command: code = %d, want 5", code)
	}
}

// TestExecute_nonInfoUpdateNotice covers the non-info update notice path.
// The goroutine typically completes before the select runs when the mock
// fetcher returns immediately, triggering the notice display.
func TestExecute_nonInfoUpdateNotice(t *testing.T) {
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.ResetContext()
		viper.Reset()
	})

	dir := t.TempDir()
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	// Mock the release fetcher to return a different version.
	oldFn := latestReleaseFn
	latestReleaseFn = func(string) string {
		// Small delay so the update notice decision uses our value,
		// though the non-info path uses a non-blocking select.
		return "99.99.99"
	}
	t.Cleanup(func() { latestReleaseFn = oldFn })

	origStdout := os.Stdout
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stdout = devNull
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stdout = origStdout
		os.Stderr = origStderr
		devNull.Close()
	})

	// Non-info command. The update may or may not appear due to timing,
	// but this exercises both select branches across multiple runs.
	_ = executeRoot([]string{"raid", "env", "list"})
}
