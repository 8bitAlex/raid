package lib

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func resetWorkspaceContextState(t *testing.T) {
	t.Helper()
	prevCtx := context
	prevGit := runGitFn
	t.Cleanup(func() {
		context = prevCtx
		runGitFn = prevGit
	})
	context = nil
}

// makeFakeGitDir creates a directory with a `.git` subdirectory so isGitRepository returns true.
func makeFakeGitDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("makeFakeGitDir: %v", err)
	}
	return dir
}

func TestGetWorkspaceContext_noContext(t *testing.T) {
	resetWorkspaceContextState(t)

	got := GetWorkspaceContext()
	if got.Profile != "" {
		t.Errorf("Profile = %q, want empty", got.Profile)
	}
	if got.Env != "" {
		t.Errorf("Env = %q, want empty", got.Env)
	}
	if len(got.Repos) != 0 {
		t.Errorf("Repos len = %d, want 0", len(got.Repos))
	}
}

func TestGetWorkspaceContext_zeroProfile(t *testing.T) {
	resetWorkspaceContextState(t)
	context = &Context{Env: "dev"}

	got := GetWorkspaceContext()
	if got.Profile != "" {
		t.Errorf("Profile = %q, want empty", got.Profile)
	}
	if got.Env != "dev" {
		t.Errorf("Env = %q, want %q", got.Env, "dev")
	}
	if len(got.Repos) != 0 {
		t.Errorf("Repos len = %d, want 0", len(got.Repos))
	}
	if len(got.Commands) != 0 {
		t.Errorf("Commands len = %d, want 0", len(got.Commands))
	}
}

func TestGetWorkspaceContext_includesCommands(t *testing.T) {
	resetWorkspaceContextState(t)
	context = &Context{
		Profile: Profile{
			Name: "demo",
			Path: "/tmp/demo.raid.yaml",
			Commands: []Command{
				{Name: "deploy", Usage: "Deploy to production"},
				{Name: "test", Usage: "Run tests"},
			},
		},
	}

	got := GetWorkspaceContext()
	if len(got.Commands) != 2 {
		t.Fatalf("Commands len = %d, want 2", len(got.Commands))
	}
	if got.Commands[0].Name != "deploy" || got.Commands[0].Description != "Deploy to production" {
		t.Errorf("Commands[0] = %+v", got.Commands[0])
	}
}

func TestGetWorkspaceContext_includesRecent(t *testing.T) {
	resetWorkspaceContextState(t)
	setupRecentTempPath(t)

	RecordRecent("deploy", nil, time.Now())
	context = &Context{
		Profile: Profile{Name: "demo", Path: "/tmp/demo.raid.yaml"},
	}

	got := GetWorkspaceContext()
	if len(got.Recent) != 1 || got.Recent[0].Command != "deploy" {
		t.Errorf("Recent = %+v, want one 'deploy' entry", got.Recent)
	}
}

func TestGetWorkspaceContext_repoNotCloned(t *testing.T) {
	resetWorkspaceContextState(t)
	context = &Context{
		Env: "dev",
		Profile: Profile{
			Name: "demo",
			Path: "/tmp/demo.raid.yaml",
			Repositories: []Repo{
				{Name: "missing", Path: "/nonexistent/path/abc123", URL: "https://example.com/r.git"},
			},
		},
	}

	got := GetWorkspaceContext()
	if got.Profile != "demo" {
		t.Errorf("Profile = %q, want %q", got.Profile, "demo")
	}
	if len(got.Repos) != 1 {
		t.Fatalf("Repos len = %d, want 1", len(got.Repos))
	}
	r := got.Repos[0]
	if r.Cloned {
		t.Errorf("Cloned = true, want false")
	}
	if r.Branch != "" {
		t.Errorf("Branch = %q, want empty", r.Branch)
	}
	if r.Dirty {
		t.Errorf("Dirty = true, want false")
	}
}

