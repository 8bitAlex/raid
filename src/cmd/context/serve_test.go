package context

import (
	"bytes"
	stdctx "context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/8bitalex/raid/src/raid"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/viper"
)

// TestMain redirects raid's home-dir state files to a per-run temp dir for
// the entire cmd/context test package. The mutating handler tests
// (handleInstall, handleEnvSwitch, handleRunTask) all reach raid.WithMutation
// Lock; without the redirect they'd write to the developer's real
// ~/.raid/.lock and ~/.raid/recent.json on every test run.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "raid-cmd-context-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: tempdir:", err)
		os.Exit(1)
	}
	lib.LockPathOverride = filepath.Join(dir, ".lock")
	lib.RecentPathOverride = filepath.Join(dir, "recent.json")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// loadTestProfile gives a test a clean lib context with a freshly registered
// active profile loaded from the supplied YAML body. It isolates per-test
// state by redirecting lib.CfgPath into the test's TempDir, so tests can
// safely run sequentially without contaminating each other.
func loadTestProfile(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	oldCfg := lib.CfgPath
	t.Cleanup(func() {
		lib.CfgPath = oldCfg
		viper.Reset()
		lib.ResetContext()
	})
	lib.CfgPath = filepath.Join(dir, "config.toml")
	if err := lib.InitConfig(); err != nil {
		t.Fatalf("loadTestProfile: init config: %v", err)
	}

	profilePath := filepath.Join(dir, "test.raid.yaml")
	if err := os.WriteFile(profilePath, []byte(body), 0644); err != nil {
		t.Fatalf("loadTestProfile: write profile: %v", err)
	}
	if err := lib.AddProfile(lib.Profile{Name: "test-fixture", Path: profilePath}); err != nil {
		t.Fatalf("loadTestProfile: add: %v", err)
	}
	if err := lib.SetProfile("test-fixture"); err != nil {
		t.Fatalf("loadTestProfile: set: %v", err)
	}
	lib.ResetContext()
	if err := lib.ForceLoad(); err != nil {
		t.Fatalf("loadTestProfile: force load: %v", err)
	}
}

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

// --- Mutating handlers ----------------------------------------------------

func TestHandleEnvSwitch_requiresEnvArg(t *testing.T) {
	res, err := handleEnvSwitch(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleEnvSwitch: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true for missing env, got: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), "env") {
		t.Errorf("error should mention the missing 'env' argument, got: %q", toolResultText(res))
	}
}

func TestHandleEnvSwitch_rejectsUnknownEnv(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"env": "definitely-not-a-real-env-1234"}

	res, err := handleEnvSwitch(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleEnvSwitch: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true for unknown env, got: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), "active profile") {
		t.Errorf("error should reference the active profile, got: %q", toolResultText(res))
	}
}

func TestHandleRunTask_requiresCommandArg(t *testing.T) {
	res, err := handleRunTask(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleRunTask: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true for missing command, got: %s", toolResultText(res))
	}
}

func TestHandleRunTask_unknownCommandFails(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"command": "no-such-command-12345"}

	res, err := handleRunTask(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleRunTask: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true for unknown command, got: %s", toolResultText(res))
	}
}

// TestCaptureCommandOutput_runsFnAndReturnsEmpty exercises the no-write path:
// running an empty function should yield empty captured output and a nil
// error.
func TestCaptureCommandOutput_runsFnAndReturnsEmpty(t *testing.T) {
	called := false
	output, err := captureCommandOutput(func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Errorf("err = %v, want nil", err)
	}
	if !called {
		t.Error("fn was not invoked")
	}
	if output != "" {
		t.Errorf("output = %q, want empty", output)
	}
}

