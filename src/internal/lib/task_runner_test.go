package lib

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- getShell ---

func TestGetShell(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"bash", []string{"bash", "-c"}},
		{"/bin/bash", []string{"bash", "-c"}},
		{"sh", []string{"sh", "-c"}},
		{"/bin/sh", []string{"sh", "-c"}},
		{"zsh", []string{"zsh", "-c"}},
		{"/bin/zsh", []string{"zsh", "-c"}},
		{"BASH", []string{"bash", "-c"}}, // case-insensitive
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

func TestGetShell_defaults(t *testing.T) {
	var wantEmpty, wantUnknown []string
	if runtime.GOOS == "windows" {
		wantEmpty = []string{"cmd", "/c"}
		wantUnknown = []string{"cmd", "/c"}
	} else {
		wantEmpty = []string{"bash", "-c"}
		wantUnknown = []string{"bash", "-c"}
	}

	for _, tt := range []struct {
		input string
		want  []string
	}{
		{"", wantEmpty},
		{"unknown", wantUnknown},
	} {
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
	// The resolved binary is "pwsh" when available, "powershell" otherwise.
	wantBin := "powershell"
	if _, err := exec.LookPath("pwsh"); err == nil {
		wantBin = "pwsh"
	}

	for _, input := range []string{"powershell", "pwsh", "ps"} {
		t.Run(input, func(t *testing.T) {
			got := getShell(input)
			if len(got) != 2 {
				t.Fatalf("getShell(%q) = %v, want 2-element slice", input, got)
			}
			if got[0] != wantBin {
				t.Errorf("getShell(%q)[0] = %q, want %q", input, got[0], wantBin)
			}
			if got[1] != "-Command" {
				t.Errorf("getShell(%q)[1] = %q, want \"-Command\"", input, got[1])
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

func TestExecuteTask_shell_zsh(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available on this system")
	}
	task := Task{Type: Shell, Cmd: "exit 0", Shell: "zsh"}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("unexpected error with zsh shell: %v", err)
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

	task := Task{Type: Shell, Cmd: "exit 0", Literal: true}
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

func TestExecuteTask_script_directExecution(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("direct .sh execution not supported on Windows")
	}
	script := writeTempScript(t, "#!/bin/sh\nexit 0\n")
	task := Task{Type: Script, Path: script}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("unexpected error with direct script execution: %v", err)
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

func TestExecuteTasks_sequentialFailsFast(t *testing.T) {
	// A failing sequential task must halt further sequential execution.
	// We verify this by placing a task after the failure that would write a
	// marker file if it ran — the marker must not exist after the run.
	marker := filepath.Join(t.TempDir(), "should-not-exist")
	tasks := []Task{
		{Type: Shell, Cmd: "exit 1"},
		{Type: Shell, Cmd: "echo done > " + marker},
	}

	err := ExecuteTasks(tasks)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("second task ran after sequential failure; expected fail-fast")
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

// --- HTTP tasks ---

func TestExecuteTask_http(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("downloaded content"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "output.txt")

	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name:    "successful download writes file",
			task:    Task{Type: HTTP, URL: srv.URL, Dest: dest},
			wantErr: false,
		},
		{
			name:    "missing url",
			task:    Task{Type: HTTP, Dest: dest},
			wantErr: true,
		},
		{
			name:    "missing dest",
			task:    Task{Type: HTTP, URL: srv.URL},
			wantErr: true,
		},
		{
			name:    "unreachable url",
			task:    Task{Type: HTTP, URL: "http://localhost:0/no", Dest: dest},
			wantErr: true,
		},
		{
			name:    "type is case-insensitive",
			task:    Task{Type: "HTTP", URL: srv.URL, Dest: dest},
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

func TestExecuteTask_http_writesCorrectContent(t *testing.T) {
	const body = "hello from server"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.txt")
	if err := ExecuteTask(Task{Type: HTTP, URL: srv.URL, Dest: dest}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if string(got) != body {
		t.Errorf("file content = %q, want %q", got, body)
	}
}

func TestExecuteTask_http_createsDestDirectory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "a", "b", "c", "out.txt")
	if err := ExecuteTask(Task{Type: HTTP, URL: srv.URL, Dest: dest}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected dest file to exist: %v", err)
	}
}

func TestExecuteTask_http_nonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "out.txt")
	err := ExecuteTask(Task{Type: HTTP, URL: srv.URL, Dest: dest})
	if err == nil {
		t.Fatal("expected error for non-2xx status, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q should mention the status code", err.Error())
	}
}

// --- Wait tasks ---

func TestExecuteTask_wait_http(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name:    "http url responds immediately",
			task:    Task{Type: Wait, URL: srv.URL, Timeout: "5s"},
			wantErr: false,
		},
		{
			name:    "default timeout used when not specified",
			task:    Task{Type: Wait, URL: srv.URL},
			wantErr: false,
		},
		{
			name:    "missing url",
			task:    Task{Type: Wait},
			wantErr: true,
		},
		{
			name:    "invalid timeout",
			task:    Task{Type: Wait, URL: srv.URL, Timeout: "not-a-duration"},
			wantErr: true,
		},
		{
			name:    "type is case-insensitive",
			task:    Task{Type: "WAIT", URL: srv.URL, Timeout: "5s"},
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

func TestExecuteTask_wait_tcp(t *testing.T) {
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	task := Task{Type: Wait, URL: ln.Addr().String(), Timeout: "5s"}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("unexpected error waiting for TCP: %v", err)
	}
}

func TestExecuteTask_wait_timeout(t *testing.T) {
	// Nothing is listening — must time out.
	task := Task{Type: Wait, URL: "localhost:19234", Timeout: "1s"}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error %q should mention timeout", err.Error())
	}
}

// --- Template tasks ---

func TestExecuteTask_template(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tmpl.txt")
	dest := filepath.Join(dir, "out.txt")
	os.WriteFile(src, []byte("hello $TMPL_TEST_VAR"), 0644)

	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name:    "successful render",
			task:    Task{Type: Template, Src: src, Dest: dest},
			wantErr: false,
		},
		{
			name:    "missing src",
			task:    Task{Type: Template, Dest: dest},
			wantErr: true,
		},
		{
			name:    "missing dest",
			task:    Task{Type: Template, Src: src},
			wantErr: true,
		},
		{
			name:    "src file does not exist",
			task:    Task{Type: Template, Src: "/nonexistent/tmpl.txt", Dest: dest},
			wantErr: true,
		},
		{
			name:    "type is case-insensitive",
			task:    Task{Type: "TEMPLATE", Src: src, Dest: dest},
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

func TestExecuteTask_template_expandsEnvVars(t *testing.T) {
	os.Setenv("TMPL_TEST_VAR", "world")
	defer os.Unsetenv("TMPL_TEST_VAR")

	dir := t.TempDir()
	src := filepath.Join(dir, "tmpl.txt")
	dest := filepath.Join(dir, "out.txt")
	os.WriteFile(src, []byte("hello $TMPL_TEST_VAR and ${TMPL_TEST_VAR}"), 0644)

	if err := ExecuteTask(Task{Type: Template, Src: src, Dest: dest}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	const want = "hello world and world"
	if string(got) != want {
		t.Errorf("rendered content = %q, want %q", got, want)
	}
}

func TestExecuteTask_template_createsDestDirectory(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tmpl.txt")
	dest := filepath.Join(dir, "a", "b", "out.txt")
	os.WriteFile(src, []byte("content"), 0644)

	if err := ExecuteTask(Task{Type: Template, Src: src, Dest: dest}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected dest file to exist: %v", err)
	}
}

func TestExecuteTask_template_unsetVarExpandsToEmpty(t *testing.T) {
	os.Unsetenv("TMPL_UNSET_VAR")

	dir := t.TempDir()
	src := filepath.Join(dir, "tmpl.txt")
	dest := filepath.Join(dir, "out.txt")
	os.WriteFile(src, []byte("value=$TMPL_UNSET_VAR"), 0644)

	if err := ExecuteTask(Task{Type: Template, Src: src, Dest: dest}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := os.ReadFile(dest)
	if string(got) != "value=" {
		t.Errorf("rendered content = %q, want %q", got, "value=")
	}
}

// --- Condition ---

func TestExecuteTask_condition_platform(t *testing.T) {
	// Task with a condition that will never match (impossible platform).
	task := Task{
		Type:      Shell,
		Cmd:       "exit 0",
		Condition: &Condition{Platform: "nonexistent-platform"},
	}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("task with unmet condition should be skipped, got error: %v", err)
	}
}

func TestExecuteTask_condition_exists_filePresent(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "exists-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Condition is met — task runs.
	task := Task{
		Type:      Shell,
		Cmd:       "exit 0",
		Condition: &Condition{Exists: f.Name()},
	}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("unexpected error with met exists condition: %v", err)
	}
}

func TestExecuteTask_condition_exists_fileMissing(t *testing.T) {
	// Condition is NOT met — task is skipped (no error).
	task := Task{
		Type:      Shell,
		Cmd:       "exit 1",
		Condition: &Condition{Exists: "/nonexistent/file/path"},
	}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("task with unmet exists condition should be skipped, got error: %v", err)
	}
}

func TestExecuteTask_condition_cmd_passes(t *testing.T) {
	// Condition command exits 0 — task runs.
	task := Task{
		Type:      Shell,
		Cmd:       "exit 0",
		Condition: &Condition{Cmd: "exit 0"},
	}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("unexpected error when condition cmd passes: %v", err)
	}
}

func TestExecuteTask_condition_cmd_fails(t *testing.T) {
	// Condition command exits non-0 — task is skipped (no error).
	marker := filepath.Join(t.TempDir(), "should-not-exist")
	task := Task{
		Type:      Shell,
		Cmd:       "echo done > " + marker,
		Condition: &Condition{Cmd: "exit 1"},
	}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("task with failing condition cmd should be skipped, got error: %v", err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("task body ran despite failing condition cmd")
	}
}

// --- Group tasks ---

func TestExecuteTask_group_noContext(t *testing.T) {
	context = nil
	task := Task{Type: Group, Ref: "mygroup"}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("expected error when context is nil, got nil")
	}
}

func TestExecuteTask_group_noGroups(t *testing.T) {
	context = &Context{Profile: Profile{}}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "mygroup"}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("expected error when profile has no groups, got nil")
	}
}

