// Snapshot the active raid workspace for agent / `raid context` consumption.
package context

import "github.com/8bitalex/raid/src/internal/lib"

// Snapshot is the full `raid context` payload — producer identity (name,
// version, websiteUrl, generatedAt) plus the inline Workspace state.
type Snapshot = lib.WorkspaceContext

// Workspace is the inline workspace state nested inside a Snapshot: profile,
// environment, repositories, user commands, and recent runs.
type Workspace = lib.Workspace

// Repo describes a single repository in the workspace snapshot.
type Repo = lib.WorkspaceRepo

// Command is a profile command, exposed as name + short description only.
type Command = lib.WorkspaceCommand

// Tool is a built-in `raid` subcommand exposed for agent discovery.
type Tool = lib.WorkspaceTool

// Recent describes a previously executed `raid <command>` invocation.
type Recent = lib.RecentEntry

// Recent status values surfaced via Workspace.Recent.
const (
	RecentStatusCompleted   = lib.RecentStatusCompleted
	RecentStatusInterrupted = lib.RecentStatusInterrupted
)

// Get returns a snapshot of the currently loaded workspace context.
func Get() Snapshot {
	return lib.GetWorkspaceContext()
}
