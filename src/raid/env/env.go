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

func GetAll() []string {
	return lib.GetEnvs()
}

func Contains(name string) bool {
	return lib.ContainsEnv(name)
}

// func Execute(name string) error {
// 	return lib.ExecuteEnv(name)
// }
