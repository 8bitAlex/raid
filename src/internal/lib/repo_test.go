package lib

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		// Local-only repos (no url) are valid as long as name and path
		// are set — the URL check was dropped to support local-only setups.
		{"name and path (local-only)", Repo{Name: "test", Path: "/path"}, false},
		{"name and url (no path)", Repo{Name: "test", URL: "http://example.com"}, true},
		{"path and url (no name)", Repo{Path: "/path", URL: "http://example.com"}, true},
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

func TestRepoIsLocalOnly(t *testing.T) {
	tests := []struct {
		name string
		repo Repo
		want bool
	}{
		{"empty url", Repo{Name: "x", Path: "/p"}, true},
		{"whitespace-only url", Repo{Name: "x", Path: "/p", URL: "   "}, true},
		{"tabs and newlines", Repo{Name: "x", Path: "/p", URL: "\t\n "}, true},
		{"padded url is remote", Repo{Name: "x", Path: "/p", URL: "  git@example.com:x.git  "}, false},
		{"with url", Repo{Name: "x", Path: "/p", URL: "git@example.com:x.git"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.repo.IsLocalOnly(); got != tt.want {
				t.Errorf("IsLocalOnly() = %v, want %v", got, tt.want)
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

func TestCloneRepository_localOnly_pathExists(t *testing.T) {
	dir := t.TempDir()
	// Local-only repo (no URL) at an existing path: must skip clone with no
	// error so install tasks can run against the directory as-is.
	repo := Repo{Name: "local", Path: dir}
	if err := CloneRepository(repo); err != nil {
		t.Errorf("CloneRepository() local-only at existing path: %v", err)
	}
}

func TestCloneRepository_localOnly_missingPath(t *testing.T) {
	repo := Repo{Name: "local", Path: filepath.Join(t.TempDir(), "missing")}
	err := CloneRepository(repo)
	if err == nil {
		t.Fatal("CloneRepository() local-only with missing path: expected error")
	}
	if !strings.Contains(err.Error(), "no url") {
		t.Errorf("error should mention missing url, got: %v", err)
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
	// badfield violates additionalProperties:false in the repo schema.
	os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte("name: test\nbranch: main\nbadfield: invalid"), 0644)

	repo := Repo{Name: "test", Path: dir, URL: "http://x.com"}
	if err := buildRepo(&repo); err == nil {
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

func TestValidateRepo_schemaViolation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raid.yaml")
	// badfield violates additionalProperties:false in the repo schema.
	os.WriteFile(path, []byte("name: test\nbranch: main\nbadfield: invalid"), 0644)

	if err := ValidateRepo(path); err == nil {
		t.Fatal("ValidateRepo() expected error for schema violation")
	}
}

// TestValidateRepo_verify exercises the repo schema's top-level
// `verify:` block: that valid entries pass, and that entries missing
// `name` or `tasks` are rejected.
func TestValidateRepo_verify(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{
			name: "verify accepted with name, tasks, and onFail",
			body: `name: r
branch: main
verify:
  - name: Go installed
    tasks:
      - type: Shell
        cmd: go version
    onFail:
      - type: Shell
        cmd: brew install go
`,
		},
		{
			name: "verify rejected when name is missing",
			body: `name: r
branch: main
verify:
  - tasks:
      - type: Shell
        cmd: go version
`,
			wantErr: true,
		},
		{
			name: "verify rejected when tasks is missing",
			body: `name: r
branch: main
verify:
  - name: Go installed
`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "raid.yaml")
			if err := os.WriteFile(path, []byte(tt.body), 0644); err != nil {
				t.Fatalf("write repo: %v", err)
			}
			err := ValidateRepo(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRepo() err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateRepo_agentBlockAccepted verifies the shared command schema
// reaches the repo validator — `agent:` on a command inside raid.yaml is
// accepted via the same $ref the profile schema uses.
func TestValidateRepo_agentBlockAccepted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "raid.yaml")
	body := `name: r
branch: main
commands:
  - name: smoke
    usage: Smoke test
    tasks:
      - type: Shell
        cmd: curl -fsS localhost:8080/healthz
    agent:
      safe: true
      reads: ["./internal/health/**"]
      description: "Idempotent smoke test"
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateRepo(path); err != nil {
		t.Errorf("ValidateRepo() unexpected err: %v", err)
	}
}
