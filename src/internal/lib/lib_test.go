package lib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// repoRoot walks up from the package directory to find the repository root
// (identified by a schemas/ subdirectory).
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for {
		if fi, err := os.Stat(filepath.Join(dir, "schemas")); err == nil && fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no schemas/ dir found)")
		}
		dir = parent
	}
}

func TestConditionIsZero(t *testing.T) {
	tests := []struct {
		name string
		c    Condition
		want bool
	}{
		{"all empty", Condition{}, true},
		{"platform set", Condition{Platform: "linux"}, false},
		{"exists set", Condition{Exists: "/tmp"}, false},
		{"cmd set", Condition{Cmd: "exit 0"}, false},
		{"all set", Condition{Platform: "linux", Exists: "/tmp", Cmd: "exit 0"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoad_noContext(t *testing.T) {
	setupTestConfig(t)

	if err := Load(); err != nil {
		t.Errorf("Load() error: %v", err)
	}
}

func TestLoad_withExistingContext(t *testing.T) {
	setupTestConfig(t)

	context = &Context{Profile: Profile{Name: "test", Path: "/path"}}

	if err := Load(); err != nil {
		t.Errorf("Load() with existing context error: %v", err)
	}
	// Should not reload — cached context must be preserved.
	if context.Profile.Name != "test" {
		t.Errorf("Load() modified existing context unexpectedly")
	}
}

func TestForceLoad_buildProfileError(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.yaml")
	// badfield violates additionalProperties:false in the profile schema.
	os.WriteFile(profilePath, []byte("name: test\nbadfield: invalid"), 0644)

	if err := AddProfile(Profile{Name: "test", Path: profilePath}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := SetProfile("test"); err != nil {
		t.Fatalf("SetProfile() error: %v", err)
	}

	if err := ForceLoad(); err == nil {
		t.Fatal("ForceLoad() expected error when buildProfile fails")
	}
}

func TestForceLoad_buildRepoError(t *testing.T) {
	root := repoRoot(t)
	setupTestConfig(t)

	dir := t.TempDir()
	repoDir := filepath.Join(dir, "repo")
	os.MkdirAll(repoDir, 0755)

	// Invalid raid.yaml: missing "branch" (required) and has extra field (additionalProperties:false).
	os.WriteFile(filepath.Join(repoDir, RaidConfigFileName), []byte("name: myrepo\nextrafield: bad"), 0644)

	profilePath := filepath.Join(dir, "profile.yaml")
	content := "name: buildrepoerr\nrepositories:\n  - name: myrepo\n    path: " + repoDir + "\n    url: http://example.com/repo.git\n"
	os.WriteFile(profilePath, []byte(content), 0644)

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	if err := AddProfile(Profile{Name: "buildrepoerr", Path: profilePath}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := SetProfile("buildrepoerr"); err != nil {
		t.Fatalf("SetProfile() error: %v", err)
	}

	if err := ForceLoad(); err == nil {
		t.Fatal("ForceLoad() expected error when buildRepo fails")
	}
}

func TestForceLoad_noActiveProfile(t *testing.T) {
	setupTestConfig(t)

	if err := ForceLoad(); err != nil {
		t.Errorf("ForceLoad() with no active profile error: %v", err)
	}
	if context == nil {
		t.Fatal("ForceLoad() did not set context")
	}
}

func TestForceLoad_withValidProfile(t *testing.T) {
	root := repoRoot(t)
	setupTestConfig(t)

	// Build a valid profile file in a temp directory with a fake git repo inside.
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)

	profilePath := filepath.Join(dir, "profile.yaml")
	content := "name: testprofile\nrepositories:\n  - name: myrepo\n    path: " + repoDir + "\n    url: http://example.com/repo.git\n"
	if err := os.WriteFile(profilePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Schema resolution requires the CWD to be the repo root.
	wd, _ := os.Getwd()
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)

	if err := AddProfile(Profile{Name: "testprofile", Path: profilePath}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := SetProfile("testprofile"); err != nil {
		t.Fatalf("SetProfile() error: %v", err)
	}

	if err := ForceLoad(); err != nil {
		t.Errorf("ForceLoad() with valid profile error: %v", err)
	}
	if context == nil || context.Profile.Name != "testprofile" {
		t.Errorf("ForceLoad() context = %v, want profile named testprofile", context)
	}
}

func TestValidateSchema_missingFile(t *testing.T) {
	err := ValidateSchema("/nonexistent/file.yaml", "/nonexistent/schema.json")
	if err == nil {
		t.Fatal("ValidateSchema() expected error for missing file")
	}
}

func TestValidateSchema_missingSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.yaml")
	os.WriteFile(path, []byte("name: test"), 0644)

	err := ValidateSchema(path, "/nonexistent/schema.json")
	if err == nil {
		t.Fatal("ValidateSchema() expected error for missing schema")
	}
}

func TestValidateSchema_emptyPaths(t *testing.T) {
	err := ValidateSchema("", "")
	if err == nil {
		t.Fatal("ValidateSchema() expected error for empty paths")
	}
}

func TestValidateSchema_badSchemaFile(t *testing.T) {
	dir := t.TempDir()
	dataPath := filepath.Join(dir, "file.json")
	os.WriteFile(dataPath, []byte(`{"name": "test"}`), 0644)

	schemaPath := filepath.Join(dir, "bad-schema.json")
	os.WriteFile(schemaPath, []byte(`not valid json`), 0644)

	err := ValidateSchema(dataPath, schemaPath)
	if err == nil {
		t.Fatal("ValidateSchema() expected error for malformed schema file")
	}
}

func TestValidateSchema_invalidJSONContent(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`), 0644)

	dataPath := filepath.Join(dir, "bad.json")
	os.WriteFile(dataPath, []byte(`not valid json`), 0644)

	err := ValidateSchema(dataPath, schemaPath)
	if err == nil {
		t.Fatal("ValidateSchema() expected error for invalid JSON content")
	}
}

func TestValidateSchema_invalidYAMLContent(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`), 0644)

	dataPath := filepath.Join(dir, "invalid.yaml")
	os.WriteFile(dataPath, []byte("key: [unclosed"), 0644)

	err := ValidateSchema(dataPath, schemaPath)
	if err == nil {
		t.Fatal("ValidateSchema() expected error for invalid YAML content")
	}
}

func TestValidateSchema_validYAML(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`), 0644)

	dataPath := filepath.Join(dir, "profile.yaml")
	os.WriteFile(dataPath, []byte("name: myprofile"), 0644)

	if err := ValidateSchema(dataPath, schemaPath); err != nil {
		t.Errorf("ValidateSchema() on valid YAML error: %v", err)
	}
}

func TestValidateSchema_validJSON(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`), 0644)

	dataPath := filepath.Join(dir, "profile.json")
	os.WriteFile(dataPath, []byte(`{"name":"myprofile"}`), 0644)

	if err := ValidateSchema(dataPath, schemaPath); err != nil {
		t.Errorf("ValidateSchema() on valid JSON error: %v", err)
	}
}

func TestValidateSchema_schemaViolation(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	// Require "name" field and disallow additional properties.
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["name"],"additionalProperties":false,"properties":{"name":{"type":"string"}}}`), 0644)

	dataPath := filepath.Join(dir, "bad.yaml")
	os.WriteFile(dataPath, []byte("unknown: field"), 0644)

	err := ValidateSchema(dataPath, schemaPath)
	if err == nil {
		t.Fatal("ValidateSchema() expected error for schema violation")
	}
}

func TestInstall_noProfile(t *testing.T) {
	setupTestConfig(t)

	context = &Context{Profile: Profile{}}

	err := Install(1)
	if err == nil {
		t.Fatal("Install() expected error when profile is zero")
	}
}

func TestInstall_noRepos(t *testing.T) {
	setupTestConfig(t)

	context = &Context{
		Profile: Profile{Name: "test", Path: "/path"},
	}

	if err := Install(0); err != nil {
		t.Errorf("Install() with no repos error: %v", err)
	}
}

func TestInstall_withSemaphoreAndExistingRepo(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	// Fake git repo — CloneRepository will skip cloning.
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Repositories: []Repo{
				{Name: "repo1", Path: dir, URL: "http://example.com"},
			},
		},
	}

	// maxThreads=1 exercises the semaphore acquisition/release paths.
	if err := Install(1); err != nil {
		t.Errorf("Install() with semaphore and existing repo error: %v", err)
	}
}

func TestInstall_cloneFailure(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Repositories: []Repo{
				// Non-existent local path causes git to fail immediately.
				{Name: "repo1", Path: filepath.Join(dir, "newrepo"), URL: "file:///nonexistent/repo.git"},
			},
		},
	}

	err := Install(0)
	if err == nil {
		t.Fatal("Install() expected error for failed clone")
	}
}

func TestInstall_installTaskFailure(t *testing.T) {
	setupTestConfig(t)

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Install: OnInstall{
				Tasks: []Task{{Type: Shell, Cmd: "exit 1"}},
			},
		},
	}

	err := Install(0)
	if err == nil {
		t.Fatal("Install() expected error from failing install task")
	}
}

func TestInstall_repoInstallTaskFailure(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Repositories: []Repo{
				{
					Name: "repo1",
					Path: dir,
					URL:  "http://example.com",
					Install: OnInstall{
						Tasks: []Task{{Type: Shell, Cmd: "exit 1"}},
					},
				},
			},
		},
	}

	err := Install(0)
	if err == nil {
		t.Fatal("Install() expected error from failing repo install task")
	}
}

func TestInstallRepo_notFound(t *testing.T) {
	setupTestConfig(t)
	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Repositories: []Repo{
				{Name: "repo1", Path: t.TempDir(), URL: "http://example.com"},
			},
		},
	}

	err := InstallRepo("doesnotexist")
	if err == nil {
		t.Fatal("InstallRepo() expected error for unknown repo name")
	}
	if !strings.Contains(err.Error(), "doesnotexist") {
		t.Errorf("error %q should mention the missing repo name", err.Error())
	}
}

func TestInstallRepo_clonesAndRunsTasks(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	// Pre-existing .git dir skips the actual clone.
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	// Save and restore raidVars so this test doesn't bleed into others.
	raidVarsMu.Lock()
	saved := raidVars
	raidVars = map[string]string{}
	raidVarsMu.Unlock()
	t.Cleanup(func() {
		raidVarsMu.Lock()
		raidVars = saved
		raidVarsMu.Unlock()
	})

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Repositories: []Repo{
				{
					Name: "target",
					Path: dir,
					URL:  "http://example.com",
					Install: OnInstall{
						Tasks: []Task{{Type: SetVar, Var: "INSTALL_RAN", Value: "yes"}},
					},
				},
				// Second repo — should NOT be installed.
				{Name: "other", Path: t.TempDir(), URL: "http://example.com"},
			},
		},
	}

	if err := InstallRepo("target"); err != nil {
		t.Fatalf("InstallRepo() error: %v", err)
	}
	raidVarsMu.RLock()
	val := raidVars["INSTALL_RAN"]
	raidVarsMu.RUnlock()
	if val != "yes" {
		t.Error("install task for target repo did not run")
	}
}

func TestInstallRepo_doesNotRunProfileTasks(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	// Save and restore raidVars so this test doesn't bleed into others.
	raidVarsMu.Lock()
	saved := raidVars
	raidVars = map[string]string{}
	raidVarsMu.Unlock()
	t.Cleanup(func() {
		raidVarsMu.Lock()
		raidVars = saved
		raidVarsMu.Unlock()
	})

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Install: OnInstall{
				Tasks: []Task{{Type: SetVar, Var: "PROFILE_RAN", Value: "yes"}},
			},
			Repositories: []Repo{
				{Name: "repo1", Path: dir, URL: "http://example.com"},
			},
		},
	}

	if err := InstallRepo("repo1"); err != nil {
		t.Fatalf("InstallRepo() error: %v", err)
	}
	raidVarsMu.RLock()
	_, set := raidVars["PROFILE_RAN"]
	raidVarsMu.RUnlock()
	if set {
		t.Error("InstallRepo() should not run profile-level install tasks")
	}
}

func TestInstallRepo_noContext(t *testing.T) {
	setupTestConfig(t)
	context = nil

	if err := InstallRepo("any"); err == nil {
		t.Fatal("InstallRepo() expected error when context is nil")
	}
}

func TestInstallRepo_noProfile(t *testing.T) {
	setupTestConfig(t)
	context = &Context{Profile: Profile{}}

	if err := InstallRepo("any"); err == nil {
		t.Fatal("InstallRepo() expected error when profile is zero")
	}
}

func TestForceLoad_mergesRepoCommands(t *testing.T) {
	root := repoRoot(t)
	setupTestConfig(t)

	dir := t.TempDir()
	repoDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)

	// repo raid.yaml defines a command "repo-cmd" alongside the required fields.
	repoYAML := "name: myrepo\nbranch: main\ncommands:\n  - name: repo-cmd\n    tasks:\n      - type: Shell\n        cmd: exit 0\n"
	os.WriteFile(filepath.Join(repoDir, RaidConfigFileName), []byte(repoYAML), 0644)

	profileContent := "name: mergetest\nrepositories:\n  - name: myrepo\n    path: " + repoDir + "\n    url: http://example.com/repo.git\ncommands:\n  - name: profile-cmd\n    tasks:\n      - type: Shell\n        cmd: exit 0\n"
	profilePath := filepath.Join(dir, "profile.yaml")
	os.WriteFile(profilePath, []byte(profileContent), 0644)

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	if err := AddProfile(Profile{Name: "mergetest", Path: profilePath}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := SetProfile("mergetest"); err != nil {
		t.Fatalf("SetProfile() error: %v", err)
	}

	if err := ForceLoad(); err != nil {
		t.Fatalf("ForceLoad() error: %v", err)
	}

	cmds := GetCommands()
	names := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		names[c.Name] = true
	}
	if !names["profile-cmd"] {
		t.Error("GetCommands() missing 'profile-cmd' from profile")
	}
	if !names["repo-cmd"] {
		t.Error("GetCommands() missing 'repo-cmd' merged from repo")
	}
}

// --- ResetContext ---

func TestResetContext_nilsContext(t *testing.T) {
	setupTestConfig(t)
	context = &Context{Profile: Profile{Name: "foo"}}
	ResetContext()
	if context != nil {
		t.Errorf("ResetContext() did not nil context, got %v", context)
	}
}

// --- initConfigReadOnly ---

func TestInitConfigReadOnly_fileAbsent(t *testing.T) {
	setupTestConfig(t)
	// Point CfgPath at a non-existent file.
	CfgPath = filepath.Join(t.TempDir(), "absent.toml")

	ok := initConfigReadOnly()
	if ok {
		t.Error("initConfigReadOnly() = true for absent file, want false")
	}
}

func TestInitConfigReadOnly_validFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")

	oldCfgPath := CfgPath
	t.Cleanup(func() {
		CfgPath = oldCfgPath
		viper.Reset()
	})
	CfgPath = cfgPath

	// Create the file first so InitConfig works, then test read-only load.
	if err := InitConfig(); err != nil {
		t.Fatalf("InitConfig() error: %v", err)
	}
	viper.Reset()

	CfgPath = cfgPath
	ok := initConfigReadOnly()
	if !ok {
		t.Error("initConfigReadOnly() = false for existing valid config, want true")
	}
}

// --- QuietLoad ---

func TestQuietLoad_noConfigFile(t *testing.T) {
	oldCfgPath := CfgPath
	t.Cleanup(func() {
		CfgPath = oldCfgPath
		viper.Reset()
	})
	CfgPath = filepath.Join(t.TempDir(), "nonexistent.toml")

	cmds := QuietLoad()
	if cmds != nil {
		t.Errorf("QuietLoad() = %v, want nil when config absent", cmds)
	}
}

func TestQuietLoad_configExistsNoProfile(t *testing.T) {
	setupTestConfig(t)

	// Config exists but no profile is set — ForceLoad will not fail fatally,
	// but GetCommands returns empty when context has no commands.
	cmds := QuietLoad()
	// Nil or empty are both acceptable; we just verify no panic.
	_ = cmds
}

func TestQuietLoad_withProfile(t *testing.T) {
	root := repoRoot(t)
	setupTestConfig(t)

	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.yaml")
	os.WriteFile(profilePath, []byte("name: quiet-test\ncommands:\n  - name: mycmd\n    usage: My command\n    tasks:\n      - type: Shell\n        cmd: exit 0\n"), 0644)

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	if err := AddProfile(Profile{Name: "quiet-test", Path: profilePath}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := SetProfile("quiet-test"); err != nil {
		t.Fatalf("SetProfile() error: %v", err)
	}

	cmds := QuietLoad()
	names := make(map[string]bool)
	for _, c := range cmds {
		names[c.Name] = true
	}
	if !names["mycmd"] {
		t.Errorf("QuietLoad() missing 'mycmd', got: %v", cmds)
	}
}

// --- getPath ---

func TestGetPath_emptyCfgPath(t *testing.T) {
	old := CfgPath
	t.Cleanup(func() { CfgPath = old })
	CfgPath = ""
	path := getPath()
	if path == "" {
		t.Error("getPath() returned empty string when CfgPath is empty")
	}
	if CfgPath == "" {
		t.Error("getPath() did not set CfgPath when it was empty")
	}
}

// --- loadRaidVars ---

func TestLoadRaidVars_invalidFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "vars")
	// Write content that godotenv cannot parse as valid dotenv.
	os.WriteFile(p, []byte("===invalid==="), 0644)

	old := raidVarsOverridePath
	t.Cleanup(func() { raidVarsOverridePath = old })
	raidVarsOverridePath = p

	// Should not panic; error is printed to stderr and silently swallowed.
	loadRaidVars()
}

// --- validateFile (empty YAML) ---

func TestValidateSchema_emptyYAML(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`), 0644)

	dataPath := filepath.Join(dir, "empty.yaml")
	os.WriteFile(dataPath, []byte(""), 0644)

	err := ValidateSchema(dataPath, schemaPath)
	if err == nil {
		t.Fatal("ValidateSchema() expected error for empty YAML file")
	}
}

