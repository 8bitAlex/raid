package lib

import (
	"bufio"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/internal/telemetry"
	"github.com/joho/godotenv"
)

// commandStdout and commandStderr are the output writers used by task
// execution and operations that surface user-facing progress (clone, env
// setup). ExecuteCommand replaces these temporarily when a command's Out
// field is set; SetCommandOutput is the exported entry point for callers
// that need to redirect them long-term (notably the MCP server, where
// os.Stdout is owned by JSON-RPC framing).
var (
	commandStdout io.Writer = os.Stdout
	commandStderr io.Writer = os.Stderr
)

// SetCommandOutput swaps the writers used by task execution, repository
// cloning, and environment setup. The returned function restores the
// previous writers — call via defer to keep state clean even if the caller
// panics. Not safe for concurrent calls; serialise at the call site if
// multiple goroutines may run mutating operations.
func SetCommandOutput(stdout, stderr io.Writer) func() {
	prevOut, prevErr := commandStdout, commandStderr
	commandStdout, commandStderr = stdout, stderr
	return func() {
		commandStdout, commandStderr = prevOut, prevErr
	}
}

// stdinMu serializes all stdin reads so that concurrent Prompt and Confirm
// tasks do not interleave reads or compete for input.
var stdinMu sync.Mutex

// stdinReader is a package-level buffered reader over os.Stdin, shared across
// all Prompt and Confirm tasks so that bytes buffered ahead by one task are
// still available to the next. Creating a fresh bufio.Reader per call would
// discard any line(s) that got pulled into the old reader's buffer along with
// the one that was actually returned — on piped input this caused the next
// Prompt to see EOF instead of waiting for input.
//
// stdinReaderFor tracks which os.Stdin the reader was built for so that tests
// that swap os.Stdin (e.g. to a pipe) get a fresh reader tied to the new fd.
var (
	stdinReader    *bufio.Reader
	stdinReaderFor *os.File
)

// getStdinReader returns the shared stdin reader, (re)creating it whenever
// os.Stdin has been swapped. Callers must hold stdinMu.
func getStdinReader() *bufio.Reader {
	if stdinReader == nil || stdinReaderFor != os.Stdin {
		stdinReader = bufio.NewReader(os.Stdin)
		stdinReaderFor = os.Stdin
	}
	return stdinReader
}

var colorCodes = map[string]string{
	"red":    "\033[31m",
	"green":  "\033[32m",
	"yellow": "\033[33m",
	"blue":   "\033[34m",
	"cyan":   "\033[36m",
	"white":  "\033[37m",
	"reset":  "\033[0m",
}

func evaluateCondition(c *Condition) bool {
	if c.Platform != "" {
		if string(sys.GetPlatform()) != strings.ToLower(c.Platform) {
			return false
		}
	}
	if c.Exists != "" {
		if !sys.FileExists(sys.ExpandPath(c.Exists)) {
			return false
		}
	}
	if c.Cmd != "" {
		shell := getShell("")
		cmd := exec.Command(shell[0], append(shell[1:], c.Cmd)...)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Run(); err != nil {
			return false
		}
	}
	return true
}

func ExecuteTasks(tasks []Task) error {
	var wg sync.WaitGroup
	errorChan := make(chan error, len(tasks))

	for _, task := range tasks {
		if task.Concurrent {
			wg.Add(1)
			go func(task Task) {
				defer wg.Done()
				// Recover any panic so a single misbehaving task
				// (or a bug in raid's own dispatch path) can't
				// crash the whole process — particularly important
				// in the MCP server where one runaway tool call
				// would tear down the long-lived stdio session.
				// Recovered panics are reported as a structured
				// internal error on the same channel as normal
				// task failures so the aggregate handler treats
				// them uniformly.
				defer func() {
					if r := recover(); r != nil {
						errorChan <- liberrs.Internal(fmt.Sprintf("panic in task %q: %v", task.Label(), r))
					}
				}()
				if err := ExecuteTask(task); err != nil {
					if isContinueOnFailure(task) {
						emitContinueOnFailureWarning(task, err)
						return
					}
					errorChan <- err
				}
			}(task)
		} else {
			if err := ExecuteTask(task); err != nil {
				if isContinueOnFailure(task) {
					// Best-effort task — log a warning and keep going.
					// The error is swallowed for the purposes of the
					// command's exit code; concurrent peers still run
					// to completion.
					emitContinueOnFailureWarning(task, err)
					continue
				}
				// Wait for any already-started concurrent tasks before returning.
				wg.Wait()
				close(errorChan)
				errs := []error{err}
				for e := range errorChan {
					errs = append(errs, e)
				}
				return errors.Join(errs...)
			}
		}
	}

	wg.Wait()
	close(errorChan)

	var errs []error
	for err := range errorChan {
		errs = append(errs, err)
	}

	return errors.Join(errs...)
}

