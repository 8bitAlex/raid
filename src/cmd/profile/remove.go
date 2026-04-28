package profile

import (
	"fmt"

	"github.com/8bitalex/raid/src/raid"
	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

var RemoveProfileCmd = &cobra.Command{
	Use:        "remove profile",
	Short:      "Remove profile(s)",
	SuggestFor: []string{"delete"},
	Args:       cobra.MinimumNArgs(1),
	// RunE so lock-acquisition failures (e.g. ~/.raid is read-only or the
	// process can't reach it) propagate up to cobra and produce a non-zero
	// exit instead of being silently swallowed.
	RunE: func(cmd *cobra.Command, args []string) error {
		return raid.WithMutationLock(func() error {
			for _, name := range args {
				if err := pro.Remove(name); err != nil {
					fmt.Printf("Profile '%s' not found. Use 'raid profile list' to see available profiles.\n", name)
				} else {
					fmt.Printf("Profile '%s' has been removed.\n", name)
				}
			}
			return nil
		})
	},
}
