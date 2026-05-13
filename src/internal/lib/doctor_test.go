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

func TestCheckRepo_localOnly_notGitIsOK(t *testing.T) {
	dir := t.TempDir()
	// Local-only repo: no URL, no .git, but path exists.

	repo := Repo{Name: "local", Path: dir}
	findings := checkRepo(repo)
	for _, f := range findings {
		if f.Severity == SeverityWarn {
			t.Errorf("checkRepo(): unexpected warning for local-only repo: %+v", f)
		}
	}
}

func TestCheckRepo_localOnly_missingPathIsError(t *testing.T) {
	repo := Repo{Name: "local", Path: filepath.Join(t.TempDir(), "missing")}
	findings := checkRepo(repo)
	if !severitySet(findings)[SeverityError] {
		t.Errorf("checkRepo(): expected error for missing local-only repo, got %+v", findings)
	}
}

// --- checkProfile in single-repo mode ---

func TestCheckProfile_singleRepoValid(t *testing.T) {
	setupTestConfig(t)

	root := repoRoot(t)
	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	os.WriteFile(repoYaml, []byte("name: sr\nbranch: main\n"), 0644)
	// Make the directory look like a git repo so the repo check passes cleanly.
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)

	if err := AddProfile(Profile{Name: "sr", Path: repoYaml}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("sr"); err != nil {
		t.Fatal(err)
	}

	findings := checkProfile()
	severities := severitySet(findings)
	if severities[SeverityError] {
		t.Errorf("checkProfile(): unexpected error findings for single-repo profile: %v", findings)
	}
	// Confirm the "single-repo" label appears so users can tell which mode they're in.
	var sawLabel bool
	for _, f := range findings {
		if f.Check == "profile schema" && f.Severity == SeverityOK {
			sawLabel = true
		}
	}
	if !sawLabel {
		t.Error("checkProfile(): expected 'profile schema' OK finding in single-repo mode")
	}
}

func TestCheckProfile_singleRepoInvalid(t *testing.T) {
	setupTestConfig(t)

	root := repoRoot(t)
	wd, _ := os.Getwd()
	os.Chdir(root)
	defer os.Chdir(wd)

	dir := t.TempDir()
	repoYaml := filepath.Join(dir, RaidConfigFileName)
	// Missing required `branch` field — repo schema violation.
	os.WriteFile(repoYaml, []byte("name: sr-bad\n"), 0644)

	if err := AddProfile(Profile{Name: "sr-bad", Path: repoYaml}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("sr-bad"); err != nil {
		t.Fatal(err)
	}

	findings := checkProfile()
	if !severitySet(findings)[SeverityError] {
		t.Error("checkProfile(): expected error finding for invalid single-repo raid.yaml")
	}
}

// --- checkVerify ---

func TestCheckVerify_passingEntryProducesOKFinding(t *testing.T) {
	entries := []Verify{
		{
			Name:  "echo-works",
			Tasks: []Task{{Type: Shell, Cmd: "exit 0"}},
		},
	}
	findings := checkVerify("verify", entries)
	if len(findings) != 1 {
		t.Fatalf("checkVerify: got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.Severity != SeverityOK {
		t.Errorf("severity = %v, want SeverityOK", f.Severity)
	}
	if f.Check != "verify/echo-works" {
		t.Errorf("check = %q, want %q", f.Check, "verify/echo-works")
	}
}

func TestCheckVerify_remediatedEntryProducesWarnFinding(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "fix")
	entries := []Verify{
		{
			Name:   "needs-fix",
			Tasks:  []Task{{Type: Shell, Cmd: "test -f " + marker}},
			OnFail: []Task{{Type: Shell, Cmd: "touch " + marker}},
		},
	}
	findings := checkVerify("verify", entries)
	if len(findings) != 1 {
		t.Fatalf("checkVerify: got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.Severity != SeverityWarn {
		t.Errorf("severity = %v, want SeverityWarn for remediated outcome", f.Severity)
	}
	if f.Suggestion == "" {
		t.Error("remediated finding should carry a suggestion")
	}
}

func TestCheckVerify_failedEntryProducesErrorFinding(t *testing.T) {
	entries := []Verify{
		{
			Name:  "broken",
			Tasks: []Task{{Type: Shell, Cmd: "exit 1"}},
		},
	}
	findings := checkVerify("verify", entries)
	if len(findings) != 1 {
		t.Fatalf("checkVerify: got %d findings, want 1", len(findings))
	}
	f := findings[0]
	if f.Severity != SeverityError {
		t.Errorf("severity = %v, want SeverityError", f.Severity)
	}
	if f.Suggestion == "" {
		t.Error("failed finding should carry a suggestion")
	}
}

func TestCheckVerify_skipsZeroEntries(t *testing.T) {
	// Zero-value verify (from a stray YAML list item) should be ignored
	// rather than producing a "passed" finding for an empty check.
	findings := checkVerify("verify", []Verify{{}, {Name: "real", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}}})
	if len(findings) != 1 {
		t.Fatalf("checkVerify: got %d findings, want 1 (zero entry skipped)", len(findings))
	}
	if findings[0].Check != "verify/real" {
		t.Errorf("check = %q, want %q", findings[0].Check, "verify/real")
	}
}

func TestCheckVerify_failureDoesNotShortCircuitSubsequent(t *testing.T) {
	// A failing verify must not prevent later entries from running —
	// doctor needs to surface every finding in a single pass.
	entries := []Verify{
		{Name: "first-fails", Tasks: []Task{{Type: Shell, Cmd: "exit 1"}}},
		{Name: "second-passes", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}},
	}
	findings := checkVerify("verify", entries)
	if len(findings) != 2 {
		t.Fatalf("checkVerify: got %d findings, want 2", len(findings))
	}
	if findings[0].Severity != SeverityError {
		t.Errorf("first finding severity = %v, want SeverityError", findings[0].Severity)
	}
	if findings[1].Severity != SeverityOK {
		t.Errorf("second finding severity = %v, want SeverityOK", findings[1].Severity)
	}
}