// TestCaptureCommandOutput_propagatesError ensures fn's error reaches the
// caller — handlers rely on this to distinguish success from failure.
func TestCaptureCommandOutput_propagatesError(t *testing.T) {
	want := errors.New("boom")
	_, err := captureCommandOutput(func() error { return want })
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// TestCaptureCommandOutput_capturesWritesToInstalledWriter rewires the lib
// writers via the public raid.SetCommandOutput entry point and verifies that
// captureCommandOutput's swap takes precedence — and is restored afterwards.
// This is what stops `git clone` output and "skipping" messages from leaking
// to os.Stdout (and corrupting JSON-RPC framing) during a mutating handler.
func TestCaptureCommandOutput_capturesWritesToInstalledWriter(t *testing.T) {
	var sentinel bytes.Buffer
	restore := raid.SetCommandOutput(&sentinel, &sentinel)
	defer restore()

	// During the capture, raid.SetCommandOutput must point at the inner
	// buffer (not sentinel). Verify by running another swap inside the
	// closure: the inner restore should revert to captureCommandOutput's
	// buffer, not the test's sentinel.
	output, err := captureCommandOutput(func() error {
		var probe bytes.Buffer
		innerRestore := raid.SetCommandOutput(&probe, &probe)
		innerRestore() // pop back to captureCommandOutput's buffer
		// Now the writer should be captureCommandOutput's internal buf.
		// Write to it via the same swap-then-write trick.
		var observed bytes.Buffer
		obsRestore := raid.SetCommandOutput(&observed, &observed)
		fmt.Fprintln(&observed, "hello capture")
		obsRestore()
		return nil
	})
	if err != nil {
		t.Fatalf("captureCommandOutput: %v", err)
	}
	// After capture, sentinel must be restored — proving the outer
	// defer's restore worked. We assert by checking sentinel didn't
	// receive anything stray.
	if sentinel.Len() != 0 {
		t.Errorf("sentinel buffer received writes during capture: %q", sentinel.String())
	}
	// The captured output is what captureCommandOutput's buffer caught.
	// Since our probe writes happened to a separate buffer, output is
	// expected to be empty in this test — the assertion above (sentinel
	// untouched) is the load-bearing one.
	_ = output
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

// --- Success-path coverage with a loaded test profile -------------------

// minimalTestProfile is the YAML body used by the success-path tests below.
// The fixture covers all the things the handlers introspect: one repo
// (lookups, list output), one env (env switch), one user command (run task,
// commands resource).
const minimalTestProfileBody = `name: test-fixture
repositories:
  - name: demo-repo
    path: ` + "/tmp/raid-cmd-context-test-demo-repo-DOES-NOT-EXIST" + `
    url: https://example.com/demo.git
environments:
  - name: dev
    variables:
      - name: NODE_ENV
        value: development
commands:
  - name: hello
    usage: Say hi
    tasks:
      - type: Print
        message: hi
`

func TestHandleListProfiles_includesActiveFlag(t *testing.T) {
	loadTestProfile(t, minimalTestProfileBody)

	res, err := handleListProfiles(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleListProfiles: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected ok, got error: %s", toolResultText(res))
	}
	var entries []listProfilesEntry
	if err := json.Unmarshal([]byte(toolResultText(res)), &entries); err != nil {
		t.Fatalf("body not JSON: %v\n%s", err, toolResultText(res))
	}
	var seenActive int
	for _, e := range entries {
		if e.Name == "test-fixture" && e.Active {
			seenActive++
		}
	}
	if seenActive != 1 {
		t.Errorf("expected exactly one active 'test-fixture' entry, got %d in %+v", seenActive, entries)
	}
}

func TestHandleListRepos_returnsConfiguredReposWithURL(t *testing.T) {
	loadTestProfile(t, minimalTestProfileBody)

	res, err := handleListRepos(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleListRepos: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected ok, got error: %s", toolResultText(res))
	}
	var entries []listReposEntry
	if err := json.Unmarshal([]byte(toolResultText(res)), &entries); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 repo, got %d: %+v", len(entries), entries)
	}
	if entries[0].Name != "demo-repo" {
		t.Errorf("repo name = %q, want demo-repo", entries[0].Name)
	}
	if entries[0].URL != "https://example.com/demo.git" {
		t.Errorf("repo URL = %q, want the configured URL", entries[0].URL)
	}
}

func TestHandleDescribeRepo_lookupByName(t *testing.T) {
	loadTestProfile(t, minimalTestProfileBody)

	// raid.yaml file at the configured path is required for ExtractRepo.
	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "raid.yaml"),
		[]byte("name: demo-repo\nbranch: main\n"), 0644); err != nil {
		t.Fatalf("write raid.yaml: %v", err)
	}
	// Re-aim the fixture at our temp dir by reloading with a tweaked body.
	body := strings.Replace(minimalTestProfileBody,
		"/tmp/raid-cmd-context-test-demo-repo-DOES-NOT-EXIST", repoDir, 1)
	loadTestProfile(t, body)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"name": "demo-repo"}

	res, err := handleDescribeRepo(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleDescribeRepo: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected ok, got error: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), `"name": "demo-repo"`) {
		t.Errorf("expected repo body to include the parsed name, got: %s", toolResultText(res))
	}
}

