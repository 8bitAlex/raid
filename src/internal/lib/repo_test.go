package lib

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRepoIsZero(t *testing.T) {
	tests := []struct {
		name string
		repo Repo
		want bool
	}{
		{"empty", Repo{}, true},
		{"name only", Repo{Name: "test"}, true},
		{"name and path", Repo{Name: "test", Path: "/path"}, true},
		{"name and url", Repo{Name: "test", URL: "http://example.com"}, true},
		{"path and url", Repo{Path: "/path", URL: "http://example.com"}, true},
		{"all three fields", Repo{Name: "test", Path: "/path", URL: "http://example.com"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.repo.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepoGetEnv(t *testing.T) {
	repo := Repo{
		Environments: []Env{
			{Name: "dev"},
			{Name: "prod"},
		},
	}

	t.Run("found", func(t *testing.T) {
		env := repo.getEnv("dev")
		if env.Name != "dev" {
			t.Errorf("getEnv(\"dev\") = %q, want \"dev\"", env.Name)
		}
	})

	t.Run("not found returns zero", func(t *testing.T) {
		env := repo.getEnv("staging")
		if !env.IsZero() {
			t.Errorf("getEnv(\"staging\") should return zero Env, got %v", env)
		}
	})
}

func TestExtractRepo_validYAML(t *testing.T) {
	dir := t.TempDir()
	content := "environments:\n  - name: dev\n"
	if err := os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := ExtractRepo(dir)
	if err != nil {
		t.Fatalf("ExtractRepo() error: %v", err)
	}
	if len(repo.Environments) != 1 || repo.Environments[0].Name != "dev" {
		t.Errorf("ExtractRepo() environments = %v, want [{dev}]", repo.Environments)
	}
}

func TestExtractRepo_invalidYAML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte("key: [unclosed"), 0644)

	_, err := ExtractRepo(dir)
	if err == nil {
		t.Fatal("ExtractRepo() expected error for invalid YAML")
	}
}

func TestExtractRepo_missingFile(t *testing.T) {
	_, err := ExtractRepo("/nonexistent/path")
	if err == nil {
		t.Fatal("ExtractRepo() expected error for missing directory")
	}
}

func TestCloneRepository_alreadyExists(t *testing.T) {
	dir := t.TempDir()
	// Simulate an existing git repository.
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	repo := Repo{Name: "test", Path: dir, URL: "http://example.com"}
	if err := CloneRepository(repo); err != nil {
		t.Errorf("CloneRepository() on existing repo should skip, got error: %v", err)
	}
}

func TestCloneRepository_pathExistsNotGitRepo(t *testing.T) {
	dir := t.TempDir()
	// Directory exists but is not a git repo, and no network URL is reachable.
	// Use a local file:// URL pointing to a nonexistent path so git fails fast.
	repo := Repo{Name: "test", Path: filepath.Join(dir, "newrepo"), URL: "file:///nonexistent/repo.git"}

	err := CloneRepository(repo)
	if err == nil {
		t.Fatal("CloneRepository() expected error for failed clone")
	}
}

func TestBuildRepo_zero(t *testing.T) {
	repo := Repo{}
	err := buildRepo(&repo)
	if err == nil {
		t.Fatal("buildRepo() expected error for zero repo")
	}
}

func TestBuildRepo_noRaidYAML(t *testing.T) {
	dir := t.TempDir()
	repo := Repo{Name: "test", Path: dir, URL: "http://x.com"}
	if err := buildRepo(&repo); err != nil {
		t.Errorf("buildRepo() with no raid.yaml should return nil, got: %v", err)
	}
}

func TestBuildRepo_validationError(t *testing.T) {
	dir := t.TempDir()
	// Create raid.yaml; schema not found from test CWD → ValidateRepo returns error.
	os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte("name: test\nbranch: main"), 0644)

	repo := Repo{Name: "test", Path: dir, URL: "http://x.com"}
	err := buildRepo(&repo)
	if err == nil {
		t.Fatal("buildRepo() expected error when schema validation fails")
	}
}

func TestBuildRepo_validRaidYAML(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()

	// Valid raid.yaml: repo schema requires "name" and "branch".
	os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte("name: myrepo\nbranch: main"), 0644)

	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	repo := Repo{Name: "myrepo", Path: dir, URL: "http://x.com"}
	if err := buildRepo(&repo); err != nil {
		t.Errorf("buildRepo() with valid raid.yaml error: %v", err)
	}
}

func TestCloneRepository_mkdirAllError(t *testing.T) {
	// Use a regular file as a parent directory — os.MkdirAll will fail with ENOTDIR.
	tmpFile, err := os.CreateTemp("", "raid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	repo := Repo{Name: "test", Path: filepath.Join(tmpFile.Name(), "subdir"), URL: "file:///nonexistent.git"}
	if err := CloneRepository(repo); err == nil {
		t.Fatal("CloneRepository() expected error when MkdirAll fails")
	}
}

func TestCloneRepository_successLocalRepo(t *testing.T) {
	if !isGitInstalled() {
		t.Skip("git not installed")
	}

	srcDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", srcDir).Run(); err != nil {
		t.Skipf("git init --bare failed: %v", err)
	}

	destDir := filepath.Join(t.TempDir(), "clone")
	repo := Repo{Name: "test", Path: destDir, URL: "file://" + srcDir}

	if err := CloneRepository(repo); err != nil {
		t.Errorf("CloneRepository() to local bare repo error: %v", err)
	}
}

func TestValidateRepo_schemaNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raid.yaml")
	os.WriteFile(path, []byte("name: test\nbranch: main"), 0644)

	// repoSchemaPath is relative to repo root; won't resolve from test CWD.
	err := ValidateRepo(path)
	if err == nil {
		t.Fatal("ValidateRepo() expected error when schema cannot be found from test CWD")
	}
}
