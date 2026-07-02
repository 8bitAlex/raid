package lib

import (
	"fmt"
	"path/filepath"

	liberrs "github.com/8bitalex/raid/src/internal/lib/errs"
	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const activeEnvKey = "env"

// Env represents a named environment with variables and tasks.
type Env struct {
	Name      string   `json:"name"`
	Variables []EnvVar `json:"variables"`
	Tasks     []Task   `json:"tasks"`
}

// IsZero reports whether the environment is uninitialized.
func (e Env) IsZero() bool {
	return e.Name == ""
}

// EnvVar is a key/value pair written into a repository's .env file.
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SetEnv sets the named environment as the active environment.
func SetEnv(name string) error {
	if name == "" || !ContainsEnv(name) {
		return liberrs.EnvNotFound(name)
	}

	return Set(activeEnvKey, name)
}

// GetEnv returns the name of the currently active environment.
func GetEnv() string {
	return viper.GetString(activeEnvKey)
}

// ListEnvs returns the names of all environments in the active profile,
// including environments declared only in a repository's raid.yaml. The
// env command's contract is that names are "searched for in the active
// profile and all repository configurations" — without the repo sweep, an
// env declared only in a repo raid.yaml would fail ContainsEnv and be
// unswitchable. Profile-level names come first; duplicates are folded.
func ListEnvs() []string {
	ctx := loadContext()
	if ctx == nil {
		return []string{}
	}

	seen := make(map[string]bool, len(ctx.Profile.Environments))
	names := make([]string, 0, len(ctx.Profile.Environments))
	for _, env := range ctx.Profile.Environments {
		if !seen[env.Name] {
			seen[env.Name] = true
			names = append(names, env.Name)
		}
	}
	for _, repo := range ctx.Profile.Repositories {
		for _, env := range repo.Environments {
			if !seen[env.Name] {
				seen[env.Name] = true
				names = append(names, env.Name)
			}
		}
	}
	return names
}

// ContainsEnv reports whether an environment with the given name exists in the active profile.
func ContainsEnv(name string) bool {
	for _, envName := range ListEnvs() {
		if envName == name {
			return true
		}
	}
	return false
}

// ExecuteEnv writes environment variables to each repo's .env file and runs the environment's tasks.
func ExecuteEnv(name string) error {
	ctx := loadContext()
	if ctx == nil {
		return liberrs.Internal("raid context is not initialized")
	}
	if err := setEnvVariablesForRepos(ctx, name); err != nil {
		return liberrs.Newf(liberrs.CodeConfigInvalid, liberrs.CategoryConfig, "failed to set env variables: %v", err)
	}
	if err := runTasksForEnv(ctx, name); err != nil {
		return liberrs.Newf(liberrs.CodeTaskFailed, liberrs.CategoryTask, "failed to run env tasks: %v", err)
	}
	return nil
}

func setEnvVariablesForRepos(ctx *Context, name string) error {
	for _, repo := range ctx.Profile.Repositories {
		// Skip repos that haven't been installed yet. buildEnvPath would
		// MkdirAll the repo directory just to hold the .env file, and a
		// pre-created non-empty directory then makes the eventual
		// `raid install` clone fail ("destination path already exists").
		if !sys.FileExists(sys.ExpandPath(repo.Path)) {
			fmt.Fprintf(commandStderr, "Skipping repo %s: not found at %s (run 'raid install' to clone it, then re-run 'raid env')\n", repo.Name, sys.ExpandPath(repo.Path))
			continue
		}

		fmt.Fprintf(commandStdout, "Setting up environment for repo: %s\n", repo.Name)

		path, err := buildEnvPath(repo.Path)
		if err != nil {
			return liberrs.Newf(liberrs.CodeConfigInvalid, liberrs.CategoryConfig, "invalid path for repo '%s': %v", repo.Name, err)
		}

		if err := setEnvVariables(ctx.Profile.getEnv(name).Variables, repo.getEnv(name).Variables, path); err != nil {
			return liberrs.Newf(liberrs.CodeConfigInvalid, liberrs.CategoryConfig, "failed to set env variables for repo '%s': %v", repo.Name, err)
		}
	}
	return nil
}

func buildEnvPath(path string) (string, error) {
	filePath := filepath.Join(sys.ExpandPath(path), ".env")
	file, err := sys.CreateFile(filePath)
	if err != nil {
		return "", err
	}
	file.Close()
	return filePath, nil
}

func setEnvVariables(profVars []EnvVar, repoVars []EnvVar, path string) error {
	envMap, err := godotenv.Read(path)
	if err != nil {
		return err
	}

	for _, v := range profVars {
		envMap[v.Name] = v.Value
	}
	for _, v := range repoVars {
		envMap[v.Name] = v.Value
	}

	return godotenv.Write(envMap, path)
}

func runTasksForEnv(ctx *Context, name string) error {
	// Profile-level env tasks run with the home dir as the default
	// working directory. In single-repo mode the profile's environments
	// are hoisted copies of the repo's (ForceLoad's merge), so the
	// profile-level pass is skipped there — the repo loop below runs the
	// same tasks once, with the repo dir as the default, which is where
	// tasks declared in a raid.yaml expect to execute.
	if !ctx.Profile.IsSingleRepo() {
		if env := ctx.Profile.getEnv(name); !env.IsZero() && len(env.Tasks) > 0 {
			if err := ExecuteTasks(withDefaultDir(env.Tasks, sys.GetHomeDir())); err != nil {
				return err
			}
		}
	}

	// Repo-level env tasks run per repo with that repo's directory as
	// the default. Without this loop, tasks declared on an environment
	// in a repo's raid.yaml were silently skipped in multi-repo
	// profiles — the docs promise repo envs "are merged with the
	// profile-level environments when applied".
	for _, repo := range ctx.Profile.Repositories {
		env := repo.getEnv(name)
		if env.IsZero() || len(env.Tasks) == 0 {
			continue
		}
		if err := ExecuteTasks(withDefaultDir(env.Tasks, sys.ExpandPath(repo.Path))); err != nil {
			return err
		}
	}
	return nil
}

// mergeEnvironments merges additional into base by Name. On name conflicts
// base wins, mirroring the mergeCommands contract — the wrapping profile
// is canonical, and a per-repo raid.yaml can't silently override an env
// the profile author chose to expose. Used by ForceLoad's single-repo
// hoist path so a wrapping profile + raid.yaml each declaring "dev"
// don't surface as two entries.
func mergeEnvironments(base, additional []Env) []Env {
	existing := make(map[string]bool, len(base))
	for _, e := range base {
		existing[e.Name] = true
	}
	result := append([]Env(nil), base...)
	for _, e := range additional {
		if !existing[e.Name] {
			result = append(result, e)
		}
	}
	return result
}

// LoadEnv loads .env files from all repositories in the active profile into the process environment.
func LoadEnv() error {
	ctx := loadContext()
	if ctx == nil {
		return liberrs.Internal("context not initialized")
	}

	var paths []string
	for _, r := range ctx.Profile.Repositories {
		p := filepath.Join(sys.ExpandPath(r.Path), ".env")
		if sys.FileExists(p) {
			paths = append(paths, p)
		}
	}

	if len(paths) == 0 {
		return nil
	}

	if err := godotenv.Load(paths...); err != nil {
		return liberrs.Newf(liberrs.CodeConfigLoadFailed, liberrs.CategoryConfig, "failed to load env files: %v", err)
	}
	return nil
}
