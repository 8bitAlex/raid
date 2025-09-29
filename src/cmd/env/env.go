package env

import (
	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/env"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(ListEnvCmd)
}

var Command = &cobra.Command{
	Use:   "env [environment-name]",
	Short: "Execute an environment",
	Long:  "Execute an environment by name. The environment will be searched for in the active profile and all repository configurations. Tasks are executed concurrently and environment variables are set globally.",
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			env := env.Get()
			if env == "" {
				cmd.PrintErrln("No active environment set.")
			} else {
				cmd.Println("Active environment:", env)
			}
		} else if len(args) == 1 {
			name := args[0]
			if !env.Contains(name) {
				cmd.PrintErrln("Environment not found:", name)
			} else {
				cmd.Println("Setting up environment:", name)
				if err := env.Set(name); err != nil {
					cmd.PrintErrln("Failed to switch environment:", err)
				}
				raid.ForceLoad()
				if err := env.Execute(env.Get()); err != nil {
					cmd.PrintErrln("Failed to execute environment:", err)
				} else {
					cmd.Println("Environment executed successfully.")
				}
			}
		} else {
			cmd.PrintErrln("Invalid number of arguments.")
		}
	},
}
