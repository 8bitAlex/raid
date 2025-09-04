package cmd

import (
	"log"

	"github.com/8bitalex/raid/src/cmd/env"
	"github.com/8bitalex/raid/src/cmd/install"
	"github.com/8bitalex/raid/src/cmd/profile"
	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "raid",
	Version: "1.0.0-Alpha",
	Short:   "Raid is a tool for orchestrating common tasks across your development environment(s).",
	Long:    `Raid is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.`,
	Args:    cobra.NoArgs,
}

func init() {
	cobra.OnInitialize(raid.Initialize)
	// Global Flags
	rootCmd.PersistentFlags().StringVarP(raid.ConfigPath, raid.ConfigPathFlag, raid.ConfigPathFlagShort, "", raid.ConfigPathFlagDesc)
	// Subcommands
	rootCmd.AddCommand(profile.Command)
	rootCmd.AddCommand(install.Command)
	rootCmd.AddCommand(env.Command)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Failed to execute root command: %v", err)
	}
}
