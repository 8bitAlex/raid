package env

import (
	"encoding/json"
	"fmt"

	"github.com/8bitalex/raid/src/raid/env"
	"github.com/spf13/cobra"
)

var listJSON bool

func init() {
	ListEnvCmd.Flags().BoolVar(&listJSON, "json", false, "Emit machine-readable JSON output")
}

// envEntry is the stable JSON shape for a single environment in `env list --json`.
type envEntry struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

var ListEnvCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	RunE: func(cmd *cobra.Command, args []string) error {
		envs := env.ListAll()
		active := env.Get()

		if listJSON {
			out := make([]envEntry, 0, len(envs))
			for _, name := range envs {
				out = append(out, envEntry{Name: name, Active: name == active})
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}

		if len(envs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No environments found.")
			return nil
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Available environments:")
		for _, name := range envs {
			fmt.Fprintf(cmd.OutOrStdout(), "\t%s\n", name)
		}
		return nil
	},
}
