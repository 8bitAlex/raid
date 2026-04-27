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
	recentFileName   = "recent.json"
	recentMaxEntries = 10
)

// Recent statuses surfaced via ReadRecent. "running" only ever appears on disk
// while a command is in flight; ReadRecent rewrites it to RecentStatusInterrupted
// so consumers always see one of these two terminal states.
const (
	RecentStatusCompleted   = "completed"
	RecentStatusInterrupted = "interrupted"

	recentStatusRunning = "running"
)

// RecentEntry records a single `raid <command>` invocation. The log is
// intentionally per-command (not per-task) — agents asking "what did the
// developer just run?" want a high-level history, not every Shell step.
//
// Lifecycle: an entry is first written on command start with Status="running",
// then updated to Status="completed" once the command exits normally. If the
// process is killed (SIGINT/SIGTERM/SIGKILL), the running entry survives on
// disk and ReadRecent reports it as Status="interrupted".
type RecentEntry struct {
	Command    string    `json:"command"`
	Status     string    `json:"status"`
	ExitCode   int       `json:"exitCode"`
	StartedAt  time.Time `json:"startedAt"`
	DurationMs int64     `json:"durationMs,omitempty"`
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
//
// Any entry still in the on-disk "running" state is rewritten to
// RecentStatusInterrupted before returning, so callers always see a terminal
// status. Concurrent in-flight invocations will be misreported, but that's an
// acceptable trade-off for a tool that is overwhelmingly run sequentially.
func ReadRecent() []RecentEntry {
	entries := readRecentRaw()
	for i := range entries {
		if entries[i].Status == recentStatusRunning {
			entries[i].Status = RecentStatusInterrupted
		}
	}
	return entries
}

// readRecentRaw reads the file without rewriting the running→interrupted
// status, so internal updaters can find their own placeholder entry.
func readRecentRaw() []RecentEntry {
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

// RecordRecentStart prepends a placeholder entry for command in the recent
// log and returns the moment recording began. Pass the same value to
// RecordRecentEnd once the command has finished. Errors writing the log are
// silenced — recording history must never break command execution.
func RecordRecentStart(command string) time.Time {
	started := recentNowFn().UTC().Truncate(time.Second)
	entry := RecentEntry{
		Command:   command,
		Status:    recentStatusRunning,
		StartedAt: started,
	}

	recentMu.Lock()
	defer recentMu.Unlock()

	entries := readRecentRaw()
	entries = append([]RecentEntry{entry}, entries...)
	if len(entries) > recentMaxEntries {
		entries = entries[:recentMaxEntries]
	}
	writeRecent(entries)
	return started
}

// RecordRecentEnd updates the placeholder entry written by RecordRecentStart
// with the command's exit code and total duration. If the matching entry has
// fallen off the cap, no update is recorded.
func RecordRecentEnd(command string, runErr error, startedAt time.Time) {
	now := recentNowFn()

	recentMu.Lock()
	defer recentMu.Unlock()

	entries := readRecentRaw()
	for i := range entries {
		if entries[i].Command == command && entries[i].StartedAt.Equal(startedAt) {
			entries[i].Status = RecentStatusCompleted
			entries[i].ExitCode = exitCodeFromError(runErr)
			entries[i].DurationMs = now.Sub(startedAt).Milliseconds()
			break
		}
	}
	writeRecent(entries)
}

// writeRecent atomically replaces the recent log file. Caller holds recentMu.
func writeRecent(entries []RecentEntry) {
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
