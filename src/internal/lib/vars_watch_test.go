package lib

import (
	stdctx "context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// withIsolatedRaidVars swaps raidVars and raidVarsOverridePath for the
// duration of the test, restoring both on cleanup so the test can write to
// a temp vars file without polluting the package-global state.
func withIsolatedRaidVars(t *testing.T) (varsPath string) {
	t.Helper()
	dir := t.TempDir()
	varsPath = filepath.Join(dir, "vars")

	raidVarsMu.Lock()
	prevVars := raidVars
	prevPath := raidVarsOverridePath
	raidVars = map[string]string{}
	raidVarsOverridePath = varsPath
	raidVarsMu.Unlock()

	t.Cleanup(func() {
		raidVarsMu.Lock()
		raidVars = prevVars
		raidVarsOverridePath = prevPath
		raidVarsMu.Unlock()
	})
	return varsPath
}

func TestSnapshotRaidVars_ReturnsNilWhenEmpty(t *testing.T) {
	withIsolatedRaidVars(t)
	if got := snapshotRaidVars(); got != nil {
		t.Fatalf("expected nil snapshot for empty map, got %v", got)
	}
}

func TestSnapshotRaidVars_CopiesMap(t *testing.T) {
	withIsolatedRaidVars(t)

	raidVarsMu.Lock()
	raidVars["FOO"] = "bar"
	raidVars["BAZ"] = "qux"
	raidVarsMu.Unlock()

	snap := snapshotRaidVars()
	if snap["FOO"] != "bar" || snap["BAZ"] != "qux" {
		t.Fatalf("snapshot missing entries: %v", snap)
	}

	// Mutating the snapshot must not leak back into raidVars.
	snap["FOO"] = "tampered"
	delete(snap, "BAZ")

	raidVarsMu.RLock()
	defer raidVarsMu.RUnlock()
	if raidVars["FOO"] != "bar" {
		t.Fatalf("raidVars leaked: FOO=%q", raidVars["FOO"])
	}
	if raidVars["BAZ"] != "qux" {
		t.Fatalf("raidVars leaked: BAZ=%q", raidVars["BAZ"])
	}
}

func TestWatchRaidVars_NilOnChangeRejected(t *testing.T) {
	if err := WatchRaidVars(stdctx.Background(), nil); err == nil {
		t.Fatal("expected error for nil onChange")
	}
}

func TestWatchRaidVars_FactoryError(t *testing.T) {
	prev := newVarsWatcherFn
	t.Cleanup(func() { newVarsWatcherFn = prev })
	sentinel := errors.New("boom")
	newVarsWatcherFn = func(_ stdctx.Context, _ string, _ func()) error {
		return sentinel
	}
	if err := WatchRaidVars(stdctx.Background(), func() {}); !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got %v", err)
	}
}

func TestWatchRaidVars_FiresOnChange(t *testing.T) {
	varsPath := withIsolatedRaidVars(t)

	// Tighten the debounce so the test doesn't sleep forever, but leave
	// it large enough to coalesce the inevitable multi-event write burst.
	prev := varsWatchDebounce
	varsWatchDebounce = 20 * time.Millisecond
	t.Cleanup(func() { varsWatchDebounce = prev })

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	t.Cleanup(cancel)

	var calls atomic.Int32
	done := make(chan struct{}, 1)
	if err := WatchRaidVars(ctx, func() {
		calls.Add(1)
		select {
		case done <- struct{}{}:
		default:
		}
	}); err != nil {
		t.Fatalf("WatchRaidVars: %v", err)
	}

	// fsnotify needs a beat to attach to the directory before our writes
	// land — without this the event is emitted before the watcher is
	// observing and the test races.
	time.Sleep(30 * time.Millisecond)

	if err := os.WriteFile(varsPath, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onChange was not invoked")
	}

	if got := calls.Load(); got < 1 {
		t.Fatalf("expected at least one onChange call, got %d", got)
	}
}

func TestWatchRaidVars_DebouncesBurst(t *testing.T) {
	varsPath := withIsolatedRaidVars(t)

	prev := varsWatchDebounce
	varsWatchDebounce = 100 * time.Millisecond
	t.Cleanup(func() { varsWatchDebounce = prev })

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	t.Cleanup(cancel)

	var calls atomic.Int32
	if err := WatchRaidVars(ctx, func() { calls.Add(1) }); err != nil {
		t.Fatalf("WatchRaidVars: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	// Five rapid writes inside one debounce window should fire onChange
	// at most a small number of times (typically once); they must not
	// fire five times.
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(varsPath, []byte("FOO=bar\n"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for the debounce window to flush plus a margin.
	time.Sleep(200 * time.Millisecond)

	if got := calls.Load(); got >= 5 {
		t.Fatalf("debounce broken: got %d calls for 5 rapid writes", got)
	}
	if got := calls.Load(); got == 0 {
		t.Fatal("expected at least one debounced call")
	}
}

func TestWatchRaidVars_StopsOnContextCancel(t *testing.T) {
	varsPath := withIsolatedRaidVars(t)

	prev := varsWatchDebounce
	varsWatchDebounce = 20 * time.Millisecond
	t.Cleanup(func() { varsWatchDebounce = prev })

	ctx, cancel := stdctx.WithCancel(stdctx.Background())

	var mu sync.Mutex
	var calls int
	if err := WatchRaidVars(ctx, func() {
		mu.Lock()
		calls++
		mu.Unlock()
	}); err != nil {
		t.Fatalf("WatchRaidVars: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	cancel()
	// Give the goroutine time to observe ctx.Done.
	time.Sleep(30 * time.Millisecond)

	// Writing after cancel must not fire onChange.
	if err := os.WriteFile(varsPath, []byte("FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	time.Sleep(80 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if calls != 0 {
		t.Fatalf("watcher fired after cancel: %d calls", calls)
	}
}

// TestNewVarsWatcher_mkdirFailureReturnsStructuredError covers the
// liberrs.Newf wrap around os.MkdirAll. Forcing MkdirAll to fail
// requires a path whose parent is a regular file (not a directory) so
// the implicit mkdir-p chain can't make a directory of the same name.
func TestNewVarsWatcher_mkdirFailureReturnsStructuredError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a regular file, then aim varsPath at a path UNDER that
	// file. MkdirAll then can't make varsPath's parent — it would have
	// to turn the existing file into a directory.
	blocker := filepath.Join(tmpDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	varsPath := filepath.Join(blocker, "child", "vars")

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	t.Cleanup(cancel)

	err := newVarsWatcher(ctx, varsPath, func() {})
	if err == nil {
		t.Fatal("expected error when parent path is a file")
	}
	if !strings.Contains(err.Error(), "ensure vars watch dir") {
		t.Errorf("error %q should mention 'ensure vars watch dir'", err.Error())
	}
}

func TestNewVarsWatcher_CreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "raid")
	varsPath := filepath.Join(dir, "vars")

	ctx, cancel := stdctx.WithCancel(stdctx.Background())
	t.Cleanup(cancel)

	if err := newVarsWatcher(ctx, varsPath, func() {}); err != nil {
		t.Fatalf("newVarsWatcher: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("parent dir was not created: %v", err)
	}
}
