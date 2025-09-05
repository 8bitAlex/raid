// Manage raid environments.
package env

import "github.com/8bitalex/raid/src/internal/lib"

type Env = lib.Env

func Set(name string) error {
	return lib.SetEnv(name)
}

func Get() Env {
	return lib.GetEnv()
}

func GetAll() []Env {
	return lib.GetEnvs()
}

func Execute() error {
	return lib.ExecuteEnv()
}
