// Emit a condensed summary of the active raid workspace.
package context

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
	"unicode/utf8"

	"github.com/8bitalex/raid/src/raid/context"
	"github.com/spf13/cobra"
)

var jsonOutput bool

func init() {
	Command.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON output")
}

// Command is the `raid context` subcommand. It prints a condensed snapshot of
// the active workspace — profile, environment, and per-repo branch / dirty
// state — for human or agent consumption, and hosts the `serve` subcommand
// that runs the same data live as an MCP server over stdio.
var Command = &cobra.Command{
	Use:   "context",
	Short: "Print a condensed summary of the active workspace, or run as an MCP server",
	Long:  "Print a condensed, token-efficient snapshot of the active workspace: profile, environment, and per-repository git state. Use --json for machine-readable output.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		ws := context.Get()
		ws.Tools = collectTools(cmd.Root())
		if jsonOutput {
			return writeJSON(cmd.OutOrStdout(), ws)
		}
		return writePretty(cmd.OutOrStdout(), ws)
	},
}

// ignoredToolNames are cobra-builtin subcommands not part of raid's own
// surface area; an agent should reach for the raid commands instead.
var ignoredToolNames = map[string]bool{
	"help":       true,
	"completion": true,
}

// These mirror cmd.CommandSourceAnnotation / cmd.CommandSourceUser. The
// constants are duplicated rather than imported because cmd already imports
// cmd/context, so pulling cmd back the other way would create a cycle. If you
// rename the values in cmd/raid.go, update them here too.
const (
	cmdSourceAnnotation = "raid:source"
	cmdSourceUser       = "user"
)

// collectTools walks root's direct subcommands and returns the raid built-in
// commands. User-defined commands (annotated by registerUserCommands) and
// cobra's auto-added help/completion commands are excluded — user commands
// are already exposed via WorkspaceContext.Commands and shouldn't appear in
// the built-in tools list.
func collectTools(root *cobra.Command) []context.Tool {
	var tools []context.Tool
	for _, sub := range root.Commands() {
		if ignoredToolNames[sub.Name()] || sub.Hidden {
			continue
		}
		if sub.Annotations[cmdSourceAnnotation] == cmdSourceUser {
			continue
		}
		tools = append(tools, context.Tool{
			Name:        sub.Name(),
			Description: sub.Short,
		})
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

func writeJSON(w io.Writer, ws context.Snapshot) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(ws)
}

func writePretty(w io.Writer, ws context.Snapshot) error {
	writeHeader(w, ws)
	if ws.Workspace.Profile == "" {
		fmt.Fprintln(w, "No active profile.")
		return nil
	}

	fmt.Fprintf(w, "Profile: %s\n", ws.Workspace.Profile)
	if ws.Workspace.Env != "" {
		fmt.Fprintf(w, "Env:     %s\n", ws.Workspace.Env)
	}

	writeRepos(w, ws.Workspace.Repos)
	writeTools(w, ws.Tools)
	writeCommands(w, ws.Workspace.Commands)
	writeRecent(w, ws.Workspace.Recent, ws.GeneratedAt)
	return nil
}

func writeTools(w io.Writer, tools []context.Tool) {
	if len(tools) == 0 {
		return
	}
	nameW := 0
	for _, t := range tools {
		if len(t.Name) > nameW {
			nameW = len(t.Name)
		}
	}
	fmt.Fprintf(w, "\nTools (%d):\n", len(tools))
	for _, t := range tools {
		fmt.Fprintf(w, "  %-*s  %s\n", nameW, t.Name, t.Description)
	}
}

// writeHeader emits an agent-readable preamble so the snapshot is
// self-describing when piped or pasted out of context. The website URL is
// included as a discoverable entry point an agent can follow for additional
// documentation, issue reporting, or source code search.
func writeHeader(w io.Writer, ws context.Snapshot) {
	name := ws.Name
	if name == "" {
		name = "raid"
	}
	if ws.Version != "" {
		fmt.Fprintf(w, "# %s v%s workspace context (%s)\n", name, ws.Version, ws.GeneratedAt.Format(time.RFC3339))
	} else {
		fmt.Fprintf(w, "# %s workspace context (%s)\n", name, ws.GeneratedAt.Format(time.RFC3339))
	}
	if ws.WebsiteUrl != "" {
		fmt.Fprintf(w, "# %s\n", ws.WebsiteUrl)
	}
	fmt.Fprintln(w)
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
		for i, step := range c.Steps {
			fmt.Fprintf(w, "  %-*s  %d. %s\n", nameW, "", i+1, step.Name)
		}
	}
}

// writeRecent renders the recent-runs section. now should be the snapshot's
// own GeneratedAt timestamp so the "Xm ago" deltas are consistent with the
// header timestamp and stay deterministic for any consumer that re-renders
// the same Snapshot.
func writeRecent(w io.Writer, entries []context.Recent, now time.Time) {
	if len(entries) == 0 {
		return
	}
	nameW, statusW, durationW := 0, 0, 0
	statuses := make([]string, len(entries))
	durations := make([]string, len(entries))
	for i, e := range entries {
		if len(e.Command) > nameW {
			nameW = len(e.Command)
		}
		statuses[i] = recentStatusText(e)
		if w := utf8.RuneCountInString(statuses[i]); w > statusW {
			statusW = w
		}
		durations[i] = recentDuration(e)
		if w := utf8.RuneCountInString(durations[i]); w > durationW {
			durationW = w
		}
	}
	fmt.Fprintf(w, "\nRecent (%d):\n", len(entries))
	for i, e := range entries {
		fmt.Fprintf(w, "  %s %-*s  %s%s  %s%s  %s\n",
			recentMark(e),
			nameW, e.Command,
			statuses[i], padRunes(statuses[i], statusW),
			durations[i], padRunes(durations[i], durationW),
			relativeTime(now, e.StartedAt),
		)
	}
}

func recentStatusText(e context.Recent) string {
	switch e.Status {
	case context.RecentStatusInterrupted:
		return "interrupted"
	default:
		if e.ExitCode != 0 {
			return "failed"
		}
		return "ok"
	}
}

// padRunes returns spaces needed to right-pad s to width visible runes. Used
// instead of "%-*s" because that flag pads by byte count, which over-pads
// non-ASCII content (e.g. the 3-byte em-dash placeholder).
func padRunes(s string, width int) string {
	n := width - utf8.RuneCountInString(s)
	if n <= 0 {
		return ""
	}
	pad := make([]byte, n)
	for i := range pad {
		pad[i] = ' '
	}
	return string(pad)
}

func recentMark(e context.Recent) string {
	switch e.Status {
	case context.RecentStatusInterrupted:
		return "⊘"
	default:
		if e.ExitCode != 0 {
			return "✗"
		}
		return "✓"
	}
}

func recentDuration(e context.Recent) string {
	if e.Status == context.RecentStatusInterrupted {
		return "—"
	}
	return formatDuration(e.DurationMs)
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
