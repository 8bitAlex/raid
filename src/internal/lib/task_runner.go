package lib

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

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
