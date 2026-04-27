package lib

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	sys "github.com/8bitalex/raid/src/internal/sys"
)

const (
	recentFileName  = "recent.json"
	recentMaxEntries = 10
)

// RecentEntry records a single completed `raid <command>` invocation. The log
// is intentionally per-command (not per-task) — agents asking "what did the
// developer just run?" want a high-level history, not every Shell step.
type RecentEntry struct {
	Command    string    `json:"command"`
	ExitCode   int       `json:"exit_code"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
}

// RecentPathOverride redirects the recent.json log path. Intended only for
// tests that exercise ExecuteCommand so they do not pollute the developer's
// real ~/.raid/recent.json.
var RecentPathOverride string

// Overridable in tests.
var (
	recentNowFn = time.Now
	recentMu    sync.Mutex
)

func recentPath() string {
	if RecentPathOverride != "" {
		return RecentPathOverride
	}
	return filepath.Join(sys.GetHomeDir(), ConfigDirName, recentFileName)
}

// ReadRecent returns the most-recent-first list of recorded command runs.
// Returns nil if the log file does not exist or cannot be parsed; callers
// should treat absence as "no history yet".
func ReadRecent() []RecentEntry {
	path := recentPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var entries []RecentEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// RecordRecent prepends a new entry to the recent log, capping the file at
// recentMaxEntries. Errors writing the log are silenced — recording history
// must never break command execution.
func RecordRecent(command string, runErr error, startedAt time.Time) {
	entry := RecentEntry{
		Command:    command,
		ExitCode:   exitCodeFromError(runErr),
		StartedAt:  startedAt.UTC().Truncate(time.Second),
		DurationMs: recentNowFn().Sub(startedAt).Milliseconds(),
	}

	recentMu.Lock()
	defer recentMu.Unlock()

	entries := ReadRecent()
	entries = append([]RecentEntry{entry}, entries...)
	if len(entries) > recentMaxEntries {
		entries = entries[:recentMaxEntries]
	}

	path := recentPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func exitCodeFromError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
