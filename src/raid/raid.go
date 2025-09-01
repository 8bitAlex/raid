/*
The `raid` package is the primary interface for the raid CLI tool functionality.

Raid Lifecycle:
 1. Initialize: set up the raid environment, including loading configurations and initializing data storage.
 2. Compile: compile the raid configurations and prepare them for execution.
 3. Execute: run the raid commands based on the compiled configurations.
 4. Shutdown: gracefully shut down the raid environment, ensuring all resources are released and saved.
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

/*
Compile the raid configurations and prepare them for execution.
*/
func Compile() {

}
