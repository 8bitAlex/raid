// The lib package is the implementation of the core functionality of the raid CLI tool.
package lib

import (
	"bytes"
	stdctx "context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/8bitalex/raid/schemas"
	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

const (
	yamlSep            = "---"
	RaidConfigFileName = "raid.yaml"
)

// Context holds the active profile and environment for the current raid session.
type Context struct {
	Profile Profile
	Env     string
}

// OnInstall holds the tasks to run during profile installation.
type OnInstall struct {
	Tasks []Task `json:"tasks"`
}

// contextPtr is the active workspace context. Replaced wholly via
// storeContext on Load/ForceLoad; never mutated in-place. Reads go
// through loadContext.
//
// Atomic.Pointer is the right primitive here: writes are rare
// (Load / ForceLoad / Reset only), reads are frequent (every
// command/task dispatch + every MCP read), and the value is always
// wholly replaced rather than partially mutated. mcp-go runs tool
// handlers from a 5-worker pool, so a mutating handler that calls
// ForceLoad can race with a read handler that reads context.Profile —
// the atomic pointer makes the swap+read interleave well-defined.
//
// Callers must use loadContext() once per logical operation and bind
// to a local. "Check nil then read field" patterns must NOT re-call
// loadContext between the check and the read — otherwise a swap
// could land in between.
var contextPtr atomic.Pointer[Context]

// loadContext returns the current active context, or nil when no
// profile has been loaded yet. Safe for concurrent reads; consumers
// should bind the result to a local before nil-checking + reading.
func loadContext() *Context {
	return contextPtr.Load()
}

// storeContext atomically replaces the active context. Passing nil
// resets the context to "not loaded" (Load / Reset paths). Callers
// outside lib should not need this — use Load / ForceLoad instead.
func storeContext(c *Context) {
	contextPtr.Store(c)
}

const raidVarsFileName = "vars"

var (
	raidVarsMu           sync.RWMutex
	raidVars             = map[string]string{}
	raidVarsOverridePath string // set in tests to redirect the vars file
)

// commandSession holds environment variables exported by Shell tasks for the
// duration of a single command execution. It is nil when no command is active.
type commandSessionStore struct {
	mu       sync.RWMutex
	vars     map[string]string
	baseline map[string]string // env+raidVars snapshot taken at session start
	// lastEnvHash is the FNV-1a hash of the last env dump
	// updateSessionFromEnv processed. Lets the next call skip the
	// parse + diff entirely when consecutive Shell tasks don't touch
	// the env (the common case for tasks that just run a binary).
	// Zero means "no prior dump seen."
	lastEnvHash uint64
}

var commandSession *commandSessionStore

// startSession initialises a fresh session store, snapshotting the current
// environment and raidVars so that Shell-task exports can be diffed later.
func startSession() {
	baseline := make(map[string]string)
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		baseline[k] = v
	}
	raidVarsMu.RLock()
	for k, v := range raidVars {
		baseline[k] = v
	}
	raidVarsMu.RUnlock()

	commandSession = &commandSessionStore{
		vars:     make(map[string]string),
		baseline: baseline,
	}
}

// endSession clears the active session store.
func endSession() {
	commandSession = nil
}

func raidVarsPath() string {
	if raidVarsOverridePath != "" {
		return raidVarsOverridePath
	}
	return filepath.Join(sys.GetHomeDir(), ConfigDirName, raidVarsFileName)
}

func loadRaidVars() {
	path := raidVarsPath()
	if !sys.FileExists(path) {
		return
	}
	// Tighten perms on existing vars files written by earlier raid
	// versions (godotenv defaults to 0644). The file may carry
	// scrubbed-but-still-private RAID_REPO_*_URL entries and any
	// Set-task values that the project author treats as secret-ish,
	// so it should be 0600.
	//
	// Use Lstat so a symlinked path doesn't get its target chmodded,
	// and only touch regular files — a directory or device at this
	// path is a misconfiguration we shouldn't compound by stripping
	// its mode bits. Best-effort: chmod failures (read-only
	// filesystems, foreign-owned files) don't block the load.
	if fi, err := os.Lstat(path); err == nil && fi.Mode().IsRegular() {
		_ = os.Chmod(path, 0600)
	}
	m, err := godotenv.Read(path)
	if err != nil {
		if !suppressLoadWarnings.Load() {
			fmt.Fprintf(os.Stderr, "raid: failed to load persisted vars from %s: %v\n", path, err)
		}
		return
	}
	raidVarsMu.Lock()
	defer raidVarsMu.Unlock()
	for k, v := range m {
		raidVars[strings.ToUpper(k)] = v
	}
}

