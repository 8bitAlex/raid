package profile

import (
	"fmt"

	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/errs"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

// removeResult is the stable JSON shape for `raid profile remove --json`.
// Fields are part of the public CLI contract; new fields ship additively.
type removeResult struct {
	Removed []string             `json:"removed"`
	Errors  []removeResultErr    `json:"errors,omitempty"`
}

type removeResultErr struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

var RemoveProfileCmd = &cobra.Command{
	Use:        "remove <name>",
	Short:      "Remove profile(s)",
	SuggestFor: []string{"delete"},
	Args:       cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var removed []string
		var failures []removeResultErr

		lockErr := raid.WithMutationLock(func() error {
			for _, name := range args {
				if err := pro.Remove(name); err != nil {
					failures = append(failures, removeResultErr{Name: name, Reason: err.Error()})
				} else {
					removed = append(removed, name)
				}
			}
			return nil
		})
		if lockErr != nil {
			return errs.Wrap(lockErr)
		}

		if jsonMode(cmd) {
			return emitJSON(cmd, removeResult{Removed: removed, Errors: failures})
		}

		out := cmd.OutOrStdout()
		for _, name := range removed {
			fmt.Fprintf(out, "Profile '%s' has been removed.\n", name)
		}
		for _, f := range failures {
			fmt.Fprintf(out, "Profile '%s' not found. Use 'raid profile list' to see available profiles.\n", f.Name)
		}

		// Surface a structured error iff every requested profile failed.
		// Mixed success: text mode shows both lines and exits 0 (the
		// removes that did happen are real); JSON consumers can pivot
		// on the errors[] field.
		if len(removed) == 0 && len(failures) > 0 {
			if len(failures) == 1 {
				return errs.ProfileNotFound(failures[0].Name)
			}
			return errs.ProfileNotFound(failures[0].Name + " (and " + fmt.Sprintf("%d", len(failures)-1) + " more)")
		}
		return nil
	},
}
