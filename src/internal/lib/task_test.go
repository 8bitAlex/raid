package lib

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTaskIsZero(t *testing.T) {
	tests := []struct {
		name string
		task Task
		want bool
	}{
		{"empty task", Task{}, true},
		{"task with type only", Task{Type: Shell}, false},
		{"task with cmd but no type", Task{Cmd: "echo hi"}, true},
		{"shell task with cmd", Task{Type: Shell, Cmd: "echo hi"}, false},
		{"script task with path", Task{Type: Script, Path: filepath.Join(os.TempDir(), "script.sh")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.task.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTaskTypeToLower(t *testing.T) {
	tests := []struct {
		input TaskType
		want  TaskType
	}{
		{"shell", "shell"},
		{"Shell", "shell"},
		{"SHELL", "shell"},
		{"script", "script"},
		{"Script", "script"},
		{"SCRIPT", "script"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			if got := tt.input.ToLower(); got != tt.want {
				t.Errorf("ToLower() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTaskExpand_expandsEnvVarsInFields(t *testing.T) {
	os.Setenv("RAID_TEST_EXPAND", "hello")
	defer os.Unsetenv("RAID_TEST_EXPAND")

	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, "$RAID_TEST_EXPAND", "script.sh")
	wantPath := filepath.Join(tmpDir, "hello", "script.sh")

	task := Task{
		Type:       Shell,
		Concurrent: true,
		Literal:    true,
		Cmd:        "echo $RAID_TEST_EXPAND",
		Shell:      "bash",
		Path:       inputPath,
		Runner:     "$RAID_TEST_EXPAND",
	}

	got := task.Expand()

	if got.Cmd != "echo hello" {
		t.Errorf("Expand().Cmd = %q, want %q", got.Cmd, "echo hello")
	}
	if got.Path != wantPath {
		t.Errorf("Expand().Path = %q, want %q", got.Path, wantPath)
	}
	if got.Runner != "hello" {
		t.Errorf("Expand().Runner = %q, want %q", got.Runner, "hello")
	}
}

func TestTaskExpand_preservesNonStringFields(t *testing.T) {
	task := Task{
		Type:       Script,
		Concurrent: true,
		Literal:    true,
		Shell:      "zsh",
	}

	got := task.Expand()

	if got.Type != task.Type {
		t.Errorf("Expand().Type = %q, want %q", got.Type, task.Type)
	}
	if got.Concurrent != task.Concurrent {
		t.Errorf("Expand().Concurrent = %v, want %v", got.Concurrent, task.Concurrent)
	}
	if got.Literal != task.Literal {
		t.Errorf("Expand().Literal = %v, want %v", got.Literal, task.Literal)
	}
	if got.Shell != task.Shell {
		t.Errorf("Expand().Shell = %q, want %q", got.Shell, task.Shell)
	}
}

func TestTaskExpand_doesNotMutateOriginal(t *testing.T) {
	os.Setenv("RAID_TEST_ORIG", "changed")
	defer os.Unsetenv("RAID_TEST_ORIG")

	original := Task{Type: Shell, Cmd: "echo $RAID_TEST_ORIG"}
	_ = original.Expand()

	if original.Cmd != "echo $RAID_TEST_ORIG" {
		t.Errorf("Expand() mutated original: Cmd = %q", original.Cmd)
	}
}
