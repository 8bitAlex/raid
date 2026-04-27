package context

import "testing"

// Get is a thin wrapper around lib.GetWorkspaceContext, which is fully
// exercised in src/internal/lib/context_test.go. This test just guards the
// wiring so the wrapper keeps returning a usable Workspace value.
func TestGet_returnsWorkspace(t *testing.T) {
	ws := Get()
	if ws.Repos == nil {
		t.Errorf("Get().Repos = nil, want non-nil slice")
	}
}
