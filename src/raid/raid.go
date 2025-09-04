/*
The primary interface for the raid CLI tool functionality.

Lifecycle:
 1. Initialize: set up the raid environment, including loading configurations and initializing data storage.
 2. Compile: compile the raid configurations and prepare them for execution.
 3. Execute: run the raid commands based on the compiled configurations.
 4. Shutdown: gracefully shut down the raid environment, ensuring all resources are released and saved.

Related packages:
  - `raid/profile`: provides the core functionality for loading and managing profiles
  - `raid/repo`: provides the core functionality for loading and managing repositories
  - `raid/env`: provides the core functionality for loading and managing environments
*/
package raid

import (
	"log"

	"github.com/8bitalex/raid/src/internal/lib"
)

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

// Initialize the raid environment, including loading configurations and initializing data storage.
func Initialize() {
	if err := lib.InitConfig(); err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}
	if err := Compile(); err != nil {
		log.Fatalf("Failed to compile configurations: %v", err)
	}
}

// Compile the raid configurations for execution. Uses cached results if available.
func Compile() error {
	return lib.Compile()
}

// Force compile the raid configurations for execution. Ignores cache.
func ForceCompile() error {
	return lib.ForceCompile()
}

// Install the active profile
func Install(maxThreads int) error {
	return lib.Install(maxThreads)
}
