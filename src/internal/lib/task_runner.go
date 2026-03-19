package lib

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/joho/godotenv"
)

// commandStdout and commandStderr are the output writers used by task execution.
// ExecuteCommand replaces these temporarily when a command's Out field is set.
var (
	commandStdout io.Writer = os.Stdout
	commandStderr io.Writer = os.Stderr
)

// stdinMu serializes all stdin reads so that concurrent Prompt and Confirm
// tasks do not interleave reads or compete for input.
var stdinMu sync.Mutex

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
				if err := ExecuteTask(task); err != nil {
					errorChan <- fmt.Errorf("failed to execute task '%s': %w", task.Type, err)
				}
			}(task)
		} else {
			if err := ExecuteTask(task); err != nil {
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

func ExecuteTask(task Task) error {
	if task.IsZero() {
		return nil
	}

	if task.Condition != nil && !evaluateCondition(task.Condition) {
		return nil
	}

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
		return fmt.Errorf("invalid task type: %s", task.Type)
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
		return fmt.Errorf("cmd is required for Shell task")
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
			// Run the command in a group so multi-line scripts work, preserve
			// the exit code, dump env, then exit with the original code.
			cmdStr = fmt.Sprintf("{\n%s\n}; __raid_exit=$?; env > '%s'; exit $__raid_exit", task.Cmd, tmpFile)
		}
	}

	cmd := exec.Command(shell[0], append(shell[1:], cmdStr)...)
	if task.Path != "" {
		cmd.Dir = sys.ExpandPath(task.Path)
	}
	setCmdOutput(cmd)

	runErr := cmd.Run()

	if tmpFile != "" {
		defer os.Remove(tmpFile)
		if data, err := os.ReadFile(tmpFile); err == nil {
			updateSessionFromEnv(data)
		}
	}

	if runErr != nil {
		return fmt.Errorf("failed to execute shell command '%s': %w", task.Cmd, runErr)
	}
	return nil
}

// updateSessionFromEnv parses the output of `env` and stores any variables
// that are new or changed relative to the session baseline.
func updateSessionFromEnv(data []byte) {
	if commandSession == nil {
		return
	}
	after := parseEnvLines(string(data))

	commandSession.mu.Lock()
	defer commandSession.mu.Unlock()
	for k, v := range after {
		baseVal, inBase := commandSession.baseline[k]
		if !inBase || baseVal != v {
			commandSession.vars[k] = v
		}
	}
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
		return fmt.Errorf("file does not exist: %s", task.Path)
	}

	var cmd *exec.Cmd
	if task.Runner != "" {
		cmd = exec.Command(task.Runner, task.Path)
	} else {
		cmd = exec.Command(task.Path)
	}

	setCmdOutput(cmd)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute script '%s': %w", task.Path, err)
	}

	return nil
}

func setCmdOutput(cmd *exec.Cmd) {
	cmd.Stdout = commandStdout
	cmd.Stderr = commandStderr
	cmd.Stdin = os.Stdin
}

func execHTTP(task Task) error {
	task = task.Expand()

	if task.URL == "" {
		return fmt.Errorf("url is required for HTTP task")
	}
	if task.Dest == "" {
		return fmt.Errorf("dest is required for HTTP task")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(task.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch '%s': %w", task.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP request to '%s' returned status %d", task.URL, resp.StatusCode)
	}

	if err := os.MkdirAll(filepath.Dir(task.Dest), 0755); err != nil {
		return fmt.Errorf("failed to create directory for '%s': %w", task.Dest, err)
	}

	f, err := os.Create(task.Dest)
	if err != nil {
		return fmt.Errorf("failed to create file '%s': %w", task.Dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(task.Dest)
		return fmt.Errorf("failed to write to '%s': %w", task.Dest, err)
	}

	return nil
}

func execWait(task Task) error {
	task = task.Expand()

	if task.URL == "" {
		return fmt.Errorf("url is required for Wait task")
	}

	timeout := 30 * time.Second
	if task.Timeout != "" {
		d, err := time.ParseDuration(task.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout '%s': %w", task.Timeout, err)
		}
		timeout = d
	}

	fmt.Fprintf(commandStdout, "Waiting for %s (timeout: %s)...\n", task.URL, timeout)

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

	return fmt.Errorf("timed out waiting for '%s' after %s", task.URL, timeout)
}

func checkHTTP(url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
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
		return fmt.Errorf("src is required for Template task")
	}
	if task.Dest == "" {
		return fmt.Errorf("dest is required for Template task")
	}

	if !sys.FileExists(task.Src) {
		return fmt.Errorf("template file does not exist: %s", task.Src)
	}

	data, err := os.ReadFile(task.Src)
	if err != nil {
		return fmt.Errorf("failed to read template '%s': %w", task.Src, err)
	}

	rendered := expandRaid(string(data))

	if err := os.MkdirAll(filepath.Dir(task.Dest), 0755); err != nil {
		return fmt.Errorf("failed to create directory for '%s': %w", task.Dest, err)
	}

	if err := os.WriteFile(task.Dest, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("failed to write output file '%s': %w", task.Dest, err)
	}

	return nil
}

