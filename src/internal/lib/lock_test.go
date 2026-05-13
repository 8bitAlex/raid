package lib

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
)

func setupLockTempPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, ".lock")
	old := LockPathOverride
	t.Cleanup(func() { LockPathOverride = old })
	LockPathOverride = path
	return path
}

// TestAcquireMutationLock_mkdirAllFails covers the LockFailed-wrap path
// when the parent directory cannot be created (parent path is a regular
// file). Verifies the returned error is a structured LOCK_FAILED with
// the user-facing hint.
func TestAcquireMutationLock_mkdirAllFails(t *testing.T) {
	tmpDir := t.TempDir()
	blocker := filepath.Join(tmpDir, "blocker-file")
	if err := os.WriteFile(blocker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	old := LockPathOverride
	t.Cleanup(func() { LockPathOverride = old })
	// LockPathOverride parent is a regular file — MkdirAll can't create
	// the intermediate dir under it.
	LockPathOverride = filepath.Join(blocker, "child", ".lock")

	release, err := AcquireMutationLock()
	if release != nil {
		release()
	}
	if err == nil {
		t.Fatal("expected error when lock parent dir cannot be created")
	}
	rErr, ok := liberrs.AsError(err)
	if !ok {
		t.Fatalf("error not structured: %v", err)
	}
	if rErr.Code() != liberrs.CodeLockFailed {
		t.Errorf("code = %q, want LOCK_FAILED", rErr.Code())
	}
}

func TestAcquireMutationLock_returnsRelease(t *testing.T) {
	setupLockTempPath(t)
	release, err := AcquireMutationLock()
	if err != nil {
		t.Fatalf("AcquireMutationLock: %v", err)
	}
	if release == nil {
		t.Fatal("release fn is nil")
	}
	release()
}

// TestAcquireMutationLock_serialisesGoroutines is the core correctness
// proof: a second AcquireMutationLock from a different goroutine must block
// until the first one releases. With a process-level mutex (the old design)
// this would also pass. With flock, the same property holds across
// processes — which is the goal — but verifying that requires spawning a
// subprocess. The single-process variant exercises flock's reentrancy
// behaviour: gofrs/flock does NOT permit recursive acquisition from the
// same process via a fresh handle, so this test legitimately blocks.
func TestAcquireMutationLock_serialisesGoroutines(t *testing.T) {
	setupLockTempPath(t)

	// Acquire the lock and hold it briefly. While held, a second goroutine
	// tries to acquire — it must block until release.
	first, err := AcquireMutationLock()
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	var secondAcquired atomic.Bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		release, err := AcquireMutationLock()
		if err != nil {
			t.Errorf("second acquire: %v", err)
			return
		}
		secondAcquired.Store(true)
		release()
	}()

	// Give the goroutine a chance to attempt acquisition.
	time.Sleep(100 * time.Millisecond)
	if secondAcquired.Load() {
		t.Fatal("second AcquireMutationLock returned while the first was still held")
	}

	first()

	// After release, the goroutine should proceed promptly.
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second AcquireMutationLock did not unblock within 2s of release")
	}
	if !secondAcquired.Load() {
		t.Error("second AcquireMutationLock never reported success")
	}
}

func TestWithMutationLock_propagatesError(t *testing.T) {
	setupLockTempPath(t)
	want := errors.New("boom")
	got := WithMutationLock(func() error { return want })
	if !errors.Is(got, want) {
		t.Errorf("got = %v, want %v", got, want)
	}
}

func TestWithMutationLock_releasesOnPanic(t *testing.T) {
	setupLockTempPath(t)
	defer func() { _ = recover() }()

	func() {
		defer func() { _ = recover() }()
		_ = WithMutationLock(func() error {
			panic("boom")
		})
	}()

	// Lock should be free now — second acquire returns quickly.
	done := make(chan struct{})
	go func() {
		release, err := AcquireMutationLock()
		if err == nil {
			release()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lock not released after panicking holder")
	}
}
