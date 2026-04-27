// Emit a condensed summary of the active raid workspace.
package context

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/8bitalex/raid/src/raid/context"
	"github.com/spf13/cobra"
)

var jsonOutput bool

func init() {
	Command.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON output")
}

// Command is the `raid context` subcommand. It prints a condensed snapshot of
// the active workspace — profile, environment, and per-repo branch / dirty
// state — for human or agent consumption.
var Command = &cobra.Command{
	Use:   "context",
	Short: "Print a condensed summary of the active workspace",
	Long:  "Print a condensed, token-efficient snapshot of the active workspace: profile, environment, and per-repository git state. Use --json for machine-readable output.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ws := context.Get()
		if jsonOutput {
			return writeJSON(cmd.OutOrStdout(), ws)
		}
		return writePretty(cmd.OutOrStdout(), ws)
	},
}

func writeJSON(w io.Writer, ws context.Workspace) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(ws)
}

func writePretty(w io.Writer, ws context.Workspace) error {
	if ws.Profile == "" {
		fmt.Fprintln(w, "No active profile.")
		return nil
	}

	fmt.Fprintf(w, "Profile: %s\n", ws.Profile)
	if ws.Env != "" {
		fmt.Fprintf(w, "Env:     %s\n", ws.Env)
	}

	if len(ws.Repos) == 0 {
		fmt.Fprintln(w, "\nNo repositories configured.")
		return nil
	}

	pathWidth, branchWidth := 0, 0
	for _, r := range ws.Repos {
		if len(r.Path) > pathWidth {
			pathWidth = len(r.Path)
		}
		if len(r.Branch) > branchWidth {
			branchWidth = len(r.Branch)
		}
	}

	fmt.Fprintf(w, "\nRepos (%d):\n", len(ws.Repos))
	for _, r := range ws.Repos {
		status := repoStatus(r)
		fmt.Fprintf(w, "  %-*s  %-*s  %-*s  %s\n",
			nameWidth(ws.Repos), r.Name,
			pathWidth, r.Path,
			branchWidth, r.Branch,
			status,
		)
	}
	return nil
}

func nameWidth(repos []context.Repo) int {
	w := 0
	for _, r := range repos {
		if len(r.Name) > w {
			w = len(r.Name)
		}
	}
	return w
}

func repoStatus(r context.Repo) string {
	if !r.Cloned {
		return "not cloned"
	}
	if r.Dirty {
		return "dirty"
	}
	return "clean"
}
