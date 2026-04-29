package lib

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

	if err := ExecuteCommand("nonexistent", nil); err == nil {
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

	if err := ExecuteCommand("noop", nil); err != nil {
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

	if err := ExecuteCommand("fail", nil); err == nil {
		t.Fatal("ExecuteCommand() expected error from failing task, got nil")
	}
}

func TestExecuteCommand_argsSetAsEnvVars(t *testing.T) {
	setupTestConfig(t)
	origOut, origErr := commandStdout, commandStderr
	commandStdout, commandStderr = io.Discard, io.Discard
	t.Cleanup(func() { commandStdout = origOut; commandStderr = origErr })

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "args.tmpl")
	outFile := filepath.Join(dir, "args.txt")
	if err := os.WriteFile(srcFile, []byte("$RAID_ARG_1\n$RAID_ARG_2"), 0644); err != nil {
		t.Fatal(err)
	}

	context = &Context{
		Profile: Profile{
			Commands: []Command{{Name: "capture", Tasks: []Task{
				{Type: Template, Src: srcFile, Dest: outFile},
			}}},
		},
	}

	if err := ExecuteCommand("capture", []string{"foo", "bar"}); err != nil {
		t.Fatalf("ExecuteCommand() error: %v", err)
	}

	// Args must be available during execution.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	lines := strings.SplitN(string(data), "\n", 2)
	if lines[0] != "foo" {
		t.Errorf("RAID_ARG_1 during exec = %q, want %q", lines[0], "foo")
	}
	if len(lines) < 2 || lines[1] != "bar" {
		arg2 := ""
		if len(lines) >= 2 {
			arg2 = lines[1]
		}
		t.Errorf("RAID_ARG_2 during exec = %q, want %q", arg2, "bar")
	}

	// Args must be cleared after execution.
	if got := os.Getenv("RAID_ARG_1"); got != "" {
		t.Errorf("RAID_ARG_1 after exec = %q, want cleared", got)
	}
	if got := os.Getenv("RAID_ARG_2"); got != "" {
		t.Errorf("RAID_ARG_2 after exec = %q, want cleared", got)
	}
}

func TestExecuteCommand_staleArgsCleared(t *testing.T) {
	setupTestConfig(t)
	origOut, origErr := commandStdout, commandStderr
	commandStdout, commandStderr = io.Discard, io.Discard
	t.Cleanup(func() { commandStdout = origOut; commandStderr = origErr })

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "args.tmpl")
	outFile := filepath.Join(dir, "args.txt")
	if err := os.WriteFile(srcFile, []byte("$RAID_ARG_2"), 0644); err != nil {
		t.Fatal(err)
	}

	context = &Context{
		Profile: Profile{
			Commands: []Command{{Name: "capture", Tasks: []Task{
				{Type: Template, Src: srcFile, Dest: outFile},
			}}},
		},
	}

	// First call with two args, second call with one — RAID_ARG_2 must not bleed through.
	if err := ExecuteCommand("capture", []string{"first", "stale"}); err != nil {
		t.Fatalf("first ExecuteCommand() error: %v", err)
	}
	if err := ExecuteCommand("capture", []string{"second"}); err != nil {
		t.Fatalf("second ExecuteCommand() error: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := string(data); got != "" {
		t.Errorf("RAID_ARG_2 on second call = %q, want empty (stale value leaked)", got)
	}
}

