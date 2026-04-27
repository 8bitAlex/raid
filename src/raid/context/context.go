// Snapshot the active raid workspace for agent / `raid context` consumption.
package context

import "github.com/8bitalex/raid/src/internal/lib"

// Workspace is a snapshot of the active profile, environment, and repositories.
type Workspace = lib.WorkspaceContext

// Repo describes a single repository in the workspace snapshot.
type Repo = lib.WorkspaceRepo

// Command is a profile command, exposed as name + short description only.
type Command = lib.WorkspaceCommand

// Recent describes a previously executed `raid <command>` invocation.
type Recent = lib.RecentEntry

// Recent status values surfaced via Workspace.Recent.
const (
	RecentStatusCompleted   = lib.RecentStatusCompleted
	RecentStatusInterrupted = lib.RecentStatusInterrupted
)

// Get returns a snapshot of the currently loaded workspace context.
func Get() Workspace {
	return lib.GetWorkspaceContext()
}
