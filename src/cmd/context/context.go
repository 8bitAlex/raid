// Emit a condensed summary of the active raid workspace.
package context

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

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

	writeRepos(w, ws.Repos)
	writeCommands(w, ws.Commands)
	writeRecent(w, ws.Recent)
	return nil
}

func writeRepos(w io.Writer, repos []context.Repo) {
	if len(repos) == 0 {
		fmt.Fprintln(w, "\nRepos: none configured")
		return
	}

	nameW, pathW, branchW := 0, 0, 0
	for _, r := range repos {
		if len(r.Name) > nameW {
			nameW = len(r.Name)
		}
		if len(r.Path) > pathW {
			pathW = len(r.Path)
		}
		if len(r.Branch) > branchW {
			branchW = len(r.Branch)
		}
	}

	fmt.Fprintf(w, "\nRepos (%d):\n", len(repos))
	for _, r := range repos {
		fmt.Fprintf(w, "  %-*s  %-*s  %-*s  %s\n",
			nameW, r.Name,
			pathW, r.Path,
			branchW, r.Branch,
			repoStatus(r),
		)
	}
}

func writeCommands(w io.Writer, cmds []context.Command) {
	if len(cmds) == 0 {
		return
	}
	nameW := 0
	for _, c := range cmds {
		if len(c.Name) > nameW {
			nameW = len(c.Name)
		}
	}
	fmt.Fprintf(w, "\nCommands (%d):\n", len(cmds))
	for _, c := range cmds {
		fmt.Fprintf(w, "  %-*s  %s\n", nameW, c.Name, c.Description)
	}
}

func writeRecent(w io.Writer, entries []context.Recent) {
	if len(entries) == 0 {
		return
	}
	nameW := 0
	for _, e := range entries {
		if len(e.Command) > nameW {
			nameW = len(e.Command)
		}
	}
	fmt.Fprintf(w, "\nRecent (%d):\n", len(entries))
	now := time.Now()
	for _, e := range entries {
		mark := "✓"
		if e.ExitCode != 0 {
			mark = "✗"
		}
		fmt.Fprintf(w, "  %s %-*s  %6s  %s\n",
			mark,
			nameW, e.Command,
			formatDuration(e.DurationMs),
			relativeTime(now, e.StartedAt),
		)
	}
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

func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", ms)
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func relativeTime(now, then time.Time) string {
	if then.IsZero() {
		return ""
	}
	d := now.Sub(then)
	switch {
	case d < 30*time.Second:
		return "just now"
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
