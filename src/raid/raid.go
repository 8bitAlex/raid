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
	"github.com/8bitalex/raid/src/resources"
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

// Injectable dependencies for testing the Initialize fatal error branch.
var (
	initConfigFn = lib.InitConfig
	logFatalf    = log.Fatalf
)

// Initialize the raid environment, including loading configurations and initializing data storage.
func Initialize() {
	if err := initConfigFn(); err != nil {
		logFatalf("init config: %v", err)
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

// InstallRepo clones a single named repository and runs its install tasks.
func InstallRepo(name string) error {
	return lib.InstallRepo(name)
}

// GetCommands returns all commands available in the active profile.
func GetCommands() []lib.Command {
	return lib.GetCommands()
}

// QuietGetCommands performs a best-effort, read-only profile load and returns
// the available commands. It does not create config files or emit warnings, and
// returns nil if the config is absent or loading fails.
func QuietGetCommands() []lib.Command {
	return lib.QuietLoad()
}

// ExecuteCommand runs the named command from the active profile.
func ExecuteCommand(name string, args []string) error {
	return lib.ExecuteCommand(name, args)
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

// Property identifies a key in app.properties.
type Property = resources.Property

const (
	PropertyVersion     = resources.PropertyVersion
	PropertyEnvironment = resources.PropertyEnvironment
)

// Environment identifies the runtime environment the binary was built for.
type Environment = resources.Environment

const (
	EnvironmentDevelopment = resources.EnvironmentDevelopment
	EnvironmentPreview     = resources.EnvironmentPreview
	EnvironmentProduction  = resources.EnvironmentProduction
)

// GetProperty returns the value of the named property from app.properties.
func GetProperty(name Property) (string, error) {
	return resources.GetProperty(name)
}
