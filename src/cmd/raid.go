package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/8bitalex/raid/src/cmd/doctor"
	"github.com/8bitalex/raid/src/cmd/env"
	"github.com/8bitalex/raid/src/cmd/install"
	"github.com/8bitalex/raid/src/cmd/profile"
	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

// reservedNames are built-in cobra subcommands that custom commands cannot shadow.
var reservedNames = map[string]bool{
	"profile":    true,
	"install":    true,
	"env":        true,
	"doctor":     true,
	"help":       true,
	"version":    true,
	"completion": true,
}

var rootCmd = &cobra.Command{
	Use:   "raid",
	Short: "Raid is a tool for orchestrating common tasks across your development environment(s).",
	Args:  cobra.NoArgs,
}

func init() {
	version, err := raid.GetProperty(raid.PropertyVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, "raid: app.properties:", err)
		os.Exit(1)
	}
	rootCmd.Version = version
	rootCmd.Long = "Raid v" + version + "\n\nRaid is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories."
	// Global Flags
	rootCmd.PersistentFlags().StringVarP(raid.ConfigPath, raid.ConfigPathFlag, raid.ConfigPathFlagShort, "", raid.ConfigPathFlagDesc)
	// Subcommands
	rootCmd.AddCommand(profile.Command)
	rootCmd.AddCommand(install.Command)
	rootCmd.AddCommand(env.Command)
	rootCmd.AddCommand(doctor.Command)
}

// isInfoCommand reports whether the invocation is for a built-in informational
// command (help, version, completion) that does not require a loaded profile.
func isInfoCommand(args []string) bool {
	if len(args) <= 1 {
		return true
	}
	for _, arg := range args[1:] {
		if arg == "--" {
			break
		}
		switch arg {
		case "help", "version", "completion", "--help", "-h", "--version", "-v":
			return true
		}
	}
	return false
}

func Execute() {
	info := isInfoCommand(os.Args)
	environment, _ := raid.GetProperty(raid.PropertyEnvironment)

	// Start version check early so network latency overlaps with initialization.
	updateCh := make(chan string, 1)
	go func() {
		if raid.Environment(environment) == raid.EnvironmentPreview {
			updateCh <- sys.LatestGitHubPreRelease("8bitalex/raid")
		} else {
			updateCh <- sys.LatestGitHubRelease("8bitalex/raid")
		}
	}()

	applyConfigFlag(os.Args)
	var cmds []lib.Command
	if info {
		// Best-effort, read-only load: no config file creation, no warnings.
		// User commands will appear in help if the config already exists.
		cmds = raid.QuietGetCommands()
	} else {
		raid.Initialize()
		cmds = raid.GetCommands()
	}

	// For info commands wait up to 1.5s; for regular commands do a non-blocking
	// check so startup is not delayed.
	var latest string
	if info {
		select {
		case v := <-updateCh:
			latest = v
		case <-time.After(1500 * time.Millisecond):
		}
	} else {
		select {
		case v := <-updateCh:
			latest = v
		default:
		}
	}

	version, _ := raid.GetProperty(raid.PropertyVersion)
	if latest != "" && latest != baseVersion(version) {
		var label string
		if raid.Environment(environment) == raid.EnvironmentPreview {
			label = "Preview update"
		} else {
			label = "Update available"
		}
		notice := sys.Yellow("(" + label + ": v" + version + " → v" + latest + ")")
		rootCmd.Long = strings.Replace(rootCmd.Long, "Raid v"+version, "Raid v"+version+" "+notice, 1)
		if !info {
			fmt.Fprintf(os.Stderr, "Raid v%s %s\n", version, notice)
		}
	}

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	registerUserCommands(rootCmd, cmds)

	if err := rootCmd.Execute(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "raid:", err)
		os.Exit(1)
	}
}

// registerUserCommands adds user-defined commands to root.
// Reserved built-in names are skipped with a warning.
func registerUserCommands(root *cobra.Command, cmds []lib.Command) {
	for _, cmd := range cmds {
		if reservedNames[cmd.Name] {
			fmt.Fprintf(os.Stderr, "warning: command '%s' conflicts with a built-in subcommand and will be ignored\n", cmd.Name)
			continue
		}
		name := cmd.Name
		root.AddCommand(&cobra.Command{
			Use:   name,
			Short: cmd.Usage,
			RunE: func(c *cobra.Command, args []string) error {
				return raid.ExecuteCommand(name, args)
			},
		})
	}
}

// baseVersion strips "-preview" and anything following it from a version string
// so that preview builds compare correctly against their corresponding release tag.
func baseVersion(version string) string {
	base, _, _ := strings.Cut(version, "-preview")
	return base
}

// applyConfigFlag scans args for --config / -c so the config path is set
// before the pre-initialization call, matching cobra's later flag parsing.
// Scanning stops at the first -- end-of-flags marker.
func applyConfigFlag(args []string) {
	for i, arg := range args {
		if arg == "--" {
			return
		}
		if v, ok := strings.CutPrefix(arg, "--config="); ok {
			*raid.ConfigPath = v
			return
		}
		if v, ok := strings.CutPrefix(arg, "-c="); ok {
			*raid.ConfigPath = v
			return
		}
		if (arg == "--config" || arg == "-c") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			*raid.ConfigPath = args[i+1]
			return
		}
	}
}
