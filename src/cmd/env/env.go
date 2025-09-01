package env

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"
)

var (
	concurrency int
)

func init() {
	Command.Flags().IntVarP(&concurrency, "threads", "t", 0, "Maximum number of concurrent task executions (0 = unlimited)")
}

var Command = &cobra.Command{
	Use:   "env [environment-name]",
	Short: "Execute an environment",
	Long:  "Execute an environment by name. The environment will be searched for in the active profile and all repository configurations. Tasks are executed concurrently and environment variables are set globally.",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		envName := args[0]

		// Create environment manager
		envManager := lib.NewEnvironmentManager(concurrency)

		// Execute the environment
		if err := envManager.ExecuteEnvironment(envName); err != nil {
			fmt.Printf("Failed to execute environment '%s': %v\n", envName, err)
			os.Exit(1)
		}

		fmt.Printf("Environment '%s' executed successfully\n", envName)
	},
}