func TestValidateSchema_validJSONArray(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`), 0644)

	dataPath := filepath.Join(dir, "data.json")
	os.WriteFile(dataPath, []byte(`[{"name":"a"},{"name":"b"}]`), 0644)

	if err := ValidateSchema(dataPath, schemaPath); err != nil {
		t.Errorf("ValidateSchema() on valid JSON array: %v", err)
	}
}

func TestValidateSchema_invalidJSONArrayElement(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["name"],"additionalProperties":false,"properties":{"name":{"type":"string"}}}`), 0644)

	dataPath := filepath.Join(dir, "data.json")
	os.WriteFile(dataPath, []byte(`[{"name":"ok"},{"bad":"field"}]`), 0644)

	err := ValidateSchema(dataPath, schemaPath)
	if err == nil {
		t.Fatal("ValidateSchema() expected error for invalid JSON array element")
	}
}

// --- validateWithEmbeddedSchema ---

func TestValidateWithEmbeddedSchema_missingFile(t *testing.T) {
	err := validateWithEmbeddedSchema("/nonexistent/file.yaml", "raid-profile.schema.json")
	if err == nil {
		t.Fatal("validateWithEmbeddedSchema: expected error for missing file")
	}
}

func TestValidateWithEmbeddedSchema_emptyPath(t *testing.T) {
	err := validateWithEmbeddedSchema("", "raid-profile.schema.json")
	if err == nil {
		t.Fatal("validateWithEmbeddedSchema: expected error for empty path")
	}
}