// isContinueOnFailure reports whether the task's options opt into
// best-effort execution. Nil-safe so callers don't need to guard the
// pointer dereference at every site.
func isContinueOnFailure(t Task) bool {
	return t.Options != nil && t.Options.ContinueOnFailure
}

// emitContinueOnFailureWarning prints a dim "warning: <label> failed
// (continueOnFailure): <err>" line to commandStderr so the ignored
// failure remains visible to the operator. Matches the dim styling
// used by showExeTime so the auxiliary lines read consistently.
// Serialized through outputMu so the warning can't land mid-line
// inside a peer concurrent task's prefixed output.
func emitContinueOnFailureWarning(t Task, err error) {
	const (
		dim   = "\033[2m"
		reset = "\033[0m"
	)
	lockedFprintf(commandStderr, "%swarning: %s failed (continueOnFailure): %v%s\n", dim, t.Label(), err, reset)
}

func ExecuteTask(task Task) error {
	if task.IsZero() {
		return nil
	}

	if task.Condition != nil && !evaluateCondition(task.Condition) {
		return nil
	}

	// Wrap the per-type dispatch with showExeTime timing so the emitted
	// line covers both happy and failure paths (the user still wants to
	// know how long the task ran when it errors).
	if task.Options != nil && task.Options.ShowExeTime {
		start := timeNowFn()
		err := dispatchTask(task)
		emitExeTime(task.Label(), timeNowFn().Sub(start))
		captureTaskTelemetry(task, err, timeNowFn().Sub(start))
		return err
	}
	start := timeNowFn()
	err := dispatchTask(task)
	captureTaskTelemetry(task, err, timeNowFn().Sub(start))
	return err
}

// captureTaskTelemetry fires the sampled raid_task_executed event.
// Only the task type, outcome, and duration leak — never the cmd
// body, path, URL, var name, default value, or any other content.
// Sampled at the call site to keep PostHog volume bounded for
// commands with hundreds of tasks.
//
// Sampled fast-paths via telemetry.IsActive when telemetry is off, so
// the per-task overhead when opted out is effectively zero (no RNG
// call).
func captureTaskTelemetry(task Task, err error, dur time.Duration) {
	if !telemetry.Sampled() {
		return
	}
	telemetry.Capture(
		telemetry.EventTaskExecuted,
		telemetry.TaskExecutedProps(string(task.Type), dur.Milliseconds(), err == nil),
	)
}

// dispatchTask is the inner switch separated from ExecuteTask so the
// timing wrapper above stays readable and so tests can target it
// directly when needed.
func dispatchTask(task Task) error {
	switch task.Type.ToLower() {
	case Shell:
		return execShell(task)
	case Script:
		return execScript(task)
	case HTTP:
		return execHTTP(task)
	case Wait:
		return execWait(task)
	case Template:
		return execTemplate(task)
	case Group:
		return execGroup(task)
	case Git:
		return execGit(task)
	case Prompt:
		return execPrompt(task)
	case Confirm:
		return execConfirm(task)
	case Print:
		return execPrint(task)
	case SetVar:
		return execSetVar(task)
	default:
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "invalid task type: %s", task.Type)
	}
}

// timeNowFn is the time source used by the showExeTime wrapper. Tests
// override it to assert deterministic elapsed-time output without
// sleeping.
var timeNowFn = time.Now

// emitExeTime writes the "<label> complete in <Xs>" line to commandStderr
// using dim-grey ANSI styling. Label falls back to the task type when no
// `name:` was given so the output is still recognisable.
func emitExeTime(label string, d time.Duration) {
	const (
		dim   = "\033[2m"
		reset = "\033[0m"
	)
	lockedFprintf(commandStderr, "%s%s complete in %s%s\n", dim, label, formatExeDuration(d), reset)
}

