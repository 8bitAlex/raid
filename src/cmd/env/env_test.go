package env

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// execCmd runs root (with sub added) and returns the combined stdout+stderr output.
func execCmd(t *testing.T, root *cobra.Command, sub *cobra.Command, args ...string) string {
	t.Helper()
	var buf bytes.Buffer
	root.AddCommand(sub)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	_ = root.Execute()
	return buf.String()
}

// TestJsonMode_handlesDetachedCmd defends the bare-cmd branch of jsonMode.
func TestJsonMode_handlesDetachedCmd(t *testing.T) {
	bare := &cobra.Command{}
	if jsonMode(bare) {
		t.Error("bare cmd with no root json flag should report false")
	}
}

// resetEnvCmdState clears any cached cobra flag-merge state on the package
// level Command vars so each test sees a fresh persistent-flag wiring. Cobra
// caches `parentsPflags` on the command the first time merge runs, and
// never refreshes — without this, a --json persistent flag from a previous
// test's root stays attached and a fresh root's flag is silently ignored.
func resetEnvCmdState(t *testing.T) {
	t.Helper()
	Command.ResetFlags()
	ListEnvCmd.ResetFlags()
}

func TestListEnvCmd_noEnvironments(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)

	// Redirect stdout since ListEnvCmd uses fmt.Println.
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(ListEnvCmd)
	root.SetArgs([]string{"list"})
	_ = root.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	got := buf.String()

	if !strings.Contains(got, "No environments found") {
		t.Errorf("ListEnvCmd with no envs: got %q, want %q", got, "No environments found.")
	}
}

func TestCommand_noArgs_noActiveEnv(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(Command)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"env"})
	_ = root.Execute()

	got := buf.String()
	if !strings.Contains(got, "No active environment set") {
		t.Errorf("Command with no args: got %q, want 'No active environment set.'", got)
	}
}

func TestCommand_noArgs_jsonNoActive(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(Command)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"env", "--json"})
	_ = root.Execute()

	var got envEntry
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
	if got.Name != "" || got.Active {
		t.Errorf("got = %+v, want zero envEntry when no env set", got)
	}
}

func TestCommand_noArgs_jsonWithActive(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)
	viper.Set("env", "staging")

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(Command)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"env", "--json"})
	_ = root.Execute()

	var got envEntry
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
	if got.Name != "staging" || !got.Active {
		t.Errorf("got = %+v, want {Name:staging Active:true}", got)
	}
}

func TestListEnvCmd_jsonEmpty(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(ListEnvCmd)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"list", "--json"})
	_ = root.Execute()

	var got []envEntry
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestCommand_jsonWithArgument_isError(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.AddCommand(Command)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"env", "anything", "--json"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when --json is combined with an argument, got nil")
	} else if !strings.Contains(err.Error(), "--json") {
		t.Errorf("error %q should mention --json", err.Error())
	}
}

func TestCommand_envNotFound(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(Command)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetArgs([]string{"env", "nonexistent-env"})
	err := root.Execute()

	if err == nil {
		t.Fatal("expected error for missing env")
	}
	if !strings.Contains(err.Error(), "nonexistent-env") {
		t.Errorf("Command with missing env: error %q should mention name", err.Error())
	}
}

func TestCommand_noArgs_withActiveEnv(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)
	// Set an active env directly in viper (bypasses ContainsEnv check).
	viper.Set("env", "staging")

	// Call Run directly to avoid cobra command state leaking between tests.
	var buf bytes.Buffer
	fakeCmd := &cobra.Command{}
	fakeCmd.SetOut(&buf)
	fakeCmd.SetErr(&buf)
	_ = Command.RunE(fakeCmd, []string{})

	got := buf.String()
	if !strings.Contains(got, "Active environment") {
		t.Errorf("Command with active env: got %q, want 'Active environment:'", got)
	}
}