func TestValidateWithEmbeddedSchema_validProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	os.WriteFile(path, []byte("name: test\n"), 0644)

	err := validateWithEmbeddedSchema(path, "raid-profile.schema.json")
	if err != nil {
		t.Errorf("validateWithEmbeddedSchema valid: %v", err)
	}
}

func TestValidateWithEmbeddedSchema_invalidProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("notaname: test\nextra: bad\n"), 0644)

	err := validateWithEmbeddedSchema(path, "raid-profile.schema.json")
	if err == nil {
		t.Fatal("validateWithEmbeddedSchema: expected error for invalid profile")
	}
}

// --- validateFile multi-doc YAML ---

func TestValidateSchema_multiDocYAML(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`), 0644)

	dataPath := filepath.Join(dir, "multi.yaml")
	os.WriteFile(dataPath, []byte("name: first\n---\nname: second\n"), 0644)

	if err := ValidateSchema(dataPath, schemaPath); err != nil {
		t.Errorf("ValidateSchema() on valid multi-doc YAML: %v", err)
	}
}

func TestValidateSchema_multiDocYAML_invalidSecond(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	os.WriteFile(schemaPath, []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","required":["name"],"additionalProperties":false,"properties":{"name":{"type":"string"}}}`), 0644)

	dataPath := filepath.Join(dir, "multi.yaml")
	os.WriteFile(dataPath, []byte("name: first\n---\nbad: field\n"), 0644)

	err := ValidateSchema(dataPath, schemaPath)
	if err == nil {
		t.Fatal("ValidateSchema() expected error for invalid second YAML document")
	}
}

