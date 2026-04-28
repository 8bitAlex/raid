package context

import (
	stdctx "context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestBuildServer_returnsConfiguredServer(t *testing.T) {
	s := BuildServer()
	if s == nil {
		t.Fatal("BuildServer returned nil")
	}
}

func TestServeCmd_isWired(t *testing.T) {
	if ServeCmd == nil {
		t.Fatal("ServeCmd is nil")
	}
	if ServeCmd.Use != "serve" {
		t.Errorf("Use = %q, want %q", ServeCmd.Use, "serve")
	}
	// Confirm registered as a child of the parent context command.
	var found bool
	for _, sub := range Command.Commands() {
		if sub == ServeCmd {
			found = true
			break
		}
	}
	if !found {
		t.Error("ServeCmd is not registered as a subcommand of context.Command")
	}
}

// --- Resource handlers ----------------------------------------------------

func TestReadWorkspaceProfile_returnsTextContent(t *testing.T) {
	got, err := readWorkspaceProfile(stdctx.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("readWorkspaceProfile: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("contents len = %d, want 1", len(got))
	}
	tc, ok := got[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", got[0])
	}
	if tc.URI != uriProfile {
		t.Errorf("URI = %q, want %q", tc.URI, uriProfile)
	}
	if tc.MIMEType != "text/plain" {
		t.Errorf("MIMEType = %q, want text/plain", tc.MIMEType)
	}
}

func TestReadWorkspaceRepos_returnsValidJSON(t *testing.T) {
	got, err := readWorkspaceRepos(stdctx.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("readWorkspaceRepos: %v", err)
	}
	tc := got[0].(mcp.TextResourceContents)
	if tc.MIMEType != "application/json" {
		t.Errorf("MIMEType = %q, want application/json", tc.MIMEType)
	}
	// The body must be parseable JSON regardless of whether the workspace
	// has repos right now.
	var parsed any
	if err := json.Unmarshal([]byte(tc.Text), &parsed); err != nil {
		t.Errorf("body is not valid JSON: %v\n%s", err, tc.Text)
	}
}

func TestReadWorkspaceCommands_returnsValidJSON(t *testing.T) {
	got, err := readWorkspaceCommands(stdctx.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("readWorkspaceCommands: %v", err)
	}
	tc := got[0].(mcp.TextResourceContents)
	var parsed any
	if err := json.Unmarshal([]byte(tc.Text), &parsed); err != nil {
		t.Errorf("body is not valid JSON: %v\n%s", err, tc.Text)
	}
}

func TestReadWorkspaceRecent_returnsValidJSON(t *testing.T) {
	got, err := readWorkspaceRecent(stdctx.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("readWorkspaceRecent: %v", err)
	}
	tc := got[0].(mcp.TextResourceContents)
	var parsed any
	if err := json.Unmarshal([]byte(tc.Text), &parsed); err != nil {
		t.Errorf("body is not valid JSON: %v\n%s", err, tc.Text)
	}
}

// --- Tool catalog ---------------------------------------------------------

// TestAgentToolDefs_namesMatchIssue45 guards the canonical raid agent tool
// names from issue #45. If a future change renames one of these, the snapshot
// docs and any host-side configs would also need updating.
func TestAgentToolDefs_namesMatchIssue45(t *testing.T) {
	defs := agentToolDefs()
	want := map[string]bool{
		"raid_list_profiles": false,
		"raid_list_repos":    false,
		"raid_describe_repo": false,
		"raid_install":       false,
		"raid_env_switch":    false,
		"raid_run_task":      false,
	}
	for _, d := range defs {
		if _, ok := want[d.tool.Name]; !ok {
			t.Errorf("unexpected tool %q in catalog", d.tool.Name)
			continue
		}
		want[d.tool.Name] = true
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("tool %q missing from agentToolDefs()", name)
		}
	}
}

// TestAgentToolDefs_haveDescriptions guards that every tool ships with a
// human-readable description so the MCP host can render it usefully. The MCP
// spec doesn't make description strictly required but a tool with no
// description gives the model nothing to work with.
func TestAgentToolDefs_haveDescriptions(t *testing.T) {
	for _, d := range agentToolDefs() {
		if d.tool.Description == "" {
			t.Errorf("tool %q has empty description", d.tool.Name)
		}
	}
}

