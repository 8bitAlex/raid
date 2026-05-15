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

		// hardErr captures the first non-"not found" failure from pro.Remove
		// (e.g. a config write error). Those represent real failures we must
		// surface; bucketing them with "not found" would hide them behind a
		// PROFILE_NOT_FOUND envelope and a possibly-zero exit on mixed runs.
		var hardErr error
		lockErr := raid.WithMutationLock(func() error {
			for _, name := range args {
				err := pro.Remove(name)
				if err == nil {
					removed = append(removed, name)
					continue
				}
				if rErr, ok := errs.AsError(err); ok && rErr.Code() == errs.CodeProfileNotFound {
					failures = append(failures, removeResultErr{Name: name, Reason: err.Error()})
					continue
				}
				// Non-"not found" failure — stop iterating; don't keep
				// hammering a broken config.
				if hardErr == nil {
					hardErr = err
				}
				return nil
			}
			return nil
		})
		if lockErr != nil {
			return errs.Wrap(lockErr)
		}
		if hardErr != nil {
			return errs.Wrap(hardErr)
		}

		// Build the structured-error sentinel (used in both text and JSON
		// modes) so a "every requested profile was missing" run exits with
		// a PROFILE_NOT_FOUND code regardless of output format.
		var allMissingErr error
		if len(removed) == 0 && len(failures) > 0 {
			if len(failures) == 1 {
				allMissingErr = errs.ProfileNotFound(failures[0].Name)
			} else {
				allMissingErr = errs.ProfileNotFound(failures[0].Name + " (and " + fmt.Sprintf("%d", len(failures)-1) + " more)")
			}
		}

		if jsonMode(cmd) {
			if err := emitJSON(cmd, removeResult{Removed: removed, Errors: failures}); err != nil {
				return err
			}
			// Emit the JSON envelope then still surface the structured
			// error so the process exits non-zero when every requested
			// profile was missing. Mixed success keeps exit 0.
			return allMissingErr
		}

		out := cmd.OutOrStdout()
		for _, name := range removed {
			fmt.Fprintf(out, "Profile '%s' has been removed.\n", name)
		}
		for _, f := range failures {
			fmt.Fprintf(out, "Profile '%s' not found. Use 'raid profile list' to see available profiles.\n", f.Name)
		}
		return allMissingErr
	},
}
