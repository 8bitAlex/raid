package lib

import (
	"os/exec"
	"strings"

	sys "github.com/8bitalex/raid/src/internal/sys"
)

// WorkspaceContext is a condensed snapshot of the active workspace, intended for
// agent / `raid context` consumption. It is derived from the loaded session
// context plus on-disk git state per repository.
type WorkspaceContext struct {
	Profile  string             `json:"profile"`
	Env      string             `json:"env,omitempty"`
	Repos    []WorkspaceRepo    `json:"repos"`
	Commands []WorkspaceCommand `json:"commands"`
	Recent   []RecentEntry      `json:"recent,omitempty"`
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
// the returned WorkspaceContext has empty Profile and empty Repos / Commands
// slices.
func GetWorkspaceContext() WorkspaceContext {
	wc := WorkspaceContext{
		Repos:    []WorkspaceRepo{},
		Commands: []WorkspaceCommand{},
		Recent:   ReadRecent(),
	}
	if context == nil {
		return wc
	}
	wc.Env = context.Env

	profile := context.Profile
	if profile.IsZero() {
		return wc
	}
	wc.Profile = profile.Name

	wc.Repos = make([]WorkspaceRepo, 0, len(profile.Repositories))
	for _, repo := range profile.Repositories {
		wc.Repos = append(wc.Repos, describeRepo(repo))
	}

	wc.Commands = make([]WorkspaceCommand, 0, len(profile.Commands))
	for _, cmd := range profile.Commands {
		wc.Commands = append(wc.Commands, WorkspaceCommand{
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
