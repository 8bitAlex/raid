package env

import (
	"fmt"

	"github.com/8bitalex/raid/src/raid/env"
	"github.com/spf13/cobra"
)

var ListEnvCmd = &cobra.Command{
	Use:   "list",
	Short: "List environments",
	Run: func(cmd *cobra.Command, args []string) {
		envs := env.GetAll()
		if len(envs) == 0 {
			fmt.Println("No environments found.")
			return
		}
		fmt.Println("Available environments:")
		for _, env := range envs {
			fmt.Printf("\t%s\n", env.Name)
		}
		fmt.Print()
	},
}
