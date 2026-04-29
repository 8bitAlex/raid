package env

import (
	"encoding/json"
	"fmt"

	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/env"
	"github.com/spf13/cobra"
)

var showJSON bool

func init() {
	Command.AddCommand(ListEnvCmd)
	Command.Flags().BoolVar(&showJSON, "json", false, "Emit machine-readable JSON output (only valid without an environment argument)")
}

var Command = &cobra.Command{
	Use:   "env [environment-name]",
	Short: "Execute an environment",
	Long:  "Execute an environment by name. The environment will be searched for in the active profile and all repository configurations. Tasks are executed concurrently and environment variables are set globally.",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if showJSON && len(args) > 0 {
			return fmt.Errorf("--json is only valid without an environment argument")
		}
		if len(args) == 0 {
			active := env.Get()
			if showJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(envEntry{Name: active, Active: active != ""})
			}
			if active == "" {
				cmd.PrintErrln("No active environment set.")
			} else {
				cmd.Println("Active environment:", active)
			}
			return nil
		}

		name := args[0]
		if !env.Contains(name) {
			cmd.PrintErrln("Environment not found:", name)
			return nil
		}

		cmd.Println("Setting up environment:", name)
		err := raid.WithMutationLock(func() error {
			if err := env.Set(name); err != nil {
				cmd.PrintErrln("Failed to switch environment:", err)
				return err
			}
			if err := raid.ForceLoad(); err != nil {
				cmd.PrintErrln("Failed to reload profile:", err)
				return err
			}
			if err := env.Execute(env.Get()); err != nil {
				cmd.PrintErrln("Failed to execute environment:", err)
				return err
			}
			return nil
		})
		if err != nil {
			return nil
		}
		cmd.Println("Environment executed successfully.")
		return nil
	},
}
