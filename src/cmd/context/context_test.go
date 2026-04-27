package context

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	rctx "github.com/8bitalex/raid/src/raid/context"
)

func TestWritePretty_noProfile(t *testing.T) {
	var buf bytes.Buffer
	if err := writePretty(&buf, rctx.Workspace{Repos: []rctx.Repo{}}); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	if !strings.Contains(buf.String(), "No active profile") {
		t.Errorf("expected 'No active profile' message, got: %q", buf.String())
	}
}

func TestWritePretty_noRepos(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Workspace{
		Profile: "demo",
		Env:     "dev",
		Repos:   []rctx.Repo{},
	}
	if err := writePretty(&buf, ws); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Profile: demo") {
		t.Errorf("missing profile line: %q", out)
	}
	if !strings.Contains(out, "Env:     dev") {
		t.Errorf("missing env line: %q", out)
	}
	if !strings.Contains(out, "Repos: none configured") {
		t.Errorf("missing 'no repos' message: %q", out)
	}
}

func TestWritePretty_repoStates(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Workspace{
		Profile: "demo",
		Env:     "dev",
		Repos: []rctx.Repo{
			{Name: "api", Path: "~/dev/api", Cloned: true, Branch: "main"},
			{Name: "frontend", Path: "~/dev/frontend", Cloned: true, Branch: "develop", Dirty: true},
			{Name: "worker", Path: "~/dev/worker"},
		},
	}
	if err := writePretty(&buf, ws); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"api", "main", "clean",
		"frontend", "develop", "dirty",
		"worker", "not cloned",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestWriteJSON_shape(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Workspace{
		Profile: "demo",
		Env:     "dev",
		Repos: []rctx.Repo{
			{Name: "api", Path: "~/dev/api", Cloned: true, Branch: "main"},
			{Name: "worker", Path: "~/dev/worker"},
		},
	}
	if err := writeJSON(&buf, ws); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}

	var decoded rctx.Workspace
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}

	if decoded.Profile != "demo" || decoded.Env != "dev" {
		t.Errorf("decoded header = %+v", decoded)
	}
	if len(decoded.Repos) != 2 {
		t.Fatalf("repos len = %d, want 2", len(decoded.Repos))
	}
	if decoded.Repos[0].Name != "api" || !decoded.Repos[0].Cloned || decoded.Repos[0].Branch != "main" {
		t.Errorf("api repo = %+v", decoded.Repos[0])
	}
	if decoded.Repos[1].Name != "worker" || decoded.Repos[1].Cloned {
		t.Errorf("worker repo = %+v (should be uncloned)", decoded.Repos[1])
	}
}

func TestRepoStatus(t *testing.T) {
	tests := []struct {
		name string
		in   rctx.Repo
		want string
	}{
		{"not cloned", rctx.Repo{}, "not cloned"},
		{"clean", rctx.Repo{Cloned: true}, "clean"},
		{"dirty", rctx.Repo{Cloned: true, Dirty: true}, "dirty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := repoStatus(tt.in); got != tt.want {
				t.Errorf("repoStatus = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWritePretty_commands(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Workspace{
		Profile:  "demo",
		Repos:    []rctx.Repo{},
		Commands: []rctx.Command{
			{Name: "deploy", Description: "Deploy to production"},
			{Name: "test", Description: "Run tests"},
		},
	}
	if err := writePretty(&buf, ws); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Commands (2)", "deploy", "Deploy to production", "test", "Run tests"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestWritePretty_recent(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Workspace{
		Profile: "demo",
		Repos:   []rctx.Repo{},
		Recent: []rctx.Recent{
			{Command: "deploy", ExitCode: 0, DurationMs: 12300},
			{Command: "test", ExitCode: 1, DurationMs: 4500},
		},
	}
	if err := writePretty(&buf, ws); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Recent (2)", "deploy", "test", "✓", "✗"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestWriteJSON_includesAllSections(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Workspace{
		Profile:  "demo",
		Env:      "dev",
		Repos:    []rctx.Repo{{Name: "api", Path: "~/dev/api", Cloned: true, Branch: "main"}},
		Commands: []rctx.Command{{Name: "deploy", Description: "Deploy"}},
		Recent:   []rctx.Recent{{Command: "deploy", ExitCode: 0, DurationMs: 1234}},
	}
	if err := writeJSON(&buf, ws); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	var decoded rctx.Workspace
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(decoded.Commands) != 1 || decoded.Commands[0].Name != "deploy" {
		t.Errorf("commands round-trip failed: %+v", decoded.Commands)
	}
	if len(decoded.Recent) != 1 || decoded.Recent[0].Command != "deploy" {
		t.Errorf("recent round-trip failed: %+v", decoded.Recent)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{500, "500ms"},
		{1500, "1.5s"},
		{75000, "1m15s"},
		{3700000, "1h01m"},
	}
	for _, tt := range tests {
		if got := formatDuration(tt.ms); got != tt.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

func TestRelativeTime(t *testing.T) {
	now, _ := time.Parse(time.RFC3339, "2026-04-27T12:00:00Z")
	tests := []struct {
		then string
		want string
	}{
		{"2026-04-27T11:59:55Z", "just now"},
		{"2026-04-27T11:59:30Z", "30s ago"},
		{"2026-04-27T11:55:00Z", "5m ago"},
		{"2026-04-27T10:00:00Z", "2h ago"},
		{"2026-04-25T12:00:00Z", "2d ago"},
	}
	for _, tt := range tests {
		then, _ := time.Parse(time.RFC3339, tt.then)
		if got := relativeTime(now, then); got != tt.want {
			t.Errorf("relativeTime(%s) = %q, want %q", tt.then, got, tt.want)
		}
	}
}

func TestCommand_isWired(t *testing.T) {
	if Command == nil {
		t.Fatal("Command is nil")
	}
	if Command.Use != "context" {
		t.Errorf("Use = %q, want %q", Command.Use, "context")
	}
	if Command.Flags().Lookup("json") == nil {
		t.Error("--json flag not registered")
	}
}