// snapshotRaidVars returns an independent copy of the raidVars map so callers
// can serialize or hand it to JSON without holding the mutex or sharing
// internal state. Returns nil when there are no vars so the JSON serializer
// honours `omitempty` instead of emitting an empty object.
func snapshotRaidVars() map[string]string {
	raidVarsMu.RLock()
	defer raidVarsMu.RUnlock()
	if len(raidVars) == 0 {
		return nil
	}
	out := make(map[string]string, len(raidVars))
	for k, v := range raidVars {
		out[k] = v
	}
	return out
}

// varsWatchDebounce is the window in which successive fsnotify events on the
// vars file are coalesced into a single onChange call. Atomic writes (temp
// file + rename, the pattern used by execSetVar and most editors) fire
// CREATE+RENAME+WRITE in quick succession; reloading per event would thrash.
var varsWatchDebounce = 50 * time.Millisecond

// newVarsWatcherFn is the watcher factory. Tests swap it for a fake that
// drives onChange synchronously instead of going through fsnotify.
var newVarsWatcherFn = newVarsWatcher

// WatchRaidVars watches the raid vars file (~/.raid/vars) for the lifetime
// of ctx and invokes onChange whenever the file is created, modified, or
// replaced. Events are debounced. The watcher is attached to the parent
// directory so atomic-rename writes (which swap the inode) keep firing —
// a watch on the file itself would silently go deaf after the first rename.
//
// onChange is the caller's reload hook; lib does not assume what to reload,
// so the MCP server passes a closure that runs ForceLoad under the
// cross-process mutation lock.
func WatchRaidVars(ctx stdctx.Context, onChange func()) error {
	if onChange == nil {
		return liberrs.Newf(liberrs.CodeArgInvalid, liberrs.CategoryConfig, "WatchRaidVars: onChange must not be nil")
	}
	return newVarsWatcherFn(ctx, raidVarsPath(), onChange)
}

func newVarsWatcher(ctx stdctx.Context, varsPath string, onChange func()) error {
	dir := filepath.Dir(varsPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return liberrs.Newf(liberrs.CodeInternal, liberrs.CategoryGeneric, "ensure vars watch dir %s: %v", dir, err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return liberrs.Newf(liberrs.CodeInternal, liberrs.CategoryGeneric, "create fsnotify watcher: %v", err)
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		return liberrs.Newf(liberrs.CodeInternal, liberrs.CategoryGeneric, "watch %s: %v", dir, err)
	}

	go runVarsWatcher(ctx, w, varsPath, onChange)
	return nil
}

func runVarsWatcher(ctx stdctx.Context, w *fsnotify.Watcher, varsPath string, onChange func()) {
	defer w.Close()
	target := filepath.Base(varsPath)

	// Use a timer.C channel rather than time.AfterFunc so onChange runs
	// from this goroutine — gated by the same select that watches
	// ctx.Done. With AfterFunc the callback could still fire after a
	// successful Stop+cancel race because the timer goroutine had already
	// scheduled it.
	timer := time.NewTimer(varsWatchDebounce)
	stopTimer(timer)
	armed := false
	arm := func() {
		if armed {
			stopTimer(timer)
		}
		timer.Reset(varsWatchDebounce)
		armed = true
	}

	for {
		var fire <-chan time.Time
		if armed {
			fire = timer.C
		}
		select {
		case <-ctx.Done():
			if armed {
				stopTimer(timer)
			}
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if filepath.Base(ev.Name) != target {
				continue
			}
			arm()
		case <-fire:
			armed = false
			// Belt-and-braces: if ctx was cancelled in the same tick the
			// timer fired, skip the reload. The for-loop will then exit
			// on the next ctx.Done iteration.
			if ctx.Err() != nil {
				return
			}
			onChange()
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "raid: vars watcher error: %v\n", err)
		}
	}
}

// stopTimer stops t and drains a pending tick if Stop reports the timer
// had already fired. Safe to call on a freshly-created timer that has not
// yet fired or been read.
func stopTimer(t *time.Timer) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
}

