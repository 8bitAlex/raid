package raid

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/viper"
)

// setupConfig initializes a throw-away viper config backed by a temp file.
func setupConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		viper.Reset()
	})
	lib.CfgPath = cfgPath

	if err := lib.InitConfig(); err != nil {
		t.Fatalf("setupConfig: InitConfig: %v", err)
	}
}

func TestGetProperty_version(t *testing.T) {
	v, err := GetProperty(PropertyVersion)
	if err != nil {
		t.Fatalf("GetProperty(version): %v", err)
	}
	if v == "" {
		t.Error("GetProperty(version) returned empty string")
	}
}

func TestGetProperty_environment(t *testing.T) {
	v, err := GetProperty(PropertyEnvironment)
	if err != nil {
		t.Fatalf("GetProperty(environment): %v", err)
	}
	if v == "" {
		t.Error("GetProperty(environment) returned empty string")
	}
}

func TestGetProperty_missing(t *testing.T) {
	_, err := GetProperty("no_such_property_xyz")
	if err == nil {
		t.Fatal("GetProperty(missing): expected error, got nil")
	}
}

func TestDoctor_returnsFindings(t *testing.T) {
	setupConfig(t)
	findings := Doctor()
	if findings == nil {
		t.Fatal("Doctor() returned nil slice")
	}
	if len(findings) == 0 {
		t.Fatal("Doctor() returned empty findings; expected at least one check")
	}
}

func TestGetCommands_emptyState(t *testing.T) {
	setupConfig(t)
	cmds := GetCommands()
	// With no profile loaded, should return an empty (or nil) slice without panic.
	_ = cmds
}

func TestQuietGetCommands_emptyState(t *testing.T) {
	// QuietGetCommands must not panic even when no config exists.
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		viper.Reset()
	})
	lib.CfgPath = filepath.Join(t.TempDir(), "nonexistent.toml")

	cmds := QuietGetCommands()
	_ = cmds
}

func TestLoad_noProfile(t *testing.T) {
	setupConfig(t)
	// Load returns an error when no profile is configured — this is expected.
	_ = Load()
}

func TestForceLoad_noProfile(t *testing.T) {
	setupConfig(t)
	_ = ForceLoad()
}

func TestConstants(t *testing.T) {
	// Ensure the re-exported constants are non-empty strings.
	checks := map[string]string{
		"RaidConfigFileName":  RaidConfigFileName,
		"ConfigPathFlag":      ConfigPathFlag,
		"ConfigPathFlagShort": ConfigPathFlagShort,
		"ConfigDirName":       ConfigDirName,
		"ConfigFileName":      ConfigFileName,
	}
	for name, val := range checks {
		if val == "" {
			t.Errorf("constant %s is empty", name)
		}
	}
}

func TestSeverityConstants(t *testing.T) {
	// Sanity-check that severity ordering is OK<Warn<Error.
	if SeverityOK >= SeverityWarn {
		t.Error("SeverityOK should be less than SeverityWarn")
	}
	if SeverityWarn >= SeverityError {
		t.Error("SeverityWarn should be less than SeverityError")
	}
}

func TestEnvironmentConstants(t *testing.T) {
	envs := []Environment{EnvironmentDevelopment, EnvironmentPreview, EnvironmentProduction}
	for _, e := range envs {
		if string(e) == "" {
			t.Errorf("Environment constant is empty string: %v", e)
		}
	}
}

func TestInstall_noProfile(t *testing.T) {
	setupConfig(t)
	err := Install(0)
	// With no profile configured, Install returns an error — not a fatal exit.
	if err == nil {
		t.Fatal("Install(0) expected error with no profile, got nil")
	}
}

func TestInstallRepo_noProfile(t *testing.T) {
	setupConfig(t)
	err := InstallRepo("some-repo")
	if err == nil {
		t.Fatal("InstallRepo() expected error with no profile, got nil")
	}
}

func TestExecuteCommand_noContext(t *testing.T) {
	setupConfig(t)
	err := ExecuteCommand("somecmd", nil)
	if err == nil {
		t.Fatal("ExecuteCommand() expected error with no context, got nil")
	}
}

func TestInitialize_withTempConfig(t *testing.T) {
	dir := t.TempDir()
	old := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = old
		viper.Reset()
	})

	// Point to a writable temp path so Initialize doesn't touch the real config.
	lib.CfgPath = filepath.Join(dir, "config.toml")
	// Suppress stderr from "warning: could not load profile"
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		devNull.Close()
	})

	Initialize()
}
