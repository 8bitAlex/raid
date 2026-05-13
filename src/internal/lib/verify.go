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
// raid.yaml files. raid doctor surfaces them as findings: a first-try
// pass is an OK finding, a successful self-heal is a warning (so the
// user knows something needed fixing), and a failure is an error.
type Verify struct {
	// Name is the human-readable label surfaced in failure messages
	// and doctor's findings list. Required.
	Name string `json:"name" yaml:"name"`
	// Tasks is the precondition assertion. All tasks must exit 0 for
	// the verify to pass on the first try.
	Tasks []Task `json:"tasks" yaml:"tasks"`
	// OnFail is the optional one-shot remediation. When present, a
	// first-pass failure triggers OnFail followed by exactly one
	// re-run of Tasks. Empty OnFail means the first failure is final.
	// The explicit yaml tag is required so YAML's default lowercasing
	// doesn't turn `onFail:` into a silently ignored key.
	OnFail []Task `json:"onFail,omitempty" yaml:"onFail,omitempty"`
}

// IsZero reports whether the verify is uninitialised — used to skip
// empty entries that the YAML parser might surface from stray list
// items.
func (v Verify) IsZero() bool {
	return v.Name == "" && len(v.Tasks) == 0 && len(v.OnFail) == 0
}

// VerifyOutcome distinguishes a first-try pass from a successful
// self-heal. Doctor maps these to OK / warn severities; a fourth state
// (failed) is conveyed by RunVerify's error return.
type VerifyOutcome int

const (
	// VerifyOutcomeOK means Tasks passed on the first try; no
	// remediation ran.
	VerifyOutcomeOK VerifyOutcome = iota
	// VerifyOutcomeRemediated means Tasks failed on the first try,
	// OnFail succeeded, and the retry of Tasks passed. The verify
	// holds *now*, but it didn't before — worth surfacing as a
	// warning so the user knows something silently fixed itself.
	VerifyOutcomeRemediated
	// VerifyOutcomeFailed means Tasks didn't pass: either no OnFail
	// was defined, OnFail itself failed, or the retry of Tasks
	// still failed. RunVerify returns a non-nil error in this case.
	VerifyOutcomeFailed
)

// RunVerify executes the verify entry per the documented semantics:
// run Tasks; on failure, if OnFail is set, run it and re-run Tasks
// once. Returns one of three outcomes — see VerifyOutcome.
//
// An empty Tasks slice is treated as a no-op pass.
//
// Tasks run with raid's normal env / raidVars / command-session
// context — same as install: tasks — so verifies see whatever the
// caller has loaded.
func RunVerify(v Verify) (VerifyOutcome, error) {
	if len(v.Tasks) == 0 {
		// No-op: an entry with no Tasks asserts nothing, so it
		// vacuously passes. Treat as OK rather than rejecting at
		// the schema layer so doctor can iterate generously.
		return VerifyOutcomeOK, nil
	}

	if err := ExecuteTasks(v.Tasks); err == nil {
		return VerifyOutcomeOK, nil
	} else if len(v.OnFail) == 0 {
		return VerifyOutcomeFailed, liberrs.VerifyFailed(v.Name, err)
	}

	// First pass failed and we have remediation. Run it once.
	if err := ExecuteTasks(v.OnFail); err != nil {
		// Remediation itself failed — don't retry the asserts; the
		// user's fix-up step is broken, that's the more useful
		// failure to surface.
		return VerifyOutcomeFailed, liberrs.VerifyFailed(v.Name, err)
	}

	// Exactly one retry of the asserts.
	if err := ExecuteTasks(v.Tasks); err != nil {
		return VerifyOutcomeFailed, liberrs.VerifyFailed(v.Name, err)
	}
	return VerifyOutcomeRemediated, nil
}
