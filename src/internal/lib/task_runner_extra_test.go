package lib

import (
	"bytes"
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
	storeContext(&Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"other": {{Type: Shell, Cmd: "exit 0"}},
			},
		},
	})
	defer func() { storeContext(nil) }()

	task := Task{Type: Group, Ref: "nonexistent", Parallel: true}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error for nonexistent group ref, got nil")
	}
}

func TestExecuteTask_group_retry_groupNotFound(t *testing.T) {
	storeContext(&Context{
		Profile: Profile{
			Groups: map[string][]Task{
				"other": {{Type: Shell, Cmd: "exit 0"}},
			},
		},
	})
	defer func() { storeContext(nil) }()

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
	if os.Getuid() == 0 {
		t.Skip("file permissions not enforced as root")
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
	if os.Getuid() == 0 {
		t.Skip("file permissions not enforced as root")
	}
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

// --- execSetVar ---

// overrideRaidVarsPath redirects the vars file to a temp location for test isolation.
func overrideRaidVarsPath(t *testing.T) {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "vars")
	orig := raidVarsOverridePath
	raidVarsOverridePath = tmp
	t.Cleanup(func() {
		raidVarsOverridePath = orig
		raidVarsMu.Lock()
		raidVars = map[string]string{}
		raidVarsMu.Unlock()
	})
}

func TestExecuteTask_setVar_storesInMemory(t *testing.T) {
	overrideRaidVarsPath(t)
	task := Task{Type: SetVar, Var: "RAID_TEST_VAR", Value: "hello"}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("ExecuteTask(SetVar) error: %v", err)
	}
	raidVarsMu.RLock()
	got := raidVars["RAID_TEST_VAR"]
	raidVarsMu.RUnlock()
	if got != "hello" {
		t.Errorf("RAID_TEST_VAR = %q, want %q", got, "hello")
	}
}

// TestExecuteTask_setVar_writesFileMode0600 pins bug C2: the vars
// file persists project-author secrets (Set values, scrubbed clone
// URLs) and must not be world-readable. godotenv defaults to 0644;
// the atomic write path chmods the tempfile down to 0600 before the
// rename so the final file always lands at the tight mode.
func TestExecuteTask_setVar_writesFileMode0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Windows file modes only toggle the read-only attribute via
		// os.Chmod; POSIX permission bits don't round-trip through
		// Stat().Mode().Perm(). The 0600 invariant is a Unix
		// guarantee — Windows uses ACLs we don't manage here.
		t.Skip("0600 perm bits aren't round-trippable on Windows")
	}
	overrideRaidVarsPath(t)
	if err := ExecuteTask(Task{Type: SetVar, Var: "RAID_PERM_TEST", Value: "secret"}); err != nil {
		t.Fatalf("ExecuteTask(SetVar): %v", err)
	}
	info, err := os.Stat(raidVarsPath())
	if err != nil {
		t.Fatalf("stat vars file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("vars file mode = %o, want 0600 (world-readable bits leak credentials/values)", mode)
	}
}

// TestLoadRaidVars_tightensExistingFilePerms covers the migration
// path: vars files written by earlier raid versions on disk at 0644
// should be tightened to 0600 on the next load. Best-effort — chmod
// failures don't block the load, so a foreign-owned or read-only-
// filesystem file just keeps its old mode.
func TestLoadRaidVars_tightensExistingFilePerms(t *testing.T) {
	if runtime.GOOS == "windows" {
		// See TestExecuteTask_setVar_writesFileMode0600 — Windows
		// doesn't preserve POSIX perm bits through os.Chmod.
		t.Skip("0600 perm bits aren't round-trippable on Windows")
	}
	overrideRaidVarsPath(t)
	path := raidVarsPath()
	if err := os.WriteFile(path, []byte("LEGACY=value\n"), 0o644); err != nil {
		t.Fatalf("seed legacy vars file: %v", err)
	}

	loadRaidVars()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("legacy file not tightened: mode = %o, want 0600", mode)
	}
}

