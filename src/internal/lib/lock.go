package lib

import (
	"fmt"
	"os"
	"path/filepath"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/gofrs/flock"
)

const lockFileName = ".lock"

// LockPathOverride redirects the mutation lock file. Tests set this so they
// don't contend with a real raid invocation running on the developer's
// machine. Empty in production.
var LockPathOverride string

func lockPath() string {
	if LockPathOverride != "" {
		return LockPathOverride
	}
	return filepath.Join(sys.GetHomeDir(), ConfigDirName, lockFileName)
}

// AcquireMutationLock blocks until this process holds the raid mutation
// lock, then returns a release function. Callers MUST defer the release.
//
// This is a cross-process exclusive lock backed by flock(2) on POSIX and
// LockFileEx on Windows. Any raid invocation (CLI or MCP server) that calls
// AcquireMutationLock while another raid process holds it will wait until
// released. The kernel releases the lock automatically if the holder exits
// unexpectedly — no stale-lock recovery code needed.
//
// Read-only operations (raid context, raid_list_*, raid doctor, etc.)
// don't need this lock; stale reads during a mutation are recoverable. Only
// paths that write ~/.raid state, .env files in repos, or run task
// sequences should acquire it.
func AcquireMutationLock() (func(), error) {
	path := lockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create raid config dir: %w", err)
	}
	lk := flock.New(path)
	if err := lk.Lock(); err != nil {
		return nil, fmt.Errorf("acquire raid mutation lock at %s: %w", path, err)
	}
	// Stamp the PID into the lock file so a curious user can `cat
	// ~/.raid/.lock` to identify the holder. This write is advisory: flock
	// is kernel-level and doesn't depend on file contents.
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
	return func() { _ = lk.Unlock() }, nil
}

// WithMutationLock acquires the mutation lock, runs fn, and releases the
// lock. Use this when an entire user-visible operation should be atomic
// across processes — for example, the env-switch sequence (set + force
// load + execute) where any gap between steps could let another raid
// invocation interleave.
func WithMutationLock(fn func() error) error {
	release, err := AcquireMutationLock()
	if err != nil {
		return err
	}
	defer release()
	return fn()
}
