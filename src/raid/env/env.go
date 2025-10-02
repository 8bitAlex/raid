// Manage raid environments.
package env

import "github.com/8bitalex/raid/src/internal/lib"

type Env = lib.Env

func Set(name string) error {
	return lib.SetEnv(name)
}

func Get() string {
	return lib.GetEnv()
}

func ListAll() []string {
	return lib.ListEnvs()
}

func Contains(name string) bool {
	return lib.ContainsEnv(name)
}

func Execute(env string) error {
	return lib.ExecuteEnv(env)
}
