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

// IsZero reports whether no condition fields are set.
func (c Condition) IsZero() bool {
	return c.Platform == "" && c.Exists == "" && c.Cmd == ""
}

// Task represents a single unit of work in a task sequence.
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
	// Group
	Ref      string `json:"ref,omitempty"`
	Parallel bool   `json:"parallel,omitempty"`
	// Git
	Op     string `json:"op,omitempty"`
	Branch string `json:"branch,omitempty"`
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

// IsZero reports whether the task has no type set.
func (t Task) IsZero() bool {
	return t.Type == ""
}

// Expand returns a copy of the task with all string fields passed through environment variable expansion.
func (t Task) Expand() Task {
	return Task{
		Type:       t.Type,
		Concurrent: t.Concurrent,
		Condition:  t.Condition,
		Cmd:        sys.Expand(t.Cmd),
		Literal:    t.Literal,
		Shell:      t.Shell,
		Path:       sys.ExpandPath(t.Path),
		Runner:     sys.ExpandPath(t.Runner),
		URL:        sys.Expand(t.URL),
		Dest:       sys.ExpandPath(t.Dest),
		Timeout:    t.Timeout,
		Src:        sys.ExpandPath(t.Src),
		Ref:        t.Ref,
		Parallel:   t.Parallel,
		Op:         t.Op,
		Branch:     sys.Expand(t.Branch),
		Message:    sys.Expand(t.Message),
		Var:        t.Var,
		Default:    sys.Expand(t.Default),
		Color:      t.Color,
		Attempts:   t.Attempts,
		Delay:      t.Delay,
	}
}

// TaskType identifies which task executor to dispatch to.
type TaskType string

const (
	Shell    TaskType = "shell"
	Script   TaskType = "script"
	HTTP     TaskType = "http"
	Wait     TaskType = "wait"
	Template TaskType = "template"
	Group   TaskType = "group"
	Git     TaskType = "git"
	Prompt  TaskType = "prompt"
	Confirm TaskType = "confirm"
	Print   TaskType = "print"
)

// ToLower returns the task type normalized to lowercase for case-insensitive comparisons.
func (t TaskType) ToLower() TaskType {
	return TaskType(strings.ToLower(string(t)))
}
