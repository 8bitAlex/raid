package context

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	rctx "github.com/8bitalex/raid/src/raid/context"
	"github.com/spf13/cobra"
)

func TestWritePretty_includesAgentHeader(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Snapshot{
		Name:       "raid",
		Version:    "1.2.3",
		WebsiteUrl: "https://github.com/8bitalex/raid",
		Workspace: rctx.Workspace{
			Profile: "demo",
			Repos:   []rctx.Repo{},
		},
	}
	if err := writePretty(&buf, ws); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"# raid", "v1.2.3", "workspace context", "https://github.com/8bitalex/raid"} {
		if !strings.Contains(out, want) {
			t.Errorf("header missing %q\n--- output ---\n%s", want, out)
		}
	}
}

func TestWriteJSON_includesAgentHeader(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Snapshot{
		Name:       "raid",
		Version:    "1.2.3",
		WebsiteUrl: "https://github.com/8bitalex/raid",
		Workspace: rctx.Workspace{
			Profile: "demo",
			Repos:   []rctx.Repo{},
		},
	}
	if err := writeJSON(&buf, ws); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	var decoded rctx.Snapshot
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.Name != "raid" {
		t.Errorf("Name = %q, want %q", decoded.Name, "raid")
	}
	if decoded.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", decoded.Version, "1.2.3")
	}
	if decoded.WebsiteUrl != "https://github.com/8bitalex/raid" {
		t.Errorf("WebsiteUrl = %q, want canonical URL", decoded.WebsiteUrl)
	}
}

func TestWritePretty_noProfile(t *testing.T) {
	var buf bytes.Buffer
	if err := writePretty(&buf, rctx.Snapshot{Workspace: rctx.Workspace{Repos: []rctx.Repo{}}}); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	if !strings.Contains(buf.String(), "No active profile") {
		t.Errorf("expected 'No active profile' message, got: %q", buf.String())
	}
}

func TestWritePretty_noRepos(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile: "demo",
			Env:     "dev",
			Repos:   []rctx.Repo{},
		},
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
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile: "demo",
			Env:     "dev",
			Repos: []rctx.Repo{
				{Name: "api", Path: "~/dev/api", Cloned: true, Branch: "main"},
				{Name: "frontend", Path: "~/dev/frontend", Cloned: true, Branch: "develop", Dirty: true},
				{Name: "worker", Path: "~/dev/worker"},
			},
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
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile: "demo",
			Env:     "dev",
			Repos: []rctx.Repo{
				{Name: "api", Path: "~/dev/api", Cloned: true, Branch: "main"},
				{Name: "worker", Path: "~/dev/worker"},
			},
		},
	}
	if err := writeJSON(&buf, ws); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}

	var decoded rctx.Snapshot
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}

	if decoded.Workspace.Profile != "demo" || decoded.Workspace.Env != "dev" {
		t.Errorf("decoded workspace = %+v", decoded.Workspace)
	}
	if len(decoded.Workspace.Repos) != 2 {
		t.Fatalf("repos len = %d, want 2", len(decoded.Workspace.Repos))
	}
	if decoded.Workspace.Repos[0].Name != "api" || !decoded.Workspace.Repos[0].Cloned || decoded.Workspace.Repos[0].Branch != "main" {
		t.Errorf("api repo = %+v", decoded.Workspace.Repos[0])
	}
	if decoded.Workspace.Repos[1].Name != "worker" || decoded.Workspace.Repos[1].Cloned {
		t.Errorf("worker repo = %+v (should be uncloned)", decoded.Workspace.Repos[1])
	}
}

