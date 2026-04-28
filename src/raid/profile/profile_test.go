package profile

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// TestDescribe_parsesRepoYAML covers the thin Describe wrapper around
// lib.ExtractRepo. It writes a minimal raid.yaml to a temp directory and
// asserts the parsed Repo carries the expected name + branch.
func TestDescribe_parsesRepoYAML(t *testing.T) {
	dir := t.TempDir()
	body := "name: parsed\nbranch: trunk\n"
	if err := os.WriteFile(filepath.Join(dir, "raid.yaml"), []byte(body), 0644); err != nil {
		t.Fatalf("write raid.yaml: %v", err)
	}
	repo, err := Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if repo.Name != "parsed" {
		t.Errorf("Name = %q, want %q", repo.Name, "parsed")
	}
	if repo.Branch != "trunk" {
		t.Errorf("Branch = %q, want %q", repo.Branch, "trunk")
	}
}

func TestDescribe_propagatesErrorWhenMissing(t *testing.T) {
	if _, err := Describe(t.TempDir()); err == nil {
		t.Fatal("expected error when raid.yaml is missing")
	}
}

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

// validProfileYAML returns the path to a temporary valid profile file.
func validProfileYAML(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name+".raid.yaml")
	data := lib.Profile{Name: name, Path: path}
	b, err := yaml.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestGet_emptyState(t *testing.T) {
	setupConfig(t)
	p := Get()
	if !p.IsZero() {
		t.Errorf("Get() = %+v, want zero profile when nothing configured", p)
	}
}

func TestListAll_emptyState(t *testing.T) {
	setupConfig(t)
	profiles := ListAll()
	if len(profiles) != 0 {
		t.Errorf("ListAll() = %v, want empty", profiles)
	}
}

func TestAdd_and_Contains(t *testing.T) {
	setupConfig(t)
	p := Profile{Name: "test-add", Path: "/some/path"}
	if err := Add(p); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if !Contains("test-add") {
		t.Error("Contains(\"test-add\") = false after Add()")
	}
}

func TestAddAll(t *testing.T) {
	setupConfig(t)
	profiles := []Profile{
		{Name: "bulk-a", Path: "/a"},
		{Name: "bulk-b", Path: "/b"},
	}
	if err := AddAll(profiles); err != nil {
		t.Fatalf("AddAll() error: %v", err)
	}
	if !Contains("bulk-a") || !Contains("bulk-b") {
		t.Error("AddAll() did not register all profiles")
	}
}

func TestSet_notFound(t *testing.T) {
	setupConfig(t)
	err := Set("nonexistent")
	if err == nil {
		t.Fatal("Set(\"nonexistent\"): expected error, got nil")
	}
}

func TestSet_found(t *testing.T) {
	setupConfig(t)
	if err := Add(Profile{Name: "myp", Path: "/p"}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := Set("myp"); err != nil {
		t.Fatalf("Set(\"myp\") error: %v", err)
	}
}

func TestRemove_notFound(t *testing.T) {
	setupConfig(t)
	err := Remove("ghost")
	if err == nil {
		t.Fatal("Remove(\"ghost\"): expected error, got nil")
	}
}

func TestRemove_found(t *testing.T) {
	setupConfig(t)
	if err := Add(Profile{Name: "toremove", Path: "/p"}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := Remove("toremove"); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}
	if Contains("toremove") {
		t.Error("Contains(\"toremove\") = true after Remove()")
	}
}

func TestUnmarshal_valid(t *testing.T) {
	path := validProfileYAML(t, "unmarshal-test")
	profiles, err := Unmarshal(path)
	if err != nil {
		t.Fatalf("Unmarshal() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "unmarshal-test" {
		t.Errorf("Unmarshal() = %v, want profile named unmarshal-test", profiles)
	}
}

func TestUnmarshal_missing(t *testing.T) {
	_, err := Unmarshal("/nonexistent/path/profile.yaml")
	if err == nil {
		t.Fatal("Unmarshal() expected error for missing file")
	}
}

func TestValidate_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("name: test\nbadfield: invalid"), 0644)
	if err := Validate(path); err == nil {
		t.Fatal("Validate() expected error for schema violation")
	}
}

func TestContains_false(t *testing.T) {
	setupConfig(t)
	if Contains("absent") {
		t.Error("Contains(\"absent\") = true, want false")
	}
}

func TestWriteFile_createsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.raid.yaml")
	draft := ProfileDraft{Name: "written"}
	if err := WriteFile(draft, path); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "written") {
		t.Errorf("WriteFile() output does not contain profile name: %s", data)
	}
}

func TestCollectRepos_noRepos(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("n\n"))
	repos := CollectRepos(reader)
	if len(repos) != 0 {
		t.Errorf("CollectRepos() = %d repos, want 0", len(repos))
	}
}

func TestCreateRepoConfigs_createsFile(t *testing.T) {
	dir := t.TempDir()
	CreateRepoConfigs([]RepoDraft{
		{Name: "my-repo", URL: "https://example.com", Path: dir, Branch: "main"},
	})
	if _, err := os.Stat(filepath.Join(dir, "raid.yaml")); err != nil {
		t.Errorf("CreateRepoConfigs() did not create raid.yaml: %v", err)
	}
}