// --- Read-only handlers --------------------------------------------------

func TestHandleListProfiles_returnsValidJSON(t *testing.T) {
	res, err := handleListProfiles(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleListProfiles: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected ok result, got error: %s", toolResultText(res))
	}
	var entries []listProfilesEntry
	if err := json.Unmarshal([]byte(toolResultText(res)), &entries); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, toolResultText(res))
	}
	// Sort invariant: alphabetical by name.
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Name > entries[i].Name {
			t.Errorf("entries not sorted: %q before %q", entries[i-1].Name, entries[i].Name)
		}
	}
	// At most one entry should be marked active.
	active := 0
	for _, e := range entries {
		if e.Active {
			active++
		}
	}
	if active > 1 {
		t.Errorf("multiple entries marked active (%d)", active)
	}
}

func TestHandleListRepos_rejectsForeignProfileArg(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"profile": "definitely-not-the-active-one"}

	res, err := handleListRepos(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleListRepos: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true for foreign profile arg, got: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), "active profile") {
		t.Errorf("error should mention the active-profile limitation, got: %q", toolResultText(res))
	}
}

func TestHandleListRepos_returnsValidJSONForActiveProfile(t *testing.T) {
	res, err := handleListRepos(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleListRepos: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected ok result, got error: %s", toolResultText(res))
	}
	var entries []listReposEntry
	if err := json.Unmarshal([]byte(toolResultText(res)), &entries); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, toolResultText(res))
	}
}

func TestHandleDescribeRepo_requiresNameOrPath(t *testing.T) {
	res, err := handleDescribeRepo(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleDescribeRepo: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true, got: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), "one of") {
		t.Errorf("error should explain that one of name/path is required, got: %q", toolResultText(res))
	}
}

func TestHandleDescribeRepo_rejectsBothNameAndPath(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "some-repo", "path": "/tmp/x"}

	res, err := handleDescribeRepo(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleDescribeRepo: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true when both args set, got: %s", toolResultText(res))
	}
}

func TestHandleDescribeRepo_unknownNameReportsActiveProfile(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "definitely-not-a-real-repo-1234"}

	res, err := handleDescribeRepo(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleDescribeRepo: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true for unknown repo, got: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), "active profile") {
		t.Errorf("error should reference the active profile so the agent knows where the lookup happened, got: %q", toolResultText(res))
	}
}

// TestNotImplementedHandler_returnsToolError confirms the stub handler
// surfaces as a tool execution error (isError=true) rather than a protocol
// error. The MCP spec treats these differently: tool errors flow back to the
// model for self-correction; protocol errors don't.
func TestNotImplementedHandler_returnsToolError(t *testing.T) {
	h := notImplemented("raid_demo")
	res, err := h(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handler returned protocol error: %v (should be a tool error instead)", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected isError=true tool result, got: %+v", res)
	}
	// Verify the body mentions the tool by name + the tracking issue, so a
	// model picking up the error has somewhere to follow up.
	body := toolResultText(res)
	if !strings.Contains(body, "raid_demo") {
		t.Errorf("error body should name the tool, got: %q", body)
	}
	if !strings.Contains(body, "issues/45") {
		t.Errorf("error body should reference the tracking issue, got: %q", body)
	}
}

// toolResultText pulls the first text content block out of a tool result.
func toolResultText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// TestServeCmd_runDelegatesToServeStdioFn verifies the cobra RunE wiring
// reaches the stdio entry point. The real entry blocks on stdin; the
// overridable serveStdioFn lets us capture the call instead.
func TestServeCmd_runDelegatesToServeStdioFn(t *testing.T) {
	old := serveStdioFn
	t.Cleanup(func() { serveStdioFn = old })

	called := false
	serveStdioFn = func(s *server.MCPServer) error {
		called = true
		if s == nil {
			t.Error("BuildServer produced a nil server")
		}
		return nil
	}

	if err := ServeCmd.RunE(ServeCmd, nil); err != nil {
		t.Fatalf("ServeCmd.RunE: %v", err)
	}
	if !called {
		t.Error("serveStdioFn was not invoked")
	}
}