// --- session ---

func TestStartEndSession(t *testing.T) {
	startSession()
	if commandSession == nil {
		t.Fatal("startSession() did not create session")
	}
	if commandSession.vars == nil || commandSession.baseline == nil {
		t.Fatal("startSession() session has nil maps")
	}
	endSession()
	if commandSession != nil {
		t.Fatal("endSession() did not clear session")
	}
}

// --- expandRaid with session ---

func TestExpandRaid_sessionVarLookup(t *testing.T) {
	startSession()
	defer endSession()

	commandSession.mu.Lock()
	commandSession.vars["RAID_EXPAND_SESS"] = "from-session"
	commandSession.mu.Unlock()

	got := expandRaid("$RAID_EXPAND_SESS")
	if got != "from-session" {
		t.Errorf("expandRaid session lookup = %q, want %q", got, "from-session")
	}
}

// --- raidVarsPath ---

func TestRaidVarsPath_overridePath(t *testing.T) {
	old := raidVarsOverridePath
	raidVarsOverridePath = "/custom/path/vars"
	t.Cleanup(func() { raidVarsOverridePath = old })

	got := raidVarsPath()
	if got != "/custom/path/vars" {
		t.Errorf("raidVarsPath() = %q, want %q", got, "/custom/path/vars")
	}
}

