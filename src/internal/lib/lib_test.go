package lib

import (
	"os"
	"path/filepath"
	"testing"
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
	os.WriteFile(profilePath, []byte("name: test"), 0644)

	AddProfile(Profile{Name: "test", Path: profilePath})
	SetProfile("test")

	// profileSchemaPath is relative; not found from test CWD → buildProfile fails.
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

	AddProfile(Profile{Name: "buildrepoerr", Path: profilePath})
	SetProfile("buildrepoerr")

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

	AddProfile(Profile{Name: "testprofile", Path: profilePath})
	SetProfile("testprofile")

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
