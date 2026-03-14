package lib

import (
	"os"
	"path/filepath"
	"testing"
)

// --- RunDoctor ---

func TestRunDoctor_returnsFindings(t *testing.T) {
	setupTestConfig(t)

	findings := RunDoctor()
	if len(findings) == 0 {
		t.Fatal("RunDoctor(): expected at least one finding, got none")
	}
}

// --- checkGit ---

func TestCheckGit_notInstalled(t *testing.T) {
	// Hide git by clearing PATH so exec.Command("git", "--version") fails.
	old := os.Getenv("PATH")
	os.Setenv("PATH", "")
	t.Cleanup(func() { os.Setenv("PATH", old) })

	findings := checkGit()
	if len(findings) != 1 {
		t.Fatalf("checkGit(): got %d findings, want 1", len(findings))
	}
	if findings[0].Severity != SeverityError {
		t.Errorf("checkGit(): severity = %v, want SeverityError", findings[0].Severity)
	}
	if findings[0].Suggestion == "" {
		t.Error("checkGit(): missing suggestion when git not found")
	}
}

func TestCheckGit_installed(t *testing.T) {
	findings := checkGit()
	if len(findings) != 1 {
		t.Fatalf("checkGit(): got %d findings, want 1", len(findings))
	}
	if findings[0].Severity != SeverityOK {
		t.Errorf("checkGit(): severity = %v, want SeverityOK", findings[0].Severity)
	}
}

// --- checkProfile ---

func TestCheckProfile_noActiveProfile(t *testing.T) {
	setupTestConfig(t)

	findings := checkProfile()
	if len(findings) != 1 {
		t.Fatalf("checkProfile(): got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.Severity != SeverityError {
		t.Errorf("checkProfile(): severity = %v, want SeverityError", f.Severity)
	}
	if f.Suggestion == "" {
		t.Error("checkProfile(): missing suggestion for no-profile case")
	}
}

func TestCheckProfile_missingFile(t *testing.T) {
	setupTestConfig(t)

	if err := AddProfile(Profile{Name: "ghost", Path: "/nonexistent/ghost.raid.yaml"}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("ghost"); err != nil {
		t.Fatal(err)
	}

	findings := checkProfile()
	severities := severitySet(findings)
	if !severities[SeverityError] {
		t.Error("checkProfile(): expected an error finding for missing profile file")
	}
}

func TestCheckProfile_invalidSchema(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	os.WriteFile(path, []byte("name: test\nbadfield: invalid"), 0644)

	if err := AddProfile(Profile{Name: "test", Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("test"); err != nil {
		t.Fatal(err)
	}

	findings := checkProfile()
	severities := severitySet(findings)
	if !severities[SeverityError] {
		t.Error("checkProfile(): expected an error finding for schema violation")
	}
}

func TestCheckProfile_noRepositories(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	os.WriteFile(path, []byte("name: empty\n"), 0644)

	if err := AddProfile(Profile{Name: "empty", Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("empty"); err != nil {
		t.Fatal(err)
	}

	findings := checkProfile()
	severities := severitySet(findings)
	if severities[SeverityError] {
		t.Errorf("checkProfile(): unexpected error findings: %v", findings)
	}
	if !severities[SeverityWarn] {
		t.Error("checkProfile(): expected a warning finding for no repositories")
	}
}

func TestCheckProfile_extractProfileError(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	// File contains "bob" but we register it as "alice" — ExtractProfile will fail.
	os.WriteFile(path, []byte("name: bob\n"), 0644)

	if err := AddProfile(Profile{Name: "alice", Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("alice"); err != nil {
		t.Fatal(err)
	}

	findings := checkProfile()
	severities := severitySet(findings)
	if !severities[SeverityError] {
		t.Error("checkProfile(): expected an error finding when profile name not found in file")
	}
}

func TestCheckProfile_validWithRepos(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	repoDir := filepath.Join(dir, "myrepo")
	os.MkdirAll(filepath.Join(repoDir, ".git"), 0755)

	path := filepath.Join(dir, "profile.yaml")
	content := "name: valid\nrepositories:\n  - name: myrepo\n    path: " + repoDir + "\n    url: http://example.com/repo.git\n"
	os.WriteFile(path, []byte(content), 0644)

	if err := AddProfile(Profile{Name: "valid", Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("valid"); err != nil {
		t.Fatal(err)
	}

	findings := checkProfile()
	severities := severitySet(findings)
	if severities[SeverityError] {
		t.Errorf("checkProfile(): unexpected error findings: %v", findings)
	}
}

// --- checkRepo ---

func TestCheckRepo_notCloned(t *testing.T) {
	repo := Repo{
		Name: "missing",
		Path: "/nonexistent/path/that/does/not/exist",
		URL:  "http://example.com/repo.git",
	}
	findings := checkRepo(repo)
	if len(findings) != 1 || findings[0].Severity != SeverityWarn {
		t.Errorf("checkRepo(): got %v, want single warn finding for uncloned repo", findings)
	}
	if findings[0].Suggestion == "" {
		t.Error("checkRepo(): missing suggestion for uncloned repo")
	}
}

func TestCheckRepo_clonedNoRaidYaml(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	repo := Repo{Name: "present", Path: dir, URL: "http://example.com/repo.git"}
	findings := checkRepo(repo)
	severities := severitySet(findings)
	if severities[SeverityError] || severities[SeverityWarn] {
		t.Errorf("checkRepo(): got unexpected warn/error findings for a valid cloned repo with no raid.yaml: %v", findings)
	}
}

func TestCheckRepo_invalidRaidYaml(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte("name: myrepo\nbadfield: invalid"), 0644)

	repo := Repo{Name: "badconfig", Path: dir, URL: "http://example.com/repo.git"}
	findings := checkRepo(repo)
	severities := severitySet(findings)
	if !severities[SeverityError] {
		t.Error("checkRepo(): expected an error finding for invalid raid.yaml")
	}
}

func TestCheckRepo_validRaidYaml(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte("name: myrepo\nbranch: main\n"), 0644)

	repo := Repo{Name: "good", Path: dir, URL: "http://example.com/repo.git"}
	findings := checkRepo(repo)
	severities := severitySet(findings)
	if severities[SeverityError] {
		t.Errorf("checkRepo(): unexpected error findings for valid raid.yaml: %v", findings)
	}
}

func TestCheckRepo_notGitRepository(t *testing.T) {
	dir := t.TempDir()
	// Directory exists but has no .git — not a git repo.

	repo := Repo{Name: "notgit", Path: dir, URL: "http://example.com/repo.git"}
	findings := checkRepo(repo)
	severities := severitySet(findings)
	if !severities[SeverityWarn] {
		t.Error("checkRepo(): expected a warning for directory that is not a git repository")
	}
}

// severitySet returns a set of all severities present in findings.
func severitySet(findings []Finding) map[Severity]bool {
	out := make(map[Severity]bool, len(findings))
	for _, f := range findings {
		out[f.Severity] = true
	}
	return out
}
