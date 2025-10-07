package lib

import (
	"fmt"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"github.com/thoas/go-funk"
)

const (
	ACTIVE_ENV_KEY = "env"
)

type Env struct {
	Name      string   `json:"name"`
	Variables []EnvVar `json:"variables"`
	Tasks     []Task   `json:"tasks"`
}

func (e Env) IsZero() bool {
	return e.Name == ""
}

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func SetEnv(name string) error {
	if name == "" || !ContainsEnv(name) {
		return fmt.Errorf("environment '%s' not found", name)
	}

	Set(ACTIVE_ENV_KEY, name)
	return nil
}

func GetEnv() string {
	return viper.GetString(ACTIVE_ENV_KEY)
}

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

func ContainsEnv(name string) bool {
	for _, envName := range ListEnvs() {
		if envName == name {
			return true
		}
	}
	return false
}

func ExecuteEnv(name string) error {
	err := setEnvVariablesForRepos(name)
	if err != nil {
		return fmt.Errorf("failed to set env variables: %w", err)
	}

	err = runTasksForEnv(name)
	if err != nil {
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

		pEnv := context.Profile.getEnv(name)
		rEnv := repo.getEnv(name)

		err = setEnvVariables(pEnv.Variables, rEnv.Variables, path)
		if err != nil {
			return fmt.Errorf("failed to set env variables for repo '%s': %w", repo.Name, err)
		}
	}
	return nil
}

func buildEnvPath(path string) (string, error) {
	filepath := sys.ExpandPath(path) + sys.Sep + ".env"
	// create file if it does not exist
	file, err := sys.CreateFile(filepath)
	if err != nil {
		return "", err
	}
	file.Close()
	return filepath, nil
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
		fmt.Printf("Setting variable %s=%s\n", v.Name, v.Value)
		envMap[v.Name] = v.Value
	}

	err = godotenv.Write(envMap, path)
	if err != nil {
		return err
	}
	return nil
}

func runTasksForEnv(name string) error {
	env := context.Profile.getEnv(name)
	if env.IsZero() || len(env.Tasks) == 0 {
		return nil
	}

	err := ExecuteTasks(env.Tasks)
	if err != nil {
		return err
	}
	return nil
}

func LoadEnv() error {
	if context == nil {
		return fmt.Errorf("context not initialized")
	}

	repos := context.Profile.Repositories
	paths := funk.Map(repos, func(r Repo) string {
		return sys.ExpandPath(r.Path) + sys.Sep + ".env"
	}).([]string)

	err := godotenv.Load(paths...)
	if err != nil {
		return fmt.Errorf("failed to load env files: %w", err)
	}
	return nil
}
