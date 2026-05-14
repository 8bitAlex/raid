package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// IDFileEnv lets tests redirect the ID file off the user's real
// $HOME without touching the global filesystem. Empty in normal use.
const IDFileEnv = "RAID_TELEMETRY_ID_FILE"

// homeDirFn is the user-home resolver — overridable in tests so they
// don't pollute the real ~/.config/raid/. os.UserHomeDir returns the
// platform-correct path on macOS/Linux/Windows.
var homeDirFn = os.UserHomeDir

// idMu guards the in-process cache around the ID file so concurrent
// Capture calls don't race on the read/write. The file itself is
// effectively single-writer (the user, on this machine), so we don't
// need OS-level locking.
var (
	idMu     sync.Mutex
	idCached string
)

// IDPath returns the on-disk path where raid stores the anonymous
// machine ID, after $RAID_TELEMETRY_ID_FILE override and $HOME
// resolution. Exposed so `raid telemetry status` can show it.
func IDPath() string {
	if override := os.Getenv(IDFileEnv); override != "" {
		return override
	}
	home, err := homeDirFn()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".config", "raid", "telemetry-id")
}

// LoadIDIfExists is the public wrapper around loadIDIfExists for
// commands that need to display the ID without forcing creation
// (`raid telemetry status` and the preview path).
func LoadIDIfExists() string {
	return loadIDIfExists()
}

// loadIDIfExists reads the persisted ID without creating one. Returns
// empty string when the file is missing or unreadable — used by the
// preview command so `raid telemetry preview` doesn't write to disk.
func loadIDIfExists() string {
	idMu.Lock()
	defer idMu.Unlock()
	if idCached != "" {
		return idCached
	}
	path := IDPath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(string(data))
	if id == "" {
		return ""
	}
	idCached = id
	return id
}

// loadOrCreateID returns the persisted ID, generating + writing a
// fresh one if the file doesn't exist. Empty return means we couldn't
// resolve the path or persist the value — Capture treats that as "do
// nothing" rather than blocking the user's command.
//
// Cross-process safe: the create path uses O_CREATE|O_EXCL so two
// concurrent first-run raid invocations can't both win and write
// different IDs. The loser observes EEXIST and re-reads whatever the
// winner persisted, so every process ends up with the same
// distinct_id from the first event onward.
func loadOrCreateID() string {
	if id := loadIDIfExists(); id != "" {
		return id
	}
	idMu.Lock()
	defer idMu.Unlock()
	// Recheck under the lock — another in-process goroutine may have
	// raced ahead while we waited.
	if idCached != "" {
		return idCached
	}
	path := IDPath()
	if path == "" {
		return ""
	}
	id, err := newID()
	if err != nil {
		return ""
	}
	persisted, err := writeIDExclusive(path, id)
	if err != nil {
		return ""
	}
	idCached = persisted
	return persisted
}

// newID generates a fresh UUIDv4 from crypto/rand. We don't depend on
// any third-party UUID library to keep the deps minimal — the layout
// is the standard RFC 4122 variant-1 / version-4 form.
func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}

// writeIDExclusive persists the ID at path using O_CREATE|O_EXCL so
// concurrent callers can't clobber each other's value. If the file
// already exists (another process won the race), the existing
// contents are read and returned instead. Permissions are 0600
// because this is a stable identifier for the user's machine — not a
// secret, but no reason to leave it world-readable either.
func writeIDExclusive(path, id string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			// Another process won the race — read what they wrote.
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", readErr
			}
			existing := strings.TrimSpace(string(data))
			if existing == "" {
				return "", fmt.Errorf("telemetry: id file present but empty at %s", path)
			}
			return existing, nil
		}
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(id + "\n"); err != nil {
		return "", err
	}
	return id, nil
}

// PurgeID deletes the on-disk ID file. PostHog can't link future
// events to past ones after a purge — `raid telemetry purge` exposes
// this so users can break continuity without losing the opt-in.
//
// Returns nil when the file is already absent (purge is idempotent).
func PurgeID() error {
	idMu.Lock()
	defer idMu.Unlock()
	idCached = ""
	path := IDPath()
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// resetIDCacheForTest clears the in-process cache. Required when a
// test points at a different IDPath via $RAID_TELEMETRY_ID_FILE — the
// cache would otherwise pin the previous test's value.
func resetIDCacheForTest() {
	idMu.Lock()
	defer idMu.Unlock()
	idCached = ""
}
