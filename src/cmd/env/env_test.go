package env

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func setupConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
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

func TestListEnvCmd_noEnvironments(t *testing.T) {
	setupConfig(t)

	// Redirect stdout since ListEnvCmd uses fmt.Println.
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	root := &cobra.Command{Use: "raid"}
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
	setupConfig(t)

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
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

func TestCommand_envNotFound(t *testing.T) {
	setupConfig(t)

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
	root.AddCommand(Command)
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"env", "nonexistent-env"})
	_ = root.Execute()

	got := buf.String()
	if !strings.Contains(got, "Environment not found") {
		t.Errorf("Command with missing env: got %q, want 'Environment not found'", got)
	}
}

func TestCommand_noArgs_withActiveEnv(t *testing.T) {
	setupConfig(t)
	// Set an active env directly in viper (bypasses ContainsEnv check).
	viper.Set("env", "staging")

	// Call Run directly to avoid cobra command state leaking between tests.
	var buf bytes.Buffer
	fakeCmd := &cobra.Command{}
	fakeCmd.SetOut(&buf)
	fakeCmd.SetErr(&buf)
	Command.Run(fakeCmd, []string{})

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
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		lib.ResetContext()
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
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
	setupConfigWithEnv(t, "exec-profile", "dev")

	var buf bytes.Buffer
	root := &cobra.Command{Use: "raid"}
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
	setupConfigWithEnv(t, "list-env-profile", "staging")

	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	root := &cobra.Command{Use: "raid"}
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

func TestCommand_envFound_fullSuccess(t *testing.T) {
	setupConfigWithEnv(t, "success-profile", "prod")

	var buf bytes.Buffer
	fakeCmd := &cobra.Command{}
	fakeCmd.SetOut(&buf)
	fakeCmd.SetErr(&buf)
	Command.Run(fakeCmd, []string{"prod"})

	got := buf.String()
	if !strings.Contains(got, "Environment executed successfully") {
		t.Errorf("Command env success: got %q, want 'Environment executed successfully'", got)
	}
}

func TestCommand_envFound_forceLoadError(t *testing.T) {
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
	Command.Run(fakeCmd, []string{"failing"})

	got := buf.String()
	if !strings.Contains(got, "Failed to reload profile") {
		t.Errorf("Command env forceLoad error: got %q, want 'Failed to reload profile'", got)
	}
}

// TestCommand_envFound_setError covers the env.Set error path by calling
// with an env name that doesn't exist in the config but passes Contains
// via a direct context manipulation.
func TestCommand_envFound_executeError(t *testing.T) {
	// Set up an env with a task that will fail.
	repoRoot := repoRootForEnv(t)

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
	Command.Run(fakeCmd, []string{"badenv"})

	got := buf.String()
	if !strings.Contains(got, "Failed to execute environment") {
		t.Errorf("Command env execute error: got %q, want 'Failed to execute environment'", got)
	}
}