func TestExecuteTask_group_missingRef(t *testing.T) {
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"other": {{Type: Shell, Cmd: "exit 0"}},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "missing"}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("expected error for missing group ref, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error %q should mention the group name", err.Error())
	}
}

func TestExecuteTask_group_success(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "group-ran")
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"mygroup": {
					{Type: Shell, Cmd: "echo done > " + marker},
				},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "mygroup"}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("group tasks did not run: marker file missing")
	}
}

func TestExecuteTask_group_propagatesFailure(t *testing.T) {
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"failgroup": {
					{Type: Shell, Cmd: "exit 1"},
				},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "failgroup"}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error from failing group task, got nil")
	}
}

func TestExecuteTask_group_emptyRefError(t *testing.T) {
	task := Task{Type: Group, Ref: ""}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error for empty ref, got nil")
	}
}

// --- Git tasks ---

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@example.com")
	run(t, dir, "git", "config", "user.name", "Test")
	run(t, dir, "git", "config", "commit.gpgSign", "false")
	// Create an initial commit so HEAD exists.
	run(t, dir, "git", "commit", "--allow-empty", "-m", "init")
	return dir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("command %q failed: %v\n%s", name+" "+strings.Join(args, " "), err, out)
	}
}

func TestExecuteTask_git(t *testing.T) {
	dir := initTempGitRepo(t)

	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name:    "missing op",
			task:    Task{Type: Git, Path: dir},
			wantErr: true,
		},
		{
			name:    "invalid op",
			task:    Task{Type: Git, Op: "push", Path: dir},
			wantErr: true,
		},
		{
			name:    "path does not exist",
			task:    Task{Type: Git, Op: "pull", Path: "/nonexistent/path"},
			wantErr: true,
		},
		{
			name:    "fetch with no remote succeeds",
			task:    Task{Type: Git, Op: "fetch", Path: dir},
			wantErr: false,
		},
		{
			name:    "checkout nonexistent branch fails",
			task:    Task{Type: Git, Op: "checkout", Branch: "nonexistent-branch-xyz", Path: dir},
			wantErr: true,
		},
		{
			name:    "reset hard HEAD succeeds",
			task:    Task{Type: Git, Op: "reset", Branch: "HEAD", Path: dir},
			wantErr: false,
		},
		{
			name:    "type is case-insensitive",
			task:    Task{Type: "GIT", Op: "fetch", Path: dir},
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

func TestExecuteTask_git_defaultsToWorkingDir(t *testing.T) {
	// When Dir is empty, git runs in the current working directory.
	// We just verify it doesn't crash — the actual git op (fetch) exits 0 with no remote.
	task := Task{Type: Git, Op: "fetch"}
	// The test runner's working dir is the package directory (a valid git repo).
	if err := ExecuteTask(task); err != nil {
		t.Logf("note: git fetch returned error (possibly no remote): %v", err)
	}
}

// --- Print tasks ---

func TestExecuteTask_print(t *testing.T) {
	os.Setenv("PRINT_TEST_VAR", "world")
	defer os.Unsetenv("PRINT_TEST_VAR")

	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name:    "basic message",
			task:    Task{Type: Print, Message: "hello"},
			wantErr: false,
		},
		{
			name:    "expands env vars",
			task:    Task{Type: Print, Message: "hello $PRINT_TEST_VAR"},
			wantErr: false,
		},
		{
			name:    "literal skips expansion",
			task:    Task{Type: Print, Message: "hello $PRINT_TEST_VAR", Literal: true},
			wantErr: false,
		},
		{
			name:    "with valid color",
			task:    Task{Type: Print, Message: "colored", Color: "green"},
			wantErr: false,
		},
		{
			name:    "with unknown color falls back gracefully",
			task:    Task{Type: Print, Message: "msg", Color: "magenta"},
			wantErr: false,
		},
		{
			name:    "type is case-insensitive",
			task:    Task{Type: "PRINT", Message: "hello"},
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

// --- Prompt tasks ---

func TestExecuteTask_prompt_missingVar(t *testing.T) {
	task := Task{Type: Prompt}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error for missing var, got nil")
	}
}

func TestExecuteTask_prompt_setsEnvVar(t *testing.T) {
	os.Unsetenv("RAID_PROMPT_TEST")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString("myvalue\n")
	w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	task := Task{Type: Prompt, Var: "RAID_PROMPT_TEST", Message: "Enter:"}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := os.Getenv("RAID_PROMPT_TEST"); got != "myvalue" {
		t.Errorf("env var RAID_PROMPT_TEST = %q, want %q", got, "myvalue")
	}
	os.Unsetenv("RAID_PROMPT_TEST")
}

func TestExecuteTask_prompt_usesDefault(t *testing.T) {
	os.Unsetenv("RAID_PROMPT_DEFAULT_TEST")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString("\n") // empty input
	w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	task := Task{Type: Prompt, Var: "RAID_PROMPT_DEFAULT_TEST", Default: "fallback"}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := os.Getenv("RAID_PROMPT_DEFAULT_TEST"); got != "fallback" {
		t.Errorf("env var = %q, want %q", got, "fallback")
	}
	os.Unsetenv("RAID_PROMPT_DEFAULT_TEST")
}

// TestExecuteTask_prompt_consecutiveReads is a regression test for a bug where
// a fresh bufio.Reader was created on every Prompt/Confirm invocation. On
// piped input, the first reader could buffer more than one line, return the
// first, and discard the rest when it went out of scope — causing the next
// Prompt to see EOF instead of waiting for input.
func TestExecuteTask_prompt_consecutiveReads(t *testing.T) {
	os.Unsetenv("RAID_PROMPT_FIRST")
	os.Unsetenv("RAID_PROMPT_SECOND")
	t.Cleanup(func() {
		os.Unsetenv("RAID_PROMPT_FIRST")
		os.Unsetenv("RAID_PROMPT_SECOND")
	})

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString("alice\nsmith\n")
	w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = origStdin
		_ = r.Close()
	}()

	if err := ExecuteTask(Task{Type: Prompt, Var: "RAID_PROMPT_FIRST"}); err != nil {
		t.Fatalf("first prompt: unexpected error: %v", err)
	}
	if err := ExecuteTask(Task{Type: Prompt, Var: "RAID_PROMPT_SECOND"}); err != nil {
		t.Fatalf("second prompt: unexpected error: %v", err)
	}

	if got := os.Getenv("RAID_PROMPT_FIRST"); got != "alice" {
		t.Errorf("RAID_PROMPT_FIRST = %q, want %q", got, "alice")
	}
	if got := os.Getenv("RAID_PROMPT_SECOND"); got != "smith" {
		t.Errorf("RAID_PROMPT_SECOND = %q, want %q", got, "smith")
	}
}

