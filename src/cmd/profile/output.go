package profile

import (
	"encoding/json"

	"github.com/8bitalex/raid/src/raid/errs"
	"github.com/spf13/cobra"
)

// jsonMode resolves the persistent --json flag from rootCmd. Used by every
// profile subcommand so JSON output stays consistent across add / remove /
// list / set-active / no-arg-show. Returns false (its zero value) when the
// flag isn't registered (e.g. test cmds with no parent) so callers don't
// need to guard.
func jsonMode(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("json")
	return v
}

// emitJSON writes v as pretty-printed JSON to the command's stdout and
// returns any encode error wrapped as a structured Unknown so the root
// handler still exits with a category-correct code rather than swallowing
// the failure silently (the recurring `_ = enc.Encode(...)` Copilot
// theme).
func emitJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return errs.Unknown(err)
	}
	return nil
}
