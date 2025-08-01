package utils

import (
	"fmt"

	"github.com/spf13/cobra"
)

// MatchOne checks for at least one valid PositionalArgs. Acts like an OR operator.
func MatchOne(pargs ...cobra.PositionalArgs) cobra.PositionalArgs {
	return func (cmd *cobra.Command, args []string) error {
		var errors []error
		for _, arg := range pargs {
			if err := arg(cmd, args); err == nil {
				return nil
			} else {
				errors = append(errors, err)
			}
		}
		return mergeErr(errors)
	}
}

func mergeErr(errs []error) error {
	var result string
	for _, err := range errs {
		if len(result) == 0 {
			result = err.Error()
		} else {
			result = result + ", " + err.Error()
		}
	}
	return fmt.Errorf("%s", result)
}