// setupConfigWithEnv creates a temp config, writes a minimal profile YAML with
// one environment named envName, registers and activates it, then calls
// ForceLoad to populate the lib context.
func setupConfigWithEnv(t *testing.T, profileName, envName string) {
	t.Helper()
	repoRoot := repoRootForEnv(t)

	dir := t.TempDir()
	oldCfg := lib.CfgPath
	oldLock := lib.LockPathOverride
	oldRecent := lib.RecentPathOverride
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.LockPathOverride = oldLock
		lib.RecentPathOverride = oldRecent
		lib.ResetContext()
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.LockPathOverride = filepath.Join(dir, ".lock")
	lib.RecentPathOverride = filepath.Join(dir, "recent.json")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("setupConfigWithEnv: InitConfig: %v", err)
	}

	// Write a minimal profile with one environment.
	profilePath := filepath.Join(dir, profileName+".raid.yaml")
	content := fmt.Sprintf("name: %s\nenvironments:\n  - name: %s\n", profileName, envName)
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Register and activate the profile.  The Path must point to the file we
	// just wrote so ForceLoad can read it back.
	if err := lib.AddProfile(lib.Profile{Name: profileName, Path: profilePath}); err != nil {
		t.Fatalf("AddProfile: %v", err)
	}
	if err := lib.SetProfile(profileName); err != nil {
		t.Fatalf("SetProfile: %v", err)
	}

	// ForceLoad needs the repo root on the Go path to resolve embedded schemas.
	wd, _ := os.Getwd()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}
}

// repoRootForEnv walks up to the directory that contains a "schemas/" subdirectory.
func repoRootForEnv(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	dir := wd
	for {
		if fi, err := os.Stat(filepath.Join(dir, "schemas")); err == nil && fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no schemas/ dir)")
		}
		dir = parent
	}
}

func TestCommand_envFound_executes(t *testing.T) {
	resetEnvCmdState(t)
	setupConfigWithEnv(t, "exec-profile", "dev")

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(Command)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"env", "dev"})
	_ = root.Execute()

	got := buf.String()
	if !strings.Contains(got, "Setting up environment") {
		t.Errorf("Command env found: got %q, want 'Setting up environment'", got)
	}
}

func TestListEnvCmd_withEnvironments(t *testing.T) {
	resetEnvCmdState(t)
	setupConfigWithEnv(t, "list-env-profile", "staging")

	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(ListEnvCmd)
	root.SetArgs([]string{"list"})
	_ = root.Execute()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	got := buf.String()
	if !strings.Contains(got, "staging") {
		t.Errorf("ListEnvCmd with envs: got %q, want 'staging'", got)
	}
}

func TestListEnvCmd_jsonWithEnvironments(t *testing.T) {
	resetEnvCmdState(t)
	setupConfigWithEnv(t, "list-json-profile", "staging")

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", false, "")
	root.AddCommand(ListEnvCmd)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"list", "--json"})
	_ = root.Execute()

	var got []envEntry
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", buf.String(), err)
	}
	if len(got) == 0 {
		t.Fatalf("got 0 entries, want >=1; output=%q", buf.String())
	}
	found := false
	for _, e := range got {
		if e.Name == "staging" {
			found = true
			if e.Active {
				t.Errorf("staging entry Active = true, want false (env not loaded)")
			}
		}
	}
	if !found {
		t.Errorf("staging missing from %+v", got)
	}
}

// failingWriter always errors, forcing enc.Encode to fail so the
// errs.Unknown wrap in the --json branch is exercised.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("simulated write failure")
}

// TestListEnvCmd_jsonEncodeError covers the encode-failure branch: the error
// must come back wrapped as a structured UNKNOWN error (consistent with the
// sibling commands) instead of a bare encoder error.
func TestListEnvCmd_jsonEncodeError(t *testing.T) {
	resetEnvCmdState(t)
	setupConfig(t)

	root := &cobra.Command{Use: "raid"}
	root.PersistentFlags().Bool("json", true, "")
	root.AddCommand(ListEnvCmd)
	ListEnvCmd.SetOut(failingWriter{})
	t.Cleanup(func() { ListEnvCmd.SetOut(nil) })

	err := ListEnvCmd.RunE(ListEnvCmd, nil)
	if err == nil {
		t.Fatal("expected error when encode fails")
	}
	rErr, ok := errs.AsError(err)
	if !ok {
		t.Fatalf("error not structured: %v", err)
	}
	if rErr.Code() != errs.CodeUnknown {
		t.Errorf("code = %q, want %q", rErr.Code(), errs.CodeUnknown)
	}
	if !strings.Contains(err.Error(), "simulated write failure") {
		t.Errorf("error %q does not carry the encoder failure", err.Error())
	}
}

