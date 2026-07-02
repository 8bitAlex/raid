package lib

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		now,                             // start A
		now.Add(200 * time.Millisecond), // start B (same second, different ns)
		now.Add(time.Second),            // end A
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

func TestReadRecent_corruptJSON(t *testing.T) {
	setupRecentTempPath(t)
	path := recentPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	os.WriteFile(path, []byte("{invalid json"), 0644)

	got := ReadRecent()
	if got != nil {
		t.Errorf("ReadRecent() with corrupt JSON = %v, want nil", got)
	}
}

func TestRecordRecent_writeError(t *testing.T) {
	// Place a regular file where writeRecent expects a directory so
	// MkdirAll(filepath.Dir(path)) returns ENOTDIR and the function
	// bails before writing. The contract is that recording must never
	// break command execution (errors are silenced) AND that nothing
	// gets persisted — so ReadRecent must still return nil afterwards.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("not-a-dir"), 0644); err != nil {
		t.Fatal(err)
	}
	old := RecentPathOverride
	t.Cleanup(func() { RecentPathOverride = old })
	RecentPathOverride = filepath.Join(blocker, "recent.json")

	started := RecordRecentStart("cmd")
	RecordRecentEnd("cmd", nil, started)

	if got := ReadRecent(); got != nil {
		t.Errorf("ReadRecent() after failed writes = %v, want nil (nothing persisted)", got)
	}
}

// --- writeRecent Windows-style rename fallback ---

// withRenameFn substitutes the rename operation for the duration of a test.
func withRenameFn(t *testing.T, fn func(old, new string) error) {
	t.Helper()
	prev := renameFn
	renameFn = fn
	t.Cleanup(func() { renameFn = prev })
}

func TestWriteRecent_fallbackSwapsViaAsideFile(t *testing.T) {
	path := setupRecentTempPath(t)
	if err := os.WriteFile(path, []byte(`[{"command":"old"}]`), 0644); err != nil {
		t.Fatal(err)
	}

	// Simulate Windows: renaming over an existing destination fails.
	withRenameFn(t, func(oldPath, newPath string) error {
		if _, err := os.Stat(newPath); err == nil {
			return errors.New("destination exists")
		}
		return os.Rename(oldPath, newPath)
	})

	writeRecent([]RecentEntry{{Command: "new"}})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("recent file missing after fallback swap: %v", err)
	}
	if got := string(data); !strings.Contains(got, `"new"`) {
		t.Errorf("recent file = %s, want the new entry", got)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Error("aside .bak file should be removed after a successful swap")
	}
}

func TestWriteRecent_fallbackAsideFailureKeepsOriginal(t *testing.T) {
	path := setupRecentTempPath(t)
	if err := os.WriteFile(path, []byte(`[{"command":"old"}]`), 0644); err != nil {
		t.Fatal(err)
	}

	// Every rename fails: the first (over existing) and the move-aside.
	withRenameFn(t, func(oldPath, newPath string) error {
		return errors.New("rename refused")
	})

	writeRecent([]RecentEntry{{Command: "new"}})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("previous recent file must survive: %v", err)
	}
	if got := string(data); !strings.Contains(got, `"old"`) {
		t.Errorf("recent file = %s, want previous content preserved", got)
	}
}

func TestWriteRecent_fallbackSecondRenameFailureRestoresOriginal(t *testing.T) {
	path := setupRecentTempPath(t)
	if err := os.WriteFile(path, []byte(`[{"command":"old"}]`), 0644); err != nil {
		t.Fatal(err)
	}

	// First rename (over existing) fails; the move-aside succeeds; the
	// swap-in fails; the restore rename succeeds. The previous history
	// must be back at path afterwards — this is the loss the aside file
	// exists to prevent.
	calls := 0
	withRenameFn(t, func(oldPath, newPath string) error {
		calls++
		switch calls {
		case 1: // tmp → path (destination exists)
			return errors.New("destination exists")
		case 2: // path → path.bak
			return os.Rename(oldPath, newPath)
		case 3: // tmp → path (simulated sharing violation)
			return errors.New("sharing violation")
		default: // path.bak → path restore
			return os.Rename(oldPath, newPath)
		}
	})

	writeRecent([]RecentEntry{{Command: "new"}})

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("previous recent file must be restored: %v", err)
	}
	if got := string(data); !strings.Contains(got, `"old"`) {
		t.Errorf("recent file = %s, want restored previous content", got)
	}
}
