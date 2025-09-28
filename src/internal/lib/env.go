package lib

import (
	"fmt"

	sys "github.com/8bitalex/raid/src/internal/sys"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	ACTIVE_ENV_KEY = "env"
)

type Env struct {
	Name      string
	Variables []EnvVar
	Repos     []RepoEnv
}

func (e Env) IsZero() bool {
	return e.Name == ""
}

type EnvVar struct {
	Name  string
	Value string
}

type RepoEnv struct {
	Name      string
	Path      string
	Variables []EnvVar
}

func SetEnv(name string) error {
	if name == "" || !ContainsEnv(name) {
		return fmt.Errorf("environment '%s' not found", name)
	}

	Set(ACTIVE_ENV_KEY, name)
	return nil
}

func GetEnv() Env {
	if context != nil && !context.Env.IsZero() {
		return context.Env
	}

	name := viper.GetString(ACTIVE_ENV_KEY)
	return Env{
		Name: name,
	}
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

func buildEnv(profile Profile, name string) (Env, error) {
	if name == "" {
		return Env{}, fmt.Errorf("invalid environment name")
	}
	if profile.IsZero() {
		return Env{}, fmt.Errorf("invalid profile")
	}

	repoEnvs := make([]RepoEnv, 0, len(profile.Repositories))
	for _, repo := range profile.Repositories {
		re := RepoEnv{
			Name: repo.Name,
			Path: repo.Path,
		}
		repoEnvs = append(repoEnvs, re)
	}

	env := Env{
		Name:      name,
		Variables: profile.getEnv(name).Variables,
		Repos:     repoEnvs,
	}
	return env, nil
}

func ExecuteEnv(env Env) error {
	for _, repo := range env.Repos {
		fmt.Printf("Setting up environment for repo: %s\n", repo.Name)

		path, err := buildEnvPath(repo.Path)
		if err != nil {
			return fmt.Errorf("invalid repo path for env '%s': %w", repo.Name, err)
		}

		err = setEnvVariables(env.Variables, repo.Variables, path)
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

func setEnvVariables(envVars []EnvVar, repoVars []EnvVar, path string) error {
		envMap, err := godotenv.Read(path)
		if err != nil {
			return err
		}

		for _, v := range envVars {
			envMap[v.Name] = v.Value
		}

		for _, v := range repoVars {
			envMap[v.Name] = v.Value
		}

		err = godotenv.Write(envMap, path)
		if err != nil {
			return err
		}
		return nil
}

