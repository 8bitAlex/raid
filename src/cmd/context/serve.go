package context

import (
	stdctx "context"
	"encoding/json"
	"fmt"

	rctx "github.com/8bitalex/raid/src/raid/context"
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
// #45 so the toolkit stays stable even as the implementations land. Each
// stub handler returns an MCP tool error indicating the call hasn't been
// implemented yet — distinct from a protocol error so MCP hosts can surface
// it to the model for self-correction.
func agentToolDefs() []agentToolDef {
	return []agentToolDef{
		{
			tool: mcp.NewTool("raid_list_profiles",
				mcp.WithDescription("List configured raid profiles."),
			),
			handler: notImplemented("raid_list_profiles"),
		},
		{
			tool: mcp.NewTool("raid_list_repos",
				mcp.WithDescription("List repositories in the active profile (or a named profile)."),
				mcp.WithString("profile", mcp.Description("Profile name. Defaults to the active profile when omitted.")),
			),
			handler: notImplemented("raid_list_repos"),
		},
		{
			tool: mcp.NewTool("raid_describe_repo",
				mcp.WithDescription("Return the parsed raid.yaml for a repository as structured JSON."),
				mcp.WithString("name", mcp.Description("Repository name from the active profile. Either name or path is required.")),
				mcp.WithString("path", mcp.Description("Filesystem path to the repository. Either name or path is required.")),
			),
			handler: notImplemented("raid_describe_repo"),
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

func notImplemented(name string) server.ToolHandlerFunc {
	return func(_ stdctx.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultError(fmt.Sprintf("%s is not yet implemented; see https://github.com/8bitalex/raid/issues/45", name)), nil
	}
}