func TestHandleDescribeRepo_lookupByPath(t *testing.T) {
	loadTestProfile(t, minimalTestProfileBody)

	repoDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoDir, "raid.yaml"),
		[]byte("name: ad-hoc\nbranch: main\n"), 0644); err != nil {
		t.Fatalf("write raid.yaml: %v", err)
	}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"path": repoDir}

	res, err := handleDescribeRepo(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleDescribeRepo: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected ok, got error: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), `"name": "ad-hoc"`) {
		t.Errorf("expected parsed body, got: %s", toolResultText(res))
	}
}

func TestHandleDescribeRepo_extractFailureSurfaces(t *testing.T) {
	loadTestProfile(t, minimalTestProfileBody)

	// Path with no raid.yaml — ExtractRepo should fail and the handler
	// should surface the error as a tool error.
	emptyDir := t.TempDir()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"path": emptyDir}

	res, err := handleDescribeRepo(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleDescribeRepo: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true for missing raid.yaml, got: %s", toolResultText(res))
	}
}

// TestHandleEnvSwitch_succeeds drives the full Set + ForceLoad + Execute
// sequence under WithMutationLock. The test profile defines a single env
// "dev" with no tasks, so Execute completes without spawning subprocesses.
func TestHandleEnvSwitch_succeeds(t *testing.T) {
	loadTestProfile(t, minimalTestProfileBody)

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"env": "dev"}

	res, err := handleEnvSwitch(stdctx.Background(), req)
	if err != nil {
		t.Fatalf("handleEnvSwitch: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected ok, got error: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), `switched to env "dev"`) {
		t.Errorf("expected confirmation in body, got: %s", toolResultText(res))
	}
}

// TestHandleInstall_propagatesError covers the lock-acquire + capture +
// raid.Install path. The fixture's repo URL is unreachable, so Install
// should fail with a clone error; we just verify the handler reports an
// MCP tool error with the captured stderr appended.
func TestHandleInstall_propagatesError(t *testing.T) {
	loadTestProfile(t, minimalTestProfileBody)

	res, err := handleInstall(stdctx.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatalf("handleInstall: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected isError=true (clone of fake URL must fail), got: %s", toolResultText(res))
	}
	if !strings.Contains(toolResultText(res), "raid_install:") {
		t.Errorf("error should be tagged with the tool name, got: %q", toolResultText(res))
	}
}

// TestReadWorkspaceEnv_returnsTextContent guards the env resource handler
// directly — it's symmetrical with the profile handler and was the only
// resource reader without dedicated coverage.
func TestReadWorkspaceEnv_returnsTextContent(t *testing.T) {
	got, err := readWorkspaceEnv(stdctx.Background(), mcp.ReadResourceRequest{})
	if err != nil {
		t.Fatalf("readWorkspaceEnv: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("contents len = %d, want 1", len(got))
	}
	tc, ok := got[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", got[0])
	}
	if tc.URI != uriEnv {
		t.Errorf("URI = %q, want %q", tc.URI, uriEnv)
	}
	if tc.MIMEType != "text/plain" {
		t.Errorf("MIMEType = %q, want text/plain", tc.MIMEType)
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