// expandRaid expands $VAR and ${VAR} references. Lookup order:
//  1. raidVars (Set tasks) — highest priority
//  2. commandSession vars (exports from Shell tasks in the current command)
//  3. OS environment — lowest priority
func expandRaid(s string) string {
	return os.Expand(s, func(key string) string {
		raidVarsMu.RLock()
		v, ok := raidVars[strings.ToUpper(key)]
		raidVarsMu.RUnlock()
		if ok {
			return v
		}
		if commandSession != nil {
			commandSession.mu.RLock()
			v, ok = commandSession.vars[key]
			commandSession.mu.RUnlock()
			if ok {
				return v
			}
		}
		return os.Getenv(key)
	})
}

// expandRaidForShell is like expandRaid but leaves variables that cannot be
// resolved as literal "$key" tokens so the shell subprocess can expand them
// itself. This prevents shell-local variable references (e.g. ${WORD} set
// earlier in the same script) from being silently replaced with empty strings.
func expandRaidForShell(s string) string {
	return os.Expand(s, func(key string) string {
		raidVarsMu.RLock()
		v, ok := raidVars[strings.ToUpper(key)]
		raidVarsMu.RUnlock()
		if ok {
			return v
		}
		if commandSession != nil {
			commandSession.mu.RLock()
			v, ok = commandSession.vars[key]
			commandSession.mu.RUnlock()
			if ok {
				return v
			}
		}
		if v, ok := os.LookupEnv(key); ok {
			return v
		}
		// Unknown — pass through using ${key} so that parameter expansions
		// like ${FOO:-default} or ${BAR:+val} are reconstructed exactly as
		// written rather than becoming the invalid $FOO:-default form.
		// For a simple identifier key this is equivalent to $key in the shell.
		return "${" + key + "}"
	})
}

// QuietLoad attempts a best-effort, read-only profile load. It does not create
// config files, does not emit warnings, and returns nil if the config is absent
// or loading fails. Intended for info-command paths (--help, --version) where
// user-command registration is opportunistic and side effects are undesirable.
func QuietLoad() []Command {
	// Suppress the load-time stderr warnings (corrupt vars file,
	// repo-name sanitization collisions) for the duration of the load —
	// `raid --help` shouldn't print diagnostics the real invocation
	// will print again anyway.
	suppressLoadWarnings.Store(true)
	defer suppressLoadWarnings.Store(false)
	if !initConfigReadOnly() {
		return nil
	}
	if err := ForceLoad(); err != nil {
		return nil
	}
	return GetCommands()
}

// suppressLoadWarnings gates the advisory stderr warnings emitted during
// profile loading. Set only by QuietLoad, whose contract is "no warnings".
var suppressLoadWarnings atomic.Bool

// ResetContext clears the cached load context, forcing the next Load or ForceLoad to
// rebuild from the current viper configuration.
func ResetContext() {
	storeContext(nil)
}

// Load initializes the context from the active profile, using cached results if available.
func Load() error {
	if loadContext() == nil {
		return ForceLoad()
	}
	return nil
}

// ForceLoad rebuilds the context from the active profile, ignoring any cached state.
func ForceLoad() error {
	raidVarsMu.Lock()
	raidVars = map[string]string{}
	raidVarsMu.Unlock()
	loadRaidVars()
	p := GetProfile()
	if p.IsZero() {
		storeContext(&Context{Env: GetEnv()})
		return nil
	}

	profile, err := buildProfile(p)
	if err != nil {
		return err
	}

	homeDir := sys.GetHomeDir()
	for i := range profile.Commands {
		profile.Commands[i].Tasks = withDefaultDir(profile.Commands[i].Tasks, homeDir)
	}
	for name, tasks := range profile.Groups {
		profile.Groups[name] = withDefaultDir(tasks, homeDir)
	}

	for i := range profile.Repositories {
		if err := buildRepo(&profile.Repositories[i]); err != nil {
			return err
		}
		repo := &profile.Repositories[i]
		repoDir := sys.ExpandPath(repo.Path)
		for j := range repo.Commands {
			repo.Commands[j].Tasks = withDefaultDir(repo.Commands[j].Tasks, repoDir)
		}
		profile.Commands = mergeCommands(profile.Commands, repo.Commands)
	}

	// In single-repo mode the raid.yaml is the only source of configuration,
	// so its environments need to surface at profile level for `raid env`
	// and friends — there's no wrapping profile YAML to host them.
	// Merged by name so a duplicate doesn't create two entries with the
	// same Name (getEnv would return the first, ExecuteEnv would write
	// variables from whichever, etc.). The wrapper profile wins on
	// conflict, mirroring the mergeCommands contract used for commands.
	if profile.IsSingleRepo() && len(profile.Repositories) == 1 {
		profile.Environments = mergeEnvironments(profile.Environments, profile.Repositories[0].Environments)
	}

	setRepoVars(profile.Repositories)

	storeContext(&Context{
		Profile: profile,
		Env:     GetEnv(),
	})
	return nil
}

