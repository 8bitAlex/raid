package lib

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
)

type Task struct {
	Type TaskType `json:"type"`
	Cmd  string   `json:"cmd"`
	Path string   `json:"path"`
}

type TaskType string
const (
	Shell  TaskType = "shell"
	Script TaskType = "script"
)

func (t Task) IsZero() bool {
	return t.Cmd == "" && t.Path == "" && t.Type == ""
}

func (t TaskType) ToLower() TaskType {
	return TaskType(strings.ToLower(string(t)))
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
	args := strings.Split(task.Cmd, " ")
	cmd := exec.Command(args[0], args[1:]...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute shell command '%s': %w", task.Cmd, err)
	}
	
	return nil
}

func execScript(task Task) error {
	path := sys.ExpandPath(task.Path)
	if !sys.FileExists(path) {
		return fmt.Errorf("script file does not exist: %s", path)
	}

	cmd := exec.Command(path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute script '%s': %w", path, err)
	}

	return nil
}