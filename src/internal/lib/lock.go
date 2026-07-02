package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/gofrs/flock"
)

const lockFileName = ".lock"

// MutationLockEnvVar marks subprocesses spawned while this raid process
// holds the mutation lock. A child raid invocation launched from a task
// running under the lock (e.g. a Shell task whose command calls
// `raid env dev`) must not try to re-acquire the flock: the parent
// already holds it and is blocked waiting on the child, so the child
// would wait forever — a permanent two-process deadlock. buildSubprocessEnv
// injects this var whenever the lock is held; AcquireMutationLock skips
// acquisition when it's inherited, treating the child as part of the
// parent's already-serialized operation.
const MutationLockEnvVar = "RAID_MUTATION_LOCK_HELD"

// mutationLockDepth counts how many times this process currently holds
// the mutation lock (nested WithMutationLock calls from concurrent MCP
// handlers each hold their own flock handle, so this is a counter, not
// a bool). Read by buildSubprocessEnv to decide whether spawned
// subprocesses should inherit MutationLockEnvVar.
var mutationLockDepth atomic.Int32

// mutationLockHeld reports whether this process holds the mutation lock.
func mutationLockHeld() bool {
	return mutationLockDepth.Load() > 0
}

// mutationLockInherited reports whether a parent raid process already
// holds the mutation lock on this invocation's behalf. Truthy values
// mirror IsHeadless so a stray export can't half-enable the skip.
func mutationLockInherited() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(MutationLockEnvVar))) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

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
	// A parent raid process already holds the lock and spawned us as part
	// of its task sequence — acquiring again would deadlock both
	// processes (see MutationLockEnvVar). Run under the parent's lock.
	if mutationLockInherited() {
		return func() {}, nil
	}
	path := lockPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		// Route through LockFailed so the user-visible hint about
		// ~/.raid/.lock is included consistently across every lock
		// acquisition failure.
		return nil, liberrs.LockFailed(fmt.Errorf("create raid config dir: %w", err))
	}
	lk := flock.New(path)
	if err := lk.Lock(); err != nil {
		return nil, liberrs.LockFailed(err)
	}
	// Stamp the PID into the lock file so a curious user can `cat
	// ~/.raid/.lock` to identify the holder. This write is advisory: flock
	// is kernel-level and doesn't depend on file contents.
	_ = os.WriteFile(path, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644)
	mutationLockDepth.Add(1)
	return func() {
		mutationLockDepth.Add(-1)
		_ = lk.Unlock()
	}, nil
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
