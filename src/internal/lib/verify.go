package lib

import (
	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
)

// Verify is a declarative precondition check. Each entry runs `Tasks` —
// if any task exits non-zero, the optional `OnFail` block runs once as
// remediation and then the original `Tasks` re-runs. Verify passes if
// the original or remediated run succeeds; otherwise it's reported as
// failed via errs.VerifyFailed.
//
// Verify entries live at the top level of profiles and per-repo
// raid.yaml files. raid doctor (#42) will surface them as findings in
// its health report; this package provides the runner so any caller
// can drive it the same way doctor will.
type Verify struct {
	// Name is the human-readable label surfaced in failure messages
	// and (eventually) doctor's findings list. Required.
	Name string `json:"name"`
	// Tasks is the precondition assertion. All tasks must exit 0 for
	// the verify to pass on the first try.
	Tasks []Task `json:"tasks"`
	// OnFail is the optional one-shot remediation. When present, a
	// first-pass failure triggers OnFail followed by exactly one
	// re-run of Tasks. Empty OnFail means the first failure is final.
	OnFail []Task `json:"onFail,omitempty"`
}

// IsZero reports whether the verify is uninitialised — used to skip
// empty entries that the YAML parser might surface from stray list
// items.
func (v Verify) IsZero() bool {
	return v.Name == "" && len(v.Tasks) == 0 && len(v.OnFail) == 0
}

// RunVerify executes the verify entry per the documented semantics:
// run Tasks; on failure, if OnFail is set, run it and re-run Tasks
// once. Returns nil on success (including remediated success), or
// errs.VerifyFailed wrapping the underlying cause otherwise.
//
// Tasks run with raid's normal env / raidVars / command-session
// context — same as install: tasks — so verifies see whatever the
// caller has loaded. The caller is responsible for setting up
// `startSession` / `endSession` if it needs session isolation; doctor
// is expected to wrap RunVerify calls accordingly.
func RunVerify(v Verify) error {
	if len(v.Tasks) == 0 {
		// No-op: an entry with no Tasks asserts nothing, so it
		// vacuously passes. Treat as success rather than rejecting
		// at the schema layer so doctor can iterate generously.
		return nil
	}

	if err := ExecuteTasks(v.Tasks); err == nil {
		return nil
	} else if len(v.OnFail) == 0 {
		return liberrs.VerifyFailed(v.Name, err)
	}

	// First pass failed and we have remediation. Run it once.
	if err := ExecuteTasks(v.OnFail); err != nil {
		// Remediation itself failed — don't retry the asserts; the
		// user's fix-up step is broken, that's the more useful
		// failure to surface.
		return liberrs.VerifyFailed(v.Name, err)
	}

	// Exactly one retry of the asserts.
	if err := ExecuteTasks(v.Tasks); err != nil {
		return liberrs.VerifyFailed(v.Name, err)
	}
	return nil
}
