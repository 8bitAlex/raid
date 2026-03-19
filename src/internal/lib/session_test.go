package lib

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- parseEnvLines ---

func TestParseEnvLines_basic(t *testing.T) {
	input := "FOO=bar\nBAZ=qux\n"
	got := parseEnvLines(input)
	if got["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", got["FOO"], "bar")
	}
	if got["BAZ"] != "qux" {
		t.Errorf("BAZ = %q, want %q", got["BAZ"], "qux")
	}
}

func TestParseEnvLines_valueContainsEquals(t *testing.T) {
	// Only the first '=' is used as the delimiter.
	input := "URL=http://host:8080/path?a=1&b=2\n"
	got := parseEnvLines(input)
	if got["URL"] != "http://host:8080/path?a=1&b=2" {
		t.Errorf("URL = %q, want %q", got["URL"], "http://host:8080/path?a=1&b=2")
	}
}

func TestParseEnvLines_emptyAndMalformedLines(t *testing.T) {
	input := "\nFOO=bar\nno-equals-sign\n=emptykey\n"
	got := parseEnvLines(input)
	// Valid line.
	if got["FOO"] != "bar" {
		t.Errorf("FOO = %q, want %q", got["FOO"], "bar")
	}
	// Line with no '=' should be skipped.
	if _, ok := got["no-equals-sign"]; ok {
		t.Error("malformed line should not produce a key")
	}
	// Line starting with '=' has empty key — should be skipped.
	if _, ok := got[""]; ok {
		t.Error("empty-key line should not be stored")
	}
}

func TestParseEnvLines_emptyInput(t *testing.T) {
	got := parseEnvLines("")
	if len(got) != 0 {
		t.Errorf("expected empty map for empty input, got %v", got)
	}
}

// --- updateSessionFromEnv ---

func TestUpdateSessionFromEnv_addsNewVars(t *testing.T) {
	startSession()
	defer endSession()

	// Simulate env output that introduces a new variable not in the baseline.
	data := []byte("RAID_SESSION_NEW=hello\n")
	updateSessionFromEnv(data)

	commandSession.mu.RLock()
	got := commandSession.vars["RAID_SESSION_NEW"]
	commandSession.mu.RUnlock()

	if got != "hello" {
		t.Errorf("session var = %q, want %q", got, "hello")
	}
}

func TestUpdateSessionFromEnv_doesNotAddBaselineVars(t *testing.T) {
	// Pre-set an OS env var so it appears in the baseline.
	os.Setenv("RAID_SESSION_BASELINE_VAR", "original")
	defer os.Unsetenv("RAID_SESSION_BASELINE_VAR")

	startSession()
	defer endSession()

	// Env output with the same key and same value — no change.
	data := []byte("RAID_SESSION_BASELINE_VAR=original\n")
	updateSessionFromEnv(data)

	commandSession.mu.RLock()
	_, exists := commandSession.vars["RAID_SESSION_BASELINE_VAR"]
	commandSession.mu.RUnlock()

	if exists {
		t.Error("unchanged baseline variable should not be added to session vars")
	}
}

func TestUpdateSessionFromEnv_addsChangedBaselineVars(t *testing.T) {
	os.Setenv("RAID_SESSION_CHANGED_VAR", "old")
	defer os.Unsetenv("RAID_SESSION_CHANGED_VAR")

	startSession()
	defer endSession()

	// Same key, different value — counts as a change.
	data := []byte("RAID_SESSION_CHANGED_VAR=new\n")
	updateSessionFromEnv(data)

	commandSession.mu.RLock()
	got := commandSession.vars["RAID_SESSION_CHANGED_VAR"]
	commandSession.mu.RUnlock()

	if got != "new" {
		t.Errorf("changed var = %q, want %q", got, "new")
	}
}

func TestUpdateSessionFromEnv_nilSession(t *testing.T) {
	// Should be a no-op without panicking.
	commandSession = nil
	updateSessionFromEnv([]byte("FOO=bar\n"))
}

// --- expandRaidForShell ---