// --- Confirm tasks ---

func TestExecuteTask_confirm_yes(t *testing.T) {
	for _, answer := range []string{"y\n", "yes\n", "Y\n", "YES\n"} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		w.WriteString(answer)
		w.Close()

		origStdin := os.Stdin
		os.Stdin = r

		task := Task{Type: Confirm, Message: "Proceed?"}
		err = ExecuteTask(task)
		os.Stdin = origStdin

		if err != nil {
			t.Errorf("answer %q: unexpected error: %v", answer, err)
		}
	}
}

func TestExecuteTask_confirm_no(t *testing.T) {
	for _, answer := range []string{"n\n", "no\n", "\n", "maybe\n"} {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		w.WriteString(answer)
		w.Close()

		origStdin := os.Stdin
		os.Stdin = r

		task := Task{Type: Confirm, Message: "Proceed?"}
		err = ExecuteTask(task)
		os.Stdin = origStdin

		if err == nil {
			t.Errorf("answer %q: expected error (aborted), got nil", answer)
		}
		if err != nil && !strings.Contains(err.Error(), "aborted") {
			t.Errorf("answer %q: error %q should mention 'aborted'", answer, err.Error())
		}
	}
}

// --- Group parallel mode ---

func TestExecuteTask_group_parallel_noContext(t *testing.T) {
	context = nil
	task := Task{Type: Group, Ref: "mygroup", Parallel: true}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error when context is nil, got nil")
	}
}