func TestCheckVerify_emptyEntriesIsNoOp(t *testing.T) {
	findings := checkVerify("verify", nil)
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// --- checkProfile with verify entries ---

func TestCheckProfile_runsProfileLevelVerify(t *testing.T) {
	setupTestConfig(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "profile.yaml")
	// Profile-level verify with one passing and one failing entry.
	body := `name: vp
verify:
  - name: ok
    tasks:
      - type: Shell
        cmd: exit 0
  - name: broken
    tasks:
      - type: Shell
        cmd: exit 1
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if err := AddProfile(Profile{Name: "vp", Path: path}); err != nil {
		t.Fatal(err)
	}
	if err := SetProfile("vp"); err != nil {
		t.Fatal(err)
	}

	findings := checkProfile()
	var sawOK, sawError bool
	for _, f := range findings {
		if f.Check == "verify/ok" && f.Severity == SeverityOK {
			sawOK = true
		}
		if f.Check == "verify/broken" && f.Severity == SeverityError {
			sawError = true
		}
	}
	if !sawOK {
		t.Error("expected 'verify/ok' OK finding")
	}
	if !sawError {
		t.Error("expected 'verify/broken' error finding")
	}
}

// TestCheckRepo_loadsVerifyFromRaidYaml covers the doctor path's
// responsibility for loading verify entries from the per-repo raid.yaml
// itself. The Repo passed in has an empty Verify — production code paths
// like BuildSingleRepoProfile only carry name/path/branch — so doctor
// must read raid.yaml to surface its verify findings.
func TestCheckRepo_loadsVerifyFromRaidYaml(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	repoYaml := `name: r
branch: main
verify:
  - name: hello
    tasks:
      - type: Shell
        cmd: exit 0
`
	if err := os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte(repoYaml), 0644); err != nil {
		t.Fatal(err)
	}

	repo := Repo{
		Name: "r",
		Path: dir,
		URL:  "http://example.com/r.git",
	}
	findings := checkRepo(repo)
	var sawVerify bool
	for _, f := range findings {
		if f.Check == "repo/r verify/hello" && f.Severity == SeverityOK {
			sawVerify = true
		}
	}
	if !sawVerify {
		t.Errorf("expected 'repo/r verify/hello' OK finding from raid.yaml, got %+v", findings)
	}
}

// TestCheckRepo_mergesProfileAndRaidYamlVerify covers the case where the
// profile-level Repo entry has its own verify and the per-repo raid.yaml
// has additional verify entries — both should surface as findings.
func TestCheckRepo_mergesProfileAndRaidYamlVerify(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	repoYaml := `name: r
branch: main
verify:
  - name: from-file
    tasks:
      - type: Shell
        cmd: exit 0
`
	if err := os.WriteFile(filepath.Join(dir, RaidConfigFileName), []byte(repoYaml), 0644); err != nil {
		t.Fatal(err)
	}

	repo := Repo{
		Name:   "r",
		Path:   dir,
		URL:    "http://example.com/r.git",
		Verify: []Verify{{Name: "from-profile", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}}},
	}
	findings := checkRepo(repo)
	var sawProfile, sawFile bool
	for _, f := range findings {
		if f.Check == "repo/r verify/from-profile" && f.Severity == SeverityOK {
			sawProfile = true
		}
		if f.Check == "repo/r verify/from-file" && f.Severity == SeverityOK {
			sawFile = true
		}
	}
	if !sawProfile {
		t.Errorf("expected 'repo/r verify/from-profile' OK finding, got %+v", findings)
	}
	if !sawFile {
		t.Errorf("expected 'repo/r verify/from-file' OK finding, got %+v", findings)
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