// formatExeDuration renders a duration as a short, human-readable string
// matching the recent-runs formatting in src/cmd/context.
func formatExeDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func execShell(task Task) error {
	if !task.Literal {
		// Expand Path and Shell with the standard expander, but expand Cmd
		// with the shell-aware expander so that variables not known to raid
		// (e.g. shell-local vars set earlier in the same script) are left as
		// "$VAR" tokens for the shell to resolve, rather than silently becoming
		// empty strings.
		task.Path = sys.ExpandPath(expandRaid(task.Path))
		task.Shell = expandRaid(task.Shell)
		task.Cmd = expandRaidForShell(task.Cmd)
	}
	if task.Cmd == "" {
		return liberrs.ArgInvalid("cmd is required for Shell task")
	}

	shell := getShell(task.Shell)
	cmdStr := task.Cmd

	// When a session is active and we're not on Windows, wrap the command so
	// that the full environment after execution is dumped to a temp file.
	// This lets us capture variables exported by the shell command and make
	// them available to subsequent tasks in the same command run.
	var tmpFile string
	if commandSession != nil && sys.GetPlatform() != sys.Windows {
		if f, err := os.CreateTemp("", ".raid-session-*"); err == nil {
			tmpFile = f.Name()
			f.Close()
			// Register an EXIT trap so the environment is dumped on all exit
			// paths — including early exits from `exit N`, `set -e`, or any
			// signal that terminates the shell. The trap captures $? before
			// running env so the original exit code is preserved.
			cmdStr = fmt.Sprintf(
				"__raid_tmp='%s'\ntrap '__raid_exit=$?; env > \"$__raid_tmp\"; exit $__raid_exit' EXIT\n%s",
				tmpFile, task.Cmd,
			)
		}
	}

	cmd := exec.Command(shell[0], append(shell[1:], cmdStr)...)
	if task.Path != "" {
		cmd.Dir = sys.ExpandPath(task.Path)
	}
	cmd.Env = buildSubprocessEnv()
	flush := setCmdOutput(cmd, task)
	defer flush()

	runErr := cmd.Run()

	if tmpFile != "" {
		defer os.Remove(tmpFile)
		if data, err := os.ReadFile(tmpFile); err == nil {
			updateSessionFromEnv(data)
		}
	}

	if runErr != nil {
		return liberrs.Newf(liberrs.CodeTaskShellFailed, liberrs.CategoryTask, "failed to execute shell command '%s': %v", task.Cmd, runErr)
	}
	return nil
}

// updateSessionFromEnv parses the output of `env` and updates the session to
// reflect variables that differ from the baseline. When a variable that was
// previously captured in the session is seen with its baseline value again
// (i.e. a later task reset it), the entry is removed so the baseline value
// is used rather than a stale override from an earlier task.
//
// Fast path: when consecutive Shell tasks don't touch the env, the
// dump is byte-identical to the previous invocation's dump. Hash the
// raw bytes and skip the parse + diff entirely on a cache hit — for a
// command with many shell tasks this avoids O(env-size × tasks)
// allocs in the common case.
func updateSessionFromEnv(data []byte) {
	if commandSession == nil {
		return
	}
	commandSession.mu.Lock()
	if commandSession.lastEnvHash != 0 && commandSession.lastEnvHash == hashBytes(data) {
		commandSession.mu.Unlock()
		return
	}
	commandSession.mu.Unlock()

	after := parseEnvLines(string(data))

	commandSession.mu.Lock()
	defer commandSession.mu.Unlock()
	for k, v := range after {
		baseVal, inBase := commandSession.baseline[k]
		if !inBase || baseVal != v {
			commandSession.vars[k] = v
		} else {
			// Value matches the baseline — remove any stale session entry so
			// a reversion by a later task is not hidden behind an older value.
			delete(commandSession.vars, k)
		}
	}
	commandSession.lastEnvHash = hashBytes(data)
}