func TestExecuteTask_setVar_expandsValue(t *testing.T) {
	overrideRaidVarsPath(t)
	os.Setenv("RAID_BASE", "world")
	t.Cleanup(func() { os.Unsetenv("RAID_BASE") })

	task := Task{Type: SetVar, Var: "RAID_TEST_VAR", Value: "hello-$RAID_BASE"}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("ExecuteTask(SetVar) error: %v", err)
	}
	raidVarsMu.RLock()
	got := raidVars["RAID_TEST_VAR"]
	raidVarsMu.RUnlock()
	if got != "hello-world" {
		t.Errorf("RAID_TEST_VAR = %q, want %q", got, "hello-world")
	}
}

func TestExecuteTask_setVar_missingVar(t *testing.T) {
	task := Task{Type: SetVar, Value: "something"}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("expected error when var is empty, got nil")
	}
}

func TestExecuteTask_setVar_visibleToSubsequentTasks(t *testing.T) {
	overrideRaidVarsPath(t)
	dir := t.TempDir()
	outFile := filepath.Join(dir, "out.txt")
	srcFile := filepath.Join(dir, "tmpl.txt")
	if err := os.WriteFile(srcFile, []byte("$RAID_TEST_VAR"), 0644); err != nil {
		t.Fatal(err)
	}
	tasks := []Task{
		{Type: SetVar, Var: "RAID_TEST_VAR", Value: "persisted"},
		{Type: Template, Src: srcFile, Dest: outFile},
	}
	if err := ExecuteTasks(tasks); err != nil {
		t.Fatalf("ExecuteTasks error: %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "persisted" {
		t.Errorf("downstream task saw %q, want %q", got, "persisted")
	}
}

func TestExpandRaid_caseInsensitiveLookup(t *testing.T) {
	overrideRaidVarsPath(t)
	if err := ExecuteTask(Task{Type: SetVar, Var: "RAID_CASE_TEST", Value: "hello"}); err != nil {
		t.Fatalf("ExecuteTask(SetVar) error: %v", err)
	}
	for _, ref := range []string{"$RAID_CASE_TEST", "$raid_case_test", "$Raid_Case_Test"} {
		if got := expandRaid(ref); got != "hello" {
			t.Errorf("expandRaid(%q) = %q, want %q", ref, got, "hello")
		}
	}
}

func TestExpandRaid_setVarOverridesOSEnv(t *testing.T) {
	overrideRaidVarsPath(t)
	os.Setenv("RAID_OVERRIDE_TEST", "from-os")
	t.Cleanup(func() { os.Unsetenv("RAID_OVERRIDE_TEST") })

	// Set task should win over the OS env value.
	if err := ExecuteTask(Task{Type: SetVar, Var: "RAID_OVERRIDE_TEST", Value: "from-set"}); err != nil {
		t.Fatalf("ExecuteTask(SetVar) error: %v", err)
	}
	if got := expandRaid("$RAID_OVERRIDE_TEST"); got != "from-set" {
		t.Errorf("expandRaid = %q, want %q", got, "from-set")
	}
}

func TestExpandRaid_setVarOverridesDotEnv(t *testing.T) {
	overrideRaidVarsPath(t)
	// Simulate a .env load: the value is in the OS environment.
	os.Setenv("RAID_DOTENV_TEST", "from-dotenv")
	t.Cleanup(func() { os.Unsetenv("RAID_DOTENV_TEST") })

	// Before Set, OS env value is visible.
	if got := expandRaid("$RAID_DOTENV_TEST"); got != "from-dotenv" {
		t.Errorf("before Set: expandRaid = %q, want %q", got, "from-dotenv")
	}

	// After Set, raid vars take precedence.
	if err := ExecuteTask(Task{Type: SetVar, Var: "RAID_DOTENV_TEST", Value: "from-set"}); err != nil {
		t.Fatalf("ExecuteTask(SetVar) error: %v", err)
	}
	if got := expandRaid("$RAID_DOTENV_TEST"); got != "from-set" {
		t.Errorf("after Set: expandRaid = %q, want %q", got, "from-set")
	}
}

// --- getShell ---

func TestGetShell_defaultLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux-specific test")
	}
	result := getShell("")
	if result[0] != "bash" || result[1] != "-c" {
		t.Errorf("getShell(\"\") = %v, want [bash -c]", result)
	}
}

