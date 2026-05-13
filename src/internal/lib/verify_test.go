package lib

import (
	"os"
	"path/filepath"
	"testing"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
	"gopkg.in/yaml.v3"
)

func TestRunVerify_passes(t *testing.T) {
	v := Verify{
		Name: "ok",
		Tasks: []Task{
			{Type: Shell, Cmd: "exit 0"},
		},
	}
	outcome, err := RunVerify(v)
	if err != nil {
		t.Fatalf("RunVerify returned %v, want nil", err)
	}
	if outcome != VerifyOutcomeOK {
		t.Errorf("outcome = %v, want VerifyOutcomeOK", outcome)
	}
}

func TestRunVerify_failsWithoutOnFail(t *testing.T) {
	v := Verify{
		Name: "missing-thing",
		Tasks: []Task{
			{Type: Shell, Cmd: "exit 1"},
		},
	}
	outcome, err := RunVerify(v)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if outcome != VerifyOutcomeFailed {
		t.Errorf("outcome = %v, want VerifyOutcomeFailed", outcome)
	}
	rErr, ok := liberrs.AsError(err)
	if !ok {
		t.Fatalf("error not structured: %v", err)
	}
	if rErr.Code() != liberrs.CodeVerifyFailed {
		t.Errorf("code = %q, want VERIFY_FAILED", rErr.Code())
	}
	if got, _ := rErr.Details()["verify"].(string); got != "missing-thing" {
		t.Errorf("details[verify] = %q, want %q", got, "missing-thing")
	}
}

func TestRunVerify_remediationSucceeds(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker")
	retryMarker := filepath.Join(t.TempDir(), "retry-ran")
	v := Verify{
		Name: "needs-marker",
		Tasks: []Task{
			// Fails when the marker is missing; succeeds once OnFail creates it.
			// Touches retryMarker so we can prove the second pass ran.
			{Type: Shell, Cmd: "touch " + retryMarker + " && test -f " + marker},
		},
		OnFail: []Task{
			{Type: Shell, Cmd: "touch " + marker},
		},
	}
	// First touch removes any prior retry marker so we can assert it was
	// written by the retry pass (not lingering from setup).
	_ = os.Remove(retryMarker)

	outcome, err := RunVerify(v)
	if err != nil {
		t.Fatalf("RunVerify returned %v, want nil after remediation", err)
	}
	if outcome != VerifyOutcomeRemediated {
		t.Errorf("outcome = %v, want VerifyOutcomeRemediated", outcome)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("OnFail did not create marker: %v", err)
	}
	if _, err := os.Stat(retryMarker); err != nil {
		t.Errorf("retry pass did not run: %v", err)
	}
}

func TestRunVerify_remediationFails(t *testing.T) {
	retryMarker := filepath.Join(t.TempDir(), "should-not-exist")
	v := Verify{
		Name: "broken-remediation",
		Tasks: []Task{
			{Type: Shell, Cmd: "exit 1"},
		},
		OnFail: []Task{
			{Type: Shell, Cmd: "exit 1"},
		},
	}
	// Sanity: marker should never be touched because retry never runs.
	v.Tasks = append(v.Tasks, Task{Type: Shell, Cmd: "touch " + retryMarker})

	outcome, err := RunVerify(v)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if outcome != VerifyOutcomeFailed {
		t.Errorf("outcome = %v, want VerifyOutcomeFailed", outcome)
	}
	rErr, ok := liberrs.AsError(err)
	if !ok {
		t.Fatalf("error not structured: %v", err)
	}
	if rErr.Code() != liberrs.CodeVerifyFailed {
		t.Errorf("code = %q, want VERIFY_FAILED", rErr.Code())
	}
	if _, err := os.Stat(retryMarker); err == nil {
		t.Error("retry pass executed despite OnFail failure")
	}
}

func TestRunVerify_secondPassFails(t *testing.T) {
	v := Verify{
		Name: "still-broken",
		Tasks: []Task{
			{Type: Shell, Cmd: "exit 1"},
		},
		OnFail: []Task{
			{Type: Shell, Cmd: "exit 0"},
		},
	}
	outcome, err := RunVerify(v)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if outcome != VerifyOutcomeFailed {
		t.Errorf("outcome = %v, want VerifyOutcomeFailed", outcome)
	}
	rErr, ok := liberrs.AsError(err)
	if !ok {
		t.Fatalf("error not structured: %v", err)
	}
	if rErr.Code() != liberrs.CodeVerifyFailed {
		t.Errorf("code = %q, want VERIFY_FAILED", rErr.Code())
	}
}

func TestRunVerify_emptyTasksIsNoop(t *testing.T) {
	retryMarker := filepath.Join(t.TempDir(), "should-not-exist")
	v := Verify{
		Name:  "no-checks",
		Tasks: nil,
		OnFail: []Task{
			{Type: Shell, Cmd: "touch " + retryMarker},
		},
	}
	outcome, err := RunVerify(v)
	if err != nil {
		t.Fatalf("RunVerify with empty tasks returned %v, want nil", err)
	}
	if outcome != VerifyOutcomeOK {
		t.Errorf("outcome = %v, want VerifyOutcomeOK for empty tasks", outcome)
	}
	if _, err := os.Stat(retryMarker); err == nil {
		t.Error("OnFail ran for a verify with no tasks")
	}
}

func TestVerify_yamlRoundTrip(t *testing.T) {
	// Documents the on-disk shape: `onFail` (camelCase) must populate
	// OnFail. Without an explicit yaml tag, gopkg.in/yaml.v3 would
	// silently treat the key as `onfail` and drop remediation tasks.
	src := []byte("name: node\ntasks:\n  - type: Shell\n    cmd: node --version\nonFail:\n  - type: Shell\n    cmd: nvm install --lts\n")
	var v Verify
	if err := yaml.Unmarshal(src, &v); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}
	if v.Name != "node" {
		t.Errorf("Name = %q, want %q", v.Name, "node")
	}
	if len(v.Tasks) != 1 || v.Tasks[0].Cmd != "node --version" {
		t.Errorf("Tasks not extracted: %+v", v.Tasks)
	}
	if len(v.OnFail) != 1 || v.OnFail[0].Cmd != "nvm install --lts" {
		t.Errorf("OnFail not extracted from `onFail:` key: %+v", v.OnFail)
	}
}

func TestVerifyIsZero(t *testing.T) {
	if !(Verify{}).IsZero() {
		t.Error("zero-value Verify reports non-zero")
	}
	cases := []Verify{
		{Name: "x"},
		{Tasks: []Task{{Type: Shell, Cmd: "exit 0"}}},
		{OnFail: []Task{{Type: Shell, Cmd: "exit 0"}}},
	}
	for i, v := range cases {
		if v.IsZero() {
			t.Errorf("case %d: non-zero Verify reports zero: %+v", i, v)
		}
	}
}
