package lib

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProfileIsZero(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    bool
	}{
		{"empty profile", Profile{}, true},
		{"name only", Profile{Name: "test"}, true},
		{"path only", Profile{Path: "/some/path"}, true},
		{"name and path", Profile{Name: "test", Path: "/some/path"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.profile.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProfileGetEnv(t *testing.T) {
	profile := Profile{
		Environments: []Env{
			{Name: "dev", Variables: []EnvVar{{Name: "KEY", Value: "val"}}},
			{Name: "prod"},
		},
	}

	t.Run("found", func(t *testing.T) {
		env := profile.getEnv("dev")
		if env.Name != "dev" {
			t.Errorf("getEnv(\"dev\") = %q, want \"dev\"", env.Name)
		}
	})

	t.Run("not found returns zero", func(t *testing.T) {
		env := profile.getEnv("staging")
		if !env.IsZero() {
			t.Errorf("getEnv(\"staging\") should return zero Env, got %v", env)
		}
	})
}

func TestAddAndContainsProfile(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "myprofile", Path: "/some/path"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}

	if !ContainsProfile("myprofile") {
		t.Error("ContainsProfile() = false after AddProfile(), want true")
	}
}

func TestContainsProfile_notFound(t *testing.T) {
	setupTestConfig(t)

	if ContainsProfile("nonexistent") {
		t.Error("ContainsProfile() = true for nonexistent profile, want false")
	}
}

func TestListProfiles(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "list-a", Path: "/a"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := AddProfile(Profile{Name: "list-b", Path: "/b"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}

	profiles := ListProfiles()
	names := make(map[string]bool)
	for _, p := range profiles {
		names[p.Name] = true
	}
	if !names["list-a"] || !names["list-b"] {
		t.Errorf("ListProfiles() = %v, missing added profiles", profiles)
	}
}

func TestAddProfiles(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfiles([]Profile{
		{Name: "bulk-a", Path: "/a"},
		{Name: "bulk-b", Path: "/b"},
	}); err != nil {
		t.Fatalf("AddProfiles() error: %v", err)
	}

	if !ContainsProfile("bulk-a") || !ContainsProfile("bulk-b") {
		t.Error("AddProfiles() did not add all profiles")
	}
}

func TestRemoveProfile(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "toremove", Path: "/path"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := RemoveProfile("toremove"); err != nil {
		t.Fatalf("RemoveProfile() error: %v", err)
	}
	if ContainsProfile("toremove") {
		t.Error("ContainsProfile() = true after RemoveProfile(), want false")
	}
}

func TestRemoveProfile_notFound(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "existing", Path: "/path"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	err := RemoveProfile("nonexistent")
	if err == nil {
		t.Fatal("RemoveProfile() expected error for nonexistent profile")
	}
}

func TestRemoveProfile_noProfiles(t *testing.T) {
	setupTestConfig(t)

	err := RemoveProfile("anything")
	if err == nil {
		t.Fatal("RemoveProfile() on empty config should error")
	}
}

func TestSetProfile_notFound(t *testing.T) {
	setupTestConfig(t)

	err := SetProfile("nonexistent")
	if err == nil {
		t.Fatal("SetProfile() expected error for nonexistent profile")
	}
}

func TestSetAndGetProfile(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "active", Path: "/active/path"}); err != nil {
		t.Fatalf("AddProfile() error: %v", err)
	}
	if err := SetProfile("active"); err != nil {
		t.Fatalf("SetProfile() error: %v", err)
	}

	got := GetProfile()
	if got.Name != "active" {
		t.Errorf("GetProfile() name = %q, want %q", got.Name, "active")
	}
}

func TestGetProfile_fromContext(t *testing.T) {
	setupTestConfig(t)

	context = &Context{
		Profile: Profile{Name: "ctx-profile", Path: "/ctx/path"},
	}

	got := GetProfile()
	if got.Name != "ctx-profile" {
		t.Errorf("GetProfile() from context = %q, want %q", got.Name, "ctx-profile")
	}
}

func TestExtractProfiles_singleYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")

	data := Profile{Name: "yamltest", Path: path}
	b, _ := yaml.Marshal(data)
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "yamltest" {
		t.Errorf("ExtractProfiles() = %v, want single profile named yamltest", profiles)
	}
}

func TestExtractProfiles_multiDocYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")

	content := "name: first\n---\nname: second\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("ExtractProfiles() returned %d profiles, want 2", len(profiles))
	}
}

func TestExtractProfiles_singleJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")

	data, _ := json.Marshal(Profile{Name: "jsontest"})
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != "jsontest" {
		t.Errorf("ExtractProfiles() = %v, want profile named jsontest", profiles)
	}
}

