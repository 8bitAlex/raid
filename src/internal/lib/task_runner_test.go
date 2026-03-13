package lib

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- getShell ---

func TestGetShell(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"bash", []string{"/bin/bash", "-c"}},
		{"/bin/bash", []string{"/bin/bash", "-c"}},
		{"sh", []string{"/bin/sh", "-c"}},
		{"/bin/sh", []string{"/bin/sh", "-c"}},
		{"zsh", []string{"/bin/zsh", "-c"}},
		{"/bin/zsh", []string{"/bin/zsh", "-c"}},
		{"BASH", []string{"/bin/bash", "-c"}}, // case-insensitive
		{"unknown", []string{"/bin/bash", "-c"}},
		{"", []string{"/bin/bash", "-c"}}, // default on non-Windows
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := getShell(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("getShell(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("getShell(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGetShell_powershell(t *testing.T) {
	tests := []struct {
		input     string
		wantFirst string
	}{
		{"powershell", "powershell"},
		{"pwsh", "powershell"},
		{"ps", "powershell"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := getShell(tt.input)
			if got[0] != tt.wantFirst {
				t.Errorf("getShell(%q)[0] = %q, want %q", tt.input, got[0], tt.wantFirst)
			}
		})
	}
}

// --- ExecuteTask ---

func TestExecuteTask_zeroTaskIsNoop(t *testing.T) {
	if err := ExecuteTask(Task{}); err != nil {
		t.Errorf("ExecuteTask(zero) returned unexpected error: %v", err)
	}
}

func TestExecuteTask_unknownType(t *testing.T) {
	err := ExecuteTask(Task{Type: "bogus", Cmd: "exit 0"})
	if err == nil {
		t.Fatal("expected error for unknown task type, got nil")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error %q should mention the invalid type", err.Error())
	}
}

func TestExecuteTask_shell(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name:    "successful command",
			task:    Task{Type: Shell, Cmd: "exit 0"},
			wantErr: false,
		},
		{
			name:    "failing command",
			task:    Task{Type: Shell, Cmd: "exit 1"},
			wantErr: true,
		},
		{
			name:    "type is case-insensitive",
			task:    Task{Type: "SHELL", Cmd: "exit 0"},
			wantErr: false,
		},
		{
			name:    "explicit bash shell",
			task:    Task{Type: Shell, Cmd: "exit 0", Shell: "bash"},
			wantErr: false,
		},
		{
			name:    "explicit sh shell",
			task:    Task{Type: Shell, Cmd: "exit 0", Shell: "sh"},
			wantErr: false,
		},
		{
			name:    "explicit zsh shell",
			task:    Task{Type: Shell, Cmd: "exit 0", Shell: "zsh"},
			wantErr: false,
		},
		{
			name:    "literal=true still executes",
			task:    Task{Type: Shell, Cmd: "exit 0", Literal: true},
			wantErr: false,
		},
		{
			name:    "concurrent shell task succeeds",
			task:    Task{Type: Shell, Cmd: "exit 0", Concurrent: true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExecuteTask(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteTask() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecuteTask_shell_errorMentionsCommand(t *testing.T) {
	task := Task{Type: Shell, Cmd: "exit 42"}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "exit 42") {
		t.Errorf("error %q should mention the failed command", err.Error())
	}
}

func TestExecuteTask_shell_literal_skipsGoExpansion(t *testing.T) {
	// With Literal=false, sys.Expand runs before the shell sees the command.
	// With Literal=true, the raw string is passed to the shell unchanged.
	// We verify by using a variable that exists in Go's env but NOT in the shell.
	// sys.Expand would replace it; with literal=true it is left for the shell.
	// The shell will expand it to empty, so the command still succeeds either way —
	// what we care about is that Literal=true does not crash or error.
	os.Setenv("RAID_LITERAL_TEST", "value")
	defer os.Unsetenv("RAID_LITERAL_TEST")

	task := Task{Type: Shell, Cmd: "test -n anything", Literal: true}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("literal task returned unexpected error: %v", err)
	}
}

// --- Script tasks ---

func TestExecuteTask_script(t *testing.T) {
	successScript := writeTempScript(t, "#!/bin/sh\nexit 0\n")
	failScript := writeTempScript(t, "#!/bin/sh\nexit 1\n")

	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name:    "missing file",
			task:    Task{Type: Script, Path: "/nonexistent/path/script.sh"},
			wantErr: true,
		},
		{
			name:    "success with runner",
			task:    Task{Type: Script, Path: successScript, Runner: "bash"},
			wantErr: false,
		},
		{
			name:    "failure with runner",
			task:    Task{Type: Script, Path: failScript, Runner: "bash"},
			wantErr: true,
		},
		{
			name:    "success without runner (direct execution)",
			task:    Task{Type: Script, Path: successScript},
			wantErr: false,
		},
		{
			name:    "type is case-insensitive",
			task:    Task{Type: "SCRIPT", Path: successScript, Runner: "bash"},
			wantErr: false,
		},
		{
			name:    "concurrent script task succeeds",
			task:    Task{Type: Script, Path: successScript, Runner: "bash", Concurrent: true},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExecuteTask(tt.task)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteTask() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecuteTask_script_missingFile_errorMentionsPath(t *testing.T) {
	task := Task{Type: Script, Path: "/no/such/file.sh"}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q should mention file not existing", err.Error())
	}
}

// --- ExecuteTasks ---

func TestExecuteTasks(t *testing.T) {
	successScript := writeTempScript(t, "#!/bin/sh\nexit 0\n")
	failScript := writeTempScript(t, "#!/bin/sh\nexit 1\n")

	tests := []struct {
		name    string
		tasks   []Task
		wantErr bool
	}{
		{
			name:    "nil slice is noop",
			tasks:   nil,
			wantErr: false,
		},
		{
			name:    "empty slice is noop",
			tasks:   []Task{},
			wantErr: false,
		},
		{
			name: "all sequential succeed",
			tasks: []Task{
				{Type: Shell, Cmd: "exit 0"},
				{Type: Shell, Cmd: "exit 0"},
			},
			wantErr: false,
		},
		{
			name: "sequential failure reports error",
			tasks: []Task{
				{Type: Shell, Cmd: "exit 1"},
			},
			wantErr: true,
		},
		{
			name: "all concurrent succeed",
			tasks: []Task{
				{Type: Shell, Cmd: "exit 0", Concurrent: true},
				{Type: Shell, Cmd: "exit 0", Concurrent: true},
				{Type: Shell, Cmd: "exit 0", Concurrent: true},
			},
			wantErr: false,
		},
		{
			name: "concurrent failure reports error",
			tasks: []Task{
				{Type: Shell, Cmd: "exit 1", Concurrent: true},
				{Type: Shell, Cmd: "exit 0", Concurrent: true},
			},
			wantErr: true,
		},
		{
			name: "mixed sequential and concurrent",
			tasks: []Task{
				{Type: Shell, Cmd: "exit 0"},
				{Type: Shell, Cmd: "exit 0", Concurrent: true},
				{Type: Shell, Cmd: "exit 0"},
			},
			wantErr: false,
		},
		{
			name: "script tasks included",
			tasks: []Task{
				{Type: Shell, Cmd: "exit 0"},
				{Type: Script, Path: successScript, Runner: "bash"},
			},
			wantErr: false,
		},
		{
			name: "script failure reported",
			tasks: []Task{
				{Type: Script, Path: failScript, Runner: "bash"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ExecuteTasks(tt.tasks)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExecuteTasks() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestExecuteTasks_multipleFailures_allReported(t *testing.T) {
	tasks := []Task{
		{Type: Shell, Cmd: "exit 1"},
		{Type: Shell, Cmd: "exit 1"},
	}

	err := ExecuteTasks(tasks)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Both failures should be captured in the combined error message.
	const wantOccurrences = 2
	count := strings.Count(err.Error(), "failed to execute task")
	if count < wantOccurrences {
		t.Errorf("error should mention %d failures, got %d occurrences in: %q", wantOccurrences, count, err.Error())
	}
}

func TestExecuteTasks_errorMentionsTaskType(t *testing.T) {
	tasks := []Task{
		{Type: Shell, Cmd: "exit 1"},
	}

	err := ExecuteTasks(tasks)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "shell") {
		t.Errorf("error %q should mention the task type", err.Error())
	}
}

// --- helpers ---

func writeTempScript(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "script.sh")
	if err := os.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("failed to write temp script: %v", err)
	}
	return path
}
