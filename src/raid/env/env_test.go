package env

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
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

func TestGet_emptyState(t *testing.T) {
	setupConfig(t)
	got := Get()
	if got != "" {
		t.Errorf("Get() = %q, want empty string when no env set", got)
	}
}

func TestListAll_emptyState(t *testing.T) {
	setupConfig(t)
	envs := ListAll()
	if len(envs) != 0 {
		t.Errorf("ListAll() = %v, want empty slice when no context", envs)
	}
}

func TestContains_emptyState(t *testing.T) {
	setupConfig(t)
	if Contains("dev") {
		t.Error("Contains(\"dev\") = true, want false when no context")
	}
}

func TestSet_notFound(t *testing.T) {
	setupConfig(t)
	err := Set("nonexistent")
	if err == nil {
		t.Fatal("Set(\"nonexistent\"): expected error, got nil")
	}
}

func TestExecute_noContext(t *testing.T) {
	setupConfig(t)
	// Suppress stderr noise.
	origStderr := os.Stderr
	devNull, _ := os.Open(os.DevNull)
	os.Stderr = devNull
	t.Cleanup(func() {
		os.Stderr = origStderr
		devNull.Close()
	})

	err := Execute("dev")
	if err == nil {
		t.Fatal("Execute(\"dev\"): expected error when context is not initialized")
	}
}