func TestExtractProfiles_arrayJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.json")

	data, _ := json.Marshal([]Profile{{Name: "a"}, {Name: "b"}})
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	profiles, err := ExtractProfiles(path)
	if err != nil {
		t.Fatalf("ExtractProfiles() error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("ExtractProfiles() returned %d profiles, want 2", len(profiles))
	}
}

func TestExtractProfiles_invalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("{invalid json}"), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for invalid JSON")
	}
}

func TestExtractProfiles_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte("key: [unclosed"), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for invalid YAML")
	}
}

func TestExtractProfiles_unsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.xml")
	os.WriteFile(path, []byte("<profile/>"), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for unsupported extension")
	}
}

func TestExtractProfiles_fileNotFound(t *testing.T) {
	_, err := ExtractProfiles("/nonexistent/path/profile.yaml")
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for missing file")
	}
}

func TestExtractProfiles_emptyYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	os.WriteFile(path, []byte(""), 0644)

	_, err := ExtractProfiles(path)
	if err == nil {
		t.Fatal("ExtractProfiles() expected error for empty YAML (no profiles)")
	}
}

func TestExtractProfile_found(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")
	os.WriteFile(path, []byte("name: first\n---\nname: second\n"), 0644)

	p, err := ExtractProfile("first", path)
	if err != nil {
		t.Fatalf("ExtractProfile() error: %v", err)
	}
	if p.Name != "first" {
		t.Errorf("ExtractProfile() name = %q, want %q", p.Name, "first")
	}
}

func TestExtractProfile_notFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profiles.yaml")
	os.WriteFile(path, []byte("name: only\n"), 0644)

	_, err := ExtractProfile("nonexistent", path)
	if err == nil {
		t.Fatal("ExtractProfile() expected error for missing profile name")
	}
}

func TestExtractProfile_extractionError(t *testing.T) {
	_, err := ExtractProfile("anyname", "/nonexistent/path/profile.yaml")
	if err == nil {
		t.Fatal("ExtractProfile() expected error for missing file")
	}
}

func TestBuildProfile_zero(t *testing.T) {
	_, err := buildProfile(Profile{})
	if err == nil {
		t.Fatal("buildProfile() expected error for zero profile")
	}
}

func TestBuildProfile_fileNotFound(t *testing.T) {
	_, err := buildProfile(Profile{Name: "test", Path: "/nonexistent/path/profile.yaml"})
	if err == nil {
		t.Fatal("buildProfile() expected error when profile file not found")
	}
}

func TestBuildProfile_validationError(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.yaml")
	// badfield violates additionalProperties:false in the profile schema.
	os.WriteFile(profilePath, []byte("name: test\nbadfield: invalid"), 0644)

	_, err := buildProfile(Profile{Name: "test", Path: profilePath})
	if err == nil {
		t.Fatal("buildProfile() expected error when schema validation fails")
	}
}

func TestBuildProfile_extractionError(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()

	profilePath := filepath.Join(dir, "profile.yaml")
	// Valid per profile schema (only requires "name"), but we'll look for a different name.
	os.WriteFile(profilePath, []byte("name: actualname"), 0644)

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	_, err := buildProfile(Profile{Name: "wrongname", Path: profilePath})
	if err == nil {
		t.Fatal("buildProfile() expected error when profile name not found in file")
	}
}

func TestValidateProfile_schemaViolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	// badfield violates additionalProperties:false in the profile schema.
	os.WriteFile(path, []byte("name: test\nbadfield: invalid"), 0644)

	if err := ValidateProfile(path); err == nil {
		t.Fatal("ValidateProfile() expected error for schema violation")
	}
}

// --- WriteProfileFile ---

func TestWriteProfileFile_createsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.raid.yaml")

	if err := WriteProfileFile(ProfileDraft{Name: "test-profile"}, path); err != nil {
		t.Fatalf("WriteProfileFile() unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "yaml-language-server") {
		t.Error("WriteProfileFile(): missing schema comment")
	}
	if !strings.Contains(content, "name: test-profile") {
		t.Error("WriteProfileFile(): missing profile name in output")
	}
}

func TestWriteProfileFile_createsParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "test.raid.yaml")

	if err := WriteProfileFile(ProfileDraft{Name: "nested"}, path); err != nil {
		t.Fatalf("WriteProfileFile() unexpected error: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("WriteProfileFile(): file not found at %s: %v", path, err)
	}
}

func TestWriteProfileFile_mkdirAllError(t *testing.T) {
	file, err := os.CreateTemp("", "raid-lib-profile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	defer os.Remove(file.Name())

	path := filepath.Join(file.Name(), "subdir", "test.raid.yaml")
	if err := WriteProfileFile(ProfileDraft{Name: "x"}, path); err == nil {
		t.Fatal("WriteProfileFile(): expected error when parent path contains a file")
	}
}

// --- CollectRepos ---

// initRepoWithBranch creates a non-bare git repo with one empty commit on the
// given branch. ls-remote requires at least one object to return the symref.
func initRepoWithBranch(t *testing.T, branch string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "symbolic-ref", "HEAD", "refs/heads/" + branch},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "config", "commit.gpgSign", "false"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, cmd := range cmds {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			t.Fatalf("%v: %v", cmd, err)
		}
	}
	return dir
}

