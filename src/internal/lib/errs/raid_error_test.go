package errs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestJoinErrors(t *testing.T) {
	// Empty slice returns nil.
	if joinErrors(nil) != nil {
		t.Error("joinErrors(nil) should be nil")
	}
	if joinErrors([]error{}) != nil {
		t.Error("joinErrors([]) should be nil")
	}
	// One element is returned verbatim so errors.Is can find it.
	sentinel := errors.New("only")
	if got := joinErrors([]error{sentinel}); got != sentinel {
		t.Errorf("joinErrors(1) = %v, want sentinel", got)
	}
	// Many elements use errors.Join — both causes are reachable.
	a := errors.New("a")
	b := errors.New("b")
	joined := joinErrors([]error{a, b})
	if !errors.Is(joined, a) || !errors.Is(joined, b) {
		t.Error("joinErrors should preserve both causes for errors.Is")
	}
}

func TestCloneFailedMulti(t *testing.T) {
	// Empty causes → message without the appended joined error, count 0.
	empty := CloneFailedMulti(nil)
	if empty.Code() != CodeCloneFailed {
		t.Errorf("Code = %q, want %q", empty.Code(), CodeCloneFailed)
	}
	if empty.Category() != CategoryNetwork {
		t.Errorf("Category = %v, want CategoryNetwork", empty.Category())
	}
	if empty.Details()["count"].(int) != 0 {
		t.Errorf("count = %v, want 0", empty.Details()["count"])
	}

	// One typed cause: failures[] carries code/category and repo/url
	// details propagated up.
	first := CloneFailed("api", "git@example.com:api.git", errors.New("auth"))
	plain := errors.New("network down")
	got := CloneFailedMulti([]error{first, plain})
	if got.Code() != CodeCloneFailed {
		t.Errorf("Code = %q, want CLONE_FAILED", got.Code())
	}
	failures, ok := got.Details()["failures"].([]map[string]any)
	if !ok {
		t.Fatalf("failures detail missing or wrong type: %v", got.Details())
	}
	if len(failures) != 2 {
		t.Fatalf("failures len = %d, want 2", len(failures))
	}
	if failures[0]["code"] != CodeCloneFailed {
		t.Errorf("first failure code = %v", failures[0]["code"])
	}
	if failures[0]["repo"] != "api" {
		t.Errorf("first failure should carry repo detail: %v", failures[0])
	}
	if _, has := failures[1]["code"]; has {
		t.Errorf("plain error shouldn't get a code: %v", failures[1])
	}
	// The aggregate cause is reachable via errors.Is.
	if !errors.Is(got, first) {
		t.Error("CloneFailedMulti should preserve typed cause for errors.Is")
	}
}

func TestNewf_capturesCauseFromTrailingErrorArg(t *testing.T) {
	cause := errors.New("io broken")
	e := Newf(CodeInternal, CategoryGeneric, "couldn't widget the %s: %v", "frobber", cause)
	if e == nil {
		t.Fatal("Newf returned nil")
	}
	if !strings.Contains(e.Error(), "frobber") {
		t.Errorf("Error() lost format args: %q", e.Error())
	}
	if !errors.Is(e, cause) {
		t.Error("errors.Is should find the trailing-arg cause")
	}
	// No trailing error → no cause captured.
	plain := Newf(CodeInternal, CategoryGeneric, "no cause here")
	if plain.Unwrap() != nil {
		t.Error("Newf without an error arg should not set a cause")
	}
}

func TestRaidError_BasicAccessors(t *testing.T) {
	cause := errors.New("network down")
	e := newRaidError("X", CategoryNetwork, "msg", "hint", map[string]any{"k": "v"}, cause)
	if e.Code() != "X" {
		t.Errorf("Code = %q", e.Code())
	}
	if e.Category() != CategoryNetwork {
		t.Errorf("Category = %v", e.Category())
	}
	if e.Error() != "msg" {
		t.Errorf("Error = %q", e.Error())
	}
	if e.Hint() != "hint" {
		t.Errorf("Hint = %q", e.Hint())
	}
	if e.Details()["k"] != "v" {
		t.Errorf("Details = %v", e.Details())
	}
	if !errors.Is(e, cause) {
		t.Errorf("errors.Is(e, cause) = false")
	}
}

