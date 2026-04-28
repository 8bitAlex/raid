package context

import (
	stdctx "context"
	"encoding/json"
	"fmt"
	"sort"

	rctx "github.com/8bitalex/raid/src/raid/context"
	"github.com/8bitalex/raid/src/raid/profile"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

// Workspace resource URIs. The same URIs are advertised in the static `raid
// context --json` snapshot's resources catalog (step 2 of the MCP work) so the
// snapshot and the live server agree on names.
const (
	uriProfile  = "raid://workspace/profile"
	uriEnv      = "raid://workspace/env"
	uriRepos    = "raid://workspace/repos"
	uriCommands = "raid://workspace/commands"
	uriRecent   = "raid://workspace/recent"
)

// serveStdioFn is the stdio entry point. Overridable in tests so we can
// exercise BuildServer without actually consuming stdin/stdout.
var serveStdioFn = func(s *server.MCPServer) error { return server.ServeStdio(s) }

func init() {
	Command.AddCommand(ServeCmd)
}

// ServeCmd runs an MCP server over stdio, exposing the active raid workspace
// as resources and the canonical raid agent toolkit as tools. It's the
// long-running counterpart to `raid context --json` — same shape, served live.
//
// Tool handlers are stubs in this first slice; they return a "not yet
// implemented" error so a host (Claude Code, Cursor, …) can still discover
// the surface area while wiring proceeds in follow-up work.
var ServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run an MCP server exposing the active workspace over stdio",
	Long:  "Start a Model Context Protocol server (stdio transport) that exposes the active raid workspace as resources and the raid agent toolkit as tools. Designed to be wired into MCP-aware clients such as Claude Code or Cursor.",
	Args:  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		return serveStdioFn(BuildServer())
	},
}

// BuildServer wires up the MCP server with the workspace resources and the
// raid agent tool catalog. Exported so tests can introspect the assembled
// server without driving it through stdio.
func BuildServer() *server.MCPServer {
	s := server.NewMCPServer(
		"raid",
		serverVersion(),
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
	)

	registerResources(s)
	registerTools(s)
	return s
}

func serverVersion() string {
	// rctx.Get() already pulls the version from app.properties via lib.
	// Calling it here means we don't need to thread the resources package
	// through cmd/context just for one string.
	return rctx.Get().Version
}

// --- Resources ----------------------------------------------------------

func registerResources(s *server.MCPServer) {
	s.AddResource(
		mcp.NewResource(uriProfile, "profile",
			mcp.WithResourceDescription("Name of the active raid profile"),
			mcp.WithMIMEType("text/plain"),
		),
		readWorkspaceProfile,
	)
	s.AddResource(
		mcp.NewResource(uriEnv, "env",
			mcp.WithResourceDescription("Name of the active environment"),
			mcp.WithMIMEType("text/plain"),
		),
		readWorkspaceEnv,
	)
	s.AddResource(
		mcp.NewResource(uriRepos, "repos",
			mcp.WithResourceDescription("Repositories in the active profile with current git state"),
			mcp.WithMIMEType("application/json"),
		),
		readWorkspaceRepos,
	)
	s.AddResource(
		mcp.NewResource(uriCommands, "commands",
			mcp.WithResourceDescription("User-defined raid commands available in the active profile"),
			mcp.WithMIMEType("application/json"),
		),
		readWorkspaceCommands,
	)
	s.AddResource(
		mcp.NewResource(uriRecent, "recent",
			mcp.WithResourceDescription("Recent raid command invocations (capped at 10)"),
			mcp.WithMIMEType("application/json"),
		),
		readWorkspaceRecent,
	)
}

func readWorkspaceProfile(_ stdctx.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return textResource(uriProfile, "text/plain", rctx.Get().Workspace.Profile), nil
}

func readWorkspaceEnv(_ stdctx.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return textResource(uriEnv, "text/plain", rctx.Get().Workspace.Env), nil
}

func readWorkspaceRepos(_ stdctx.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return jsonResource(uriRepos, rctx.Get().Workspace.Repos)
}

func readWorkspaceCommands(_ stdctx.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return jsonResource(uriCommands, rctx.Get().Workspace.Commands)
}

func readWorkspaceRecent(_ stdctx.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	return jsonResource(uriRecent, rctx.Get().Workspace.Recent)
}

func textResource(uri, mime, text string) []mcp.ResourceContents {
	return []mcp.ResourceContents{mcp.TextResourceContents{URI: uri, MIMEType: mime, Text: text}}
}

func jsonResource(uri string, payload any) ([]mcp.ResourceContents, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("serializing %s: %w", uri, err)
	}
	return []mcp.ResourceContents{mcp.TextResourceContents{
		URI:      uri,
		MIMEType: "application/json",
		Text:     string(data),
	}}, nil
}

// --- Tools --------------------------------------------------------------

// agentToolDef carries the metadata needed to register one of raid's MCP
// tools. The handler is a stub for now; round 2 will swap in real adapters
// over the existing lib.* business logic.
type agentToolDef struct {
	tool    mcp.Tool
	handler server.ToolHandlerFunc
}

func registerTools(s *server.MCPServer) {
	for _, def := range agentToolDefs() {
		s.AddTool(def.tool, def.handler)
	}
}

