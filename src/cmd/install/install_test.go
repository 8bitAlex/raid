package install

import (
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
