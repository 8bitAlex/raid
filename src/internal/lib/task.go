package lib

import (
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
)

// Condition guards a task — all specified fields must be satisfied for the task to run.
type Condition struct {
	Platform string `json:"platform,omitempty"`
	Exists   string `json:"exists,omitempty"`
	Cmd      string `json:"cmd,omitempty"`
}

func (c Condition) IsZero() bool {
	return c.Platform == "" && c.Exists == "" && c.Cmd == ""
}

// There has to be a better way to do this... todo
type Task struct {
	Type       TaskType   `json:"type"`
	Concurrent bool       `json:"concurrent,omitempty"`
	Condition  *Condition `json:"condition,omitempty"`
	// Shell
	Cmd     string `json:"cmd,omitempty"`
	Literal bool   `json:"literal,omitempty"`
	Shell   string `json:"shell,omitempty"`
	// Script
	Path   string `json:"path,omitempty"`
	Runner string `json:"runner,omitempty"`
	// HTTP
	URL  string `json:"url,omitempty"`
	Dest string `json:"dest,omitempty"`
	// Wait
	Timeout string `json:"timeout,omitempty"`
	// Template
	Src string `json:"src,omitempty"`
	// Group / Parallel / Retry
	Ref string `json:"ref,omitempty"`
	// Git
	Op     string `json:"op,omitempty"`
	Branch string `json:"branch,omitempty"`
	Dir    string `json:"dir,omitempty"`
	// Prompt / Confirm / Print
	Message string `json:"message,omitempty"`
	// Prompt
	Var     string `json:"var,omitempty"`
	Default string `json:"default,omitempty"`
	// Print
	Color string `json:"color,omitempty"`
	// Retry
	Attempts int    `json:"attempts,omitempty"`
	Delay    string `json:"delay,omitempty"`
}

func (t Task) IsZero() bool {
	return t.Type == ""
}

func (t Task) Expand() Task {
	return Task{
		Type:       t.Type,
		Concurrent: t.Concurrent,
		Condition:  t.Condition,
		Cmd:        sys.Expand(t.Cmd),
		Literal:    t.Literal,
		Shell:      t.Shell,
		Path:       sys.Expand(t.Path),
		Runner:     sys.Expand(t.Runner),
		URL:        sys.Expand(t.URL),
		Dest:       sys.Expand(t.Dest),
		Timeout:    t.Timeout,
		Src:        sys.Expand(t.Src),
		Ref:        t.Ref,
		Op:         t.Op,
		Branch:     sys.Expand(t.Branch),
		Dir:        sys.Expand(t.Dir),
		Message:    sys.Expand(t.Message),
		Var:        t.Var,
		Default:    sys.Expand(t.Default),
		Color:      t.Color,
		Attempts:   t.Attempts,
		Delay:      t.Delay,
	}
}

type TaskType string

const (
	Shell    TaskType = "shell"
	Script   TaskType = "script"
	HTTP     TaskType = "http"
	Wait     TaskType = "wait"
	Template TaskType = "template"
	Group    TaskType = "group"
	Git      TaskType = "git"
	Prompt   TaskType = "prompt"
	Confirm  TaskType = "confirm"
	Parallel TaskType = "parallel"
	Print    TaskType = "print"
	Retry    TaskType = "retry"
)

func (t TaskType) ToLower() TaskType {
	return TaskType(strings.ToLower(string(t)))
}