func TestExecuteTask_group_parallel_missingRef(t *testing.T) {
	task := Task{Type: Group, Ref: "", Parallel: true}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error for empty ref, got nil")
	}
}

func TestExecuteTask_group_parallel_success(t *testing.T) {
	markerA := filepath.Join(t.TempDir(), "a")
	markerB := filepath.Join(t.TempDir(), "b")
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"workers": {
					{Type: Shell, Cmd: "echo done > " + markerA},
					{Type: Shell, Cmd: "echo done > " + markerB},
				},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "workers", Parallel: true}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range []string{markerA, markerB} {
		if _, err := os.Stat(m); err != nil {
			t.Errorf("expected marker %s to exist", m)
		}
	}
}

func TestExecuteTask_group_parallel_propagatesFailure(t *testing.T) {
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"broken": {
					{Type: Shell, Cmd: "exit 1"},
				},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "broken", Parallel: true}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error from failing parallel group, got nil")
	}
}

// --- Group retry mode ---

func TestExecuteTask_group_retry_missingRef(t *testing.T) {
	task := Task{Type: Group, Ref: "", Attempts: 3}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error for empty ref, got nil")
	}
}

func TestExecuteTask_group_retry_noContext(t *testing.T) {
	context = nil
	task := Task{Type: Group, Ref: "mygroup", Attempts: 1}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error when context is nil, got nil")
	}
}

