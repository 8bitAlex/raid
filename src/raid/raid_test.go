package raid

import (
	"fmt"
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

	oldCfg := lib.CfgPath
	oldLock := lib.LockPathOverride
	oldRecent := lib.RecentPathOverride
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		lib.LockPathOverride = oldLock
		lib.RecentPathOverride = oldRecent
		viper.Reset()
	})
	lib.CfgPath = cfgPath
	// Redirect raid's home-dir state files so this test stays isolated
	// from any concurrent raid run on the developer's machine.
	lib.LockPathOverride = filepath.Join(dir, ".lock")
	lib.RecentPathOverride = filepath.Join(dir, "recent.json")

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

func TestGetRepos_emptyState(t *testing.T) {
	setupConfig(t)
	repos := GetRepos()
	_ = repos
}

func TestExecuteRepoCommand_noContext(t *testing.T) {
	setupConfig(t)
	err := ExecuteRepoCommand("repo", "cmd", nil)
	if err == nil {
		t.Fatal("ExecuteRepoCommand() expected error with no context, got nil")
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

func TestInitialize_withProfile(t *testing.T) {
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

	// Create a valid profile
	profilePath := filepath.Join(dir, "test.raid.yaml")
	if err := os.WriteFile(profilePath, []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "test", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("test"); err != nil {
		t.Fatal(err)
	}

	// Suppress stderr warnings
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		devNull.Close()
	})

	Initialize()
}

func TestConfigPath_pointer(t *testing.T) {
	if ConfigPath == nil {
		t.Fatal("ConfigPath is nil")
	}
	// It should point to the same underlying value as lib.CfgPath
	old := *ConfigPath
	*ConfigPath = "/test/path"
	if lib.CfgPath != "/test/path" {
		t.Errorf("ConfigPath doesn't point to lib.CfgPath")
	}
	*ConfigPath = old
}

func TestForceLoad_withProfile(t *testing.T) {
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

	profilePath := filepath.Join(dir, "test.raid.yaml")
	if err := os.WriteFile(profilePath, []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "test", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("test"); err != nil {
		t.Fatal(err)
	}

	err := ForceLoad()
	if err != nil {
		t.Fatalf("ForceLoad with profile: %v", err)
	}

	// After ForceLoad, GetCommands should work without panic
	cmds := GetCommands()
	_ = cmds
}

func TestLoad_cached(t *testing.T) {
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

	profilePath := filepath.Join(dir, "test.raid.yaml")
	if err := os.WriteFile(profilePath, []byte("name: test\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "test", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("test"); err != nil {
		t.Fatal(err)
	}

	// First call loads
	if err := ForceLoad(); err != nil {
		t.Fatalf("ForceLoad: %v", err)
	}
	// Second call should use cache
	if err := Load(); err != nil {
		t.Fatalf("Load (cached): %v", err)
	}
}

// TestInitialize_initConfigFatal covers the fatal branch when initConfigFn
// returns an error, using the injected logFatalf to intercept os.Exit.
func TestInitialize_initConfigFatal(t *testing.T) {
	oldInit := initConfigFn
	oldFatalf := logFatalf
	t.Cleanup(func() {
		initConfigFn = oldInit
		logFatalf = oldFatalf
	})

	initConfigFn = func() error { return errTest }
	fatalCalled := false
	logFatalf = func(format string, args ...any) {
		fatalCalled = true
	}

	Initialize()
	if !fatalCalled {
		t.Error("Initialize: logFatalf was not called on InitConfig error")
	}
}

var errTest = fmt.Errorf("test error")

// TestInitialize_profileLoadError covers the error branches in Initialize()
// where Load() fails (broken profile) and LoadEnv() fails (nil context).
func TestInitialize_profileLoadError(t *testing.T) {
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

	// Register a profile pointing to a non-existent file so Load() fails.
	if err := lib.AddProfile(lib.Profile{Name: "broken", Path: "/nonexistent/broken.yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := lib.SetProfile("broken"); err != nil {
		t.Fatal(err)
	}

	// Suppress stderr (Initialize prints warnings there).
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		devNull.Close()
	})

	// Initialize should not panic — Load error is non-fatal.
	Initialize()
}