func TestGetShell_explicitBash(t *testing.T) {
	for _, name := range []string{"bash", "/bin/bash"} {
		result := getShell(name)
		if result[0] != "bash" || result[1] != "-c" {
			t.Errorf("getShell(%q) = %v, want [bash -c]", name, result)
		}
	}
}

func TestGetShell_sh(t *testing.T) {
	for _, name := range []string{"sh", "/bin/sh"} {
		result := getShell(name)
		if result[0] != "sh" || result[1] != "-c" {
			t.Errorf("getShell(%q) = %v, want [sh -c]", name, result)
		}
	}
}

func TestGetShell_zsh(t *testing.T) {
	for _, name := range []string{"zsh", "/bin/zsh"} {
		result := getShell(name)
		if result[0] != "zsh" || result[1] != "-c" {
			t.Errorf("getShell(%q) = %v, want [zsh -c]", name, result)
		}
	}
}

func TestGetShell_cmd(t *testing.T) {
	result := getShell("cmd")
	if result[0] != "cmd" || result[1] != "/c" {
		t.Errorf("getShell(\"cmd\") = %v, want [cmd /c]", result)
	}
}


func TestGetShell_unknownDefaultsOnLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux-specific test")
	}
	result := getShell("fish") // unknown shell
	if result[0] != "bash" || result[1] != "-c" {
		t.Errorf("getShell(\"fish\") on linux = %v, want [bash -c]", result)
	}
}

// --- execGit additional operations ---

func TestExecuteTask_git_missingOp(t *testing.T) {
	task := Task{Type: Git, Path: t.TempDir()}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("execGit with missing op: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "op is required") {
		t.Errorf("execGit error = %q, want 'op is required'", err.Error())
	}
}

func TestExecuteTask_git_invalidOp(t *testing.T) {
	task := Task{Type: Git, Op: "invalid", Path: t.TempDir()}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("execGit with invalid op: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid git operation") {
		t.Errorf("execGit error = %q, want 'invalid git operation'", err.Error())
	}
}

func TestExecuteTask_git_pathNotDirectory(t *testing.T) {
	// Create a file (not a directory) and use it as path
	f, err := os.CreateTemp("", "raid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	task := Task{Type: Git, Op: "pull", Path: f.Name()}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execGit with file-as-path: expected error, got nil")
	}
}

func TestExecuteTask_git_nonexistentPath(t *testing.T) {
	task := Task{Type: Git, Op: "pull", Path: "/nonexistent/path/raid-test"}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execGit with nonexistent path: expected error, got nil")
	}
}

func TestExecuteTask_git_resetWithBranch(t *testing.T) {
	dir := t.TempDir()
	task := Task{Type: Git, Op: "reset", Path: dir, Branch: "main"}
	// Will fail because dir isn't a git repo, but it exercises the code path
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("execGit reset: expected error on non-git dir")
	}
	if !strings.Contains(err.Error(), "git reset failed") {
		t.Errorf("execGit reset error = %q, want 'git reset failed'", err.Error())
	}
}

func TestExecuteTask_git_fetchNoBranch(t *testing.T) {
	dir := t.TempDir()
	task := Task{Type: Git, Op: "fetch", Path: dir}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("execGit fetch: expected error on non-git dir")
	}
	if !strings.Contains(err.Error(), "git fetch failed") {
		t.Errorf("execGit fetch error = %q, want 'git fetch failed'", err.Error())
	}
}

func TestExecuteTask_git_pullNoBranch(t *testing.T) {
	dir := t.TempDir()
	task := Task{Type: Git, Op: "pull", Path: dir}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("execGit pull: expected error on non-git dir")
	}
}

// --- expandRaidForShell ---

