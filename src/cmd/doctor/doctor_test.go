package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const subprocEnv = "RAID_TEST_DOCTOR_SUBPROCESS"

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