func execGroup(task Task) error {
	if task.Ref == "" {
		return fmt.Errorf("ref is required for Group task")
	}
	if context == nil || context.Profile.Groups == nil {
		return fmt.Errorf("no task_groups defined in the active profile")
	}

	tasks, ok := context.Profile.Groups[task.Ref]
	if !ok {
		return fmt.Errorf("task group '%s' not found in profile", task.Ref)
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
			return fmt.Errorf("invalid delay '%s': %w", delayStr, err)
		}
		delay = d
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			fmt.Fprintf(commandStdout, "Retrying... (attempt %d/%d)\n", i+1, attempts)
			time.Sleep(delay)
		}
		if err := ExecuteTasks(tasks); err != nil {
			lastErr = err
			continue
		}
		return nil
	}

	return fmt.Errorf("all %d attempts failed: %w", attempts, lastErr)
}

func execGit(task Task) error {
	task = task.Expand()

	if task.Op == "" {
		return fmt.Errorf("op is required for Git task")
	}

	dir := task.Path
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	info, statErr := os.Stat(dir)
	if statErr != nil || !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
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
			return fmt.Errorf("branch is required for git checkout")
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
		return fmt.Errorf("invalid git operation '%s' (supported: pull, checkout, fetch, reset)", task.Op)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	setCmdOutput(cmd)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed in '%s': %w", task.Op, dir, err)
	}

	return nil
}

func execPrompt(task Task) error {
	if task.Var == "" {
		return fmt.Errorf("var is required for Prompt task")
	}

	message := task.Message
	if message == "" {
		message = fmt.Sprintf("Enter value for %s:", task.Var)
	}

	stdinMu.Lock()
	defer stdinMu.Unlock()

	fmt.Fprint(commandStdout, message+" ")

	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	value = strings.TrimRight(value, "\r\n")

	if value == "" && task.Default != "" {
		value = task.Default
	}

	os.Setenv(task.Var, value)
	return nil
}

func execConfirm(task Task) error {
	message := task.Message
	if message == "" {
		message = "Continue?"
	}

	stdinMu.Lock()
	defer stdinMu.Unlock()

	fmt.Fprint(commandStdout, message+" [y/N] ")

	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read input: %w", err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "y" && answer != "yes" {
		return fmt.Errorf("aborted by user")
	}
	return nil
}

func execPrint(task Task) error {
	msg := task.Message
	if !task.Literal {
		msg = expandRaid(msg)
	}

	if task.Color != "" {
		if code, ok := colorCodes[strings.ToLower(task.Color)]; ok {
			fmt.Fprintf(commandStdout, "%s%s%s\n", code, msg, colorCodes["reset"])
			return nil
		}
	}

	fmt.Fprintln(commandStdout, msg)
	return nil
}

func execSetVar(task Task) error {
	if task.Var == "" {
		return fmt.Errorf("var is required for Set task")
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
		return fmt.Errorf("failed to create vars file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close vars file: %w", err)
	}

	m, err := godotenv.Read(path)
	if err != nil {
		return fmt.Errorf("failed to read vars file: %w", err)
	}
	m[task.Var] = task.Value

	// Write updated vars atomically: write to a temporary file in the same
	// directory and then rename it over the original.
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".raid-vars-*")
	if err != nil {
		return fmt.Errorf("failed to create temp vars file: %w", err)
	}
	tmpName := tmpFile.Name()
	// Ensure the temp file is removed if we hit an error before a successful rename.
	defer os.Remove(tmpName)

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp vars file: %w", err)
	}

	if err := godotenv.Write(m, tmpName); err != nil {
		return fmt.Errorf("failed to write temp vars file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("failed to replace vars file: %w", err)
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
