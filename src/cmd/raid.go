package cmd

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/cmd/env"
	"github.com/8bitalex/raid/src/cmd/install"
	"github.com/8bitalex/raid/src/cmd/profile"
	"github.com/8bitalex/raid/src/cmd/test"
	"github.com/8bitalex/raid/src/internal/lib/data"
	"github.com/spf13/cobra"
)

func init() {
	cobra.OnInitialize(data.Initialize)
	// Global Flags
	rootCmd.PersistentFlags().StringVarP(&data.CfgPath, "config", "c", "", "config file path (default is $HOME/.raid/config.toml)")
	// Subcommands
	rootCmd.AddCommand(profile.Command)
	rootCmd.AddCommand(install.Command)
	rootCmd.AddCommand(env.Command)
	rootCmd.AddCommand(test.Command)
	// todo build custom commands
}

var rootCmd = &cobra.Command{
	Use:     "raid",
	Version: "alpha",
	Short:   "Raid is a tool for orchestrating common tasks across your development environment(s).",
	Long:    `Raid is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.`,
	Args:    cobra.NoArgs,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
