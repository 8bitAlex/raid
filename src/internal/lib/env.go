package lib

import (
	"fmt"

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
		return fmt.Errorf("environment '%s' not found", name)
	}

	Set(activeEnvKey, name)
	return nil
}

// GetEnv returns the name of the currently active environment.
func GetEnv() string {
	return viper.GetString(activeEnvKey)
}

// ListEnvs returns the names of all environments in the active profile.
func ListEnvs() []string {
	if context == nil || len(context.Profile.Environments) == 0 {
		return []string{}
	}

	names := make([]string, 0, len(context.Profile.Environments))
	for _, env := range context.Profile.Environments {
		names = append(names, env.Name)
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
	if err := setEnvVariablesForRepos(name); err != nil {
		return fmt.Errorf("failed to set env variables: %w", err)
	}
	if err := runTasksForEnv(name); err != nil {
		return fmt.Errorf("failed to run env tasks: %w", err)
	}
	return nil
}

func setEnvVariablesForRepos(name string) error {
	for _, repo := range context.Profile.Repositories {
		fmt.Printf("Setting up environment for repo: %s\n", repo.Name)

		path, err := buildEnvPath(repo.Path)
		if err != nil {
			return fmt.Errorf("invalid path for repo '%s': %w", repo.Name, err)
		}

		if err := setEnvVariables(context.Profile.getEnv(name).Variables, repo.getEnv(name).Variables, path); err != nil {
			return fmt.Errorf("failed to set env variables for repo '%s': %w", repo.Name, err)
		}
	}
	return nil
}

func buildEnvPath(path string) (string, error) {
	filePath := sys.ExpandPath(path) + sys.Sep + ".env"
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

func runTasksForEnv(name string) error {
	env := context.Profile.getEnv(name)
	if env.IsZero() || len(env.Tasks) == 0 {
		return nil
	}
	return ExecuteTasks(withDefaultDir(env.Tasks, sys.GetHomeDir()))
}

// LoadEnv loads .env files from all repositories in the active profile into the process environment.
func LoadEnv() error {
	if context == nil {
		return fmt.Errorf("context not initialized")
	}

	var paths []string
	for _, r := range context.Profile.Repositories {
		p := sys.ExpandPath(r.Path) + sys.Sep + ".env"
		if sys.FileExists(p) {
			paths = append(paths, p)
		}
	}

	if len(paths) == 0 {
		return nil
	}

	if err := godotenv.Load(paths...); err != nil {
		return fmt.Errorf("failed to load env files: %w", err)
	}
	return nil
}
