package env

import (
	"encoding/json"

	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/env"
	"github.com/8bitalex/raid/src/raid/errs"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(ListEnvCmd)
}

// jsonMode resolves --json by walking up to the root's persistent flag, so
// the read always reflects the current invocation's args even when the
// package-level Command var is reused across tests. GetBool returns false
// (and an ignored error) when the flag isn't registered, so a bare cmd
// without parents yields false naturally.
func jsonMode(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("json")
	return v
}

var Command = &cobra.Command{
	Use:   "env [environment-name]",
	Short: "Execute an environment",
	Long:  "Execute an environment by name. The environment will be searched for in the active profile and all repository configurations. Tasks are executed concurrently and environment variables are set globally.",
	Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput := jsonMode(cmd)
		if jsonOutput && len(args) > 0 {
			return errs.ArgInvalid("--json is only valid without an environment argument")
		}
		if len(args) == 0 {
			active := env.Get()
			if jsonOutput {
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
			return errs.EnvNotFound(name)
		}

		cmd.Println("Setting up environment:", name)
		err := raid.WithMutationLock(func() error {
			if err := env.Set(name); err != nil {
				return err
			}
			if err := raid.ForceLoad(); err != nil {
				return err
			}
			if err := env.Execute(env.Get()); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return errs.Wrap(err)
		}
		cmd.Println("Environment executed successfully.")
		return nil
	},
}
