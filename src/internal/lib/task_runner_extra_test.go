package lib

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- execConfirm ---

func TestExecuteTask_confirm_noMessage(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	w.WriteString("y\n")
	w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	// Empty Message exercises the `message = "Continue?"` default branch.
	task := Task{Type: Confirm}
	if err := ExecuteTask(task); err != nil {
		t.Errorf("confirm with no message and answer 'y': unexpected error: %v", err)
	}
}

func TestExecuteTask_confirm_readError(t *testing.T) {
	r, _, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	r.Close() // Close the read end so ReadString returns an error.

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	task := Task{Type: Confirm, Message: "Proceed?"}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error from closed stdin, got nil")
	}
}

// --- execPrompt ---

func TestExecuteTask_prompt_readError(t *testing.T) {
	r, _, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	r.Close()

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	task := Task{Type: Prompt, Var: "RAID_PROMPT_ERR_TEST"}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error from closed stdin, got nil")
	}
}

// --- Group parallel/retry group not found ---

func TestExecuteTask_group_parallel_groupNotFound(t *testing.T) {
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"other": {{Type: Shell, Cmd: "exit 0"}},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "nonexistent", Parallel: true}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error for nonexistent group ref, got nil")
	}
}

func TestExecuteTask_group_retry_groupNotFound(t *testing.T) {
	context = &Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"other": {{Type: Shell, Cmd: "exit 0"}},
			},
		},
	}
	defer func() { context = nil }()

	task := Task{Type: Group, Ref: "nonexistent", Attempts: 1}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error for nonexistent group ref, got nil")
	}
}

// --- execGit branch coverage ---

func TestExecuteTask_git_checkoutNoBranch(t *testing.T) {
	dir := t.TempDir()
	task := Task{Type: Git, Op: "checkout", Path: dir}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("expected error for checkout without branch")
	}
	if !strings.Contains(err.Error(), "branch is required") {
		t.Errorf("error %q should mention 'branch is required'", err.Error())
	}
}

func TestExecuteTask_git_pullWithBranch(t *testing.T) {
	// Exercises the `args = append(args, "origin", task.Branch)` branch.
	// Git will fail (not a real repo), but the code path is covered.
	dir := t.TempDir()
	_ = ExecuteTask(Task{Type: Git, Op: "pull", Branch: "main", Path: dir})
}

func TestExecuteTask_git_fetchWithBranch(t *testing.T) {
	dir := t.TempDir()
	_ = ExecuteTask(Task{Type: Git, Op: "fetch", Branch: "main", Path: dir})
}

// --- execHTTP error paths ---

func TestExecuteTask_http_mkdirAllError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("content"))
	}))
	defer srv.Close()

	f, err := os.CreateTemp("", "raid-http-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	// Parent path contains a file component — MkdirAll fails.
	task := Task{
		Type: HTTP,
		URL:  srv.URL,
		Dest: filepath.Join(f.Name(), "subdir", "output.txt"),
	}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execHTTP: expected error when MkdirAll fails")
	}
}

func TestExecuteTask_http_createError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Create a directory where the output file should be — os.Create on a dir fails.
	dest := filepath.Join(dir, "output")
	os.MkdirAll(dest, 0755)

	task := Task{
		Type: HTTP,
		URL:  srv.URL,
		Dest: dest,
	}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execHTTP: expected error when dest is a directory")
	}
}

// --- execTemplate error paths ---

func TestExecuteTask_template_readError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission 0000 not enforced on Windows")
	}
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "template.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(srcPath, 0644)

	task := Task{
		Type: Template,
		Src:  srcPath,
		Dest: filepath.Join(dir, "output.txt"),
	}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execTemplate: expected error when src is unreadable")
	}
}

func TestExecuteTask_template_mkdirAllError(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "template.txt")
	os.WriteFile(srcPath, []byte("hello"), 0644)

	f, err := os.CreateTemp("", "raid-template-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	task := Task{
		Type: Template,
		Src:  srcPath,
		Dest: filepath.Join(f.Name(), "subdir", "output.txt"),
	}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execTemplate: expected error when dest parent MkdirAll fails")
	}
}

func TestExecuteTask_template_writeError(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "template.txt")
	os.WriteFile(srcPath, []byte("hello"), 0644)

	destPath := filepath.Join(dir, "output.txt")
	os.WriteFile(destPath, []byte(""), 0444) // read-only
	defer os.Chmod(destPath, 0644)

	task := Task{
		Type: Template,
		Src:  srcPath,
		Dest: destPath,
	}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execTemplate: expected error when dest is read-only")
	}
}

// --- checkHTTP error path ---

func TestCheckHTTP_unreachableURL(t *testing.T) {
	// Port 0 is reserved; connection should be refused immediately.
	err := checkHTTP("http://127.0.0.1:1/")
	if err == nil {
		t.Fatal("checkHTTP() expected error for unreachable URL, got nil")
	}
}

// --- ExecuteTasks concurrent error collection ---

func TestExecuteTasks_concurrentErrorCollected(t *testing.T) {
	// A concurrent task fails AND the following sequential task also fails.
	// When the sequential task fails, wg.Wait() completes then errorChan is drained,
	// exercising the `errs = append(errs, e)` line.
	tasks := []Task{
		{Type: Shell, Cmd: "exit 1", Concurrent: true},
		{Type: Shell, Cmd: "exit 1"}, // sequential failure triggers the drain
	}
	if err := ExecuteTasks(tasks); err == nil {
		t.Fatal("expected error when concurrent and sequential tasks both fail")
	}
}