func TestRaidVarsPath_default(t *testing.T) {
	old := raidVarsOverridePath
	raidVarsOverridePath = ""
	t.Cleanup(func() { raidVarsOverridePath = old })

	got := raidVarsPath()
	if got == "" {
		t.Error("raidVarsPath() returned empty string")
	}
}

// TestInstall_nilContext tests the early return branch when context is nil.
func TestInstall_nilContext(t *testing.T) {
	setupTestConfig(t)
	context = nil

	if err := Install(0); err == nil {
		t.Error("Install() expected error when context is nil")
	}
}

// TestQuietLoad_forceLoadFails covers QuietLoad when ForceLoad returns an error.
func TestQuietLoad_forceLoadFails(t *testing.T) {
	root := repoRoot(t)
	setupTestConfig(t)

	// Set up a profile that references a bad repo config to cause ForceLoad to fail.
	dir := t.TempDir()
	repoDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(repoDir, 0755)
	// Invalid raid.yaml: violates schema.
	os.WriteFile(filepath.Join(repoDir, RaidConfigFileName), []byte("invalid: [unclosed"), 0644)

	profilePath := filepath.Join(dir, "profile.yaml")
	content := "name: qlfail\nrepositories:\n  - name: myrepo\n    path: " + repoDir + "\n    url: http://example.com/repo.git\n"
	os.WriteFile(profilePath, []byte(content), 0644)

	if err := AddProfile(Profile{Name: "qlfail", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("qlfail"); err != nil {
		t.Fatal(err)
	}

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	ResetContext()
	cmds := QuietLoad()
	// Should return nil when ForceLoad fails.
	if cmds != nil {
		t.Errorf("QuietLoad with ForceLoad failure = %v, want nil", cmds)
	}
}

// TestForceLoad_withGroups covers the loop over profile.Groups.
func TestForceLoad_withGroups(t *testing.T) {
	root := repoRoot(t)
	setupTestConfig(t)

	dir := t.TempDir()
	profilePath := filepath.Join(dir, "groups.yaml")
	content := "name: groups\ntask_groups:\n  build:\n    - type: Shell\n      cmd: echo build\n  test:\n    - type: Shell\n      cmd: echo test\n"
	os.WriteFile(profilePath, []byte(content), 0644)

	if err := AddProfile(Profile{Name: "groups", Path: profilePath}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("groups"); err != nil {
		t.Fatal(err)
	}

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	if err := ForceLoad(); err != nil {
		t.Fatalf("ForceLoad with groups: %v", err)
	}
	if context == nil || len(context.Profile.Groups) != 2 {
		t.Errorf("ForceLoad: expected 2 groups, got %v", context)
	}
}

// TestInstallRepo_installTaskError covers the installRepo error path when
// install tasks fail (ExecuteTasks returns an error).
func TestInstallRepo_installTaskError(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	context = &Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Repositories: []Repo{
				{
					Name: "repo1",
					Path: dir,
					URL:  "http://example.com",
					Install: OnInstall{
						Tasks: []Task{{Type: Shell, Cmd: "exit 1"}},
					},
				},
			},
		},
	}
	defer func() { context = nil }()

	err := InstallRepo("repo1")
	if err == nil {
		t.Fatal("InstallRepo expected error from failing install task")
	}
	if !strings.Contains(err.Error(), "failed to execute install tasks") {
		t.Errorf("error %q should mention install task failure", err.Error())
	}
}