// hashBytes returns a fast non-cryptographic hash of p. FNV-1a chosen
// for its allocation-free streaming API — we don't need cryptographic
// guarantees, just "did the env dump change since last time."
func hashBytes(p []byte) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(p)
	return h.Sum64()
}

// parseEnvLines parses newline-separated KEY=VALUE pairs as produced by `env`.
// Values may contain '=' characters; only the first '=' is used as delimiter.
func parseEnvLines(output string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		k, v, found := strings.Cut(line, "=")
		if found && k != "" {
			result[k] = v
		}
	}
	return result
}

func getShell(shell string) []string {
	if shell == "" {
		if sys.GetPlatform() == sys.Windows {
			return []string{"cmd", "/c"}
		}
		return []string{"bash", "-c"}
	}

	switch strings.ToLower(shell) {
	case "/bin/bash", "bash":
		return []string{"bash", "-c"}
	case "/bin/sh", "sh":
		return []string{"sh", "-c"}
	case "/bin/zsh", "zsh":
		return []string{"zsh", "-c"}
	case "powershell", "pwsh", "ps":
		if _, err := exec.LookPath("pwsh"); err == nil {
			return []string{"pwsh", "-Command"}
		}
		return []string{"powershell", "-Command"}
	case "cmd":
		return []string{"cmd", "/c"}
	default:
		if sys.GetPlatform() == sys.Windows {
			return []string{"cmd", "/c"}
		}
		return []string{"bash", "-c"}
	}
}

func execScript(task Task) error {
	task = task.Expand()

	if !sys.FileExists(task.Path) {
		return liberrs.Newf(liberrs.CodeArgInvalid, liberrs.CategoryConfig, "file does not exist: %s", task.Path)
	}

	var cmd *exec.Cmd
	if task.Runner != "" {
		cmd = exec.Command(task.Runner, task.Path)
	} else {
		cmd = exec.Command(task.Path)
	}

	cmd.Env = buildSubprocessEnv()
	flush := setCmdOutput(cmd, task)
	defer flush()

	err := cmd.Run()
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskScriptFailed, liberrs.CategoryTask, "failed to execute script '%s': %v", task.Path, err)
	}

	return nil
}

// setCmdOutput wires a subprocess's stdout/stderr/stdin. For tasks
// opted into concurrent execution on a TTY sink, the writers are
// wrapped with per-line prefixers so output stays attributable when
// peers interleave; the returned cleanup must be deferred to flush
// any trailing partial line on subprocess exit. For sequential tasks
// or non-TTY sinks the cleanup is a no-op and writers pass through
// to commandStdout/commandStderr unchanged — pipes, file redirects,
// CI logs, and the MCP server's syncBuffer capture all stay
// byte-identical to today.
func setCmdOutput(cmd *exec.Cmd, task Task) func() {
	cmd.Stdin = os.Stdin
	wrapOut := shouldPrefix(task, commandStdout)
	wrapErr := shouldPrefix(task, commandStderr)
	if !wrapOut && !wrapErr {
		cmd.Stdout = commandStdout
		cmd.Stderr = commandStderr
		return func() {}
	}
	color := ""
	if !colorDisabled() {
		color = colorForName(task.Label())
	}
	prefix := buildPrefix(task.Label(), color)
	var out, errW *prefixedWriter
	if wrapOut {
		out = newPrefixedWriter(commandStdout, prefix)
		cmd.Stdout = out
	} else {
		cmd.Stdout = commandStdout
	}
	if wrapErr {
		errW = newPrefixedWriter(commandStderr, prefix)
		cmd.Stderr = errW
	} else {
		cmd.Stderr = commandStderr
	}
	return func() {
		if out != nil {
			_ = out.Flush()
		}
		if errW != nil {
			_ = errW.Flush()
		}
	}
}

