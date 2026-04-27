package lib

import (
	"os/exec"
	"strings"
	"time"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/resources"
)

// raidWebsiteUrl is the canonical project URL surfaced in WorkspaceContext so
// agents have a discoverable entry point for additional documentation, issue
// reporting, and source code search.
const raidWebsiteUrl = "https://github.com/8bitalex/raid"

// WorkspaceContext is a condensed snapshot of the active workspace, intended for
// agent / `raid context` consumption. It is derived from the loaded session
// context plus on-disk git state per repository.
//
// The top-level Name / Version / WebsiteUrl / GeneratedAt fields identify the
// producer so an agent that picks up the snapshot in isolation can find the
// project on GitHub for further context and judge freshness. The Workspace
// sub-object holds the actual workspace state.
//
// Field naming follows MCP (camelCase) so this shape can be lifted directly
// into the future `raid context serve` MCP server responses with no further
// translation.
type WorkspaceContext struct {
	Name        string          `json:"name"`
	Title       string          `json:"title,omitempty"`
	Version     string          `json:"version,omitempty"`
	WebsiteUrl  string          `json:"websiteUrl,omitempty"`
	GeneratedAt time.Time       `json:"generatedAt"`
	Tools       []WorkspaceTool `json:"tools,omitempty"`
	Workspace   Workspace       `json:"workspace"`
}

// Workspace is the inline workspace state — what would be served via
// `resources/read` against a `raid://workspace/*` URI by an MCP server.
type Workspace struct {
	Profile  string             `json:"profile"`
	Env      string             `json:"env,omitempty"`
	Repos    []WorkspaceRepo    `json:"repos"`
	Commands []WorkspaceCommand `json:"commands"`
	Recent   []RecentEntry      `json:"recent,omitempty"`
}

// WorkspaceTool describes a built-in `raid` subcommand the agent can invoke
// directly (e.g. `raid install`, `raid env`). User-defined commands live in
// Workspace.Commands and are not duplicated here.
type WorkspaceTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// WorkspaceRepo describes a single repository in the active profile, as it
// exists on disk right now (not just as configured).
type WorkspaceRepo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Cloned bool   `json:"cloned"`
	Branch string `json:"branch,omitempty"`
	Dirty  bool   `json:"dirty,omitempty"`
}

// WorkspaceCommand exposes a profile command's name and short description
// only — the script bodies are intentionally excluded so the snapshot stays
// token-efficient and free of secrets.
type WorkspaceCommand struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// runGitFn invokes git in dir with the given args and returns trimmed stdout.
// Overridable in tests.
var runGitFn = func(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// GetWorkspaceContext returns a snapshot of the active workspace. The session
// context must already be loaded (Load / ForceLoad). If no profile is active
// the returned WorkspaceContext has empty Workspace.Profile and empty Repos /
// Commands slices.
func GetWorkspaceContext() WorkspaceContext {
	version, _ := resources.GetProperty(resources.PropertyVersion)
	wc := WorkspaceContext{
		Name:        "raid",
		Title:       "Raid",
		Version:     version,
		WebsiteUrl:  raidWebsiteUrl,
		GeneratedAt: recentNowFn().UTC().Truncate(time.Second),
		Workspace: Workspace{
			Repos:    []WorkspaceRepo{},
			Commands: []WorkspaceCommand{},
			Recent:   ReadRecent(),
		},
	}
	if context == nil {
		return wc
	}
	wc.Workspace.Env = context.Env

	profile := context.Profile
	if profile.IsZero() {
		return wc
	}
	wc.Workspace.Profile = profile.Name

	wc.Workspace.Repos = make([]WorkspaceRepo, 0, len(profile.Repositories))
	for _, repo := range profile.Repositories {
		wc.Workspace.Repos = append(wc.Workspace.Repos, describeRepo(repo))
	}

	wc.Workspace.Commands = make([]WorkspaceCommand, 0, len(profile.Commands))
	for _, cmd := range profile.Commands {
		wc.Workspace.Commands = append(wc.Workspace.Commands, WorkspaceCommand{
			Name:        cmd.Name,
			Description: cmd.Usage,
		})
	}
	return wc
}

func describeRepo(repo Repo) WorkspaceRepo {
	wr := WorkspaceRepo{
		Name: repo.Name,
		Path: repo.Path,
	}

	expanded := sys.ExpandPath(repo.Path)
	if !sys.FileExists(expanded) || !isGitRepository(expanded) {
		return wr
	}
	wr.Cloned = true

	if branch, err := runGitFn(expanded, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		wr.Branch = branch
	}
	if status, err := runGitFn(expanded, "status", "--porcelain"); err == nil {
		wr.Dirty = strings.TrimSpace(status) != ""
	}
	return wr
}