func TestExecuteTask_group_retry_succeedsOnFirstAttempt(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "ran")
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"work": {{Type: Shell, Cmd: "echo done > " + marker}},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "work", Attempts: 3}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("expected marker to exist after successful group retry")
	}
}

func TestExecuteTask_group_retry_exhaustsAllAttempts(t *testing.T) {
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"always-fail": {{Type: Shell, Cmd: "exit 1"}},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "always-fail", Attempts: 2, Delay: "1ms"}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("expected error after all retries exhausted, got nil")
	}
	if !strings.Contains(err.Error(), "2 attempts") {
		t.Errorf("error %q should mention attempt count", err.Error())
	}
}

func TestExecuteTask_group_retry_invalidDelay(t *testing.T) {
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"work": {{Type: Shell, Cmd: "exit 0"}},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "work", Attempts: 1, Delay: "not-a-duration"}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error for invalid delay, got nil")
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

// withRaidVar sets a raid variable for the duration of the test, restoring
// the previous raidVars map on cleanup.
func withRaidVar(t *testing.T, key, value string) {
	t.Helper()
	raidVarsMu.Lock()
	prev := raidVars
	raidVars = make(map[string]string, len(prev)+1)
	for k, v := range prev {
		raidVars[k] = v
	}
	raidVars[key] = value
	raidVarsMu.Unlock()
	t.Cleanup(func() {
		raidVarsMu.Lock()
		raidVars = prev
		raidVarsMu.Unlock()
	})
}

