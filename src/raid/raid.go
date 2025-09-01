/*
The primary interface for the raid CLI tool functionality.
*/
package raid

import "github.com/8bitalex/raid/src/internal/lib"

const (
	ConfigPathFlag      = lib.ConfigPathFlag
	ConfigPathFlagDesc  = lib.ConfigPathFlagDesc
	ConfigPathFlagShort = lib.ConfigPathFlagShort
	ConfigPathDefault   = lib.ConfigPathDefault
	ConfigDirName       = lib.ConfigDirName
	ConfigFileName      = lib.ConfigFileName
)

// Pointer to the configuration path
var ConfigPath = &lib.CfgPath

/*
Initialize the raid environment, including loading configurations and initializing data storage.
*/
func Initialize() {
	lib.InitConfig()
}

func Install() error {
	return nil
}
