package profile

import (
	"fmt"

	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/errs"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(AddProfileCmd)
	Command.AddCommand(CreateProfileCmd)
	Command.AddCommand(ListProfileCmd)
	Command.AddCommand(RemoveProfileCmd)
}

// activeResult is the JSON shape for `raid profile` (no args) and the
// switch path. Both surface the currently active profile name; the
// switch path additionally signals the action so JSON consumers can
// distinguish a no-op probe from a successful switch.
type activeResult struct {
	Action string `json:"action,omitempty"` // "active" | "switched"
	Name   string `json:"name,omitempty"`
	Path   string `json:"path,omitempty"`
}

var Command = &cobra.Command{
	Use:     "profile",
	Aliases: []string{"p"},
	Short:   "Manage raid profiles",
	Args:    cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		json := jsonMode(cmd)

		// No-arg form: show the active profile (or surface that none is set).
		if len(args) == 0 {
			profile := pro.Get()
			if profile.IsZero() {
				if json {
					return emitJSON(cmd, activeResult{Action: "active"})
				}
				fmt.Fprintln(out, "No active profile found. Use 'raid profile <name>' to set one.")
				return nil
			}
			if json {
				return emitJSON(cmd, activeResult{Action: "active", Name: profile.Name, Path: profile.Path})
			}
			fmt.Fprintln(out, profile.Name)
			return nil
		}

		// One-arg form: switch the active profile.
		name := args[0]
		err := raid.WithMutationLock(func() error {
			return pro.Set(name)
		})
		if err != nil {
			return errs.Wrap(err)
		}
		switched := pro.Get()
		if json {
			return emitJSON(cmd, activeResult{Action: "switched", Name: switched.Name, Path: switched.Path})
		}
		fmt.Fprintf(out, "Profile '%s' is now active.\n", name)
		return nil
	},
}