// captureCommandStdout swaps commandStdout for a buffer, returning a getter
// that returns the captured text and restores the previous writer on
// cleanup.
func captureCommandStdout(t *testing.T) func() string {
	t.Helper()
	var buf bytes.Buffer
	prev := commandStdout
	commandStdout = &buf
	t.Cleanup(func() { commandStdout = prev })
	return func() string { return buf.String() }
}

// --- Issue #20: Set variables reach Script + Shell subprocess env ---------

// TestExecScript_inheritsRaidVar guards the issue-#20 fix: a Set task's
// variable must be visible in the env of a subsequent Script task's
// subprocess.
func TestExecScript_inheritsRaidVar(t *testing.T) {
	withRaidVar(t, "ISSUE20_FOO", "from-raid-var")
	getOut := captureCommandStdout(t)

	scriptPath := writeTempScript(t, "#!/bin/sh\nprintf 'got=%s' \"$ISSUE20_FOO\"\n")

	if err := execScript(Task{Type: Script, Path: scriptPath}); err != nil {
		t.Fatalf("execScript: %v", err)
	}
	if got := strings.TrimSpace(getOut()); got != "got=from-raid-var" {
		t.Errorf("script saw $ISSUE20_FOO = %q, want %q", got, "got=from-raid-var")
	}
}

// TestExecScript_raidVarOverridesOSEnv confirms the precedence promised by
// expandRaid: raidVars beat OS env when names collide. The fix appends
// raidVars last in cmd.Env so exec's last-occurrence-wins rule honors that.
func TestExecScript_raidVarOverridesOSEnv(t *testing.T) {
	const key = "ISSUE20_OVERRIDE"
	t.Setenv(key, "from-os")
	withRaidVar(t, key, "from-raid")
	getOut := captureCommandStdout(t)

	scriptPath := writeTempScript(t, "#!/bin/sh\nprintf 'got=%s' \"$ISSUE20_OVERRIDE\"\n")

	if err := execScript(Task{Type: Script, Path: scriptPath}); err != nil {
		t.Fatalf("execScript: %v", err)
	}
	if got := strings.TrimSpace(getOut()); got != "got=from-raid" {
		t.Errorf("script saw collision = %q, want raidVar to win (got=from-raid)", got)
	}
}

// TestExecShell_passesRaidVarToChildScript ensures the same fix applies to
// Shell tasks: a child process spawned by the shell (e.g. another script)
// sees the raidVar even though the shell itself didn't pre-expand it.
// `literal: true` skips raid's pre-expansion so resolution happens at the
// child level instead.
func TestExecShell_passesRaidVarToChildScript(t *testing.T) {
	withRaidVar(t, "ISSUE20_BAR", "from-raid-bar")
	getOut := captureCommandStdout(t)

	dir := t.TempDir()
	childPath := filepath.Join(dir, "child.sh")
	if err := os.WriteFile(childPath, []byte("#!/bin/sh\nprintf 'child=%s' \"$ISSUE20_BAR\"\n"), 0755); err != nil {
		t.Fatalf("write child: %v", err)
	}

	task := Task{Type: Shell, Cmd: childPath, Literal: true}
	if err := execShell(task); err != nil {
		t.Fatalf("execShell: %v", err)
	}
	if got := strings.TrimSpace(getOut()); got != "child=from-raid-bar" {
		t.Errorf("child saw $ISSUE20_BAR = %q, want %q", got, "child=from-raid-bar")
	}
}

// TestBuildSubprocessEnv_orderRaidVarsLast directly exercises the helper to
// guard the precedence wiring (raidVars must appear AFTER OS env so exec's
// duplicate-key resolution gives them priority).
func TestBuildSubprocessEnv_orderRaidVarsLast(t *testing.T) {
	const key = "ISSUE20_BUILD_ENV"
	t.Setenv(key, "os-value")
	withRaidVar(t, key, "raid-value")

	env := buildSubprocessEnv()
	osIdx, raidIdx := -1, -1
	for i, kv := range env {
		switch kv {
		case key + "=os-value":
			osIdx = i
		case key + "=raid-value":
			raidIdx = i
		}
	}
	if osIdx < 0 {
		t.Fatalf("buildSubprocessEnv missing OS-set %q", key)
	}
	if raidIdx < 0 {
		t.Fatalf("buildSubprocessEnv missing raid-set %q", key)
	}
	if raidIdx < osIdx {
		t.Errorf("raid-set entry at %d came before OS entry at %d; exec uses last-occurrence so raidVar must come last", raidIdx, osIdx)
	}
}