// setRepoVars seeds raidVars with RAID_REPO_<NAME>_{URL,PATH,BRANCH} entries
// for every repo in the active profile, so tasks and subprocesses can
// reference them as $RAID_REPO_API_URL, etc. PATH is the expanded absolute
// path. Repos with empty fields contribute empty values; URL/BRANCH entries
// are still defined so unset references don't fall through to the OS env.
// Profile values overwrite anything from the persisted vars file — the
// profile is canonical, so any stale RAID_REPO_* keys (from a removed or
// renamed repo persisted to ~/.raid/vars) are pruned first. Sanitized-name
// collisions between repos (e.g. "my-api" and "my_api" both → MY_API) are
// reported to stderr; the last repo wins so behavior is deterministic.
//
// URLs are scrubbed of userinfo before storage so an HTTPS clone URL
// embedding credentials (`https://user:token@host/...`) doesn't end up
// persisted to ~/.raid/vars or served verbatim through the MCP vars
// resource. See ScrubURL for the contract.
func setRepoVars(repos []Repo) {
	raidVarsMu.Lock()
	defer raidVarsMu.Unlock()
	for k := range raidVars {
		if strings.HasPrefix(k, "RAID_REPO_") {
			delete(raidVars, k)
		}
	}
	seen := make(map[string]string, len(repos))
	for _, repo := range repos {
		key := sanitizeRepoVarName(repo.Name)
		if key == "" {
			continue
		}
		if prev, ok := seen[key]; ok && prev != repo.Name {
			if !suppressLoadWarnings.Load() {
				fmt.Fprintf(os.Stderr,
					"raid: warning: repos %q and %q both map to RAID_REPO_%s_*; %q wins\n",
					prev, repo.Name, key, repo.Name)
			}
		}
		seen[key] = repo.Name
		raidVars["RAID_REPO_"+key+"_URL"] = ScrubURL(repo.URL)
		raidVars["RAID_REPO_"+key+"_PATH"] = sys.ExpandPath(repo.Path)
		raidVars["RAID_REPO_"+key+"_BRANCH"] = repo.Branch
	}
}

// ScrubURL strips userinfo (user:password@) from a credential-bearing
// URL so secrets embedded in a clone URL never get persisted or
// surfaced to MCP clients. Returns the input unchanged when:
//
//   - it's empty
//   - it's an SSH-style URL (`git@host:repo.git` or `ssh://git@host/…`)
//     where the username is the protocol's required login, not a
//     credential — stripping it would change clone semantics
//   - it can't be parsed as a URL (treated as opaque)
//
// Only `http` and `https` schemes get scrubbed: those are the ones
// where userinfo carries tokens / basic-auth secrets. SSH userinfo
// (the `git` user) is preserved so a scrubbed URL is still a valid
// clone URL for downstream consumers.
func ScrubURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		u.User = nil
		return u.String()
	default:
		return raw
	}
}

// sanitizeRepoVarName converts a repo name into the uppercase identifier
// fragment used in RAID_REPO_<NAME>_* var names. Non-alphanumerics become
// underscores so names like "my-api" or "frontend.web" produce valid env
// var keys. Returns "" if the name has no usable characters.
func sanitizeRepoVarName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - ('a' - 'A'))
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if strings.Trim(out, "_") == "" {
		return ""
	}
	return out
}

// installRepo clones a single repository and runs its install tasks.
func installRepo(repo Repo) error {
	if err := CloneRepository(repo); err != nil {
		// CloneRepository already returns structured errors with codes,
		// categories, and hints (CLONE_FAILED, GIT_NOT_INSTALLED,
		// REPO_NOT_CLONED). Re-wrapping would misclassify non-network
		// failures and drop the original hint/details.
		return err
	}
	if err := ExecuteTasks(withDefaultDir(repo.Install.Tasks, sys.ExpandPath(repo.Path))); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to execute install tasks for '%s': %v", repo.Name, err)
	}
	return nil
}

