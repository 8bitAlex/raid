package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
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

// --- TaskProps ---

func TestTaskProps_namePromotedAndSet(t *testing.T) {
	task := Task{TaskProps: TaskProps{Name: "build app"}, Type: Shell, Cmd: "make"}
	if task.Name != "build app" {
		t.Errorf("task.Name (promoted) = %q, want %q", task.Name, "build app")
	}
	if task.TaskProps.Name != "build app" {
		t.Errorf("task.TaskProps.Name = %q, want %q", task.TaskProps.Name, "build app")
	}
}

func TestTaskExpand_preservesName(t *testing.T) {
	task := Task{TaskProps: TaskProps{Name: "build app"}, Type: Shell, Cmd: "echo hi"}
	got := task.Expand()
	if got.Name != "build app" {
		t.Errorf("Expand().Name = %q, want %q", got.Name, "build app")
	}
}

func TestTaskProps_yamlRoundTrip(t *testing.T) {
	src := []byte("name: build app\ntype: Shell\ncmd: make\n")
	var task Task
	if err := yaml.Unmarshal(src, &task); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if task.Name != "build app" {
		t.Errorf("Name after YAML unmarshal = %q, want %q", task.Name, "build app")
	}
	if task.Type != "Shell" || task.Cmd != "make" {
		t.Errorf("other fields lost: type=%q cmd=%q", task.Type, task.Cmd)
	}

	out, err := yaml.Marshal(task)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if !containsLine(out, "name: build app") {
		t.Errorf("re-marshalled YAML missing 'name' line:\n%s", out)
	}
}

func TestTaskProps_jsonRoundTrip(t *testing.T) {
	src := []byte(`{"name":"build app","type":"Shell","cmd":"make"}`)
	var task Task
	if err := json.Unmarshal(src, &task); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if task.Name != "build app" {
		t.Errorf("Name after JSON unmarshal = %q, want %q", task.Name, "build app")
	}

	out, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("json.Unmarshal of marshalled output: %v", err)
	}
	if decoded["name"] != "build app" {
		t.Errorf("re-marshalled JSON missing or wrong 'name': %v", decoded["name"])
	}
}

func TestTaskProps_omittedWhenEmpty(t *testing.T) {
	task := Task{Type: Shell, Cmd: "echo hi"}
	out, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, present := decoded["name"]; present {
		t.Errorf("expected 'name' to be omitted when empty, got: %v", decoded)
	}
}

// containsLine reports whether buf contains line as a complete line (between newlines or at edges).
func containsLine(buf []byte, line string) bool {
	s := string(buf)
	for _, l := range splitLines(s) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
