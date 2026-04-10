package lib

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/viper"
)

// setupTestConfig initializes a fresh viper config backed by a temp file and
// registers cleanup to restore global state after the test.
func setupTestConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	oldCfgPath := CfgPath
	oldContext := context
	t.Cleanup(func() {
		CfgPath = oldCfgPath
		context = oldContext
		viper.Reset()
	})

	CfgPath = configPath
	context = nil

	if err := InitConfig(); err != nil {
		t.Fatalf("setupTestConfig: InitConfig() error: %v", err)
	}
}

func TestInitConfig_createsAndReadsConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	oldCfgPath := CfgPath
	t.Cleanup(func() {
		CfgPath = oldCfgPath
		viper.Reset()
	})
	CfgPath = configPath

	if err := InitConfig(); err != nil {
		t.Fatalf("InitConfig() error: %v", err)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("InitConfig() did not create config file: %v", err)
	}
}

func TestInitConfig_existingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	oldCfgPath := CfgPath
	t.Cleanup(func() {
		CfgPath = oldCfgPath
		viper.Reset()
	})
	CfgPath = configPath

	if err := InitConfig(); err != nil {
		t.Fatalf("InitConfig() on existing file error: %v", err)
	}
}

func TestInitConfig_invalidTOML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configPath, []byte("invalid = [toml"), 0644); err != nil {
		t.Fatal(err)
	}

	oldCfgPath := CfgPath
	t.Cleanup(func() {
		CfgPath = oldCfgPath
		viper.Reset()
	})
	CfgPath = configPath

	if err := InitConfig(); err == nil {
		t.Fatal("InitConfig() expected error for invalid TOML, got nil")
	}
}

func TestSet_persistsKeyInViper(t *testing.T) {
	setupTestConfig(t)

	if err := Set("testkey", "testvalue"); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	if got := viper.GetString("testkey"); got != "testvalue" {
		t.Errorf("Set() did not persist key: got %q, want %q", got, "testvalue")
	}
}

// TestInitConfig_createFileError covers the error branch where CreateFile fails.
func TestInitConfig_createFileError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file-as-parent-dir error semantics differ on Windows")
	}
	// Use a regular file as a parent component, so CreateFile will fail with ENOTDIR.
	f, err := os.CreateTemp("", "raid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	oldCfgPath := CfgPath
	t.Cleanup(func() {
		CfgPath = oldCfgPath
		viper.Reset()
	})
	CfgPath = filepath.Join(f.Name(), "subdir", "config.toml")

	if err := InitConfig(); err == nil {
		t.Fatal("InitConfig() expected error for invalid config path")
	}
}

// TestGetOrCreateConfigFile_createError covers the CreateFile error branch.
func TestGetOrCreateConfigFile_createError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file-as-parent-dir error semantics differ on Windows")
	}
	f, err := os.CreateTemp("", "raid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	oldCfgPath := CfgPath
	t.Cleanup(func() { CfgPath = oldCfgPath })
	CfgPath = filepath.Join(f.Name(), "subdir", "config.toml")

	_, err = getOrCreateConfigFile()
	if err == nil {
		t.Fatal("getOrCreateConfigFile() expected error")
	}
}
