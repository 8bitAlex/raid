package lib

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func setupRecentTempPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "recent.json")
	old := RecentPathOverride
	t.Cleanup(func() { RecentPathOverride = old })
	RecentPathOverride = path
	return path
}

func TestReadRecent_missingFileReturnsNil(t *testing.T) {
	setupRecentTempPath(t)
	if got := ReadRecent(); got != nil {
		t.Errorf("ReadRecent() = %v, want nil", got)
	}
}

func TestRecordRecent_writesEntry(t *testing.T) {
	setupRecentTempPath(t)

	now := time.Date(2026, 4, 27, 12, 0, 5, 0, time.UTC)
	oldNow := recentNowFn
	t.Cleanup(func() { recentNowFn = oldNow })
	recentNowFn = func() time.Time { return now }

	start := now.Add(-2 * time.Second)
	RecordRecent("deploy", nil, start)

	entries := ReadRecent()
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Command != "deploy" {
		t.Errorf("Command = %q, want %q", e.Command, "deploy")
	}
	if e.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", e.ExitCode)
	}
	if e.DurationMs != 2000 {
		t.Errorf("DurationMs = %d, want 2000", e.DurationMs)
	}
}

func TestRecordRecent_capturesExitCodeFromError(t *testing.T) {
	setupRecentTempPath(t)
	RecordRecent("test", errors.New("boom"), time.Now())

	entries := ReadRecent()
	if len(entries) != 1 || entries[0].ExitCode != 1 {
		t.Errorf("expected exit_code=1 from generic error, got: %+v", entries)
	}
}

func TestRecordRecent_prependsAndCaps(t *testing.T) {
	setupRecentTempPath(t)

	for i := 0; i < recentMaxEntries+5; i++ {
		RecordRecent("cmd", nil, time.Now())
	}

	entries := ReadRecent()
	if len(entries) != recentMaxEntries {
		t.Errorf("entries len = %d, want %d", len(entries), recentMaxEntries)
	}
}

func TestRecordRecent_mostRecentFirst(t *testing.T) {
	setupRecentTempPath(t)

	RecordRecent("first", nil, time.Now())
	RecordRecent("second", nil, time.Now())
	RecordRecent("third", nil, time.Now())

	entries := ReadRecent()
	if len(entries) != 3 {
		t.Fatalf("entries len = %d, want 3", len(entries))
	}
	if entries[0].Command != "third" || entries[2].Command != "first" {
		t.Errorf("order wrong: %+v", entries)
	}
}

func TestExitCodeFromError(t *testing.T) {
	if got := exitCodeFromError(nil); got != 0 {
		t.Errorf("nil error = %d, want 0", got)
	}
	if got := exitCodeFromError(errors.New("plain")); got != 1 {
		t.Errorf("plain error = %d, want 1", got)
	}
	// Synthesise an *exec.ExitError by running a command that exits non-zero.
	cmd := exec.Command("sh", "-c", "exit 42")
	err := cmd.Run()
	if got := exitCodeFromError(err); got != 42 {
		t.Errorf("exec.ExitError(42) = %d, want 42", got)
	}
}
