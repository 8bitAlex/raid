package sys

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetPlatform(t *testing.T) {
	p := GetPlatform()
	switch p {
	case Windows, Linux, Darwin, Other:
		// valid Platform value
	default:
		t.Errorf("GetPlatform() = %q, not a valid Platform value", p)
	}
}

func TestGetHomeDir(t *testing.T) {
	home := GetHomeDir()
	if home == "" {
		t.Error("GetHomeDir() returned empty string")
	}
}

func TestFileExists(t *testing.T) {
	tests := []struct {
		name string
		path func(t *testing.T) string
		want bool
	}{
		{
			name: "existing file",
			path: func(t *testing.T) string {
				f, err := os.CreateTemp(t.TempDir(), "raid-test-*")
				if err != nil {
					t.Fatal(err)
				}
				f.Close()
				return f.Name()
			},
			want: true,
		},
		{
			name: "non-existing file",
			path: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "does-not-exist")
			},
			want: false,
		},
		{
			name: "existing directory",
			path: func(t *testing.T) string {
				return t.TempDir()
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path(t)
			if got := FileExists(path); got != tt.want {
				t.Errorf("FileExists(%q) = %v, want %v", path, got, tt.want)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		if got := ExpandPath(""); got != "" {
			t.Errorf("ExpandPath(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("expands env var", func(t *testing.T) {
		os.Setenv("RAID_SYS_TEST", "testvalue")
		defer os.Unsetenv("RAID_SYS_TEST")

		got := ExpandPath("/tmp/$RAID_SYS_TEST/path")
		if got != "/tmp/testvalue/path" {
			t.Errorf("ExpandPath() = %q, want %q", got, "/tmp/testvalue/path")
		}
	})

	t.Run("expands tilde", func(t *testing.T) {
		got := ExpandPath("~/something")
		if got == "~/something" {
			t.Error("ExpandPath() did not expand tilde")
		}
		if got == "" {
			t.Error("ExpandPath() returned empty string for ~/something")
		}
	})

	t.Run("absolute path unchanged", func(t *testing.T) {
		got := ExpandPath("/usr/local/bin")
		if got != "/usr/local/bin" {
			t.Errorf("ExpandPath(%q) = %q, want unchanged", "/usr/local/bin", got)
		}
	})
}

func TestExpand(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		if got := Expand(""); got != "" {
			t.Errorf("Expand(\"\") = %q, want \"\"", got)
		}
	})

	t.Run("single token with env var", func(t *testing.T) {
		os.Setenv("RAID_EXPAND_A", "hello")
		defer os.Unsetenv("RAID_EXPAND_A")

		got := Expand("$RAID_EXPAND_A")
		if got != "hello" {
			t.Errorf("Expand() = %q, want %q", got, "hello")
		}
	})

	t.Run("multiple tokens with env vars", func(t *testing.T) {
		os.Setenv("RAID_EXPAND_X", "foo")
		os.Setenv("RAID_EXPAND_Y", "bar")
		defer os.Unsetenv("RAID_EXPAND_X")
		defer os.Unsetenv("RAID_EXPAND_Y")

		got := Expand("$RAID_EXPAND_X $RAID_EXPAND_Y")
		if got != "foo bar" {
			t.Errorf("Expand() = %q, want %q", got, "foo bar")
		}
	})
}

func TestSplitInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", nil},
		{"single word", "hello", []string{"hello"}},
		{"two words", "hello world", []string{"hello", "world"}},
		{"quoted string with space", `"hello world"`, []string{"hello world"}},
		{"word then quoted", `echo "hello world"`, []string{"echo", "hello world"}},
		{"multiple spaces", "a  b", []string{"a", "b"}},
		{"trailing space", "a b ", []string{"a", "b"}},
		{"leading space", " a b", []string{"a", "b"}},
		{"empty quotes", `""`, nil},
		{"quoted then unquoted", `"foo" bar`, []string{"foo", "bar"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitInput(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("SplitInput(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitInput(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCreateFile_newFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "nested", "file.txt")

	f, err := CreateFile(path)
	if err != nil {
		t.Fatalf("CreateFile() error: %v", err)
	}
	f.Close()

	if !FileExists(path) {
		t.Errorf("CreateFile() did not create file at %q", path)
	}
}

func TestCreateFile_mkdirAllError(t *testing.T) {
	// Use a regular file as a parent directory component — os.MkdirAll will fail with ENOTDIR.
	f, err := os.CreateTemp("", "raid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	path := filepath.Join(f.Name(), "subdir", "file.txt")
	if _, err := CreateFile(path); err == nil {
		t.Fatal("CreateFile() expected error when parent path contains a file component")
	}
}

func TestCreateFile_existingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "existing.txt")

	existing, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	existing.Close()

	f, err := CreateFile(path)
	if err != nil {
		t.Fatalf("CreateFile() on existing file error: %v", err)
	}
	f.Close()
}

// --- ValidateFileName ---

func TestValidateFileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple name", "my-profile", false},
		{"valid with underscores", "my_profile_2", false},
		{"empty string", "", true},
		{"forward slash", "foo/bar", true},
		{"backslash", `foo\bar`, true},
		{"colon", "foo:bar", true},
		{"asterisk", "foo*bar", true},
		{"question mark", "foo?bar", true},
		{"double quote", `foo"bar`, true},
		{"less than", "foo<bar", true},
		{"greater than", "foo>bar", true},
		{"pipe", "foo|bar", true},
		{"null byte", "foo\x00bar", true},
		{"control character", "foo\x01bar", true},
		{"dots only", "...", true},
		{"spaces only", "   ", true},
		{"dots and spaces", " . ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

// --- ReadLine ---

func TestReadLine(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello world\n", "hello world"},
		{"  trimmed  \n", "trimmed"},
		{"\n", ""},
	}
	for _, tc := range cases {
		reader := bufio.NewReader(strings.NewReader(tc.input))
		got := ReadLine(reader, "")
		if got != tc.want {
			t.Errorf("ReadLine(%q): got %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- ReadYesNo ---

func TestReadYesNo(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"n\n", false},
		{"no\n", false},
		{"\n", false},
		{"maybe\n", false},
	}
	for _, tc := range cases {
		reader := bufio.NewReader(strings.NewReader(tc.input))
		got := ReadYesNo(reader, "")
		if got != tc.want {
			t.Errorf("ReadYesNo(%q): got %v, want %v", tc.input, got, tc.want)
		}
	}
}

// --- DetectGitDefaultBranch ---

// initRepoWithBranch creates a non-bare git repo with one empty commit on the
// given branch. ls-remote requires at least one object to return the symref.
func initRepoWithBranch(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "symbolic-ref", "HEAD", "refs/heads/" + branch},
		{"git", "-C", dir, "config", "user.email", "test@example.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, cmd := range cmds {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			t.Fatalf("%v: %v", cmd, err)
		}
	}
	return dir
}

func TestDetectGitDefaultBranch_localRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := initRepoWithBranch(t, "develop")
	got := DetectGitDefaultBranch("file://" + dir)
	if got != "develop" {
		t.Errorf("DetectGitDefaultBranch: got %q, want %q", got, "develop")
	}
}

func TestDetectGitDefaultBranch_unreachable(t *testing.T) {
	got := DetectGitDefaultBranch("https://127.0.0.1:1/nonexistent.git")
	if got != "" {
		t.Errorf("DetectGitDefaultBranch: got %q, want empty string for unreachable remote", got)
	}
}

// --- LatestGitHubRelease ---

func TestLatestGitHubRelease(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{
			name: "returns version without v prefix",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"tag_name":"v1.2.3"}`))
			},
			want: "1.2.3",
		},
		{
			name: "non-200 response returns empty string",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			want: "",
		},
		{
			name: "invalid JSON returns empty string",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`not json`))
			},
			want: "",
		},
		{
			name: "tag without v prefix is returned as-is",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"tag_name":"2.0.0"}`))
			},
			want: "2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			got := latestGitHubRelease(server.URL, "owner/repo")
			if got != tt.want {
				t.Errorf("latestGitHubRelease() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLatestGitHubRelease_unreachable(t *testing.T) {
	got := latestGitHubRelease("http://127.0.0.1:1", "owner/repo")
	if got != "" {
		t.Errorf("latestGitHubRelease() = %q, want empty string for unreachable server", got)
	}
}

// --- LatestGitHubPreRelease ---

func TestLatestGitHubPreRelease(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    string
	}{
		{
			name: "returns prerelease version without v prefix",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[{"tag_name":"v2.0.0-rc1","prerelease":true}]`))
			},
			want: "2.0.0-rc1",
		},
		{
			name: "non-200 response returns empty string",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			want: "",
		},
		{
			name: "invalid JSON returns empty string",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`not json`))
			},
			want: "",
		},
		{
			name: "tag without v prefix is returned as-is",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`[{"tag_name":"3.0.0-beta1","prerelease":true}]`))
			},
			want: "3.0.0-beta1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.handler)
			defer server.Close()

			got := latestGitHubPreRelease(server.URL, "owner/repo")
			if got != tt.want {
				t.Errorf("latestGitHubPreRelease() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLatestGitHubPreRelease_pagination(t *testing.T) {
	// Simulate two pages: first page has no prereleases, second page has one.
	callCount := 0
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		if callCount == 1 {
			// First page: only non-prerelease releases.
			w.Write([]byte(`[
				{"tag_name":"v0.9.0","prerelease":false},
				{"tag_name":"v0.8.0","prerelease":false}
			]`))
			return
		}
		// Second page: contains a prerelease.
		w.Write([]byte(`[
			{"tag_name":"v1.0.0-rc1","prerelease":true},
			{"tag_name":"v0.9.1","prerelease":false}
		]`))
	}

	server := httptest.NewServer(http.HandlerFunc(handler))
	defer server.Close()

	got := latestGitHubPreRelease(server.URL, "owner/repo")
	want := "1.0.0-rc1"
	if got != want {
		t.Errorf("latestGitHubPreRelease() with pagination = %q, want %q", got, want)
	}
	if callCount < 2 {
		t.Errorf("latestGitHubPreRelease() did not request a second page when first page had no prereleases; callCount = %d", callCount)
	}
}

func TestLatestGitHubPreRelease_unreachable(t *testing.T) {
	got := latestGitHubPreRelease("http://127.0.0.1:1", "owner/repo")
	if got != "" {
		t.Errorf("latestGitHubPreRelease() = %q, want empty string for unreachable server", got)
	}
}

func TestDetectGitDefaultBranch_detachedHEAD(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	// Create a repo, then detach HEAD — ls-remote will return no symref line.
	dir := initRepoWithBranch(t, "main")
	// Detach HEAD by checking out the commit hash directly.
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	hash := strings.TrimSpace(string(out))
	if err := exec.Command("git", "-C", dir, "checkout", "--detach", hash).Run(); err != nil {
		t.Fatalf("checkout --detach: %v", err)
	}

	got := DetectGitDefaultBranch("file://" + dir)
	if got != "" {
		t.Errorf("DetectGitDefaultBranch with detached HEAD: got %q, want empty string", got)
	}
}
