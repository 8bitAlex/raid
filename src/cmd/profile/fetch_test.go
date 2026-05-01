package profile

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

// saveFetchMocks saves and returns a restore function for the fetch injectable vars.
func saveFetchMocks() func() {
	origGitClone := gitCloneFunc
	origHTTPGet := httpGetFunc
	origDetect := detectGitURL
	origHome := getHomeDir
	return func() {
		gitCloneFunc = origGitClone
		httpGetFunc = origHTTPGet
		detectGitURL = origDetect
		getHomeDir = origHome
	}
}

// writeRaidYAML writes a minimal valid profile YAML to dir/<name>.yaml and returns its path.
func writeRaidYAML(t *testing.T, dir, filename, profileName string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte("name: "+profileName+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- isURL ---

func TestIsURL(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"http://example.com/p.yaml", true},
		{"https://example.com/p.yaml", true},
		{"git@github.com:user/repo.git", true},
		{"/local/path/profile.yaml", false},
		{"./relative/profile.yaml", false},
		{"profile.yaml", false},
	}
	for _, c := range cases {
		if got := isURL(c.input); got != c.want {
			t.Errorf("isURL(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

// --- isGitURL ---

func TestIsGitURL_gitAtPrefix(t *testing.T) {
	if !isGitURL("git@github.com:user/repo.git") {
		t.Error("git@ URL should be detected as git")
	}
}

func TestIsGitURL_dotGitSuffix(t *testing.T) {
	if !isGitURL("https://github.com/user/repo.git") {
		t.Error(".git suffix URL should be detected as git")
	}
}

func TestIsGitURL_yamlExtension(t *testing.T) {
	if isGitURL("https://raw.githubusercontent.com/user/repo/main/profile.yaml") {
		t.Error(".yaml URL should not be detected as git")
	}
}

func TestIsGitURL_ymlExtension(t *testing.T) {
	if isGitURL("https://example.com/profile.yml") {
		t.Error(".yml URL should not be detected as git")
	}
}

func TestIsGitURL_jsonExtension(t *testing.T) {
	if isGitURL("https://example.com/profile.json") {
		t.Error(".json URL should not be detected as git")
	}
}

// --- findProfileFilesInDir ---

func TestFindProfileFilesInDir_empty(t *testing.T) {
	dir := t.TempDir()
	if got := findProfileFilesInDir(dir); len(got) != 0 {
		t.Errorf("findProfileFilesInDir empty dir: got %v, want none", got)
	}
}

func TestFindProfileFilesInDir_exactName(t *testing.T) {
	dir := t.TempDir()
	writeRaidYAML(t, dir, "profile.raid.yaml", "p")
	got := findProfileFilesInDir(dir)
	if len(got) != 1 || !strings.HasSuffix(got[0], "profile.raid.yaml") {
		t.Errorf("findProfileFilesInDir exact: got %v", got)
	}
}

func TestFindProfileFilesInDir_multipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeRaidYAML(t, dir, "profile.raid.yaml", "p1") // high-priority
	writeRaidYAML(t, dir, "team.raid.yaml", "p2")
	got := findProfileFilesInDir(dir)
	if len(got) != 2 {
		t.Errorf("findProfileFilesInDir multi: got %d files, want 2", len(got))
	}
	if !strings.HasSuffix(got[0], "profile.raid.yaml") {
		t.Errorf("findProfileFilesInDir multi: first should be profile.raid.yaml, got %s", got[0])
	}
}

func TestFindProfileFilesInDir_jsonFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(path, []byte(`{"name":"jsonprofile"}`), 0644); err != nil {
		t.Fatal(err)
	}
	got := findProfileFilesInDir(dir)
	if len(got) != 1 || !strings.HasSuffix(got[0], "profile.json") {
		t.Errorf("findProfileFilesInDir json: got %v", got)
	}
}

func TestFindProfileFilesInDir_noDuplicates(t *testing.T) {
	dir := t.TempDir()
	// profile.raid.yaml also matches the *.raid.yaml glob — must appear once only.
	writeRaidYAML(t, dir, "profile.raid.yaml", "p")
	got := findProfileFilesInDir(dir)
	if len(got) != 1 {
		t.Errorf("findProfileFilesInDir dedup: got %d files, want 1", len(got))
	}
}

func TestFindProfileFilesInDir_noRaidYAML_noJSON(t *testing.T) {
	dir := t.TempDir()
	// A plain .yaml file should not be picked up.
	writeRaidYAML(t, dir, "plain.yaml", "p")
	got := findProfileFilesInDir(dir)
	if len(got) != 0 {
		t.Errorf("findProfileFilesInDir plain yaml: got %v, want none", got)
	}
}

// --- runAddProfile (URL paths) ---

func TestRunAddProfile_gitURL_success(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return true }
	gitCloneFunc = func(_, dir string) error {
		writeRaidYAML(t, dir, "cloned.raid.yaml", "cloned")
		return nil
	}

	out := captureStdout(t, func() {
		if code := runAddProfile("https://github.com/example/repo"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "cloned") {
		t.Errorf("got %q, want 'cloned' in output", out)
	}
	if _, err := os.Stat(filepath.Join(homeDir, "cloned.raid.yaml")); err != nil {
		t.Errorf("saved file not found: %v", err)
	}
}

func TestRunAddProfile_gitURL_setsActive(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return true }
	gitCloneFunc = func(_, dir string) error {
		writeRaidYAML(t, dir, "active.raid.yaml", "active")
		return nil
	}

	out := captureStdout(t, func() {
		if code := runAddProfile("https://github.com/example/repo"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "set as active") {
		t.Errorf("got %q, want 'set as active'", out)
	}
	if lib.GetProfile().Name != "active" {
		t.Errorf("active profile = %q, want 'active'", lib.GetProfile().Name)
	}
}

func TestRunAddProfile_gitURL_existingSkipped(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()

	if err := lib.AddProfile(lib.Profile{Name: "existing", Path: "/fake"}); err != nil {
		t.Fatal(err)
	}
	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return true }
	gitCloneFunc = func(_, dir string) error {
		writeRaidYAML(t, dir, "existing.raid.yaml", "existing")
		return nil
	}

	out := captureStdout(t, func() {
		if code := runAddProfile("https://github.com/example/repo"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "already exist") {
		t.Errorf("got %q, want 'already exist'", out)
	}
}

func TestRunAddProfile_gitURL_cloneError(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()

	detectGitURL = func(string) bool { return true }
	gitCloneFunc = func(_, _ string) error { return fmt.Errorf("network error") }

	out := captureStdout(t, func() {
		if code := runAddProfile("https://github.com/example/repo"); code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
	})
	if !strings.Contains(out, "Failed to clone") {
		t.Errorf("got %q, want 'Failed to clone'", out)
	}
}

func TestRunAddProfile_gitURL_noProfiles(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()

	detectGitURL = func(string) bool { return true }
	gitCloneFunc = func(_, _ string) error { return nil } // writes nothing

	out := captureStdout(t, func() {
		if code := runAddProfile("https://github.com/example/repo"); code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
	})
	if !strings.Contains(out, "No profile files found") {
		t.Errorf("got %q, want 'No profile files found'", out)
	}
}

func TestRunAddProfile_httpURL_success(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return false }
	httpGetFunc = func(string) ([]byte, error) {
		return []byte("name: httpprofile\n"), nil
	}

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "httpprofile") {
		t.Errorf("got %q, want 'httpprofile' in output", out)
	}
	if _, err := os.Stat(filepath.Join(homeDir, "httpprofile.raid.yaml")); err != nil {
		t.Errorf("saved file not found: %v", err)
	}
}

func TestRunAddProfile_httpURL_downloadError(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()

	detectGitURL = func(string) bool { return false }
	httpGetFunc = func(string) ([]byte, error) { return nil, fmt.Errorf("connection refused") }

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
	})
	if !strings.Contains(out, "Failed to download") {
		t.Errorf("got %q, want 'Failed to download'", out)
	}
}

func TestRunAddProfile_httpURL_invalidProfile(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return false }
	// Valid YAML syntax but fails schema (extra property).
	httpGetFunc = func(string) ([]byte, error) {
		return []byte("name: bad\nextra: notallowed\n"), nil
	}

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "No new profiles found") {
		t.Errorf("got %q, want 'No new profiles found'", out)
	}
}