func TestExpandRaidForShell_knownRaidVar(t *testing.T) {
	overrideRaidVarsPath(t)
	if err := ExecuteTask(Task{Type: SetVar, Var: "RAID_SHELL_KNOWN", Value: "raid-value"}); err != nil {
		t.Fatal(err)
	}
	got := expandRaidForShell("prefix-$RAID_SHELL_KNOWN-suffix")
	if got != "prefix-raid-value-suffix" {
		t.Errorf("expandRaidForShell = %q, want %q", got, "prefix-raid-value-suffix")
	}
}

func TestExpandRaidForShell_knownOSEnvVar(t *testing.T) {
	os.Setenv("RAID_SHELL_OS_VAR", "os-value")
	defer os.Unsetenv("RAID_SHELL_OS_VAR")

	got := expandRaidForShell("$RAID_SHELL_OS_VAR")
	if got != "os-value" {
		t.Errorf("expandRaidForShell = %q, want %q", got, "os-value")
	}
}

func TestExpandRaidForShell_unknownSimpleVarPreserved(t *testing.T) {
	// A simple unknown variable should come back as ${VAR} (equivalent to $VAR
	// in the shell but always uses the braced form for consistency).
	os.Unsetenv("RAID_DEFINITELY_UNDEFINED_XYZ")
	got := expandRaidForShell("echo $RAID_DEFINITELY_UNDEFINED_XYZ")
	if got != "echo ${RAID_DEFINITELY_UNDEFINED_XYZ}" {
		t.Errorf("unknown var = %q, want %q", got, "echo ${RAID_DEFINITELY_UNDEFINED_XYZ}")
	}
}

