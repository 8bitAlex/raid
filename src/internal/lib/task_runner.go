package lib

import (
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
)

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
				errs := []error{fmt.Errorf("failed to execute task '%s': %w", task.Type, err)}
				for e := range errorChan {
					errs = append(errs, e)
				}
				return fmt.Errorf("some tasks failed to execute: %v", errs)
			}
		}
	}

	wg.Wait()
	close(errorChan)

	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("some tasks failed to execute: %v", errors)
	}

	return nil
}

func ExecuteTask(task Task) error {
	if task.IsZero() {
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
	default:
		return fmt.Errorf("invalid task type: %s", task.Type)
	}
}

func execShell(task Task) error {
	if !task.Literal {
		task = task.Expand()
	}

	shell := getShell(task.Shell)
	cmd := exec.Command(shell[0], append(shell[1:], task.Cmd)...)
	if !task.Concurrent {
		setCmdOutput(cmd)
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute shell command '%s': %w", task.Cmd, err)
	}

	return nil
}

func getShell(shell string) []string {
	if shell == "" {
		if sys.GetPlatform() == sys.Windows {
			return []string{"cmd", "/c"}
		}
		return []string{"/bin/bash", "-c"}
	}

	shell = strings.ToLower(shell)
	switch shell {
	case "/bin/bash", "bash":
		return []string{"/bin/bash", "-c"}
	case "/bin/sh", "sh":
		return []string{"/bin/sh", "-c"}
	case "/bin/zsh", "zsh":
		return []string{"/bin/zsh", "-c"}
	case "powershell", "pwsh", "ps":
		return []string{"powershell"}
	case "cmd":
		return []string{"cmd", "/c"}
	default:
		return []string{"/bin/bash", "-c"}
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

	if !task.Concurrent {
		setCmdOutput(cmd)
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute script '%s': %w", task.Path, err)
	}

	return nil
}

func setCmdOutput(cmd *exec.Cmd) {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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

	resp, err := http.Get(task.URL)
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

	fmt.Printf("Waiting for %s (timeout: %s)...\n", task.URL, timeout)

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

	rendered := os.ExpandEnv(string(data))

	if err := os.MkdirAll(filepath.Dir(task.Dest), 0755); err != nil {
		return fmt.Errorf("failed to create directory for '%s': %w", task.Dest, err)
	}

	if err := os.WriteFile(task.Dest, []byte(rendered), 0644); err != nil {
		return fmt.Errorf("failed to write output file '%s': %w", task.Dest, err)
	}

	return nil
}
