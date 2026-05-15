// Package errs is the concrete implementation of raid's structured-error
// surface. The public re-export at src/raid/errs aliases every symbol from
// here, so callers in src/cmd/ and other public packages depend on the
// alias and never reach in directly. Internal raid code (src/internal/lib/…)
// imports this package straight.
//
// Future error implementations slot in alongside RaidError and satisfy
// the same Error interface — call sites at every layer stay unchanged.
package errs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Error is the structured-error interface every raid failure satisfies.
//
// Error() returns the human-readable message and is preserved across
// versions for existing wording, so tests that match on substrings keep
// working. Code, Category, Hint, and Details are the additive
// machine-parseable surface.
type Error interface {
	error
	Code() string
	Category() Category
	Hint() string
	Details() map[string]any
}

// Category buckets errors by exit-code class. Values match the documented
// CLI exit codes; do not renumber.
type Category int

const (
	// CategoryGeneric — exit 1. Unexpected / unclassified failures.
	CategoryGeneric Category = 1
	// CategoryConfig — exit 2. Invalid profile / repo / schema input.
	CategoryConfig Category = 2
	// CategoryTask — exit 3. A user task failed during execution.
	CategoryTask Category = 3
	// CategoryNetwork — exit 4. Clone / HTTP / network-bound failures.
	CategoryNetwork Category = 4
	// CategoryNotFound — exit 5. A referenced profile, repo, env, or
	// command does not exist.
	CategoryNotFound Category = 5
)

// String returns the lowercased category name for JSON / display.
func (c Category) String() string {
	switch c {
	case CategoryGeneric:
		return "generic"
	case CategoryConfig:
		return "config"
	case CategoryTask:
		return "task"
	case CategoryNetwork:
		return "network"
	case CategoryNotFound:
		return "not-found"
	default:
		return "generic"
	}
}

// Stable code strings. Documented at /docs/references/errors. Add new
// codes additively; never rename or repurpose an existing one.
const (
	CodeUnknown                = "UNKNOWN"
	CodeInternal               = "INTERNAL"
	CodeGitNotInstalled        = "GIT_NOT_INSTALLED"
	CodeLockFailed             = "LOCK_FAILED"
	CodeProfileInvalid         = "PROFILE_INVALID"
	CodeProfileFileRead        = "PROFILE_FILE_READ"
	CodeProfileAlreadyExists   = "PROFILE_ALREADY_EXISTS"
	CodeRepoInvalid            = "REPO_INVALID"
	CodeConfigInvalid          = "CONFIG_INVALID"
	CodeConfigLoadFailed       = "CONFIG_LOAD_FAILED"
	CodeSchemaValidationFailed = "SCHEMA_VALIDATION_FAILED"
	CodeArgInvalid             = "ARG_INVALID"
	CodeTaskFailed             = "TASK_FAILED"
	CodeTaskShellFailed        = "TASK_SHELL_FAILED"
	CodeTaskScriptFailed       = "TASK_SCRIPT_FAILED"
	CodeTaskWaitTimeout        = "TASK_WAIT_TIMEOUT"
	CodeTaskTemplateFailed     = "TASK_TEMPLATE_FAILED"
	CodeTaskGitFailed          = "TASK_GIT_FAILED"
	CodeCloneFailed            = "CLONE_FAILED"
	CodeTaskHTTPFailed         = "TASK_HTTP_FAILED"
	CodeProfileNotFound        = "PROFILE_NOT_FOUND"
	CodeProfileNotActive       = "PROFILE_NOT_ACTIVE"
	CodeProfileFileMissing     = "PROFILE_FILE_MISSING"
	CodeRepoNotFound           = "REPO_NOT_FOUND"
	CodeRepoNotCloned          = "REPO_NOT_CLONED"
	CodeEnvNotFound            = "ENV_NOT_FOUND"
	CodeCommandNotFound        = "COMMAND_NOT_FOUND"
	CodeVerifyFailed           = "VERIFY_FAILED"
	CodeHeadlessPromptNoDefault = "HEADLESS_PROMPT_NO_DEFAULT"
)

// RaidError is the canonical implementation of raid's Error interface.
// Fields are unexported so the struct can evolve without becoming a
// public API surface — accessors are the stable contract.
type RaidError struct {
	code     string
	category Category
	message  string
	hint     string
	details  map[string]any
	cause    error
}

// Compile-time check: *RaidError implements Error.
var _ Error = (*RaidError)(nil)

// Error returns just the message — wrapped causes are not appended so
// existing substring-matching tests keep passing. Use Unwrap or the
// JSON output to recover the cause.
func (e *RaidError) Error() string { return e.message }

// Code returns the stable error code string.
func (e *RaidError) Code() string { return e.code }