func TestExpandRaidForShell_parameterExpansionPreserved(t *testing.T) {
	// Shell parameter expansions like ${FOO:-default} must survive intact.
	// os.Expand extracts the key as "FOO:-default"; wrapping it back in ${}
	// reconstructs the original form exactly.
	os.Unsetenv("RAID_PARAM_EXP_VAR")
	cases := []struct {
		input string
		want  string
	}{
		{"${RAID_PARAM_EXP_VAR:-fallback}", "${RAID_PARAM_EXP_VAR:-fallback}"},
		{"${RAID_PARAM_EXP_VAR:+present}", "${RAID_PARAM_EXP_VAR:+present}"},
		{"${RAID_PARAM_EXP_VAR:?error msg}", "${RAID_PARAM_EXP_VAR:?error msg}"},
		{"${RAID_PARAM_EXP_VAR:0:5}", "${RAID_PARAM_EXP_VAR:0:5}"},
		{"${#RAID_PARAM_EXP_VAR}", "${#RAID_PARAM_EXP_VAR}"},
	}
	for _, tc := range cases {
		got := expandRaidForShell(tc.input)
		if got != tc.want {
			t.Errorf("expandRaidForShell(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestExpandRaidForShell_raidVarWinsOverSession(t *testing.T) {
	overrideRaidVarsPath(t)
	if err := ExecuteTask(Task{Type: SetVar, Var: "RAID_PRIORITY_VAR", Value: "from-set"}); err != nil {
		t.Fatal(err)
	}

	startSession()
	defer endSession()
	commandSession.mu.Lock()
	commandSession.vars["RAID_PRIORITY_VAR"] = "from-session"
	commandSession.mu.Unlock()

	got := expandRaidForShell("$RAID_PRIORITY_VAR")
	if got != "from-set" {
		t.Errorf("expandRaidForShell = %q, want raid var %q", got, "from-set")
	}
}

// --- session priority in expandRaid ---

func TestExpandRaid_sessionVarWinsOverOSEnv(t *testing.T) {
	os.Setenv("RAID_SESSION_PRIO_VAR", "from-os")
	defer os.Unsetenv("RAID_SESSION_PRIO_VAR")

	startSession()
	defer endSession()
	commandSession.mu.Lock()
	commandSession.vars["RAID_SESSION_PRIO_VAR"] = "from-session"
	commandSession.mu.Unlock()

	got := expandRaid("$RAID_SESSION_PRIO_VAR")
	if got != "from-session" {
		t.Errorf("expandRaid = %q, want session var %q", got, "from-session")
	}
}

func TestExpandRaid_raidVarWinsOverSession(t *testing.T) {
	overrideRaidVarsPath(t)
	if err := ExecuteTask(Task{Type: SetVar, Var: "RAID_SET_WINS_VAR", Value: "from-set"}); err != nil {
		t.Fatal(err)
	}

	startSession()
	defer endSession()
	commandSession.mu.Lock()
	commandSession.vars["RAID_SET_WINS_VAR"] = "from-session"
	commandSession.mu.Unlock()

	got := expandRaid("$RAID_SET_WINS_VAR")
	if got != "from-set" {
		t.Errorf("expandRaid = %q, want Set task var %q", got, "from-set")
	}
}

// --- session lifecycle ---

func TestStartSession_capturesOSEnvBaseline(t *testing.T) {
	os.Setenv("RAID_BASELINE_CHECK_VAR", "exists")
	defer os.Unsetenv("RAID_BASELINE_CHECK_VAR")

	startSession()
	defer endSession()

	if commandSession == nil {
		t.Fatal("startSession should initialise commandSession")
	}
	if _, ok := commandSession.baseline["RAID_BASELINE_CHECK_VAR"]; !ok {
		t.Error("baseline should include current OS env vars")
	}
}

func TestEndSession_clearsSession(t *testing.T) {
	startSession()
	if commandSession == nil {
		t.Fatal("session not started")
	}
	endSession()
	if commandSession != nil {
		t.Error("endSession should set commandSession to nil")
	}
}

// --- end-to-end: shell export → Set → Shell ---

func TestShellSession_exportedVarFlowsToSetThenShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("session capture not supported on Windows")
	}
	setupTestConfig(t)
	overrideRaidVarsPath(t)

	var buf bytes.Buffer
	origOut := commandStdout
	commandStdout = &buf
	t.Cleanup(func() { commandStdout = origOut })

	context = &Context{
		Profile: Profile{
			Commands: []Command{{
				Name: "test",
				Tasks: []Task{
					// Export WORD inside a literal shell script.
					{Type: Shell, Literal: true, Cmd: "WORD=\"Hello, World!\"\nexport WORD\n"},
					// Set task should pick up WORD from the session.
					{Type: SetVar, Var: "GREET", Value: "$WORD"},
					// Final shell task should see GREET via raidVars expansion.
					{Type: Shell, Cmd: "echo $GREET"},
				},
			}},
		},
	}

	if err := ExecuteCommand("test", nil); err != nil {
		t.Fatalf("ExecuteCommand error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != "Hello, World!" {
		t.Errorf("final echo = %q, want %q", got, "Hello, World!")
	}
}

func TestShellSession_earlyExitStillCapturesEnv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("session capture not supported on Windows")
	}
	setupTestConfig(t)
	overrideRaidVarsPath(t)

	commandStdout = bytes.NewBuffer(nil)
	t.Cleanup(func() { commandStdout = os.Stdout })

	// The script exports a variable then calls `exit 0` explicitly — the env
	// dump must still happen via the EXIT trap, not the sequential append.
	context = &Context{
		Profile: Profile{
			Commands: []Command{{
				Name: "early",
				Tasks: []Task{
					{Type: Shell, Literal: true, Cmd: "export EARLY_VAR=captured\nexit 0\n"},
					{Type: SetVar, Var: "RESULT", Value: "$EARLY_VAR"},
					{Type: Shell, Cmd: "echo $RESULT"},
				},
			}},
		},
	}

	var buf bytes.Buffer
	origOut := commandStdout
	commandStdout = &buf
	t.Cleanup(func() { commandStdout = origOut })

	if err := ExecuteCommand("early", nil); err != nil {
		t.Fatalf("ExecuteCommand error: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got != "captured" {
		t.Errorf("echo = %q, want %q", got, "captured")
	}
}

func TestShellSession_setECapturesEnvBeforeFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("session capture not supported on Windows")
	}
	setupTestConfig(t)
	overrideRaidVarsPath(t)

	// With set -e, a failing command causes an immediate exit. The EXIT trap
	// must still capture whatever was exported before the failure.
	context = &Context{
		Profile: Profile{
			Commands: []Command{{
				Name: "sete",
				Tasks: []Task{
					{Type: Shell, Literal: true, Cmd: "set -e\nexport BEFORE_FAIL=yes\nfalse\nexport AFTER_FAIL=no\n"},
				},
			}},
		},
	}

	// The command fails (false exits 1), but BEFORE_FAIL must be captured.
	_ = ExecuteCommand("sete", nil)

	commandSession = &commandSessionStore{
		vars:     map[string]string{},
		baseline: map[string]string{},
	}
	// Manually verify via a direct ExecuteTask + session pair.
	startSession()
	defer endSession()
	_ = ExecuteTask(Task{Type: Shell, Literal: true, Cmd: "set -e\nexport SETE_VAR=hello\nfalse\n"})

	commandSession.mu.RLock()
	got := commandSession.vars["SETE_VAR"]
	commandSession.mu.RUnlock()

	if got != "hello" {
		t.Errorf("SETE_VAR = %q, want %q — EXIT trap should capture env even after set -e failure", got, "hello")
	}
}