func TestExpandRaidForShell_unknownVar(t *testing.T) {
	overrideRaidVarsPath(t)
	os.Unsetenv("RAID_UNKNOWN_XYZ_TEST")

	result := expandRaidForShell("hello $RAID_UNKNOWN_XYZ_TEST world")
	if !strings.Contains(result, "${RAID_UNKNOWN_XYZ_TEST}") {
		t.Errorf("expandRaidForShell unknown var = %q, want ${RAID_UNKNOWN_XYZ_TEST} preserved", result)
	}
}

func TestExpandRaidForShell_raidVarExpanded(t *testing.T) {
	overrideRaidVarsPath(t)
	if err := ExecuteTask(Task{Type: SetVar, Var: "RAID_SHELL_TEST_EXTRA", Value: "expanded"}); err != nil {
		t.Fatal(err)
	}
	result := expandRaidForShell("$RAID_SHELL_TEST_EXTRA")
	if result != "expanded" {
		t.Errorf("expandRaidForShell known var = %q, want %q", result, "expanded")
	}
}

func TestExpandRaidForShell_sessionVar(t *testing.T) {
	overrideRaidVarsPath(t)
	startSession()
	defer endSession()

	commandSession.mu.Lock()
	commandSession.vars["RAID_SESS_VAR"] = "session-value"
	commandSession.mu.Unlock()

	result := expandRaidForShell("$RAID_SESS_VAR")
	if result != "session-value" {
		t.Errorf("expandRaidForShell session var = %q, want %q", result, "session-value")
	}
}

func TestExpandRaidForShell_osEnvVar(t *testing.T) {
	overrideRaidVarsPath(t)
	os.Setenv("RAID_OS_SHELL_TEST", "from-os")
	t.Cleanup(func() { os.Unsetenv("RAID_OS_SHELL_TEST") })

	result := expandRaidForShell("$RAID_OS_SHELL_TEST")
	if result != "from-os" {
		t.Errorf("expandRaidForShell OS var = %q, want %q", result, "from-os")
	}
}

// --- loadRaidVars ---

func TestLoadRaidVars_noFile(t *testing.T) {
	orig := raidVarsOverridePath
	raidVarsOverridePath = filepath.Join(t.TempDir(), "nonexistent")
	defer func() { raidVarsOverridePath = orig }()

	raidVarsMu.Lock()
	savedVars := raidVars
	raidVars = map[string]string{}
	raidVarsMu.Unlock()
	defer func() {
		raidVarsMu.Lock()
		raidVars = savedVars
		raidVarsMu.Unlock()
	}()

	loadRaidVars() // should be a no-op since file doesn't exist
	raidVarsMu.RLock()
	count := len(raidVars)
	raidVarsMu.RUnlock()
	if count != 0 {
		t.Errorf("loadRaidVars with no file: expected 0 vars, got %d", count)
	}
}

func TestLoadRaidVars_validFile(t *testing.T) {
	dir := t.TempDir()
	varsPath := filepath.Join(dir, "vars")
	os.WriteFile(varsPath, []byte("RAID_LOAD_TEST=hello\n"), 0644)

	orig := raidVarsOverridePath
	raidVarsOverridePath = varsPath
	defer func() { raidVarsOverridePath = orig }()

	raidVarsMu.Lock()
	savedVars := raidVars
	raidVars = map[string]string{}
	raidVarsMu.Unlock()
	defer func() {
		raidVarsMu.Lock()
		raidVars = savedVars
		raidVarsMu.Unlock()
	}()

	loadRaidVars()
	raidVarsMu.RLock()
	got := raidVars["RAID_LOAD_TEST"]
	raidVarsMu.RUnlock()
	if got != "hello" {
		t.Errorf("loadRaidVars = %q, want %q", got, "hello")
	}
}

// --- checkHTTP success ---

func TestCheckHTTP_success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := checkHTTP(server.URL); err != nil {
		t.Errorf("checkHTTP success: unexpected error: %v", err)
	}
}

func TestCheckHTTP_non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	if err := checkHTTP(server.URL); err == nil {
		t.Error("checkHTTP non-200: expected error, got nil")
	}
}

// --- checkTCP ---