func TestExecuteCommand_notFoundDoesNotSetArgs(t *testing.T) {
	setupTestConfig(t)
	os.Unsetenv("RAID_ARG_1")
	t.Cleanup(func() { os.Unsetenv("RAID_ARG_1") })

	context = &Context{
		Profile: Profile{
			Commands: []Command{{Name: "other", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}}},
		},
	}

	_ = ExecuteCommand("nonexistent", []string{"should-not-be-set"})
	if got := os.Getenv("RAID_ARG_1"); got != "" {
		t.Errorf("RAID_ARG_1 set to %q after not-found error, want empty", got)
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
	if runtime.GOOS == "windows" {
		t.Skip("permission-based path tricks don't apply on Windows")
	}
	// Use a path whose parent is a regular file, so MkdirAll fails.
	dir := t.TempDir()
	blockingFile := filepath.Join(dir, "not-a-dir")
	if err := os.WriteFile(blockingFile, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	outFile := filepath.Join(blockingFile, "output.txt")
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

// --- GetRepos ---

func TestGetRepos_nilContext(t *testing.T) {
	old := context
	context = nil
	defer func() { context = old }()

	if got := GetRepos(); got != nil {
		t.Errorf("GetRepos() = %v, want nil when context is nil", got)
	}
}

func TestGetRepos_withRepos(t *testing.T) {
	context = &Context{
		Profile: Profile{
			Repositories: []Repo{
				{Name: "backend"},
				{Name: "frontend"},
			},
		},
	}
	defer func() { context = nil }()

	got := GetRepos()
	if len(got) != 2 {
		t.Fatalf("GetRepos() = %d repos, want 2", len(got))
	}
	if got[0].Name != "backend" || got[1].Name != "frontend" {
		t.Errorf("GetRepos() names = %v/%v, want backend/frontend", got[0].Name, got[1].Name)
	}
}

// --- ExecuteRepoCommand ---

func TestExecuteRepoCommand_repoNotFound(t *testing.T) {
	setupTestConfig(t)
	context = &Context{
		Profile: Profile{
			Repositories: []Repo{{Name: "backend"}},
		},
	}

	err := ExecuteRepoCommand("nonexistent", "test", nil)
	if err == nil {
		t.Fatal("expected error for unknown repo, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error %q should mention the repo name", err.Error())
	}
}

func TestExecuteRepoCommand_cmdNotFound(t *testing.T) {
	setupTestConfig(t)
	context = &Context{
		Profile: Profile{
			Repositories: []Repo{{
				Name:     "backend",
				Commands: []Command{{Name: "build", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}}},
			}},
		},
	}

	err := ExecuteRepoCommand("backend", "nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for unknown command, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") || !strings.Contains(err.Error(), "backend") {
		t.Errorf("error %q should mention both repo and command", err.Error())
	}
}

func TestExecuteRepoCommand_success(t *testing.T) {
	setupTestConfig(t)
	origOut := commandStdout
	commandStdout = io.Discard
	t.Cleanup(func() { commandStdout = origOut })

	context = &Context{
		Profile: Profile{
			Repositories: []Repo{{
				Name:     "backend",
				Commands: []Command{{Name: "noop", Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}}},
			}},
		},
	}

	if err := ExecuteRepoCommand("backend", "noop", nil); err != nil {
		t.Errorf("ExecuteRepoCommand() error: %v", err)
	}
}

func TestExecuteRepoCommand_taskFailure(t *testing.T) {
	setupTestConfig(t)
	context = &Context{
		Profile: Profile{
			Repositories: []Repo{{
				Name:     "backend",
				Commands: []Command{{Name: "fail", Tasks: []Task{{Type: Shell, Cmd: "exit 1"}}}},
			}},
		},
	}

	if err := ExecuteRepoCommand("backend", "fail", nil); err == nil {
		t.Fatal("expected error from failing task, got nil")
	}
}

func TestExecuteRepoCommand_setsArgs(t *testing.T) {
	setupTestConfig(t)
	origOut := commandStdout
	commandStdout = io.Discard
	t.Cleanup(func() { commandStdout = origOut })

	dir := t.TempDir()
	srcFile := filepath.Join(dir, "args.tmpl")
	outFile := filepath.Join(dir, "args.txt")
	if err := os.WriteFile(srcFile, []byte("$RAID_ARG_1\n$RAID_ARG_2"), 0644); err != nil {
		t.Fatal(err)
	}

	context = &Context{
		Profile: Profile{
			Repositories: []Repo{{
				Name: "backend",
				Commands: []Command{{
					Name: "tmpl",
					Tasks: []Task{{Type: Template, Src: srcFile, Dest: outFile}},
				}},
			}},
		},
	}

	if err := ExecuteRepoCommand("backend", "tmpl", []string{"hello", "world"}); err != nil {
		t.Fatalf("ExecuteRepoCommand() error: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if got := string(data); got != "hello\nworld" {
		t.Errorf("template output = %q, want %q", got, "hello\nworld")
	}
}

func TestExecuteRepoCommand_nilContext(t *testing.T) {
	old := context
	context = nil
	defer func() { context = old }()

	if err := ExecuteRepoCommand("any", "any", nil); err == nil {
		t.Fatal("expected error with nil context, got nil")
	}
}
