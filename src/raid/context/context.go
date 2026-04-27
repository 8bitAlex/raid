// Snapshot the active raid workspace for agent / `raid context` consumption.
package context

import "github.com/8bitalex/raid/src/internal/lib"

// Workspace is a snapshot of the active profile, environment, and repositories.
type Workspace = lib.WorkspaceContext

// Repo describes a single repository in the workspace snapshot.
type Repo = lib.WorkspaceRepo

// Get returns a snapshot of the currently loaded workspace context.
func Get() Workspace {
	return lib.GetWorkspaceContext()
}