// Category returns the exit-code class.
func (e *RaidError) Category() Category { return e.category }

// Hint returns an optional human-readable suggestion.
func (e *RaidError) Hint() string { return e.hint }

// Details returns code-specific structured fields or nil. Treat as
// read-only; mutation may corrupt cached error values.
func (e *RaidError) Details() map[string]any { return e.details }

// Unwrap supports errors.Is / errors.As traversal.
func (e *RaidError) Unwrap() error { return e.cause }

// new is the single construction point so all errors are tagged
// uniformly — future cross-cutting concerns (telemetry, origin tag)
// land here without sweeping every constructor.
func newRaidError(code string, category Category, message, hint string, details map[string]any, cause error) *RaidError {
	return &RaidError{
		code:     code,
		category: category,
		message:  message,
		hint:     hint,
		details:  details,
		cause:    cause,
	}
}

// Newf is a generic escape hatch for call sites that need a specific
// (preserve-back-compat) error message but want to participate in the
// structured error system. Prefer the dedicated constructors below when
// the situation matches an existing code.
//
// If the final argument is an error, it is captured as the wrapped cause
// (so errors.Is / errors.As keep working) regardless of the format
// verbs. Callers that pass an error purely as a formatting argument and
// do NOT want it captured should convert it (e.g. err.Error()) first.
func Newf(code string, category Category, format string, args ...any) *RaidError {
	var cause error
	if n := len(args); n > 0 {
		if e, ok := args[n-1].(error); ok {
			cause = e
		}
	}
	return newRaidError(code, category, formatMsg(format, args...), "", nil, cause)
}

// joinErrors wraps a slice of errors so errors.Is / errors.As can walk
// each one. nil-safe; returns nil for empty input.
func joinErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return errors.Join(errs...)
	}
}

// AsError walks the wrapped-error chain and returns the first Error.
func AsError(err error) (Error, bool) {
	if err == nil {
		return nil, false
	}
	var rErr Error
	if errors.As(err, &rErr) {
		return rErr, true
	}
	return nil, false
}

// ExitCode returns the raid CLI exit code for an error. Nil → 0. A
// structured error returns its category. Anything else is generic
// failure (1).
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if rErr, ok := AsError(err); ok {
		return int(rErr.Category())
	}
	return int(CategoryGeneric)
}

// reservedErrorKeys are JSON keys produced by the structured error
// envelope itself. Details() entries with these names are skipped so a
// future error code adding `code: "..."` (or similar) as a detail
// can't accidentally shadow the canonical envelope field. Shared with
// emitters in other packages (e.g. cmd/context/serve.go's MCP path)
// so the CLI and MCP surfaces stay in lockstep — a new reserved key
// added here propagates everywhere.
var reservedErrorKeys = map[string]bool{
	"code":     true,
	"message":  true,
	"category": true,
	"hint":     true,
}

// IsReservedErrorKey reports whether k is reserved by the structured
// error envelope. Exported so peer emitters (MCP tool-result builder,
// future log adapters) can apply the same exclusion when flattening
// Details() into their own JSON shape.
func IsReservedErrorKey(k string) bool {
	return reservedErrorKeys[k]
}

// EmitJSON writes a structured `{"error": {…}}` document to w. Errors
// that don't implement Error are auto-wrapped as code UNKNOWN with
// category Generic, so the shape is always stable. Details() entries
// flatten into the top-level error object alongside code/message/hint.
func EmitJSON(w io.Writer, err error) {
	if err == nil {
		return
	}
	rErr, ok := AsError(err)
	if !ok {
		rErr = Unknown(err)
	}
	payload := map[string]any{
		"code":     rErr.Code(),
		"category": rErr.Category().String(),
		"message":  rErr.Error(),
	}
	if hint := rErr.Hint(); hint != "" {
		payload["hint"] = hint
	}
	for k, v := range rErr.Details() {
		if IsReservedErrorKey(k) {
			continue
		}
		payload[k] = v
	}
	enc := json.NewEncoder(w)
	_ = enc.Encode(map[string]any{"error": payload})
}

// Wrap is a convenience: if err already implements Error, return it
// unchanged; otherwise wrap as Unknown so callers always get a typed
// error without losing the original cause. Returns nil for nil.
func Wrap(err error) Error {
	if err == nil {
		return nil
	}
	if rErr, ok := AsError(err); ok {
		return rErr
	}
	return Unknown(err)
}

// formatMsg is a small helper for constructors that want fmt.Sprintf-style
// templating without leaking %w semantics into the stored message (the
// cause is separately tracked via Unwrap).
func formatMsg(tmpl string, args ...any) string {
	return fmt.Sprintf(tmpl, args...)
}