// agentToolDefs returns the canonical raid agent toolkit. Names match issue
// #45 so the toolkit stays stable even as the implementations land.
//
// Read-only tools (list_profiles, list_repos, describe_repo) are implemented
// here; mutating tools (install, env_switch, run_task) are still stubs that
// return an MCP tool error indicating the call hasn't been implemented yet.
// Tool errors flow back to the model for self-correction; protocol errors
// don't, which is why the stubs return a successful result-with-isError
// rather than `error` from the handler.
func agentToolDefs() []agentToolDef {
	return []agentToolDef{
		{
			tool: mcp.NewTool("raid_list_profiles",
				mcp.WithDescription("List configured raid profiles. Each entry includes the profile's name, the path to its YAML file, and whether it's the currently active profile."),
			),
			handler: handleListProfiles,
		},
		{
			tool: mcp.NewTool("raid_list_repos",
				mcp.WithDescription("List repositories in the active profile with their configured URL and live git state (current branch, dirty status, whether cloned). The optional 'profile' argument is reserved; only the active profile is supported in this release."),
				mcp.WithString("profile", mcp.Description("Profile name. Defaults to the active profile when omitted. Currently must match the active profile.")),
			),
			handler: handleListRepos,
		},
		{
			tool: mcp.NewTool("raid_describe_repo",
				mcp.WithDescription("Return the parsed raid.yaml for a repository as structured JSON — environments, install tasks, and commands. Look up by 'name' (resolved against the active profile) or by 'path' (a filesystem directory containing raid.yaml). Exactly one is required."),
				mcp.WithString("name", mcp.Description("Repository name from the active profile.")),
				mcp.WithString("path", mcp.Description("Filesystem path to the repository directory.")),
			),
			handler: handleDescribeRepo,
		},
		{
			tool: mcp.NewTool("raid_install",
				mcp.WithDescription("Clone repositories and run install tasks. Targets the whole active profile by default."),
				mcp.WithString("repo", mcp.Description("Limit installation to this repo by name. Omit to install all repos in the profile.")),
			),
			handler: notImplemented("raid_install"),
		},
		{
			tool: mcp.NewTool("raid_env_switch",
				mcp.WithDescription("Switch the active environment, writing per-repo .env files and running environment tasks."),
				mcp.WithString("env", mcp.Required(), mcp.Description("Environment name to activate.")),
			),
			handler: notImplemented("raid_env_switch"),
		},
		{
			tool: mcp.NewTool("raid_run_task",
				mcp.WithDescription("Run a user-defined raid command (`raid <command>`) from the active profile."),
				mcp.WithString("command", mcp.Required(), mcp.Description("Command name as exposed by `raid context`'s commands list.")),
				mcp.WithArray("args", mcp.Description("Positional arguments passed to the command.")),
			),
			handler: notImplemented("raid_run_task"),
		},
	}
}

// --- Read-only handlers --------------------------------------------------

// listProfilesEntry is the JSON shape returned by raid_list_profiles. Defined
// here (not in lib) because it's specific to this MCP tool's contract.
type listProfilesEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Active bool   `json:"active"`
}

func handleListProfiles(_ stdctx.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	all := profile.ListAll()
	active := profile.Get().Name

	out := make([]listProfilesEntry, 0, len(all))
	for _, p := range all {
		out = append(out, listProfilesEntry{
			Name:   p.Name,
			Path:   p.Path,
			Active: p.Name == active,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return jsonToolResult(out)
}

// listReposEntry merges the workspace snapshot's live git state with each
// repo's configured URL — agents need both to clone or follow up.
type listReposEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	URL    string `json:"url,omitempty"`
	Cloned bool   `json:"cloned"`
	Branch string `json:"branch,omitempty"`
	Dirty  bool   `json:"dirty,omitempty"`
}

func handleListRepos(_ stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	requested := req.GetString("profile", "")
	active := profile.Get()
	if requested != "" && requested != active.Name {
		return mcp.NewToolResultError(fmt.Sprintf(
			"raid_list_repos: only the active profile (%q) is supported in this release; got %q",
			active.Name, requested,
		)), nil
	}

	urls := make(map[string]string, len(active.Repositories))
	for _, r := range active.Repositories {
		urls[r.Name] = r.URL
	}

	live := rctx.Get().Workspace.Repos
	out := make([]listReposEntry, 0, len(live))
	for _, r := range live {
		out = append(out, listReposEntry{
			Name:   r.Name,
			Path:   r.Path,
			URL:    urls[r.Name],
			Cloned: r.Cloned,
			Branch: r.Branch,
			Dirty:  r.Dirty,
		})
	}
	return jsonToolResult(out)
}

func handleDescribeRepo(_ stdctx.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := req.GetString("name", "")
	path := req.GetString("path", "")
	if name == "" && path == "" {
		return mcp.NewToolResultError("raid_describe_repo: one of 'name' or 'path' is required"), nil
	}
	if name != "" && path != "" {
		return mcp.NewToolResultError("raid_describe_repo: provide either 'name' or 'path', not both"), nil
	}

	target := path
	if name != "" {
		active := profile.Get()
		var found *profile.Repo
		for i := range active.Repositories {
			if active.Repositories[i].Name == name {
				found = &active.Repositories[i]
				break
			}
		}
		if found == nil {
			return mcp.NewToolResultError(fmt.Sprintf(
				"raid_describe_repo: repo %q not found in active profile %q",
				name, active.Name,
			)), nil
		}
		target = found.Path
	}

	repo, err := profile.Describe(target)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("raid_describe_repo: %v", err)), nil
	}
	return jsonToolResult(repo)
}

// jsonToolResult marshals payload to indented JSON and wraps it as an MCP
// text-content tool result. Marshal failures produce a tool error rather than
// a protocol error so the model can surface them.
func jsonToolResult(payload any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("internal: marshal: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func notImplemented(name string) server.ToolHandlerFunc {
	return func(_ stdctx.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError(fmt.Sprintf("%s is not yet implemented; see https://github.com/8bitalex/raid/issues/45", name)), nil
	}
}