// TestWriteJSON_workspaceIsNested guards the structural promise of step 1: the
// workspace data must live under a single `workspace` key, with no leakage of
// snake_case or flat top-level fields. A regression here defeats the whole
// MCP-shape alignment.
func TestWriteJSON_workspaceIsNested(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Snapshot{
		Name:       "raid",
		Version:    "1.2.3",
		WebsiteUrl: "https://github.com/8bitalex/raid",
		Workspace: rctx.Workspace{
			Profile: "demo",
			Env:     "dev",
			Repos:   []rctx.Repo{{Name: "api", Path: "~/dev/api"}},
			Commands: []rctx.Command{{Name: "deploy"}},
		},
	}
	if err := writeJSON(&buf, ws); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Top-level must NOT contain flat workspace keys or any snake_case names.
	for _, forbidden := range []string{"profile", "env", "repos", "commands", "recent", "tool", "repository", "generated_at"} {
		if _, present := decoded[forbidden]; present {
			t.Errorf("top-level should not contain %q (workspace data must nest under 'workspace')", forbidden)
		}
	}
	// Workspace key must exist and contain the data.
	wsObj, ok := decoded["workspace"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'workspace' object, got: %T", decoded["workspace"])
	}
	for _, want := range []string{"profile", "env", "repos", "commands"} {
		if _, present := wsObj[want]; !present {
			t.Errorf("workspace missing key %q", want)
		}
	}
}

// TestWriteJSON_recentUsesCamelCase ensures the recent log fields are emitted
// with MCP-aligned camelCase keys, not the prior snake_case form.
func TestWriteJSON_recentUsesCamelCase(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile: "demo",
			Repos:   []rctx.Repo{},
			Recent: []rctx.Recent{
				{Command: "deploy", Status: rctx.RecentStatusCompleted, ExitCode: 0, DurationMs: 1234},
			},
		},
	}
	if err := writeJSON(&buf, ws); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	wsObj := decoded["workspace"].(map[string]any)
	recent := wsObj["recent"].([]any)
	first := recent[0].(map[string]any)

	for _, camel := range []string{"exitCode", "startedAt", "durationMs"} {
		if _, present := first[camel]; !present {
			t.Errorf("recent entry missing camelCase key %q (got %v)", camel, first)
		}
	}
	for _, snake := range []string{"exit_code", "started_at", "duration_ms"} {
		if _, present := first[snake]; present {
			t.Errorf("recent entry should not contain snake_case key %q", snake)
		}
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
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile: "demo",
			Repos:   []rctx.Repo{},
			Commands: []rctx.Command{
				{Name: "deploy", Description: "Deploy to production"},
				{Name: "test", Description: "Run tests"},
			},
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

// TestWritePretty_commandSteps confirms that commands with named tasks expose
// numbered step rows under their main row, mirroring what the JSON output
// emits via the steps[] array.
func TestWritePretty_commandSteps(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile: "demo",
			Repos:   []rctx.Repo{},
			Commands: []rctx.Command{
				{
					Name:        "release",
					Description: "Cut a release",
					Steps: []rctx.Step{
						{Name: "Build artifact"},
						{Name: "Push to registry"},
						{Name: "Tag release"},
					},
				},
				{Name: "lint", Description: "Lint everything"}, // no steps — must not render any
			},
		},
	}
	if err := writePretty(&buf, ws); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"release",
		"1. Build artifact",
		"2. Push to registry",
		"3. Tag release",
		"lint",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
		}
	}
	// Lint command has no steps; ensure no numbered lines snuck in for it.
	if strings.Contains(out, "1. ") && strings.Count(out, "1. ") != 1 {
		t.Errorf("expected exactly one '1.' step row (release's first step), got: %s", out)
	}
}