// buildSubprocessEnv returns the OS environment merged with the current
// commandSession exports and raidVars (Set tasks). Order matters: exec.Cmd
// uses the LAST occurrence of a duplicate key, so we append in increasing
// priority — OS env first, session next, raidVars last so they win on
// collision. Mirrors the lookup order used by expandRaid.
//
// Applied to Shell and Script tasks so a `Set FOO bar` task earlier in the
// sequence is visible to the spawned process (and any children it spawns)
// as $FOO. Without this, raidVars only resolved via raid's pre-expansion of
// the cmd string — which couldn't reach into a Script's source or into a
// child process spawned from inside a Shell command.
//
// Entries with empty keys or with NUL / "=" bytes in the key (or NUL bytes
// in the value) are silently dropped — those would either be re-parsed as
// a different key at exec time or get rejected outright by the OS.
// Defensive guard since Set / Prompt task input isn't yet schema-constrained.
func buildSubprocessEnv() []string {
	env := os.Environ()
	if commandSession != nil {
		commandSession.mu.RLock()
		for k, v := range commandSession.vars {
			if validEnvPair(k, v) {
				env = append(env, k+"="+v)
			}
		}
		commandSession.mu.RUnlock()
	}
	raidVarsMu.RLock()
	for k, v := range raidVars {
		if validEnvPair(k, v) {
			env = append(env, k+"="+v)
		}
	}
	raidVarsMu.RUnlock()
	return env
}

// validEnvPair reports whether (key, value) is safe to inject into a
// subprocess environment. NUL bytes terminate C strings and would corrupt
// the entry; "=" in the key would be re-split into a different key by
// exec at process-start time.
func validEnvPair(key, value string) bool {
	if key == "" {
		return false
	}
	if strings.ContainsAny(key, "=\x00") {
		return false
	}
	if strings.ContainsRune(value, '\x00') {
		return false
	}
	return true
}

func execHTTP(task Task) error {
	task = task.Expand()

	if task.URL == "" {
		return liberrs.ArgInvalid("url is required for HTTP task")
	}
	if task.Dest == "" {
		return liberrs.ArgInvalid("dest is required for HTTP task")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(task.URL)
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskHTTPFailed, liberrs.CategoryNetwork, "failed to fetch '%s': %v", task.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return liberrs.Newf(liberrs.CodeTaskHTTPFailed, liberrs.CategoryNetwork, "HTTP request to '%s' returned status %d", task.URL, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(task.Dest), 0755); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to create directory for '%s': %v", task.Dest, err)
	}

	f, err := os.Create(task.Dest)
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to create file '%s': %v", task.Dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(task.Dest)
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to write to '%s': %v", task.Dest, err)
	}

	return nil
}

func execWait(task Task) error {
	task = task.Expand()

	if task.URL == "" {
		return liberrs.ArgInvalid("url is required for Wait task")
	}

	timeout := 30 * time.Second
	if task.Timeout != "" {
		d, err := time.ParseDuration(task.Timeout)
		if err != nil {
			return liberrs.Newf(liberrs.CodeArgInvalid, liberrs.CategoryConfig, "invalid timeout '%s': %v", task.Timeout, err)
		}
		timeout = d
	}

	lockedFprintf(commandStdout, "Waiting for %s (timeout: %s)...\n", task.URL, timeout)

	check := checkHTTP
	if !strings.HasPrefix(task.URL, "http://") && !strings.HasPrefix(task.URL, "https://") {
		check = checkTCP
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check(task.URL) == nil {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return liberrs.Newf(liberrs.CodeTaskWaitTimeout, liberrs.CategoryTask, "timed out waiting for '%s' after %s", task.URL, timeout)
}

func checkHTTP(url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return liberrs.Newf(liberrs.CodeTaskHTTPFailed, liberrs.CategoryNetwork, "HTTP %d", resp.StatusCode)
	}
	return nil
}

func checkTCP(address string) error {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func execTemplate(task Task) error {
	task = task.Expand()

	if task.Src == "" {
		return liberrs.ArgInvalid("src is required for Template task")
	}
	if task.Dest == "" {
		return liberrs.ArgInvalid("dest is required for Template task")
	}

	if !sys.FileExists(task.Src) {
		return liberrs.Newf(liberrs.CodeArgInvalid, liberrs.CategoryConfig, "template file does not exist: %s", task.Src)
	}

	data, err := os.ReadFile(task.Src)
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskTemplateFailed, liberrs.CategoryTask, "failed to read template '%s': %v", task.Src, err)
	}

	rendered := expandRaid(string(data))

	if err := os.MkdirAll(filepath.Dir(task.Dest), 0755); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to create directory for '%s': %v", task.Dest, err)
	}

	if err := os.WriteFile(task.Dest, []byte(rendered), 0644); err != nil {
		return liberrs.Newf(liberrs.CodeTaskTemplateFailed, liberrs.CategoryTask, "failed to write output file '%s': %v", task.Dest, err)
	}

	return nil
}