func TestCategory_String(t *testing.T) {
	tests := []struct {
		c    Category
		want string
	}{
		{CategoryGeneric, "generic"},
		{CategoryConfig, "config"},
		{CategoryTask, "task"},
		{CategoryNetwork, "network"},
		{CategoryNotFound, "not-found"},
		{Category(99), "generic"},
	}
	for _, tt := range tests {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("Category(%d).String() = %q, want %q", int(tt.c), got, tt.want)
		}
	}
}

func TestAsError(t *testing.T) {
	if _, ok := AsError(nil); ok {
		t.Errorf("AsError(nil) ok = true, want false")
	}
	if _, ok := AsError(errors.New("plain")); ok {
		t.Errorf("AsError(plain) ok = true, want false")
	}
	e := ProfileNotFound("x")
	got, ok := AsError(e)
	if !ok || got.Code() != CodeProfileNotFound {
		t.Errorf("AsError(profileNotFound) = %v, ok=%v", got, ok)
	}
	// Wrapped via fmt.Errorf still resolves.
	wrapped := fmt.Errorf("outer: %w", e)
	got, ok = AsError(wrapped)
	if !ok || got.Code() != CodeProfileNotFound {
		t.Errorf("AsError(wrapped) = %v, ok=%v", got, ok)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"plain error", errors.New("oops"), 1},
		{"generic", Internal("x"), 1},
		{"config", ProfileInvalid("/p", errors.New("x")), 2},
		{"task", TaskShellFailed(errors.New("x")), 3},
		{"network", CloneFailed("r", "u", errors.New("x")), 4},
		{"not-found", RepoNotFound("r"), 5},
		{"wrapped not-found", fmt.Errorf("wrap: %w", EnvNotFound("e")), 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(tt.err); got != tt.want {
				t.Errorf("ExitCode = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestEmitJSON_shape(t *testing.T) {
	var buf bytes.Buffer
	EmitJSON(&buf, RepoNotCloned("api", "/x/api"))

	var doc struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("Unmarshal: %v\n%s", err, buf.String())
	}
	if doc.Error["code"] != "REPO_NOT_CLONED" {
		t.Errorf("code = %v", doc.Error["code"])
	}
	if doc.Error["category"] != "not-found" {
		t.Errorf("category = %v", doc.Error["category"])
	}
	if !strings.Contains(doc.Error["message"].(string), "api") {
		t.Errorf("message = %v", doc.Error["message"])
	}
	if doc.Error["hint"] == nil {
		t.Errorf("hint missing")
	}
	if doc.Error["repo"] != "api" || doc.Error["path"] != "/x/api" {
		t.Errorf("details not flattened: %v", doc.Error)
	}
}

func TestEmitJSON_wrapsPlainError(t *testing.T) {
	var buf bytes.Buffer
	EmitJSON(&buf, errors.New("bare failure"))
	var doc struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if doc.Error["code"] != "UNKNOWN" {
		t.Errorf("code = %v, want UNKNOWN", doc.Error["code"])
	}
	if doc.Error["message"] != "bare failure" {
		t.Errorf("message = %v", doc.Error["message"])
	}
}

func TestEmitJSON_nilNoOp(t *testing.T) {
	var buf bytes.Buffer
	EmitJSON(&buf, nil)
	if buf.Len() != 0 {
		t.Errorf("nil should not write anything, got %q", buf.String())
	}
}

func TestEmitJSON_reservedKeysFromDetailsIgnored(t *testing.T) {
	e := newRaidError("Z", CategoryGeneric, "m", "", map[string]any{
		"code":    "WRONG",
		"message": "wrong",
		"extra":   "kept",
	}, nil)
	var buf bytes.Buffer
	EmitJSON(&buf, e)
	var doc struct {
		Error map[string]any `json:"error"`
	}
	_ = json.Unmarshal(buf.Bytes(), &doc)
	if doc.Error["code"] != "Z" {
		t.Errorf("reserved code was overridden: %v", doc.Error["code"])
	}
	if doc.Error["message"] != "m" {
		t.Errorf("reserved message was overridden: %v", doc.Error["message"])
	}
	if doc.Error["extra"] != "kept" {
		t.Errorf("non-reserved details should pass through: %v", doc.Error)
	}
}

func TestWrap(t *testing.T) {
	if Wrap(nil) != nil {
		t.Errorf("Wrap(nil) should be nil")
	}
	e := ProfileNotFound("x")
	if got := Wrap(e); got != e {
		t.Errorf("Wrap(typedErr) should return it unchanged")
	}
	if got := Wrap(errors.New("plain")); got.Code() != CodeUnknown {
		t.Errorf("Wrap(plain).Code = %q, want UNKNOWN", got.Code())
	}
}

// TestEveryConstructor_isWellFormed locks in the contract that every
// constructor returns a non-nil error with a non-empty message and a
// valid category mapping to a known exit code.
func TestEveryConstructor_isWellFormed(t *testing.T) {
	tests := []struct {
		name string
		fn   func() *RaidError
		code string
	}{
		{"Unknown", func() *RaidError { return Unknown(errors.New("c")) }, CodeUnknown},
		{"Unknown(nil)", func() *RaidError { return Unknown(nil) }, CodeUnknown},
		{"Internal", func() *RaidError { return Internal("x") }, CodeInternal},
		{"GitNotInstalled", GitNotInstalled, CodeGitNotInstalled},
		{"LockFailed", func() *RaidError { return LockFailed(errors.New("c")) }, CodeLockFailed},
		{"LockFailed(nil)", func() *RaidError { return LockFailed(nil) }, CodeLockFailed},
		{"ProfileNotFound", func() *RaidError { return ProfileNotFound("p") }, CodeProfileNotFound},
		{"ProfileNotActive", ProfileNotActive, CodeProfileNotActive},
		{"ProfileFileMissing", func() *RaidError { return ProfileFileMissing("/p") }, CodeProfileFileMissing},
		{"ProfileFileRead", func() *RaidError { return ProfileFileRead("/p", errors.New("c")) }, CodeProfileFileRead},
		{"ProfileFileRead(nil)", func() *RaidError { return ProfileFileRead("/p", nil) }, CodeProfileFileRead},
		{"ProfileInvalid", func() *RaidError { return ProfileInvalid("/p", errors.New("c")) }, CodeProfileInvalid},
		{"ProfileInvalid(nil)", func() *RaidError { return ProfileInvalid("/p", nil) }, CodeProfileInvalid},
		{"ProfileAlreadyExists", func() *RaidError { return ProfileAlreadyExists("p") }, CodeProfileAlreadyExists},
		{"RepoNotFound", func() *RaidError { return RepoNotFound("r") }, CodeRepoNotFound},
		{"RepoNotCloned", func() *RaidError { return RepoNotCloned("r", "/p") }, CodeRepoNotCloned},
		{"RepoInvalid", func() *RaidError { return RepoInvalid("r", errors.New("c")) }, CodeRepoInvalid},
		{"RepoInvalid(nil)", func() *RaidError { return RepoInvalid("r", nil) }, CodeRepoInvalid},
		{"CloneFailed", func() *RaidError { return CloneFailed("r", "u", errors.New("c")) }, CodeCloneFailed},
		{"CloneFailed(nil)", func() *RaidError { return CloneFailed("r", "u", nil) }, CodeCloneFailed},
		{"EnvNotFound", func() *RaidError { return EnvNotFound("e") }, CodeEnvNotFound},
		{"CommandNotFound", func() *RaidError { return CommandNotFound("c") }, CodeCommandNotFound},
		{"ArgInvalid", func() *RaidError { return ArgInvalid("x") }, CodeArgInvalid},
		{"ConfigInvalid", func() *RaidError { return ConfigInvalid(errors.New("c")) }, CodeConfigInvalid},
		{"ConfigInvalid(nil)", func() *RaidError { return ConfigInvalid(nil) }, CodeConfigInvalid},
		{"ConfigLoadFailed", func() *RaidError { return ConfigLoadFailed(errors.New("c")) }, CodeConfigLoadFailed},
		{"ConfigLoadFailed(nil)", func() *RaidError { return ConfigLoadFailed(nil) }, CodeConfigLoadFailed},
		{"SchemaValidationFailed", func() *RaidError { return SchemaValidationFailed("/p", errors.New("c")) }, CodeSchemaValidationFailed},
		{"SchemaValidationFailed(nil)", func() *RaidError { return SchemaValidationFailed("/p", nil) }, CodeSchemaValidationFailed},
		{"TaskFailed", func() *RaidError { return TaskFailed("Print", errors.New("c")) }, CodeTaskFailed},
		{"TaskFailed(nil)", func() *RaidError { return TaskFailed("Print", nil) }, CodeTaskFailed},
		{"TaskShellFailed", func() *RaidError { return TaskShellFailed(errors.New("c")) }, CodeTaskShellFailed},
		{"TaskShellFailed(nil)", func() *RaidError { return TaskShellFailed(nil) }, CodeTaskShellFailed},
		{"TaskScriptFailed", func() *RaidError { return TaskScriptFailed(errors.New("c")) }, CodeTaskScriptFailed},
		{"TaskScriptFailed(nil)", func() *RaidError { return TaskScriptFailed(nil) }, CodeTaskScriptFailed},
		{"TaskWaitTimeout", func() *RaidError { return TaskWaitTimeout("t", errors.New("c")) }, CodeTaskWaitTimeout},
		{"TaskWaitTimeout(nil)", func() *RaidError { return TaskWaitTimeout("t", nil) }, CodeTaskWaitTimeout},
		{"TaskTemplateFailed", func() *RaidError { return TaskTemplateFailed(errors.New("c")) }, CodeTaskTemplateFailed},
		{"TaskTemplateFailed(nil)", func() *RaidError { return TaskTemplateFailed(nil) }, CodeTaskTemplateFailed},
		{"TaskGitFailed", func() *RaidError { return TaskGitFailed(errors.New("c")) }, CodeTaskGitFailed},
		{"TaskGitFailed(nil)", func() *RaidError { return TaskGitFailed(nil) }, CodeTaskGitFailed},
		{"TaskHTTPFailed", func() *RaidError { return TaskHTTPFailed("u", errors.New("c")) }, CodeTaskHTTPFailed},
		{"TaskHTTPFailed(nil)", func() *RaidError { return TaskHTTPFailed("u", nil) }, CodeTaskHTTPFailed},
		{"VerifyFailed", func() *RaidError { return VerifyFailed("v", errors.New("c")) }, CodeVerifyFailed},
		{"VerifyFailed(nil)", func() *RaidError { return VerifyFailed("v", nil) }, CodeVerifyFailed},
		{"HeadlessPromptNoDefault", func() *RaidError { return HeadlessPromptNoDefault("VAR") }, CodeHeadlessPromptNoDefault},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := tt.fn()
			if e == nil {
				t.Fatal("constructor returned nil")
			}
			if e.Code() != tt.code {
				t.Errorf("Code = %q, want %q", e.Code(), tt.code)
			}
			if e.Error() == "" {
				t.Errorf("Error() is empty")
			}
			// Round-trip through EmitJSON to confirm JSON-marshalability.
			var buf bytes.Buffer
			EmitJSON(&buf, e)
			if buf.Len() == 0 {
				t.Errorf("EmitJSON produced no output")
			}
			// Every category is one of the documented exit codes.
			if c := int(e.Category()); c < 1 || c > 5 {
				t.Errorf("Category %d outside documented 1-5 range", c)
			}
		})
	}
}