func TestRecentMark(t *testing.T) {
	tests := []struct {
		name string
		in   rctx.Recent
		want string
	}{
		{"ok", rctx.Recent{Status: rctx.RecentStatusCompleted, ExitCode: 0}, "✓"},
		{"failed", rctx.Recent{Status: rctx.RecentStatusCompleted, ExitCode: 7}, "✗"},
		{"interrupted", rctx.Recent{Status: rctx.RecentStatusInterrupted}, "⊘"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recentMark(tt.in); got != tt.want {
				t.Errorf("recentMark = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRecentDuration(t *testing.T) {
	if got := recentDuration(rctx.Recent{Status: rctx.RecentStatusCompleted, DurationMs: 1500}); got != "1.5s" {
		t.Errorf("completed duration = %q, want 1.5s", got)
	}
	if got := recentDuration(rctx.Recent{Status: rctx.RecentStatusInterrupted}); got != "—" {
		t.Errorf("interrupted duration = %q, want — (em dash)", got)
	}
}

func TestRecentStatusText(t *testing.T) {
	tests := []struct {
		name string
		in   rctx.Recent
		want string
	}{
		{"completed ok", rctx.Recent{Status: rctx.RecentStatusCompleted, ExitCode: 0}, "ok"},
		{"completed fail", rctx.Recent{Status: rctx.RecentStatusCompleted, ExitCode: 1}, "failed"},
		{"interrupted", rctx.Recent{Status: rctx.RecentStatusInterrupted}, "interrupted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := recentStatusText(tt.in); got != tt.want {
				t.Errorf("recentStatusText = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWritePretty_recent(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile: "demo",
			Repos:   []rctx.Repo{},
			Recent: []rctx.Recent{
				{Command: "deploy", ExitCode: 0, DurationMs: 12300},
				{Command: "test", ExitCode: 1, DurationMs: 4500},
			},
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
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile:  "demo",
			Env:      "dev",
			Repos:    []rctx.Repo{{Name: "api", Path: "~/dev/api", Cloned: true, Branch: "main"}},
			Commands: []rctx.Command{{Name: "deploy", Description: "Deploy"}},
			Recent:   []rctx.Recent{{Command: "deploy", ExitCode: 0, DurationMs: 1234}},
		},
	}
	if err := writeJSON(&buf, ws); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	var decoded rctx.Snapshot
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(decoded.Workspace.Commands) != 1 || decoded.Workspace.Commands[0].Name != "deploy" {
		t.Errorf("commands round-trip failed: %+v", decoded.Workspace.Commands)
	}
	if len(decoded.Workspace.Recent) != 1 || decoded.Workspace.Recent[0].Command != "deploy" {
		t.Errorf("recent round-trip failed: %+v", decoded.Workspace.Recent)
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

func TestCollectTools_filtersOutHelpCompletionAndUserCommands(t *testing.T) {
	root := &cobra.Command{Use: "raid"}
	// Built-in raid subcommands.
	root.AddCommand(&cobra.Command{Use: "install", Short: "Install the active profile"})
	root.AddCommand(&cobra.Command{Use: "env", Short: "Execute an environment"})
	root.AddCommand(&cobra.Command{Use: "doctor", Short: "Check raid config"})
	// Cobra-internal commands that should be filtered out.
	root.AddCommand(&cobra.Command{Use: "help"})
	root.AddCommand(&cobra.Command{Use: "completion"})
	// User-defined command, mirrored on how registerUserCommands annotates them.
	root.AddCommand(&cobra.Command{
		Use:         "deploy",
		Short:       "Deploy to production",
		Annotations: map[string]string{cmdSourceAnnotation: cmdSourceUser},
	})

	tools := collectTools(root)

	gotNames := make([]string, len(tools))
	for i, tool := range tools {
		gotNames[i] = tool.Name
	}
	wantNames := []string{"doctor", "env", "install"} // sorted alphabetically
	if len(gotNames) != len(wantNames) {
		t.Fatalf("collectTools = %v, want %v", gotNames, wantNames)
	}
	for i, want := range wantNames {
		if gotNames[i] != want {
			t.Errorf("tools[%d] = %q, want %q (full = %v)", i, gotNames[i], want, gotNames)
		}
	}
}

func TestCollectTools_keepsCobraShortAsDescription(t *testing.T) {
	root := &cobra.Command{Use: "raid"}
	root.AddCommand(&cobra.Command{Use: "install", Short: "Install the active profile"})
	tools := collectTools(root)
	if len(tools) != 1 || tools[0].Description != "Install the active profile" {
		t.Errorf("description not propagated: %+v", tools)
	}
}

func TestWritePretty_includesToolsSection(t *testing.T) {
	var buf bytes.Buffer
	ws := rctx.Snapshot{
		Workspace: rctx.Workspace{
			Profile: "demo",
			Repos:   []rctx.Repo{},
		},
		Tools: []rctx.Tool{
			{Name: "install", Description: "Install the active profile"},
			{Name: "env", Description: "Execute an environment"},
		},
	}
	if err := writePretty(&buf, ws); err != nil {
		t.Fatalf("writePretty: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Tools (2)", "install", "Install the active profile", "env"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n--- output ---\n%s", want, out)
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