func TestCheckTCP_unreachable(t *testing.T) {
	err := checkTCP("127.0.0.1:1")
	if err == nil {
		t.Fatal("checkTCP unreachable: expected error, got nil")
	}
}

// --- execScript ---

func TestExecuteTask_script_notFound(t *testing.T) {
	task := Task{Type: Script, Path: "/nonexistent/script.sh"}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("execScript not found: expected error, got nil")
	}
}

func TestExecuteTask_script_withRunner(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bash not reliably available on Windows CI")
	}
	script := writeTempScript(t, "#!/bin/sh\necho hello")
	task := Task{Type: Script, Path: script, Runner: "bash"}
	err := ExecuteTask(task)
	if err != nil {
		t.Errorf("execScript with runner: %v", err)
	}
}

// --- execSetVar error paths ---

func TestExecuteTask_setVar_createFileError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file-as-parent-dir error semantics differ on Windows")
	}
	// Use a regular file as parent directory to force CreateFile to fail.
	f, err := os.CreateTemp("", "raid-test-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	orig := raidVarsOverridePath
	raidVarsOverridePath = filepath.Join(f.Name(), "subdir", "vars")
	t.Cleanup(func() { raidVarsOverridePath = orig })

	raidVarsMu.Lock()
	saved := raidVars
	raidVars = map[string]string{}
	raidVarsMu.Unlock()
	defer func() {
		raidVarsMu.Lock()
		raidVars = saved
		raidVarsMu.Unlock()
	}()

	task := Task{Type: SetVar, Var: "TEST_ERR", Value: "val"}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execSetVar: expected error when CreateFile fails")
	}
}

// --- installRepo ---

func TestInstallRepo_cloneError(t *testing.T) {
	storeContext(&Context{
		Profile: Profile{
			Name: "test",
			Path: "/path",
			Repositories: []Repo{
				{
					Name: "badrepo",
					Path: filepath.Join(t.TempDir(), "clone-target"),
					URL:  "file:///nonexistent/repo.git",
				},
			},
		},
	})
	defer func() { storeContext(nil) }()

	err := InstallRepo("badrepo")
	if err == nil {
		t.Fatal("InstallRepo with bad URL: expected error")
	}
	if !strings.Contains(err.Error(), "failed to clone") {
		t.Errorf("error %q should mention 'failed to clone'", err.Error())
	}
}

// --- getShell Windows default ---

func TestGetShell_emptyOnLinux(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux-specific")
	}
	result := getShell("")
	if len(result) != 2 || result[0] != "bash" {
		t.Errorf("getShell empty on linux = %v, want [bash -c]", result)
	}
}

// --- execHTTP additional coverage ---

func TestExecuteTask_http_downloadToDir(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("file content"))
	}))
	defer server.Close()

	dir := t.TempDir()
	destDir := filepath.Join(dir, "sub", "deep")
	destPath := filepath.Join(destDir, "downloaded.txt")

	task := Task{
		Type: HTTP,
		URL:  server.URL + "/file.txt",
		Dest: destPath,
	}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("execHTTP download: %v", err)
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(data) != "file content" {
		t.Errorf("downloaded content = %q, want %q", string(data), "file content")
	}
}

func TestExecuteTask_http_serverError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	task := Task{
		Type: HTTP,
		URL:  server.URL,
		Dest: filepath.Join(t.TempDir(), "file.txt"),
	}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execHTTP server error: expected error")
	}
}

// --- execShell empty cmd ---

func TestExecuteTask_shell_emptyCmd(t *testing.T) {
	task := Task{Type: Shell, Cmd: ""}
	err := ExecuteTask(task)
	if err == nil {
		t.Fatal("execShell with empty cmd: expected error")
	}
}

// --- execTemplate missing src/dest ---

func TestExecuteTask_template_missingSrc(t *testing.T) {
	task := Task{Type: Template, Dest: filepath.Join(t.TempDir(), "out.txt")}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execTemplate missing src: expected error")
	}
}