func TestGetWorkspaceContext_repoClean(t *testing.T) {
	resetWorkspaceContextState(t)
	dir := makeFakeGitDir(t)

	gitCalls := 0
	runGitFn = func(_ string, args ...string) (string, error) {
		gitCalls++
		switch args[0] {
		case "rev-parse":
			return "main", nil
		case "status":
			return "", nil
		}
		t.Fatalf("unexpected git call: %v", args)
		return "", nil
	}

	context = &Context{
		Env: "dev",
		Profile: Profile{
			Name: "demo",
			Path: "/tmp/demo.raid.yaml",
			Repositories: []Repo{
				{Name: "api", Path: dir, URL: "https://example.com/api.git"},
			},
		},
	}

	got := GetWorkspaceContext()
	if len(got.Repos) != 1 {
		t.Fatalf("Repos len = %d, want 1", len(got.Repos))
	}
	r := got.Repos[0]
	if !r.Cloned {
		t.Errorf("Cloned = false, want true")
	}
	if r.Branch != "main" {
		t.Errorf("Branch = %q, want %q", r.Branch, "main")
	}
	if r.Dirty {
		t.Errorf("Dirty = true, want false")
	}
	if gitCalls != 2 {
		t.Errorf("git calls = %d, want 2", gitCalls)
	}
}

func TestGetWorkspaceContext_repoDirty(t *testing.T) {
	resetWorkspaceContextState(t)
	dir := makeFakeGitDir(t)

	runGitFn = func(_ string, args ...string) (string, error) {
		switch args[0] {
		case "rev-parse":
			return "feature/x", nil
		case "status":
			return " M README.md\n?? new.txt\n", nil
		}
		return "", nil
	}

	context = &Context{
		Profile: Profile{
			Name: "demo",
			Path: "/tmp/demo.raid.yaml",
			Repositories: []Repo{
				{Name: "api", Path: dir, URL: "https://example.com/api.git"},
			},
		},
	}

	got := GetWorkspaceContext()
	r := got.Repos[0]
	if !r.Cloned {
		t.Errorf("Cloned = false, want true")
	}
	if r.Branch != "feature/x" {
		t.Errorf("Branch = %q, want %q", r.Branch, "feature/x")
	}
	if !r.Dirty {
		t.Errorf("Dirty = false, want true")
	}
}

func TestGetWorkspaceContext_directoryWithoutGit(t *testing.T) {
	resetWorkspaceContextState(t)
	// A real directory but no .git inside.
	dir := t.TempDir()

	runGitFn = func(_ string, _ ...string) (string, error) {
		t.Fatal("git should not be invoked for non-git directory")
		return "", nil
	}

	context = &Context{
		Profile: Profile{
			Name: "demo",
			Path: "/tmp/demo.raid.yaml",
			Repositories: []Repo{
				{Name: "scratch", Path: dir, URL: "https://example.com/s.git"},
			},
		},
	}

	got := GetWorkspaceContext()
	r := got.Repos[0]
	if r.Cloned {
		t.Errorf("Cloned = true, want false (no .git dir)")
	}
}

func TestGetWorkspaceContext_gitErrorsLeaveFieldsZeroed(t *testing.T) {
	resetWorkspaceContextState(t)
	dir := makeFakeGitDir(t)

	runGitFn = func(_ string, args ...string) (string, error) {
		return "", os.ErrNotExist
	}

	context = &Context{
		Profile: Profile{
			Name: "demo",
			Path: "/tmp/demo.raid.yaml",
			Repositories: []Repo{
				{Name: "broken", Path: dir, URL: "https://example.com/b.git"},
			},
		},
	}

	got := GetWorkspaceContext()
	r := got.Repos[0]
	if !r.Cloned {
		t.Errorf("Cloned = false, want true (.git dir present even if git failed)")
	}
	if r.Branch != "" {
		t.Errorf("Branch = %q, want empty after git error", r.Branch)
	}
	if r.Dirty {
		t.Errorf("Dirty = true, want false after git error")
	}
}