// InstallRepo clones a single named repository and runs its install tasks.
// The profile-level install tasks are not run.
func InstallRepo(name string) error {
	ctx := loadContext()
	if ctx == nil {
		return liberrs.Internal("raid context is not initialized")
	}
	profile := ctx.Profile
	if profile.IsZero() {
		return liberrs.Newf(liberrs.CodeProfileNotActive, liberrs.CategoryNotFound, "profile not found")
	}

	var repo *Repo
	for i := range profile.Repositories {
		if profile.Repositories[i].Name == name {
			repo = &profile.Repositories[i]
			break
		}
	}
	if repo == nil {
		return liberrs.Newf(liberrs.CodeRepoNotFound, liberrs.CategoryNotFound, "repository '%s' not found in active profile", name)
	}

	return installRepo(*repo)
}

// Install clones all repositories in the active profile and runs install tasks.
func Install(maxThreads int) error {
	ctx := loadContext()
	if ctx == nil {
		return liberrs.Internal("raid context is not initialized")
	}
	profile := ctx.Profile
	if profile.IsZero() {
		return liberrs.Newf(liberrs.CodeProfileNotActive, liberrs.CategoryNotFound, "profile not found")
	}

	var semaphore chan struct{}
	if maxThreads > 0 {
		semaphore = make(chan struct{}, maxThreads)
	}

	// Phase 1: clone all repos concurrently, throttled by semaphore.
	var wg sync.WaitGroup
	cloneErrs := make(chan error, len(profile.Repositories))

	for _, repo := range profile.Repositories {
		wg.Add(1)
		go func(repo Repo) {
			defer wg.Done()
			if semaphore != nil {
				semaphore <- struct{}{}
				// Release via defer so a CloneRepository panic
				// (or any unanticipated runtime crash) can't strand
				// the slot, deadlocking subsequent acquires and the
				// parent wg.Wait().
				defer func() { <-semaphore }()
			}
			// Recover any panic from CloneRepository so a single
			// runaway goroutine doesn't crash the whole raid process
			// (or the MCP server). The recovered panic is reported
			// as a structured error so the aggregate below surfaces
			// it like any other clone failure.
			defer func() {
				if r := recover(); r != nil {
					cloneErrs <- liberrs.Internal(fmt.Sprintf("panic cloning %q: %v", repo.Name, r))
				}
			}()
			if err := CloneRepository(repo); err != nil {
				// Preserve the structured error from CloneRepository
				// (CLONE_FAILED, GIT_NOT_INSTALLED, REPO_NOT_CLONED, etc.)
				// so the aggregate below can expose each per-repo cause.
				cloneErrs <- err
			}
		}(repo)
	}

	wg.Wait()
	close(cloneErrs)

	var collected []error
	for err := range cloneErrs {
		collected = append(collected, err)
	}
	if len(collected) > 0 {
		// If only one repo failed, surface its structured error directly
		// so its code/category/hint/details survive untouched.
		if len(collected) == 1 {
			return collected[0]
		}
		return liberrs.CloneFailedMulti(collected)
	}

	// Phase 2: run profile-level install tasks before any repo tasks.
	if err := ExecuteTasks(withDefaultDir(profile.Install.Tasks, sys.GetHomeDir())); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to execute install tasks: %v", err)
	}

	// Phase 3: run each repo's install tasks sequentially in profile order.
	for _, repo := range profile.Repositories {
		if err := ExecuteTasks(withDefaultDir(repo.Install.Tasks, sys.ExpandPath(repo.Path))); err != nil {
			return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to execute install tasks for '%s': %v", repo.Name, err)
		}
	}

	return nil
}

// ValidateSchema validates the file at path against the JSON schema at schemaPath.
// schemaPath must be an absolute or CWD-relative path to a schema file on disk.
func ValidateSchema(path string, schemaPath string) error {
	path = sys.ExpandPath(path)
	schemaPath = sys.ExpandPath(schemaPath)

	if path == "" || !sys.FileExists(path) {
		return liberrs.Newf(liberrs.CodeProfileFileMissing, liberrs.CategoryNotFound, "file not found at %s", path)
	}
	if schemaPath == "" || !sys.FileExists(schemaPath) {
		return liberrs.Newf(liberrs.CodeProfileFileMissing, liberrs.CategoryNotFound, "file not found at %s", schemaPath)
	}

	c := jsonschema.NewCompiler()
	sch, err := c.Compile(schemaPath)
	if err != nil {
		return err
	}

	return validateFile(path, sch)
}