func TestExecuteTask_template_missingDest(t *testing.T) {
	task := Task{Type: Template, Src: "/some/src.txt"}
	if err := ExecuteTask(task); err == nil {
		t.Fatal("execTemplate missing dest: expected error")
	}
}

// --- execTemplate success ---

func TestExecuteTask_template_success(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "tmpl.txt")
	os.WriteFile(srcPath, []byte("hello {{.Name}}"), 0644)

	destPath := filepath.Join(dir, "output", "result.txt")

	os.Setenv("Name", "world")
	defer os.Unsetenv("Name")

	task := Task{
		Type: Template,
		Src:  srcPath,
		Dest: destPath,
	}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("execTemplate success: %v", err)
	}

	data, _ := os.ReadFile(destPath)
	if !strings.Contains(string(data), "hello") {
		t.Errorf("template output = %q, expected 'hello'", string(data))
	}
}

// --- validEnvPair ---

func TestValidEnvPair(t *testing.T) {
	tests := []struct {
		key, value string
		want       bool
	}{
		{"VAR", "value", true},
		{"", "value", false},
		{"K=V", "value", false},
		{"K\x00V", "value", false},
		{"KEY", "val\x00ue", false},
	}
	for _, tt := range tests {
		if got := validEnvPair(tt.key, tt.value); got != tt.want {
			t.Errorf("validEnvPair(%q, %q) = %v, want %v", tt.key, tt.value, got, tt.want)
		}
	}
}

// --- execPrint ---

func TestExecPrint_withColor(t *testing.T) {
	var buf bytes.Buffer
	origOut := commandStdout
	commandStdout = &buf
	t.Cleanup(func() { commandStdout = origOut })

	task := Task{Type: Print, Message: "colored", Color: "green"}
	if err := ExecuteTask(task); err != nil {
		t.Fatalf("execPrint: %v", err)
	}
	if !strings.Contains(buf.String(), "colored") {
		t.Errorf("output = %q, want 'colored'", buf.String())
	}
}

// --- SetCommandOutput ---

// TestSetCommandOutput_redirectsAndRestores asserts the real behaviour of
// SetCommandOutput: writes to commandStdout/commandStderr land in the
// caller-provided buffers, and after restore() the original writers are
// back in place. Copilot flagged the raid-package wrapper test for not
// asserting this — we own the strong assertion here, where the
// unexported writers are reachable.
func TestSetCommandOutput_redirectsAndRestores(t *testing.T) {
	origOut, origErr := commandStdout, commandStderr
	t.Cleanup(func() { commandStdout, commandStderr = origOut, origErr })

	var outBuf, errBuf bytes.Buffer
	restore := SetCommandOutput(&outBuf, &errBuf)

	if commandStdout != &outBuf {
		t.Fatalf("commandStdout = %p, want %p (the passed-in buffer)", commandStdout, &outBuf)
	}
	if commandStderr != &errBuf {
		t.Fatalf("commandStderr = %p, want %p (the passed-in buffer)", commandStderr, &errBuf)
	}

	// Writes through the package writers must land in the buffers.
	commandStdout.Write([]byte("hello-out"))
	commandStderr.Write([]byte("hello-err"))
	if got := outBuf.String(); got != "hello-out" {
		t.Errorf("outBuf = %q, want %q", got, "hello-out")
	}
	if got := errBuf.String(); got != "hello-err" {
		t.Errorf("errBuf = %q, want %q", got, "hello-err")
	}

	restore()

	if commandStdout != origOut {
		t.Errorf("after restore, commandStdout = %p, want original %p", commandStdout, origOut)
	}
	if commandStderr != origErr {
		t.Errorf("after restore, commandStderr = %p, want original %p", commandStderr, origErr)
	}

	// Subsequent writes must not land in the original buffers.
	outBuf.Reset()
	errBuf.Reset()
	commandStdout.Write([]byte("post-restore"))
	commandStderr.Write([]byte("post-restore"))
	if outBuf.Len() != 0 || errBuf.Len() != 0 {
		t.Errorf("post-restore writes leaked into buffers: out=%q err=%q", outBuf.String(), errBuf.String())
	}
}
