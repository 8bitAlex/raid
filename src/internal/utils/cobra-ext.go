package utils

import (
	"github.com/spf13/cobra"
)

// MatchOne checks for at least one valid PositionalArgs. Acts like an OR operator.
func MatchOne(pargs ...cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		var errors []error
		for _, arg := range pargs {
			if err := arg(cmd, args); err == nil {
				return nil
			} else {
				errors = append(errors, err)
			}
		}
		return MergeErr(errors)
	}
}