// validateWithEmbeddedSchema validates path against a schema embedded in the binary.
// schemaID must be the canonical $id URL of an embedded schema
// (e.g. "https://raidcli.dev/schema/v1/raid-profile.schema.json"). All embedded
// schemas are registered under their $id so cross-schema $ref values resolve
// correctly without any network access.
func validateWithEmbeddedSchema(path, schemaID string) error {
	path = sys.ExpandPath(path)
	if path == "" || !sys.FileExists(path) {
		return liberrs.Newf(liberrs.CodeProfileFileMissing, liberrs.CategoryNotFound, "file not found at %s", path)
	}

	c := jsonschema.NewCompiler()
	entries, err := schemas.FS.ReadDir(".")
	if err != nil {
		return liberrs.Newf(liberrs.CodeInternal, liberrs.CategoryGeneric, "failed to read embedded schemas: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".schema.json") {
			continue
		}
		data, err := schemas.FS.ReadFile(name)
		if err != nil {
			return liberrs.Newf(liberrs.CodeInternal, liberrs.CategoryGeneric, "failed to read embedded schema %s: %v", name, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(data, &doc); err != nil {
			return liberrs.Newf(liberrs.CodeInternal, liberrs.CategoryGeneric, "failed to parse embedded schema %s: %v", name, err)
		}
		id, _ := doc["$id"].(string)
		if id == "" {
			return liberrs.Newf(liberrs.CodeInternal, liberrs.CategoryGeneric, "embedded schema %s is missing $id", name)
		}
		if err := c.AddResource(id, doc); err != nil {
			return liberrs.Newf(liberrs.CodeInternal, liberrs.CategoryGeneric, "failed to register embedded schema %s: %v", name, err)
		}
	}

	sch, err := c.Compile(schemaID)
	if err != nil {
		return err
	}

	return validateFile(path, sch)
}

func validateFile(path string, sch *jsonschema.Schema) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".yaml" || ext == ".yml" {
		// Validate every document in a multi-doc YAML stream individually so
		// that profile files using --- separators are fully validated.
		dec := yaml.NewDecoder(f)
		count := 0
		for {
			var raw any
			if err := dec.Decode(&raw); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			// Skip explicitly-empty documents (a trailing `---`, or a
			// document holding only comments) instead of validating nil
			// against the schema — extractProfilesFromYAML skips them
			// too, and the two paths must agree or a file that extracts
			// cleanly would refuse to validate.
			if raw == nil {
				continue
			}
			count++
			jsonBytes, err := json.Marshal(raw)
			if err != nil {
				return err
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
			if err != nil {
				return err
			}
			if err := sch.Validate(doc); err != nil {
				return liberrs.Newf(liberrs.CodeSchemaValidationFailed, liberrs.CategoryConfig, "invalid format: %v", err)
			}
		}
		if count == 0 {
			return liberrs.Newf(liberrs.CodeSchemaValidationFailed, liberrs.CategoryConfig, "invalid format: file contains no YAML documents")
		}
		return nil
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	// Detect a top-level JSON array and validate each element individually,
	// mirroring how extractProfilesFromJSON supports both single-object and
	// array-of-objects JSON profile files. The array branch is taken only
	// when the document *syntactically* starts with '[' — `null` also
	// unmarshals into a nil slice, and treating it as a zero-element array
	// would skip schema validation entirely and report the file as valid.
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var arr []any
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		if len(arr) == 0 {
			return liberrs.Newf(liberrs.CodeSchemaValidationFailed, liberrs.CategoryConfig, "invalid format: file contains no JSON documents")
		}
		for _, elem := range arr {
			jsonBytes, err := json.Marshal(elem)
			if err != nil {
				return err
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonBytes))
			if err != nil {
				return err
			}
			if err := sch.Validate(doc); err != nil {
				return liberrs.Newf(liberrs.CodeSchemaValidationFailed, liberrs.CategoryConfig, "invalid format: %v", err)
			}
		}
		return nil
	}

	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if err := sch.Validate(doc); err != nil {
		return liberrs.Newf(liberrs.CodeSchemaValidationFailed, liberrs.CategoryConfig, "invalid format: %v", err)
	}
	return nil
}
