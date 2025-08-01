package cmd

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/cmd/install"
	"github.com/8bitalex/raid/src/cmd/profile"
	"github.com/8bitalex/raid/src/internal/lib/data"
	"github.com/spf13/cobra"
)

func init() {
	cobra.OnInitialize(data.Initialize)
	// Global Flags
	rootCmd.PersistentFlags().StringVarP(&data.CfgPath, "config", "c", "", "config file path (default is $HOME/.raid/config.toml)")
	// Subcommands
	rootCmd.AddCommand(profile.ProfileCmd)
	rootCmd.AddCommand(install.InstallCmd)
	// todo build custom commands
}

var rootCmd = &cobra.Command{
	Use:   "raid",
	Version: "alpha",
	Short: "Raid is a tool for orchestrating your development environment",
	Long:  `Raid is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories.`,
	Args: cobra.NoArgs,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func Prepare() {
	if profile.Profile == "" {
		fmt.Println("Profile not set")
		os.Exit(1)
	}
}
