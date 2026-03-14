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
	"fmt"
	"log"
	"os"

	"github.com/8bitalex/raid/src/internal/lib"
)

const (
	RaidConfigFileName  = lib.RaidConfigFileName
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
		log.Fatalf("init config: %v", err)
	}
	if err := Load(); err != nil {
		// Non-fatal: allows 'raid doctor' to run and report the underlying issue.
		fmt.Fprintf(os.Stderr, "warning: could not load profile: %v\n", err)
	}
	if err := lib.LoadEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}

// Load the raid configurations for execution. Uses cached results if available.
func Load() error {
	return lib.Load()
}

// Force load the raid configurations for execution. Ignores cache.
func ForceLoad() error {
	return lib.ForceLoad()
}

// Install the active profile
func Install(maxThreads int) error {
	return lib.Install(maxThreads)
}

// GetCommands returns all commands available in the active profile.
func GetCommands() []lib.Command {
	return lib.GetCommands()
}

// ExecuteCommand runs the named command from the active profile.
func ExecuteCommand(name string) error {
	return lib.ExecuteCommand(name)
}

// Severity indicates the importance of a Doctor finding.
type Severity = lib.Severity

const (
	SeverityOK    = lib.SeverityOK
	SeverityWarn  = lib.SeverityWarn
	SeverityError = lib.SeverityError
)

// Finding represents the result of a single Doctor check.
type Finding = lib.Finding

// Doctor performs all configuration checks and returns the findings.
func Doctor() []Finding {
	return lib.RunDoctor()
}
