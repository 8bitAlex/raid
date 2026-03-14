package sys

import (
	"os"
	"path/filepath"
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
