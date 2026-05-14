package lib

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestResolveAgent_nilProducesSafeFalse(t *testing.T) {
	got := resolveAgent(Command{Name: "lint", Usage: "Run linters"})
	if got.Safe {
		t.Errorf("Safe = true, want false for nil Agent")
	}
	if got.Description != "Run linters" {
		t.Errorf("Description = %q, want fallback to Usage", got.Description)
	}
	if got.Reads != nil || got.Writes != nil {
		t.Errorf("Reads/Writes = %v/%v, want nil for nil Agent", got.Reads, got.Writes)
	}
}

func TestResolveAgent_safeTrueRoundTrips(t *testing.T) {
	got := resolveAgent(Command{
		Name:  "test",
		Usage: "Run tests",
		Agent: &Agent{Safe: true},
	})
	if !got.Safe {
		t.Error("Safe = false, want true")
	}
}

func TestResolveAgent_descriptionOverridesUsage(t *testing.T) {
	got := resolveAgent(Command{
		Name:  "deploy",
		Usage: "deploy",
		Agent: &Agent{Description: "Deploy the service to prod — destructive"},
	})
	if got.Description != "Deploy the service to prod — destructive" {
		t.Errorf("Description = %q, want explicit override", got.Description)
	}
}

func TestResolveAgent_emptyDescriptionFallsBackToUsage(t *testing.T) {
	got := resolveAgent(Command{
		Name:  "test",
		Usage: "Run tests",
		Agent: &Agent{Safe: true}, // Description left empty
	})
	if got.Description != "Run tests" {
		t.Errorf("Description = %q, want fallback to Usage when Agent.Description empty", got.Description)
	}
}

func TestResolveAgent_readsWritesPreserved(t *testing.T) {
	reads := []string{"src/**/*.go", "go.mod"}
	writes := []string{"dist/"}
	got := resolveAgent(Command{
		Name:  "build",
		Usage: "Build",
		Agent: &Agent{Safe: false, Reads: reads, Writes: writes},
	})
	if !reflect.DeepEqual(got.Reads, reads) {
		t.Errorf("Reads = %v, want %v", got.Reads, reads)
	}
	if !reflect.DeepEqual(got.Writes, writes) {
		t.Errorf("Writes = %v, want %v", got.Writes, writes)
	}
}

func TestWorkspaceAgent_JSONAlwaysEmitsSafeField(t *testing.T) {
	// Public-contract assertion: even when the Command has no Agent block,
	// the marshalled WorkspaceCommand must include "safe":false. MCP
	// clients rely on reading agent.safe without checking presence first.
	wc := WorkspaceCommand{
		Name:        "lint",
		Description: "Lint everything",
		Agent:       resolveAgent(Command{Name: "lint", Usage: "Lint everything"}),
	}
	buf, err := json.Marshal(wc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(buf), `"safe":false`) {
		t.Errorf("JSON missing safe field: %s", buf)
	}
}

func TestWorkspaceAgent_omitsEmptySlicesAndDescription(t *testing.T) {
	// A nil-Agent command falls back to Usage for Description; Reads and
	// Writes should be omitted from the JSON since they're nil.
	wc := WorkspaceCommand{
		Name:  "lint",
		Agent: resolveAgent(Command{Name: "lint", Usage: "Lint"}),
	}
	buf, err := json.Marshal(wc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(buf)
	if strings.Contains(got, "reads") || strings.Contains(got, "writes") {
		t.Errorf("JSON should omit empty reads/writes: %s", got)
	}
}
