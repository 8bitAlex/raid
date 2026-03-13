package lib

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Command.IsZero ---

func TestCommandIsZero(t *testing.T) {
	tests := []struct {
		name string
		cmd  Command
		want bool
	}{
		{"empty", Command{}, true},
		{"name only", Command{Name: "build"}, false},
		{"name and usage", Command{Name: "build", Usage: "Build services"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cmd.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- GetCommands ---

func TestGetCommands_nilContext(t *testing.T) {
	setupTestConfig(t)
	if got := GetCommands(); got != nil {
		t.Errorf("GetCommands() with nil context = %v, want nil", got)
	}
}

func TestGetCommands_withCommands(t *testing.T) {
	setupTestConfig(t)
	context = &Context{
		Profile: Profile{
			Commands: []Command{
				{Name: "build", Usage: "Build services"},
				{Name: "test", Usage: "Run tests"},
			},
		},
	}

	got := GetCommands()
	if len(got) != 2 {
		t.Fatalf("GetCommands() = %d commands, want 2", len(got))
	}
	if got[0].Name != "build" || got[1].Name != "test" {
		t.Errorf("GetCommands() names = %v/%v, want build/test", got[0].Name, got[1].Name)
	}
}

// --- ExecuteCommand ---

func TestExecuteCommand_notFound(t *testing.T) {
	setupTestConfig(t)
	context = &Context{
		Profile: Profile{
			Commands: []Command{{Name: "other", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}}},
		},
	}

	if err := ExecuteCommand("nonexistent"); err == nil {
		t.Fatal("ExecuteCommand() expected error for unknown command, got nil")
	}
}

func TestExecuteCommand_success(t *testing.T) {
	setupTestConfig(t)
	origOut := commandStdout
	commandStdout = io.Discard
	t.Cleanup(func() { commandStdout = origOut })

	context = &Context{
		Profile: Profile{
			Commands: []Command{{Name: "noop", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}}},
		},
	}

	if err := ExecuteCommand("noop"); err != nil {
		t.Errorf("ExecuteCommand() error: %v", err)
	}
}

func TestExecuteCommand_taskFailure(t *testing.T) {
	setupTestConfig(t)
	context = &Context{
		Profile: Profile{
			Commands: []Command{{Name: "fail", Tasks: []Task{{Type: Shell, Cmd: "exit 1"}}}},
		},
	}

	if err := ExecuteCommand("fail"); err == nil {
		t.Fatal("ExecuteCommand() expected error from failing task, got nil")
	}
}

// --- runCommand ---

func TestRunCommand_nilOut(t *testing.T) {
	origOut := commandStdout
	commandStdout = io.Discard
	t.Cleanup(func() { commandStdout = origOut })

	cmd := Command{Name: "noop", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}}
	if err := runCommand(cmd); err != nil {
		t.Errorf("runCommand() with nil Out error: %v", err)
	}
}

func TestRunCommand_suppressOutput(t *testing.T) {
	// Replace writers with buffers so we can verify nothing was written.
	bufOut := &bytes.Buffer{}
	bufErr := &bytes.Buffer{}
	origOut, origErr := commandStdout, commandStderr
	commandStdout, commandStderr = bufOut, bufErr
	t.Cleanup(func() {
		commandStdout = origOut
		commandStderr = origErr
	})

	cmd := Command{
		Name:  "silent",
		Tasks: []Task{{Type: Shell, Cmd: "echo suppressed"}},
		Out:   &Output{Stdout: false, Stderr: false},
	}
	if err := runCommand(cmd); err != nil {
		t.Fatalf("runCommand() error: %v", err)
	}

	if bufOut.Len() > 0 {
		t.Errorf("commandStdout received %q, want nothing (stdout suppressed)", bufOut.String())
	}
}

func TestRunCommand_restoresWriters(t *testing.T) {
	// Set sentinel writers and confirm they are restored after runCommand.
	bufOut := &bytes.Buffer{}
	bufErr := &bytes.Buffer{}
	origOut, origErr := commandStdout, commandStderr
	commandStdout, commandStderr = bufOut, bufErr
	t.Cleanup(func() {
		commandStdout = origOut
		commandStderr = origErr
	})

	cmd := Command{
		Name:  "restore-check",
		Tasks: []Task{{Type: Shell, Cmd: "exit 0"}},
		Out:   &Output{Stdout: false, Stderr: false},
	}
	_ = runCommand(cmd)

	if commandStdout != bufOut {
		t.Error("commandStdout not restored to original writer after runCommand")
	}
	if commandStderr != bufErr {
		t.Error("commandStderr not restored to original writer after runCommand")
	}
}

func TestRunCommand_fileOutput(t *testing.T) {
	// Silence normal output so the test stays clean.
	origOut, origErr := commandStdout, commandStderr
	commandStdout, commandStderr = io.Discard, io.Discard
	t.Cleanup(func() {
		commandStdout = origOut
		commandStderr = origErr
	})

	outFile := filepath.Join(t.TempDir(), "output.txt")
	cmd := Command{
		Name:  "file-out",
		Tasks: []Task{{Type: Shell, Cmd: "echo filetest"}},
		Out:   &Output{Stdout: true, Stderr: false, File: outFile},
	}
	if err := runCommand(cmd); err != nil {
		t.Fatalf("runCommand() error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error: %v", outFile, err)
	}
	if !strings.Contains(string(data), "filetest") {
		t.Errorf("output file = %q, want to contain 'filetest'", string(data))
	}
}

func TestRunCommand_fileCreateError(t *testing.T) {
	// A parent directory that doesn't exist causes os.Create to fail.
	outFile := filepath.Join(t.TempDir(), "nonexistent", "output.txt")
	cmd := Command{
		Name:  "bad-file",
		Tasks: []Task{{Type: Shell, Cmd: "exit 0"}},
		Out:   &Output{Stdout: true, File: outFile},
	}
	if err := runCommand(cmd); err == nil {
		t.Fatal("runCommand() expected error for bad output file path, got nil")
	}
}

// --- mergeCommands ---

func TestMergeCommands(t *testing.T) {
	a := Command{Name: "a", Usage: "from-a"}
	b := Command{Name: "b"}
	c := Command{Name: "c"}
	aAlt := Command{Name: "a", Usage: "from-alt"}

	tests := []struct {
		name       string
		base       []Command
		additional []Command
		wantNames  []string
		wantUsage  map[string]string // optional spot-check
	}{
		{"both nil", nil, nil, nil, nil},
		{"empty additional", []Command{a, b}, nil, []string{"a", "b"}, nil},
		{"empty base", nil, []Command{c}, []string{"c"}, nil},
		{"no conflicts", []Command{a}, []Command{b}, []string{"a", "b"}, nil},
		{
			"conflict: base wins",
			[]Command{a},
			[]Command{aAlt, c},
			[]string{"a", "c"},
			map[string]string{"a": "from-a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeCommands(tt.base, tt.additional)

			if len(got) != len(tt.wantNames) {
				t.Fatalf("mergeCommands() len = %d, want %d", len(got), len(tt.wantNames))
			}
			for i, cmd := range got {
				if cmd.Name != tt.wantNames[i] {
					t.Errorf("result[%d].Name = %q, want %q", i, cmd.Name, tt.wantNames[i])
				}
				if tt.wantUsage != nil {
					if expected, ok := tt.wantUsage[cmd.Name]; ok && cmd.Usage != expected {
						t.Errorf("result[%d].Usage = %q, want %q", i, cmd.Usage, expected)
					}
				}
			}
		})
	}
}
