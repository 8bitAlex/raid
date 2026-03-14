package lib

import (
	"os"
	"path/filepath"
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
