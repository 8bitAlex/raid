package lib

import (
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
)

// There has to be a better way to do this... todo
type Task struct {
	Type       TaskType `json:"type"`
	Concurrent bool     `json:"concurrent,omitempty"`
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
}

func (t Task) IsZero() bool {
	return t.Type == ""
}

func (t Task) Expand() Task {
	return Task{
		Type:       t.Type,
		Concurrent: t.Concurrent,
		Cmd:        sys.Expand(t.Cmd),
		Literal:    t.Literal,
		Shell:      t.Shell,
		Path:       sys.Expand(t.Path),
		Runner:     sys.Expand(t.Runner),
		URL:        sys.Expand(t.URL),
		Dest:       sys.Expand(t.Dest),
		Timeout:    t.Timeout,
		Src:        sys.Expand(t.Src),
	}
}

type TaskType string

const (
	Shell    TaskType = "shell"
	Script   TaskType = "script"
	HTTP     TaskType = "http"
	Wait     TaskType = "wait"
	Template TaskType = "template"
)

func (t TaskType) ToLower() TaskType {
	return TaskType(strings.ToLower(string(t)))
}