func execGroup(task Task) error {
	if task.Ref == "" {
		return liberrs.ArgInvalid("ref is required for Group task")
	}
	ctx := loadContext()
	if ctx == nil || ctx.Profile.Groups == nil {
		return liberrs.Newf(liberrs.CodeArgInvalid, liberrs.CategoryConfig, "no task_groups defined in the active profile")
	}

	tasks, ok := ctx.Profile.Groups[task.Ref]
	if !ok {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "task group '%s' not found in profile", task.Ref)
	}

	if task.Parallel {
		concurrent := make([]Task, len(tasks))
		for i, t := range tasks {
			t.Concurrent = true
			concurrent[i] = t
		}
		tasks = concurrent
	}

	if task.Attempts > 0 {
		return execGroupWithRetry(tasks, task.Attempts, task.Delay)
	}

	return ExecuteTasks(tasks)
}

func execGroupWithRetry(tasks []Task, attempts int, delayStr string) error {
	delay := time.Second
	if delayStr != "" {
		d, err := time.ParseDuration(delayStr)
		if err != nil {
			return liberrs.Newf(liberrs.CodeArgInvalid, liberrs.CategoryConfig, "invalid delay '%s': %v", delayStr, err)
		}
		delay = d
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			lockedFprintf(commandStdout, "Retrying... (attempt %d/%d)\n", i+1, attempts)
			time.Sleep(delay)
		}
		if err := ExecuteTasks(tasks); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "all %d attempts failed: %v", attempts, lastErr)
}

func execGit(task Task) error {
	task = task.Expand()

	if task.Op == "" {
		return liberrs.ArgInvalid("op is required for Git task")
	}

	dir := task.Path
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return liberrs.Newf(liberrs.CodeTaskGitFailed, liberrs.CategoryTask, "failed to get working directory: %v", err)
		}
	}

	info, statErr := os.Stat(dir)
	if statErr != nil || !info.IsDir() {
		return liberrs.Newf(liberrs.CodeArgInvalid, liberrs.CategoryConfig, "path is not a directory: %s", dir)
	}

	var args []string
	switch strings.ToLower(task.Op) {
	case "pull":
		args = []string{"pull"}
		if task.Branch != "" {
			args = append(args, "origin", task.Branch)
		}
	case "checkout":
		if task.Branch == "" {
			return liberrs.ArgInvalid("branch is required for git checkout")
		}
		args = []string{"checkout", task.Branch}
	case "fetch":
		args = []string{"fetch"}
		if task.Branch != "" {
			args = append(args, "origin", task.Branch)
		}
	case "reset":
		args = []string{"reset", "--hard"}
		if task.Branch != "" {
			args = append(args, task.Branch)
		}
	default:
		return liberrs.Newf(liberrs.CodeArgInvalid, liberrs.CategoryConfig, "invalid git operation '%s' (supported: pull, checkout, fetch, reset)", task.Op)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	flush := setCmdOutput(cmd, task)
	defer flush()

	if err := cmd.Run(); err != nil {
		return liberrs.Newf(liberrs.CodeTaskGitFailed, liberrs.CategoryTask, "git %s failed in '%s': %v", task.Op, dir, err)
	}

	return nil
}

func execPrompt(task Task) error {
	if task.Var == "" {
		return liberrs.ArgInvalid("var is required for Prompt task")
	}

	// Headless mode: skip stdin entirely. Use the declared default if
	// present; otherwise fail fast with a structured error. We refuse
	// to set the variable to "" silently because callers downstream
	// would silently misbehave on the missing value.
	if IsHeadless() {
		if task.Default == "" {
			return liberrs.HeadlessPromptNoDefault(task.Var)
		}
		os.Setenv(task.Var, task.Default)
		return nil
	}

	message := task.Message
	if message == "" {
		message = fmt.Sprintf("Enter value for %s:", task.Var)
	}

	stdinMu.Lock()
	defer stdinMu.Unlock()

	// outputMu held only for the banner write; releasing before the
	// stdin read so a blocking ReadString can't freeze concurrent
	// task output.
	lockedFprint(commandStdout, message+" ")

	value, err := getStdinReader().ReadString('\n')
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to read input: %v", err)
	}
	value = strings.TrimRight(value, "\r\n")

	if value == "" && task.Default != "" {
		value = task.Default
	}

	os.Setenv(task.Var, value)
	return nil
}

