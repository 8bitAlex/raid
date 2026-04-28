package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	subprocEnvNoArgs  = "RAID_TEST_INSTALL_NOARGS"
	subprocEnvOneArg  = "RAID_TEST_INSTALL_ONEARG"
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

// TestCommand_isConfigured verifies the exported Command is properly initialised
// (the init() function registered the --threads flag).
func TestCommand_isConfigured(t *testing.T) {
	if Command.Use != "install [repo]" {
		t.Errorf("Command.Use = %q, want %q", Command.Use, "install [repo]")
	}

	f := Command.Flags().Lookup("threads")
	if f == nil {
		t.Fatal("--threads flag not registered")
	}
	if f.DefValue != "0" {
		t.Errorf("--threads default = %q, want %q", f.DefValue, "0")
	}
}

// TestInstallCommand_noArgs_subprocess exercises the Run branch where no repo
// arg is given.  With no profile configured, raid.Install returns an error and
// log.Fatalf exits the process — so we run it in a subprocess.
func TestInstallCommand_noArgs_subprocess(t *testing.T) {
	if os.Getenv(subprocEnvNoArgs) == "1" {
		setupConfig(t)
		cmd := &cobra.Command{}
		Command.Run(cmd, []string{})
		return
	}

	proc := exec.Command(os.Args[0], "-test.run=TestInstallCommand_noArgs_subprocess", "-test.v")
	proc.Env = append(os.Environ(), subprocEnvNoArgs+"=1")
	err := proc.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got: %T %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("install no-args exit code = %d, want 1", exitErr.ExitCode())
	}
}

// TestInstallCommand_oneArg_subprocess exercises the Run branch where a single
// repo name is given.  With no profile configured, raid.InstallRepo returns an
// error and log.Fatalf exits — so we run it in a subprocess.
func TestInstallCommand_oneArg_subprocess(t *testing.T) {
	if os.Getenv(subprocEnvOneArg) == "1" {
		setupConfig(t)
		cmd := &cobra.Command{}
		Command.Run(cmd, []string{"some-repo"})
		return
	}

	proc := exec.Command(os.Args[0], "-test.run=TestInstallCommand_oneArg_subprocess", "-test.v")
	proc.Env = append(os.Environ(), subprocEnvOneArg+"=1")
	err := proc.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got: %T %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("install one-arg exit code = %d, want 1", exitErr.ExitCode())
	}
}

// setupConfigWithProfile creates a config with a profile that has a local repo
// that can actually be cloned.
func setupConfigWithProfile(t *testing.T) string {
	t.Helper()
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

	// Create a bare git repo that can be cloned
	bareRepo := filepath.Join(dir, "bare.git")
	cmd := exec.Command("git", "init", "--bare", bareRepo)
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init --bare: %v", err)
	}

	cloneDest := filepath.Join(dir, "cloned")

	profilePath := filepath.Join(dir, "test.raid.yaml")
	content := fmt.Sprintf("name: test\nrepositories:\n  - name: repo1\n    url: file://%s\n    path: %s\n", bareRepo, cloneDest)
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := lib.AddProfile(lib.Profile{Name: "test", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("test"); err != nil {
		t.Fatal(err)
	}
	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}

	return cloneDest
}

func TestInstallCommand_noArgs_success(t *testing.T) {
	cloneDest := setupConfigWithProfile(t)

	// Call the Run handler directly - on success it just returns without log.Fatalf
	cmd := &cobra.Command{}
	Command.Run(cmd, []string{})

	// Verify the repo was cloned
	if _, err := os.Stat(cloneDest); err != nil {
		t.Errorf("install: expected repo cloned at %s, got: %v", cloneDest, err)
	}
}

func TestInstallCommand_oneArg_success(t *testing.T) {
	cloneDest := setupConfigWithProfile(t)

	cmd := &cobra.Command{}
	Command.Run(cmd, []string{"repo1"})

	if _, err := os.Stat(cloneDest); err != nil {
		t.Errorf("install repo1: expected repo cloned at %s, got: %v", cloneDest, err)
	}
}
