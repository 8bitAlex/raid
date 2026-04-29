package profile

import (
	"encoding/json"
	"fmt"

	pro "github.com/8bitalex/raid/src/raid/profile"
	"github.com/spf13/cobra"
)

var listJSON bool

func init() {
	ListProfileCmd.Flags().BoolVar(&listJSON, "json", false, "Emit machine-readable JSON output")
}

// profileEntry is the stable JSON shape for a single profile in `--json` mode.
// Field names and types are part of the public CLI contract.
type profileEntry struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Active bool   `json:"active"`
}

var ListProfileCmd = &cobra.Command{
	Use:   "list",
	Short: "List profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles := pro.ListAll()
		activeProfile := pro.Get()

		if listJSON {
			out := make([]profileEntry, 0, len(profiles))
			for _, p := range profiles {
				out = append(out, profileEntry{
					Name:   p.Name,
					Path:   p.Path,
					Active: p.Name == activeProfile.Name,
				})
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}

		if len(profiles) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No profiles found.")
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Available profiles:")
		for _, profile := range profiles {
			activeIndicator := ""
			if profile.Name == activeProfile.Name {
				activeIndicator = " (active)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\t%s%s\t%s\n", profile.Name, activeIndicator, profile.Path)
		}
		return nil
	},
}
