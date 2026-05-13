package errs

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

// The public package is mostly an alias façade — these tests just make
// sure the re-exported constructors and helpers reach the underlying
// implementation, so the package shows up in coverage reports and a
// breaking move from internal would be caught at compile and at test.

func TestPublicSurface_constructorsRouteThrough(t *testing.T) {
	cases := []struct {
		name string
		got  Error
		code string
		cat  Category
	}{
		{"Unknown", Unknown(errors.New("x")), CodeUnknown, CategoryGeneric},
		{"Internal", Internal("x"), CodeInternal, CategoryGeneric},
		{"GitNotInstalled", GitNotInstalled(), CodeGitNotInstalled, CategoryGeneric},
		{"LockFailed", LockFailed(errors.New("x")), CodeLockFailed, CategoryGeneric},
		{"ProfileNotFound", ProfileNotFound("p"), CodeProfileNotFound, CategoryNotFound},
		{"ProfileNotActive", ProfileNotActive(), CodeProfileNotActive, CategoryNotFound},
		{"ProfileFileMissing", ProfileFileMissing("/p"), CodeProfileFileMissing, CategoryNotFound},
		{"ProfileFileRead", ProfileFileRead("/p", nil), CodeProfileFileRead, CategoryConfig},
		{"ProfileInvalid", ProfileInvalid("/p", nil), CodeProfileInvalid, CategoryConfig},
		{"ProfileAlreadyExists", ProfileAlreadyExists("p"), CodeProfileAlreadyExists, CategoryConfig},
		{"RepoNotFound", RepoNotFound("r"), CodeRepoNotFound, CategoryNotFound},
		{"RepoNotCloned", RepoNotCloned("r", "/p"), CodeRepoNotCloned, CategoryNotFound},
		{"RepoInvalid", RepoInvalid("r", nil), CodeRepoInvalid, CategoryConfig},
		{"CloneFailed", CloneFailed("r", "u", nil), CodeCloneFailed, CategoryNetwork},
		{"EnvNotFound", EnvNotFound("e"), CodeEnvNotFound, CategoryNotFound},
		{"CommandNotFound", CommandNotFound("c"), CodeCommandNotFound, CategoryNotFound},
		{"ArgInvalid", ArgInvalid("x"), CodeArgInvalid, CategoryConfig},
		{"ConfigInvalid", ConfigInvalid(nil), CodeConfigInvalid, CategoryConfig},
		{"ConfigLoadFailed", ConfigLoadFailed(nil), CodeConfigLoadFailed, CategoryConfig},
		{"SchemaValidationFailed", SchemaValidationFailed("/p", nil), CodeSchemaValidationFailed, CategoryConfig},
		{"TaskFailed", TaskFailed("Print", nil), CodeTaskFailed, CategoryTask},
		{"TaskShellFailed", TaskShellFailed(nil), CodeTaskShellFailed, CategoryTask},
		{"TaskScriptFailed", TaskScriptFailed(nil), CodeTaskScriptFailed, CategoryTask},
		{"TaskWaitTimeout", TaskWaitTimeout("t", nil), CodeTaskWaitTimeout, CategoryTask},
		{"TaskTemplateFailed", TaskTemplateFailed(nil), CodeTaskTemplateFailed, CategoryTask},
		{"TaskGitFailed", TaskGitFailed(nil), CodeTaskGitFailed, CategoryTask},
		{"TaskHTTPFailed", TaskHTTPFailed("u", nil), CodeTaskHTTPFailed, CategoryNetwork},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got == nil {
				t.Fatal("constructor returned nil")
			}
			if c.got.Code() != c.code {
				t.Errorf("Code = %q, want %q", c.got.Code(), c.code)
			}
			if c.got.Category() != c.cat {
				t.Errorf("Category = %v, want %v", c.got.Category(), c.cat)
			}
		})
	}
}

func TestPublicSurface_Newf(t *testing.T) {
	e := Newf(CodeInternal, CategoryGeneric, "thing %d failed", 7)
	if e.Code() != CodeInternal {
		t.Errorf("Code = %q, want %q", e.Code(), CodeInternal)
	}
	if e.Error() != "thing 7 failed" {
		t.Errorf("Error = %q", e.Error())
	}
}

func TestPublicSurface_helpers(t *testing.T) {
	if got, ok := AsError(nil); got != nil || ok {
		t.Errorf("AsError(nil) = (%v, %v), want (nil, false)", got, ok)
	}
	if got := ExitCode(nil); got != 0 {
		t.Errorf("ExitCode(nil) = %d, want 0", got)
	}
	if got := ExitCode(EnvNotFound("e")); got != 5 {
		t.Errorf("ExitCode(EnvNotFound) = %d, want 5", got)
	}
	if Wrap(nil) != nil {
		t.Errorf("Wrap(nil) should be nil")
	}
	if Wrap(errors.New("p")).Code() != CodeUnknown {
		t.Errorf("Wrap(plain) should produce UNKNOWN code")
	}

	var buf bytes.Buffer
	EmitJSON(&buf, EnvNotFound("dev"))
	var doc struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if doc.Error["code"] != "ENV_NOT_FOUND" {
		t.Errorf("code = %v", doc.Error["code"])
	}
}