func TestRunAddProfile_httpURL_unmarshalError(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()
	defer saveProMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return false }
	httpGetFunc = func(string) ([]byte, error) {
		return []byte("name: mockprofile\n"), nil
	}
	proValidate = func(string) error { return nil }
	proUnmarshal = func(string) ([]pro.Profile, error) { return nil, errMock }

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "No new profiles found") {
		t.Errorf("got %q, want 'No new profiles found'", out)
	}
}

func TestRunAddProfile_httpURL_addAllError(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()
	defer saveProMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return false }
	httpGetFunc = func(string) ([]byte, error) {
		return []byte("name: addallprofile\n"), nil
	}
	proValidate = func(string) error { return nil }
	proUnmarshal = func(string) ([]pro.Profile, error) {
		return []pro.Profile{{Name: "addallprofile"}}, nil
	}
	proContains = func(string) bool { return false }
	proAddAll = func([]pro.Profile) error { return errMock }

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
	})
	if !strings.Contains(out, "Failed to save profiles") {
		t.Errorf("got %q, want 'Failed to save profiles'", out)
	}
}

func TestRunAddProfile_httpURL_setActiveError(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()
	defer saveProMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return false }
	httpGetFunc = func(string) ([]byte, error) {
		return []byte("name: setactiveerr\n"), nil
	}
	proValidate = func(string) error { return nil }
	proUnmarshal = func(string) ([]pro.Profile, error) {
		return []pro.Profile{{Name: "setactiveerr"}}, nil
	}
	proContains = func(string) bool { return false }
	proAddAll = func([]pro.Profile) error { return nil }
	proGet = func() pro.Profile { return pro.Profile{} }
	proSet = func(string) error { return errMock }

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
	})
	if !strings.Contains(out, "Failed to save profiles") {
		t.Errorf("got %q, want 'Failed to save profiles'", out)
	}
}

