// Package errs is the public re-export surface for raid's structured
// errors. The concrete types and constructors live in
// src/internal/lib/errs; this package aliases them so callers in
// src/cmd/ and other public packages depend only on this stable surface.
//
// Every code is part of raid's CLI contract: codes never change name or
// category across minor versions; new codes ship additively. See
// /docs/references/errors for the full table and the JSON shape.
package errs

import (
	"io"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
)

// Error is the structured-error interface every raid failure satisfies.
type Error = liberrs.Error

// Category buckets errors by exit-code class.
type Category = liberrs.Category

// Category constants — values match the documented CLI exit codes.
const (
	CategoryGeneric  = liberrs.CategoryGeneric
	CategoryConfig   = liberrs.CategoryConfig
	CategoryTask     = liberrs.CategoryTask
	CategoryNetwork  = liberrs.CategoryNetwork
	CategoryNotFound = liberrs.CategoryNotFound
)

// Stable code strings. Documented at /docs/references/errors.
const (
	CodeUnknown                = liberrs.CodeUnknown
	CodeInternal               = liberrs.CodeInternal
	CodeGitNotInstalled        = liberrs.CodeGitNotInstalled
	CodeLockFailed             = liberrs.CodeLockFailed
	CodeProfileInvalid         = liberrs.CodeProfileInvalid
	CodeProfileFileRead        = liberrs.CodeProfileFileRead
	CodeProfileAlreadyExists   = liberrs.CodeProfileAlreadyExists
	CodeRepoInvalid            = liberrs.CodeRepoInvalid
	CodeConfigInvalid          = liberrs.CodeConfigInvalid
	CodeConfigLoadFailed       = liberrs.CodeConfigLoadFailed
	CodeSchemaValidationFailed = liberrs.CodeSchemaValidationFailed
	CodeArgInvalid             = liberrs.CodeArgInvalid
	CodeTaskFailed             = liberrs.CodeTaskFailed
	CodeTaskShellFailed        = liberrs.CodeTaskShellFailed
	CodeTaskScriptFailed       = liberrs.CodeTaskScriptFailed
	CodeTaskWaitTimeout        = liberrs.CodeTaskWaitTimeout
	CodeTaskTemplateFailed     = liberrs.CodeTaskTemplateFailed
	CodeTaskGitFailed          = liberrs.CodeTaskGitFailed
	CodeCloneFailed            = liberrs.CodeCloneFailed
	CodeTaskHTTPFailed         = liberrs.CodeTaskHTTPFailed
	CodeProfileNotFound        = liberrs.CodeProfileNotFound
	CodeProfileNotActive       = liberrs.CodeProfileNotActive
	CodeProfileFileMissing     = liberrs.CodeProfileFileMissing
	CodeRepoNotFound           = liberrs.CodeRepoNotFound
	CodeRepoNotCloned          = liberrs.CodeRepoNotCloned
	CodeEnvNotFound            = liberrs.CodeEnvNotFound
	CodeCommandNotFound        = liberrs.CodeCommandNotFound
	CodeVerifyFailed           = liberrs.CodeVerifyFailed
	CodeHeadlessPromptNoDefault = liberrs.CodeHeadlessPromptNoDefault
)

// AsError walks the wrapped-error chain and returns the first Error.
func AsError(err error) (Error, bool) { return liberrs.AsError(err) }

// IsReservedErrorKey reports whether k is reserved by the structured
// error envelope (code/message/category/hint). Peer emitters that
// flatten Details() into their own JSON shape (e.g. the MCP tool-
// result builder) should consult this so envelope keys don't get
// overwritten by accidental Details collisions.
func IsReservedErrorKey(k string) bool { return liberrs.IsReservedErrorKey(k) }

// ExitCode returns the raid CLI exit code for an error. Nil → 0.
func ExitCode(err error) int { return liberrs.ExitCode(err) }

// EmitJSON writes a structured `{"error": {…}}` document to w.
func EmitJSON(w io.Writer, err error) { liberrs.EmitJSON(w, err) }

// Wrap returns err unchanged if it already implements Error, else wraps
// it as Unknown. Returns nil for nil.
func Wrap(err error) Error { return liberrs.Wrap(err) }

// Newf builds a structured error with an arbitrary formatted message.
// Prefer the dedicated constructors below — this exists for call sites
// that need to preserve a specific historic wording for back-compat.
func Newf(code string, category Category, format string, args ...any) Error {
	return liberrs.Newf(code, category, format, args...)
}

// Constructors — thin shims over the internal package.

func Unknown(cause error) Error                          { return liberrs.Unknown(cause) }
func Internal(msg string) Error                          { return liberrs.Internal(msg) }
func GitNotInstalled() Error                             { return liberrs.GitNotInstalled() }
func LockFailed(cause error) Error                       { return liberrs.LockFailed(cause) }
func ProfileNotFound(name string) Error                  { return liberrs.ProfileNotFound(name) }
func ProfileNotActive() Error                            { return liberrs.ProfileNotActive() }
func ProfileFileMissing(path string) Error               { return liberrs.ProfileFileMissing(path) }
func ProfileFileRead(path string, cause error) Error     { return liberrs.ProfileFileRead(path, cause) }
func ProfileInvalid(path string, cause error) Error      { return liberrs.ProfileInvalid(path, cause) }
func ProfileAlreadyExists(name string) Error             { return liberrs.ProfileAlreadyExists(name) }
func RepoNotFound(name string) Error                     { return liberrs.RepoNotFound(name) }
func RepoNotCloned(name, path string) Error              { return liberrs.RepoNotCloned(name, path) }
func RepoInvalid(name string, cause error) Error         { return liberrs.RepoInvalid(name, cause) }
func CloneFailed(name, url string, cause error) Error    { return liberrs.CloneFailed(name, url, cause) }
func EnvNotFound(name string) Error                      { return liberrs.EnvNotFound(name) }
func CommandNotFound(name string) Error                  { return liberrs.CommandNotFound(name) }
func ArgInvalid(msg string) Error                        { return liberrs.ArgInvalid(msg) }
func ConfigInvalid(cause error) Error                    { return liberrs.ConfigInvalid(cause) }
func ConfigLoadFailed(cause error) Error                 { return liberrs.ConfigLoadFailed(cause) }
func SchemaValidationFailed(path string, cause error) Error {
	return liberrs.SchemaValidationFailed(path, cause)
}
func TaskFailed(taskType string, cause error) Error    { return liberrs.TaskFailed(taskType, cause) }
func TaskShellFailed(cause error) Error                { return liberrs.TaskShellFailed(cause) }
func TaskScriptFailed(cause error) Error               { return liberrs.TaskScriptFailed(cause) }
func TaskWaitTimeout(target string, cause error) Error { return liberrs.TaskWaitTimeout(target, cause) }
func TaskTemplateFailed(cause error) Error             { return liberrs.TaskTemplateFailed(cause) }
func TaskGitFailed(cause error) Error                  { return liberrs.TaskGitFailed(cause) }
func TaskHTTPFailed(url string, cause error) Error     { return liberrs.TaskHTTPFailed(url, cause) }
func VerifyFailed(name string, cause error) Error      { return liberrs.VerifyFailed(name, cause) }
func HeadlessPromptNoDefault(varName string) Error     { return liberrs.HeadlessPromptNoDefault(varName) }