func TestCommand_envFound_fullSuccess(t *testing.T) {
	resetEnvCmdState(t)
	setupConfigWithEnv(t, "success-profile", "prod")

	var buf bytes.Buffer
	fakeCmd := &cobra.Command{}
	fakeCmd.SetOut(&buf)
	fakeCmd.SetErr(&buf)
	_ = Command.RunE(fakeCmd, []string{"prod"})

	got := buf.String()
	if !strings.Contains(got, "Environment executed successfully") {
		t.Errorf("Command env success: got %q, want 'Environment executed successfully'", got)
	}
}

func TestCommand_envFound_forceLoadError(t *testing.T) {
	resetEnvCmdState(t)
	setupConfigWithEnv(t, "exec-err-profile", "failing")

	// Delete the profile file so that ForceLoad (which re-reads it) fails,
	// while the cached context still has the env for Contains to succeed.
	profilePath := lib.GetProfile().Path
	if err := os.Remove(profilePath); err != nil {
		t.Fatalf("remove profile file: %v", err)
	}

	var buf bytes.Buffer
	fakeCmd := &cobra.Command{}
	fakeCmd.SetOut(&buf)
	fakeCmd.SetErr(&buf)
	err := Command.RunE(fakeCmd, []string{"failing"})

	if err == nil {
		t.Fatal("expected error when ForceLoad fails")
	}
}

// TestCommand_envFound_setError covers the env.Set error path by calling
// with an env name that doesn't exist in the config but passes Contains
// via a direct context manipulation.
func TestCommand_envFound_executeError(t *testing.T) {
	resetEnvCmdState(t)
	// Set up an env with a task that will fail.
	repoRoot := repoRootForEnv(t)

	dir := t.TempDir()
	oldCfg := lib.CfgPath
	oldLock := lib.LockPathOverride
	oldRecent := lib.RecentPathOverride
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.LockPathOverride = oldLock
		lib.RecentPathOverride = oldRecent
		lib.ResetContext()
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	lib.LockPathOverride = filepath.Join(dir, ".lock")
	lib.RecentPathOverride = filepath.Join(dir, "recent.json")
	lib.ResetContext()
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("InitConfig: %v", err)
	}

	// Profile with env that has a failing task.
	profilePath := filepath.Join(dir, "failenv.raid.yaml")
	content := "name: failenv\nenvironments:\n  - name: badenv\n    tasks:\n      - type: Shell\n        cmd: exit 1\n"
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "failenv", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("failenv"); err != nil {
		t.Fatal(err)
	}

	wd, _ := os.Getwd()
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}

	var buf bytes.Buffer
	fakeCmd := &cobra.Command{}
	fakeCmd.SetOut(&buf)
	fakeCmd.SetErr(&buf)
	err := Command.RunE(fakeCmd, []string{"badenv"})

	if err == nil {
		t.Fatal("expected error when env task fails")
	}
}

// TestCommand_envSetError covers the env.Set error path by making the config
// file read-only after setup so viper.WriteConfig fails.
func TestCommand_envSetError(t *testing.T) {
	resetEnvCmdState(t)
	if os.Getuid() == 0 {
		t.Skip("file permissions not enforced as root")
	}
	if runtime.GOOS == "windows" {
		t.Skip("chmod file permissions behave differently on Windows")
	}
	setupConfigWithEnv(t, "setfail-profile", "dev")

	// Make the config file read-only so Set (which calls viper.WriteConfig) fails.
	if err := os.Chmod(lib.CfgPath, 0444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(lib.CfgPath, 0644) })

	var buf bytes.Buffer
	fakeCmd := &cobra.Command{}
	fakeCmd.SetOut(&buf)
	fakeCmd.SetErr(&buf)
	err := Command.RunE(fakeCmd, []string{"dev"})

	if err == nil {
		t.Fatal("expected error when env.Set fails")
	}
}
