package lib

import (
	"os"
	"path/filepath"
	"testing"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
)

func TestRunVerify_passes(t *testing.T) {
	v := Verify{
		Name: "ok",
		Tasks: []Task{
			{Type: Shell, Cmd: "exit 0"},
		},
	}
	if err := RunVerify(v); err != nil {
		t.Fatalf("RunVerify returned %v, want nil", err)
	}
}

func TestRunVerify_failsWithoutOnFail(t *testing.T) {
	v := Verify{
		Name: "missing-thing",
		Tasks: []Task{
			{Type: Shell, Cmd: "exit 1"},
		},
	}
	err := RunVerify(v)
	if err == nil {
		t.Fatal("expected error, got nil")
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

	if err := RunVerify(v); err != nil {
		t.Fatalf("RunVerify returned %v, want nil after remediation", err)
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

	err := RunVerify(v)
	if err == nil {
		t.Fatal("expected error, got nil")
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
	err := RunVerify(v)
	if err == nil {
		t.Fatal("expected error, got nil")
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
	if err := RunVerify(v); err != nil {
		t.Fatalf("RunVerify with empty tasks returned %v, want nil", err)
	}
	if _, err := os.Stat(retryMarker); err == nil {
		t.Error("OnFail ran for a verify with no tasks")
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
