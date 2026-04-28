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

// recordCompleted runs the full Start→End lifecycle for a single command.
func recordCompleted(command string, runErr error) {
	started := RecordRecentStart(command)
	RecordRecentEnd(command, runErr, started)
}

func TestReadRecent_missingFileReturnsNil(t *testing.T) {
	setupRecentTempPath(t)
	if got := ReadRecent(); got != nil {
		t.Errorf("ReadRecent() = %v, want nil", got)
	}
}

func TestRecordRecent_writesCompletedEntry(t *testing.T) {
	setupRecentTempPath(t)

	now := time.Date(2026, 4, 27, 12, 0, 5, 0, time.UTC)
	oldNow := recentNowFn
	t.Cleanup(func() { recentNowFn = oldNow })

	tickCount := 0
	recentNowFn = func() time.Time {
		tickCount++
		// First call (Start) returns t0; second call (End) returns t0 + 2s.
		if tickCount == 1 {
			return now
		}
		return now.Add(2 * time.Second)
	}

	recordCompleted("deploy", nil)

	entries := ReadRecent()
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Command != "deploy" {
		t.Errorf("Command = %q, want %q", e.Command, "deploy")
	}
	if e.Status != RecentStatusCompleted {
		t.Errorf("Status = %q, want %q", e.Status, RecentStatusCompleted)
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
	recordCompleted("test", errors.New("boom"))

	entries := ReadRecent()
	if len(entries) != 1 || entries[0].ExitCode != 1 {
		t.Errorf("expected exit_code=1 from generic error, got: %+v", entries)
	}
	if entries[0].Status != RecentStatusCompleted {
		t.Errorf("Status = %q, want completed", entries[0].Status)
	}
}

func TestRecordRecent_prependsAndCaps(t *testing.T) {
	setupRecentTempPath(t)

	for i := 0; i < recentMaxEntries+5; i++ {
		recordCompleted("cmd", nil)
	}

	entries := ReadRecent()
	if len(entries) != recentMaxEntries {
		t.Errorf("entries len = %d, want %d", len(entries), recentMaxEntries)
	}
}

func TestRecordRecent_mostRecentFirst(t *testing.T) {
	setupRecentTempPath(t)

	recordCompleted("first", nil)
	recordCompleted("second", nil)
	recordCompleted("third", nil)

	entries := ReadRecent()
	if len(entries) != 3 {
		t.Fatalf("entries len = %d, want 3", len(entries))
	}
	if entries[0].Command != "third" || entries[2].Command != "first" {
		t.Errorf("order wrong: %+v", entries)
	}
}

// TestReadRecent_runningEntryReportedAsInterrupted simulates a process that
// was killed mid-command: it called RecordRecentStart but never reached
// RecordRecentEnd. ReadRecent should rewrite the surviving "running" entry to
// "interrupted" so callers see a terminal state.
func TestReadRecent_runningEntryReportedAsInterrupted(t *testing.T) {
	setupRecentTempPath(t)

	RecordRecentStart("deploy") // no matching End

	entries := ReadRecent()
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if entries[0].Status != RecentStatusInterrupted {
		t.Errorf("Status = %q, want %q", entries[0].Status, RecentStatusInterrupted)
	}
}

// TestRecordRecentEnd_updatesMatchingEntry confirms that when a command runs
// to completion, the placeholder pre-recorded by Start is upgraded in place,
// not duplicated.
func TestRecordRecentEnd_updatesMatchingEntry(t *testing.T) {
	setupRecentTempPath(t)

	started := RecordRecentStart("deploy")
	RecordRecentEnd("deploy", nil, started)

	entries := ReadRecent()
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1 (no duplicates)", len(entries))
	}
	if entries[0].Status != RecentStatusCompleted {
		t.Errorf("Status = %q, want completed", entries[0].Status)
	}
}

// TestRecordRecentEnd_interleavedRuns ensures End updates the matching Start
// even when an unrelated command runs in between (the entry is identified by
// command name + StartedAt, not by position).
func TestRecordRecentEnd_interleavedRuns(t *testing.T) {
	setupRecentTempPath(t)

	deployStart := RecordRecentStart("deploy")
	recordCompleted("test", nil)
	RecordRecentEnd("deploy", errors.New("oops"), deployStart)

	entries := ReadRecent()
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	// Entries are most-recent first, so test (the second Start) is index 0,
	// deploy (the first Start) is index 1.
	if entries[1].Command != "deploy" {
		t.Fatalf("expected deploy at index 1, got: %+v", entries[1])
	}
	if entries[1].Status != RecentStatusCompleted {
		t.Errorf("deploy Status = %q, want completed", entries[1].Status)
	}
	if entries[1].ExitCode != 1 {
		t.Errorf("deploy ExitCode = %d, want 1", entries[1].ExitCode)
	}
}

// TestRecordRecentEnd_distinguishesSubSecondStarts guards the precision fix:
// RecordRecentStart returns full-precision timestamps so two starts within
// the same wall-clock second can still be matched unambiguously by End.
// Earlier code truncated to whole seconds and would alias.
func TestRecordRecentEnd_distinguishesSubSecondStarts(t *testing.T) {
	setupRecentTempPath(t)

	now := time.Date(2026, 4, 27, 12, 0, 5, 100_000_000, time.UTC) // .100s
	oldNow := recentNowFn
	t.Cleanup(func() { recentNowFn = oldNow })

	// Drive recentNowFn through a sequence: two Starts in the same second,
	// then two Ends a moment later.
	calls := 0
	timeline := []time.Time{
		now,                                         // start A
		now.Add(200 * time.Millisecond),             // start B (same second, different ns)
		now.Add(time.Second),                        // end A
		now.Add(time.Second + 100*time.Millisecond), // end B
	}
	recentNowFn = func() time.Time {
		t := timeline[calls]
		calls++
		return t
	}

	startA := RecordRecentStart("alpha")
	startB := RecordRecentStart("beta")

	// Both starts share a wall-clock second but must remain distinguishable.
	if startA.Second() != startB.Second() {
		t.Fatalf("test fixture broken: starts must share a second; got %v / %v", startA, startB)
	}
	if startA.Equal(startB) {
		t.Fatalf("StartedAt timestamps must keep sub-second precision; both = %v", startA)
	}

	RecordRecentEnd("alpha", nil, startA)
	RecordRecentEnd("beta", errors.New("boom"), startB)

	entries := ReadRecent()
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	// Most recent first: beta then alpha. Each must be marked completed —
	// proving End found its own placeholder rather than aliasing the other.
	if entries[0].Command != "beta" || entries[0].Status != RecentStatusCompleted || entries[0].ExitCode != 1 {
		t.Errorf("beta entry wrong: %+v", entries[0])
	}
	if entries[1].Command != "alpha" || entries[1].Status != RecentStatusCompleted || entries[1].ExitCode != 0 {
		t.Errorf("alpha entry wrong: %+v", entries[1])
	}
}

func TestExitCodeFromError(t *testing.T) {
	if got := exitCodeFromError(nil); got != 0 {
		t.Errorf("nil error = %d, want 0", got)
	}
	if got := exitCodeFromError(errors.New("plain")); got != 1 {
		t.Errorf("plain error = %d, want 1", got)
	}
	cmd := exec.Command("sh", "-c", "exit 42")
	err := cmd.Run()
	if got := exitCodeFromError(err); got != 42 {
		t.Errorf("exec.ExitError(42) = %d, want 42", got)
	}
}