func TestRunAddProfile_copyError(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()
	defer saveProMocks()()

	// Use a plain file as the "home dir" so os.WriteFile inside the subdirectory fails.
	f, err := os.CreateTemp("", "raid-home-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	getHomeDir = func() string { return f.Name() }
	detectGitURL = func(string) bool { return false }
	httpGetFunc = func(string) ([]byte, error) {
		return []byte("name: copyerr\n"), nil
	}
	proValidate = func(string) error { return nil }
	proUnmarshal = func(string) ([]pro.Profile, error) {
		return []pro.Profile{{Name: "copyerr"}}, nil
	}
	proContains = func(string) bool { return false }

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "No new profiles found") {
		t.Errorf("got %q, want 'No new profiles found'", out)
	}
}

func TestRunAddProfile_invalidProfileName(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()
	defer saveProMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return false }
	httpGetFunc = func(string) ([]byte, error) {
		return []byte("name: goodprofile\n"), nil
	}
	proValidate = func(string) error { return nil }
	proUnmarshal = func(string) ([]pro.Profile, error) {
		return []pro.Profile{{Name: "../../evil"}}, nil
	}
	proContains = func(string) bool { return false }

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "invalid name") {
		t.Errorf("got %q, want 'invalid name' in output", out)
	}
	if _, err := os.Stat(filepath.Join(homeDir, "..evil.raid.yaml")); err == nil {
		t.Error("traversal file must not be written")
	}
}

func TestRunAddProfile_duplicateProfileName(t *testing.T) {
	setupConfig(t)
	defer saveFetchMocks()()
	defer saveProMocks()()

	homeDir := t.TempDir()
	getHomeDir = func() string { return homeDir }
	detectGitURL = func(string) bool { return false }
	httpGetFunc = func(string) ([]byte, error) {
		return []byte("name: dupprofile\n"), nil
	}
	proValidate = func(string) error { return nil }
	proUnmarshal = func(string) ([]pro.Profile, error) {
		return []pro.Profile{{Name: "dupprofile"}, {Name: "dupprofile"}}, nil
	}
	proContains = func(string) bool { return false }

	out := captureStdout(t, func() {
		if code := runAddProfile("https://example.com/profile.yaml"); code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
	})
	if !strings.Contains(out, "duplicate") {
		t.Errorf("got %q, want 'duplicate' in output", out)
	}
	// Only one file should be written.
	if _, err := os.Stat(filepath.Join(homeDir, "dupprofile.raid.yaml")); err != nil {
		t.Errorf("expected one saved file: %v", err)
	}
}

// --- Subprocess test: AddProfileCmd.Run wrapper calls osExit on clone failure ---

const subprocURLCloneFail = "RAID_TEST_URL_CLONE_FAIL"

func TestAddProfileCmd_urlCloneFail_subprocess(t *testing.T) {
	if os.Getenv(subprocURLCloneFail) == "1" {
		setupConfig(t)
		detectGitURL = func(string) bool { return true }
		gitCloneFunc = func(_, _ string) error { return fmt.Errorf("forced clone failure") }
		root := &cobra.Command{Use: "raid"}
		root.AddCommand(AddProfileCmd)
		root.SetArgs([]string{"add", "https://github.com/example/repo"})
		_ = root.Execute()
		return
	}

	proc := exec.Command(os.Args[0], "-test.run=^TestAddProfileCmd_urlCloneFail_subprocess$", "-test.v")
	proc.Env = append(os.Environ(), subprocURLCloneFail+"=1")
	err := proc.Run()
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected *exec.ExitError, got: %T %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("exit code = %d, want 1", exitErr.ExitCode())
	}
}