func execConfirm(task Task) error {
	// Headless mode auto-accepts every Confirm. This is the documented
	// trade-off for CI / agent invocations — destructive guards must be
	// expressed via something stricter than a Confirm prompt (an
	// environment-gated condition, a verify entry, etc.) when headless
	// callers are in scope.
	if IsHeadless() {
		return nil
	}

	message := task.Message
	if message == "" {
		message = "Continue?"
	}

	stdinMu.Lock()
	defer stdinMu.Unlock()

	// Banner held under outputMu but released before the stdin read
	// (which can block indefinitely); see execPrompt for rationale.
	lockedFprint(commandStdout, message+" [y/N] ")

	answer, err := getStdinReader().ReadString('\n')
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to read input: %v", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "aborted by user")
	}
	return nil
}

func execPrint(task Task) error {
	msg := task.Message
	if !task.Literal {
		msg = expandRaid(msg)
	}

	sink := io.Writer(commandStdout)
	if shouldPrefix(task, commandStdout) {
		color := ""
		if !colorDisabled() {
			color = colorForName(task.Label())
		}
		pw := newPrefixedWriter(commandStdout, buildPrefix(task.Label(), color))
		defer pw.Flush()
		sink = pw
	}

	if task.Color != "" {
		if code, ok := colorCodes[strings.ToLower(task.Color)]; ok {
			fmt.Fprintf(sink, "%s%s%s\n", code, msg, colorCodes["reset"])
			return nil
		}
	}

	fmt.Fprintln(sink, msg)
	return nil
}

func execSetVar(task Task) error {
	if task.Var == "" {
		return liberrs.ArgInvalid("var is required for Set task")
	}
	task = task.Expand()
	task.Var = strings.ToUpper(task.Var)

	path := raidVarsPath()

	// Serialize access to the shared vars file to avoid lost updates when
	// multiple Set tasks run concurrently.
	raidVarsMu.Lock()
	defer raidVarsMu.Unlock()

	f, err := sys.CreateFile(path)
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to create vars file: %v", err)
	}
	if err := f.Close(); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to close vars file: %v", err)
	}

	m, err := godotenv.Read(path)
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to read vars file: %v", err)
	}
	m[task.Var] = task.Value

	// Write updated vars atomically: write to a temporary file in the same
	// directory and then rename it over the original.
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".raid-vars-*")
	if err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to create temp vars file: %v", err)
	}
	tmpName := tmpFile.Name()
	// Ensure the temp file is removed if we hit an error before a successful rename.
	defer os.Remove(tmpName)

	if err := tmpFile.Close(); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to close temp vars file: %v", err)
	}

	if err := godotenv.Write(m, tmpName); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to write temp vars file: %v", err)
	}

	// Tighten perms BEFORE the rename so the final file lands at 0600
	// atomically — godotenv.Write defaults to 0644 (world-readable),
	// which is wrong for a file that holds raid vars (RAID_REPO_*_URL
	// can be a clone URL, and user-defined Set values can be anything
	// the project author treats as secret-ish). Chmod on the tempfile
	// path: if it fails we abort before rename so we never leave a
	// 0644 vars file on disk.
	if err := os.Chmod(tmpName, 0600); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to set vars file permissions: %v", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to replace vars file: %v", err)
	}

	raidVars[task.Var] = task.Value
	return nil
}

// withDefaultDir returns a copy of tasks with path set to dir on any Shell task
// that does not already have an explicit path. Used to apply profile-level (home)
// and repository-level (repo path) defaults without modifying the original slice.
func withDefaultDir(tasks []Task, dir string) []Task {
	if dir == "" || len(tasks) == 0 {
		return tasks
	}
	result := make([]Task, len(tasks))
	for i, t := range tasks {
		if t.Type.ToLower() == Shell && t.Path == "" {
			t.Path = dir
		}
		result[i] = t
	}
	return result
}
