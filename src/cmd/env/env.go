package env

import (
	"fmt"
	"log"

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
		if len(args) > 0 {
			if err := env.Set(args[0]); err != nil {
				log.Fatalf("Environment '%s' not found. Use 'raid env list' to see available environments.\n", args[0])
			}
			fmt.Printf("Setting environment to '%s'\n", args[0])
			if err := env.Execute(); err != nil {
				log.Fatalf("Failed to execute environment: %v", err)
			}
		} else {
			env := env.Get()
			if env.IsZero() {
				fmt.Println("No active environment found. Use 'raid env <environment>' to set one.")
				return
			}
			fmt.Println(env.Name)
		}
	},
}
