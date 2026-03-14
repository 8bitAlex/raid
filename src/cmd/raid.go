package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/8bitalex/raid/src/cmd/env"
	"github.com/8bitalex/raid/src/cmd/install"
	"github.com/8bitalex/raid/src/cmd/profile"
	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

// reservedNames are built-in cobra subcommands that custom commands cannot shadow.
var reservedNames = map[string]bool{
	"profile":    true,
	"install":    true,
	"env":        true,
	"help":       true,
	"version":    true,
	"completion": true,
}

var rootCmd = &cobra.Command{
	Use:     "raid",
	Version: "1.0.0-Alpha",
	Short:   "Raid is a tool for orchestrating common tasks across your development environment(s).",
	Long:    `Raid is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.`,
	Args:    cobra.NoArgs,
}

func init() {
	// Global Flags
	rootCmd.PersistentFlags().StringVarP(raid.ConfigPath, raid.ConfigPathFlag, raid.ConfigPathFlagShort, "", raid.ConfigPathFlagDesc)
	// Subcommands
	rootCmd.AddCommand(profile.Command)
	rootCmd.AddCommand(install.Command)
	rootCmd.AddCommand(env.Command)
}

func Execute() {
	// Pre-initialize before cobra parses args so that profile commands can be
	// registered as subcommands. cobra.OnInitialize runs after arg parsing,
	// which is too late for dynamic subcommand registration.
	applyConfigFlag(os.Args)
	raid.Initialize()

	for _, cmd := range raid.GetCommands() {
		if reservedNames[cmd.Name] {
			fmt.Fprintf(os.Stderr, "warning: command '%s' conflicts with a built-in subcommand and will be ignored\n", cmd.Name)
			continue
		}
		name := cmd.Name
		rootCmd.AddCommand(&cobra.Command{
			Use:   name,
			Short: cmd.Usage,
			RunE: func(c *cobra.Command, args []string) error {
				return raid.ExecuteCommand(name)
			},
		})
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Failed to execute root command: %v", err)
	}
}

// applyConfigFlag scans args for --config / -c so the config path is set
// before the pre-initialization call, matching cobra's later flag parsing.
// Scanning stops at the first -- end-of-flags marker.
func applyConfigFlag(args []string) {
	for i, arg := range args {
		if arg == "--" {
			return
		}
		if strings.HasPrefix(arg, "--config=") {
			*raid.ConfigPath = strings.TrimPrefix(arg, "--config=")
			return
		}
		if strings.HasPrefix(arg, "-c=") {
			*raid.ConfigPath = strings.TrimPrefix(arg, "-c=")
			return
		}
		if (arg == "--config" || arg == "-c") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			*raid.ConfigPath = args[i+1]
			return
		}
	}
}
