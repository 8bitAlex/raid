package lib

import (
	"fmt"

	"github.com/spf13/viper"
)

const ACTIVE_ENV_KEY = "environment"

type Env struct {
	Name string
}

func (e Env) IsZero() bool {
	return e.Name == ""
}

func SetEnv(name string) error {
	if !ContainsEnv(name) {
		return fmt.Errorf("environment '%s' not found", name)
	}
	Set(ACTIVE_ENV_KEY, name)
	return nil
}

func GetEnv() Env {
	name := viper.GetString(ACTIVE_ENV_KEY)
	for _, env := range context.Envs {
		if env.Name == name {
			return env
		}
	}
	return Env{}
}

func GetEnvs() []Env {
	return context.Envs
}

func ContainsEnv(name string) bool {
	for _, env := range context.Envs {
		if env.Name == name {
			return true
		}
	}
	return false
}

func ExecuteEnv() error {
	env := GetEnv()
	if env.IsZero() {
		return fmt.Errorf("no active environment found")
	}
	return nil
}
