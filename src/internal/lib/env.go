package lib

import (
	"fmt"

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
	Key   string
	Value string
}

type RepoEnv struct {
	RepoName  string
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

func GetEnvs() []string {
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
	for _, envName := range GetEnvs() {
		if envName == name {
			return true
		}
	}
	return false
}

// func ExecuteEnv(name string) error {
// 	profileEnv := context.Profile.getEnv(name)
// 	for _, repo := range context.Profile.Repositories {
// 		path := repo.Path + sys.Sep + ".env"
// 		file, err := sys.CreateFile(path)
// 		if err != nil {
// 			return err
// 		}
// 		file.Close()

// 		godotenv.Load(path)
// 	}
// 	return nil
// }

// func getRepoEnv(name string) Env {
// 	for _, repo := range context.Profile.Repositories {
// 		if repo.Name == name {
// 			return repo.Envs
// 		}
// 	}
// 	return Env{}
// }

// func setEnvVars(env Env) error {
// 	for _, v := range env.Variables {
// 		godotenv.Overload(v.Key, v.Value)
// 	}
// 	return nil
// }
