package lib

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
)

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
	
	args := strings.Split(task.Cmd, " ")
	if len(args) == 0 {
		return fmt.Errorf("no command provided for task")
	}

	cmd := exec.Command(args[0], args[1:]...)
	setCmdOutput(cmd)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute shell command '%s': %w", task.Cmd, err)
	}
	
	return nil
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
}