func TestShellSession_shellLocalVarNotExpandedByRaid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("session capture not supported on Windows")
	}
	setupTestConfig(t)

	var buf bytes.Buffer
	origOut := commandStdout
	commandStdout = &buf
	t.Cleanup(func() { commandStdout = origOut })

	// A shell-local variable ($LOCAL) is set and echoed within the SAME task.
	// expandRaidForShell should leave $LOCAL intact so the shell resolves it.
	context = &Context{
		Profile: Profile{
			Commands: []Command{{
				Name: "local",
				Tasks: []Task{
					{Type: Shell, Cmd: "LOCAL=from-shell; echo $LOCAL"},
				},
			}},
		},
	}

	if err := ExecuteCommand("local", nil); err != nil {
		t.Fatalf("ExecuteCommand error: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	if got != "from-shell" {
		t.Errorf("echo = %q, want %q", got, "from-shell")
	}
}

// --- shell exit code propagation ---

func TestExecuteTasks_shellExitCodePreserved(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("exit code test uses bash exit built-in")
	}
	tasks := []Task{{Type: Shell, Cmd: "exit 42"}}
	err := ExecuteTasks(tasks)
	if err == nil {
		t.Fatal("expected error for non-zero exit, got nil")
	}
	var exitErr *exec.ExitError
	if !isExitError(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError in chain, got: %T %v", err, err)
	}
	if exitErr.ExitCode() != 42 {
		t.Errorf("exit code = %d, want 42", exitErr.ExitCode())
	}
}

// isExitError checks if any error in the chain (including errors.Join trees) is
// an *exec.ExitError, mirroring the logic in cmd/raid.go.
func isExitError(err error, target **exec.ExitError) bool {
	if err == nil {
		return false
	}
	// errors.As traverses Unwrap() []error chains from errors.Join.
	return errorsAs(err, target)
}

func errorsAs(err error, target **exec.ExitError) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	type unwrapList interface{ Unwrap() []error }
	if u, ok := err.(unwrapList); ok {
		for _, e := range u.Unwrap() {
			if errorsAs(e, target) {
				return true
			}
		}
	}
	type unwrapSingle interface{ Unwrap() error }
	if u, ok := err.(unwrapSingle); ok {
		return errorsAs(u.Unwrap(), target)
	}
	return false
}

// --- session cleanup: temp files are removed after shell task ---

func TestShellSession_tempFileCleanedUp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("session capture not supported on Windows")
	}
	setupTestConfig(t)

	// Run a shell task with a session active and verify no raid-session temp
	// files are left behind in the system temp directory afterwards.
	before := raidSessionTempFiles(t)

	startSession()
	_ = ExecuteTask(Task{Type: Shell, Cmd: "echo hi"})
	endSession()

	after := raidSessionTempFiles(t)
	for f := range after {
		if _, seen := before[f]; !seen {
			t.Errorf("temp file not cleaned up: %s", f)
		}
	}
}

func raidSessionTempFiles(t *testing.T) map[string]struct{} {
	t.Helper()
	dir := os.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[string]struct{})
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".raid-session-") {
			m[filepath.Join(dir, e.Name())] = struct{}{}
		}
	}
	return m
}
