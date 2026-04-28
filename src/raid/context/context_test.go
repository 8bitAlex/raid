package context

import "testing"

// Get is a thin wrapper around lib.GetWorkspaceContext, which is fully
// exercised in src/internal/lib/context_test.go. This test just guards the
// wiring so the wrapper keeps returning a usable Snapshot value.
func TestGet_returnsSnapshot(t *testing.T) {
	ws := Get()
	if ws.Workspace.Repos == nil {
		t.Errorf("Get().Workspace.Repos = nil, want non-nil slice")
	}
}