func TestCollectRepos_noRepos(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("n\n"))
	repos := CollectRepos(reader)
	if len(repos) != 0 {
		t.Errorf("CollectRepos(): got %d repos, want 0", len(repos))
	}
}

func TestCollectRepos_skipsWhenRequiredFieldMissing(t *testing.T) {
	// Answer yes but leave name blank — repo should be skipped.
	input := "y\n\nhttps://127.0.0.1:1/repo.git\n/some/path\nmain\nn\n"
	reader := bufio.NewReader(strings.NewReader(input))
	repos := CollectRepos(reader)
	if len(repos) != 0 {
		t.Errorf("CollectRepos(): expected skipped repo, got %d", len(repos))
	}
}

func TestCollectRepos_collectsCompleteRepo(t *testing.T) {
	// Use 127.0.0.1:1 so DetectGitDefaultBranch fails fast; branch is supplied manually.
	input := "y\nmy-repo\nhttps://127.0.0.1:1/repo.git\n/tmp/my-repo\nmain\nn\n"
	reader := bufio.NewReader(strings.NewReader(input))
	repos := CollectRepos(reader)
	if len(repos) != 1 {
		t.Fatalf("CollectRepos(): got %d repos, want 1", len(repos))
	}
	r := repos[0]
	if r.Name != "my-repo" {
		t.Errorf("Name: got %q, want %q", r.Name, "my-repo")
	}
	if r.Branch != "main" {
		t.Errorf("Branch: got %q, want %q", r.Branch, "main")
	}
}

func TestCollectRepos_usesDetectedBranch(t *testing.T) {
	repoDir := initRepoWithBranch(t, "trunk")

	// Leave branch input blank — should pick up "trunk" from the remote.
	input := "y\nmy-repo\nfile://" + repoDir + "\n/tmp/my-repo\n\nn\n"
	reader := bufio.NewReader(strings.NewReader(input))
	repos := CollectRepos(reader)
	if len(repos) != 1 {
		t.Fatalf("CollectRepos(): got %d repos, want 1", len(repos))
	}
	if repos[0].Branch != "trunk" {
		t.Errorf("Branch: got %q, want %q", repos[0].Branch, "trunk")
	}
}

// --- CreateRepoConfigs ---

func TestCreateRepoConfigs_createsConfig(t *testing.T) {
	dir := t.TempDir()
	CreateRepoConfigs([]RepoDraft{
		{Name: "my-repo", URL: "https://example.com", Path: dir, Branch: "main"},
	})

	data, err := os.ReadFile(filepath.Join(dir, "raid.yaml"))
	if err != nil {
		t.Fatalf("raid.yaml not created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "name: my-repo") {
		t.Error("raid.yaml: missing name field")
	}
	if !strings.Contains(content, "branch: main") {
		t.Error("raid.yaml: missing branch field")
	}
}

func TestCreateRepoConfigs_skipsExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "raid.yaml")
	original := "original content\n"
	os.WriteFile(configPath, []byte(original), 0644)

	CreateRepoConfigs([]RepoDraft{
		{Name: "my-repo", URL: "https://example.com", Path: dir, Branch: "main"},
	})

	data, _ := os.ReadFile(configPath)
	if string(data) != original {
		t.Error("CreateRepoConfigs(): overwrote existing raid.yaml")
	}
}

func TestCreateRepoConfigs_omitsBranchWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	CreateRepoConfigs([]RepoDraft{
		{Name: "no-branch", URL: "https://example.com", Path: dir, Branch: ""},
	})

	data, err := os.ReadFile(filepath.Join(dir, "raid.yaml"))
	if err != nil {
		t.Fatalf("raid.yaml not created: %v", err)
	}
	if strings.Contains(string(data), "branch:") {
		t.Error("CreateRepoConfigs(): wrote branch field when Branch is empty")
	}
}

func TestCreateRepoConfigs_mkdirAllError(t *testing.T) {
	file, err := os.CreateTemp("", "raid-lib-profile-test-*")
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	defer os.Remove(file.Name())

	// Should not panic — just print error and continue.
	CreateRepoConfigs([]RepoDraft{
		{Name: "x", URL: "https://example.com", Path: filepath.Join(file.Name(), "subdir"), Branch: "main"},
	})
}

func TestCreateRepoConfigs_writeError(t *testing.T) {
	dir := t.TempDir()
	// Make the repo directory read-only so os.WriteFile cannot create raid.yaml.
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	// Should not panic — just print error and continue.
	CreateRepoConfigs([]RepoDraft{
		{Name: "x", URL: "https://example.com", Path: dir, Branch: "main"},
	})
}
