package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	contextcmd "github.com/8bitalex/raid/src/cmd/context"
	"github.com/8bitalex/raid/src/cmd/doctor"
	"github.com/8bitalex/raid/src/cmd/env"
	"github.com/8bitalex/raid/src/cmd/install"
	"github.com/8bitalex/raid/src/cmd/profile"
	telemetrycmd "github.com/8bitalex/raid/src/cmd/telemetry"
	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/internal/telemetry"
	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/errs"
	"github.com/spf13/cobra"
)

// reservedNames are built-in cobra subcommands that custom commands cannot shadow.
var reservedNames = map[string]bool{
	"profile":    true,
	"install":    true,
	"env":        true,
	"doctor":     true,
	"context":    true,
	"telemetry":  true,
	"help":       true,
	"version":    true,
	"completion": true,
}

var rootCmd = &cobra.Command{
	Use:   "raid",
	Short: "Raid is a tool for orchestrating common tasks across your development environment(s).",
	Args:  cobra.NoArgs,
}

func init() {
	version, err := raid.GetProperty(raid.PropertyVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, "raid: app.properties:", err)
		os.Exit(1)
	}
	rootCmd.Version = version
	rootCmd.Long = "Raid v" + version + "\n\nRaid is a configurable command-line application that orchestrates common development tasks, environments, and dependencies across distributed code repositories."
	// Global Flags
	rootCmd.PersistentFlags().StringVarP(raid.ConfigPath, raid.ConfigPathFlag, raid.ConfigPathFlagShort, "", raid.ConfigPathFlagDesc)
	rootCmd.PersistentFlags().Bool("json", false, "Emit JSON output for scriptable / agent consumption (where supported)")
	rootCmd.PersistentFlags().BoolP("yes", "y", false, "Auto-resolve interactive prompts (Confirm/Prompt tasks). Confirm auto-accepts; Prompt uses its `default:` or fails with HEADLESS_PROMPT_NO_DEFAULT.")
	rootCmd.PersistentFlags().Bool("headless", false, "Alias for --yes; intended for CI, scheduled runs, and agent hosts. Also enabled by setting RAID_HEADLESS=1 in the environment.")
	rootCmd.PersistentFlags().Bool("no-prefix", false, "Disable per-task output prefixing in concurrent runs. Equivalent to RAID_NO_PREFIX=1.")
	rootCmd.PersistentPreRunE = applyPersistentEnvFlags
	// Subcommands
	rootCmd.AddCommand(profile.Command)
	rootCmd.AddCommand(install.Command)
	rootCmd.AddCommand(env.Command)
	rootCmd.AddCommand(doctor.Command)
	rootCmd.AddCommand(contextcmd.Command)
	rootCmd.AddCommand(telemetrycmd.Command)
}

// isInfoCommand reports whether the invocation is for a built-in informational
// command (help, version, completion) that does not require a loaded profile.
func isInfoCommand(args []string) bool {
	if len(args) <= 1 {
		return true
	}
	for _, arg := range args[1:] {
		if arg == "--" {
			break
		}
		switch arg {
		case "help", "version", "completion", "--help", "-h", "--version", "-v":
			return true
		}
	}
	return false
}

// Injectable dependencies for testing.
var (
	latestReleaseFn    = sys.LatestGitHubRelease
	latestPreReleaseFn = sys.LatestGitHubPreRelease
	osExit             = os.Exit
)

func Execute() {
	code := executeRoot(os.Args)
	if code != 0 {
		osExit(code)
	}
}

// executeRoot performs the full Execute flow and returns an exit code.
// Returns 0 on success. Extracted from Execute() so tests can observe the
// exit code without os.Exit terminating the test process.
func executeRoot(args []string) int {
	info := isInfoCommand(args)
	environment, _ := raid.GetProperty(raid.PropertyEnvironment)

	// Start version check early so network latency overlaps with initialization.
	updateCh := make(chan string, 1)
	go func() {
		if raid.Environment(environment) == raid.EnvironmentPreview {
			updateCh <- latestPreReleaseFn("8bitalex/raid")
		} else {
			updateCh <- latestReleaseFn("8bitalex/raid")
		}
	}()

	applyConfigFlag(args)
	var cmds []lib.Command
	if info {
		// Best-effort, read-only load: no config file creation, no warnings.
		// User commands will appear in help if the config already exists.
		cmds = raid.QuietGetCommands()
	} else {
		raid.Initialize()
		cmds = raid.GetCommands()
	}

	// For info commands wait up to 1.5s; for regular commands do a non-blocking
	// check so startup is not delayed.
	var latest string
	if info {
		select {
		case v := <-updateCh:
			latest = v
		case <-time.After(1500 * time.Millisecond):
		}
	} else {
		select {
		case v := <-updateCh:
			latest = v
		default:
		}
	}

	version, _ := raid.GetProperty(raid.PropertyVersion)
	if latest != "" && latest != baseVersion(version) {
		var label string
		if raid.Environment(environment) == raid.EnvironmentPreview {
			label = "Preview update"
		} else {
			label = "Update available"
		}
		notice := sys.Yellow("(" + label + ": v" + version + " → v" + latest + ")")
		rootCmd.Long = strings.Replace(rootCmd.Long, "Raid v"+version, "Raid v"+version+" "+notice, 1)
		if !info {
			fmt.Fprintf(os.Stderr, "Raid v%s %s\n", version, notice)
		}
	}

	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true
	registerUserCommands(rootCmd, cmds)
	registerRepoCommands(rootCmd, raid.GetRepos())

	// Reset args so cobra parses from our slice rather than os.Args when the
	// caller is providing a different args list (e.g. during tests).
	rootCmd.SetArgs(args[1:])

	// First-run consent prompt for telemetry. Runs only for non-info,
	// non-telemetry-subcommand invocations to avoid prompting on
	// `raid --help`, `raid telemetry on`, and similar.
	//
	// Skip signals split into two tiers. Persistent — `--yes` /
	// `--headless`, `RAID_HEADLESS=1`, non-TTY stdin, DO_NOT_TRACK —
	// all reflect "this host is non-interactive long-term" and cause
	// MaybePromptForConsent to persist decided=off so we don't keep
	// re-prompting. Transient — `--json` — is just "this one command
	// wants machine-readable output" and must NOT poison the consent
	// state. The split was added after a real bug where one
	// `raid context --json | jq` silently opted the user out for life.
	if !info && !isTelemetrySubcommand(args) {
		skipPersistent := headlessFromArgs(args) || lib.IsHeadless()
		skipTransient := jsonModeFromArgs(args)
		switch telemetry.MaybePromptForConsent(skipPersistent, skipTransient) {
		case telemetry.PromptAccepted:
			// User opted in via the first-run prompt — fire the
			// adoption event so `raid telemetry on` and the prompt
			// path both produce raid_first_run. Synchronous so the
			// event lands even if the user's command crashes before
			// Flush runs.
			telemetry.CaptureSync(telemetry.EventFirstRun, telemetry.FirstRunProps(""))
		}
	}

	// Flush any pending telemetry events before exit so async sends
	// don't get dropped when raid returns. The deadline is short so a
	// stuck network can't drag out shutdown.
	defer telemetry.Flush(1500 * time.Millisecond)

	if err := rootCmd.Execute(); err != nil {
		rErr, isStructured := errs.AsError(err)
		var exitErr *exec.ExitError
		hasExit := errors.As(err, &exitErr)

		// A plain exec.ExitError (no structured wrapping) means the
		// subprocess already printed its own stderr — don't double-report,
		// just propagate the subprocess exit status so $? matches.
		if hasExit && !isStructured {
			return exitErr.ExitCode()
		}

		// Structured errors (including ones that wrap exec.ExitError, e.g.
		// TASK_SHELL_FAILED) should still surface their code/hint/JSON so
		// --json consumers see the error shape. After emitting, preserve
		// the subprocess exit code when one is available so shell pipelines
		// keep working.
		if jsonModeFromArgs(args) {
			errs.EmitJSON(os.Stderr, err)
		} else {
			fmt.Fprintln(os.Stderr, "raid:", err)
			if isStructured && rErr.Hint() != "" {
				fmt.Fprintln(os.Stderr, "hint:", rErr.Hint())
			}
		}
		if hasExit {
			return exitErr.ExitCode()
		}
		return errs.ExitCode(err)
	}
	return 0
}

// applyPersistentEnvFlags is rootCmd's PersistentPreRunE. It runs
// after cobra resolves persistent flags but before any subcommand's
// RunE, so it's the right hook to translate flag-driven toggles
// into the env vars that lib reads.
//
// Setting an env var instead of a Go variable keeps lib free of any
// cobra dependency — and the env var is also the documented
// programmatic entry point for callers that bypass the CLI (CI
// runners, agent hosts) so a single read site in lib serves both.
//
// We only set each var when its corresponding flag is true; existing
// values in the environment are left untouched so external callers
// can still flip the toggles on without passing flags.
func applyPersistentEnvFlags(cmd *cobra.Command, _ []string) error {
	yes, _ := cmd.Flags().GetBool("yes")
	headless, _ := cmd.Flags().GetBool("headless")
	if yes || headless {
		os.Setenv(lib.HeadlessEnvVar, "1")
	}
	noPrefix, _ := cmd.Flags().GetBool("no-prefix")
	if noPrefix {
		os.Setenv(lib.NoPrefixEnvVar, "1")
	}
	return nil
}

// applyHeadlessFlag is the pre-rename alias kept as a thin shim so
// existing tests (and any external pinning) continue to work. New
// code should reference applyPersistentEnvFlags.
//
// Deprecated: use applyPersistentEnvFlags.
func applyHeadlessFlag(cmd *cobra.Command, args []string) error {
	return applyPersistentEnvFlags(cmd, args)
}

// isTelemetrySubcommand reports whether the user invoked one of the
// `raid telemetry ...` subcommands. The first-run consent prompt must
// skip these — prompting "do you want telemetry?" right before
// running `raid telemetry on` is hostile UX, and the off/status/
// purge/preview commands need to work for users who haven't opted in.
//
// Flag-aware: persistent flags that take a value (`--config <path>`
// / `-c <path>`) consume the following token, so an invocation like
// `raid --config telemetry install` should resolve to the `install`
// subcommand and not be misread as `telemetry`. The bool persistent
// flags (`--json`, `--yes`/`-y`, `--headless`) do not consume a
// following token.
func isTelemetrySubcommand(args []string) bool {
	skipNext := false
	for _, a := range args[1:] {
		if a == "--" {
			return false
		}
		if skipNext {
			skipNext = false
			continue
		}
		if strings.HasPrefix(a, "-") {
			// Only the value-taking root flags consume the next
			// token (and only in their bare form — `--config=path`
			// keeps the value attached).
			if a == "--config" || a == "-c" {
				skipNext = true
			}
			continue
		}
		return a == "telemetry"
	}
	return false
}

// headlessFromArgs is the early-scan counterpart to jsonModeFromArgs
// for the headless persistent flag. The first-run prompt needs to
// know the user's headless intent before cobra has parsed flags so
// it can skip prompting in non-interactive contexts. Matches every
// flag form: `-y`, `--yes`, `--yes=true`, `--headless`,
// `--headless=true`, plus their explicit `=false` opt-outs.
//
// Mirrors pflag's "last value wins" behavior: if the user passes
// `--yes=true --yes=false`, the parsed value is false, so this scan
// must also resolve to false. We walk the full arg list and only
// commit the final occurrence.
func headlessFromArgs(args []string) bool {
	out := false
	for _, a := range args[1:] {
		if a == "--" {
			break
		}
		switch {
		case a == "-y", a == "--yes", a == "--yes=true", a == "--headless", a == "--headless=true":
			out = true
		case a == "--yes=false", a == "--headless=false":
			out = false
		}
	}
	return out
}

// jsonModeFromArgs reports whether the user passed `--json` (or
// `--json=true`) anywhere in args. Cobra resolves persistent flags
// during Execute, but on the error path we need to know before falling
// out of the dispatch loop — and we want JSON even when the failure
// happens before flag parsing completes.
func jsonModeFromArgs(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		switch {
		case a == "--json", a == "--json=true":
			return true
		case a == "--json=false":
			return false
		}
	}
	return false
}

// CommandSourceAnnotation tags a cobra.Command with how it was registered:
// CommandSourceUser for profile-defined commands, absent for raid built-ins.
// `raid context` reads this to keep its built-in tool list separate from the
// user's commands.
const (
	CommandSourceAnnotation = "raid:source"
	CommandSourceUser       = "user"
)

// registerUserCommands adds user-defined commands to root.
// Reserved built-in names are skipped with a warning.
func registerUserCommands(root *cobra.Command, cmds []lib.Command) {
	for _, cmd := range cmds {
		if reservedNames[cmd.Name] {
			fmt.Fprintf(os.Stderr, "warning: command '%s' conflicts with a built-in subcommand and will be ignored\n", cmd.Name)
			continue
		}
		name := cmd.Name
		def := cmd
		coCmd := &cobra.Command{
			Use:         buildCommandUse(name, def.Args),
			Short:       cmd.Usage,
			Annotations: map[string]string{CommandSourceAnnotation: CommandSourceUser},
		}
		attachCommandArgsAndFlags(coCmd, def)
		coCmd.RunE = func(c *cobra.Command, args []string) error {
			named := gatherCommandValues(c, def, args)
			return raid.WithMutationLock(func() error {
				return raid.ExecuteCommand(name, args, named)
			})
		}
		root.AddCommand(coCmd)
	}
}

// registerRepoCommands adds one subcommand per repository that has commands.
// Each repo subcommand exposes the repo's own commands as sub-subcommands,
// letting users run `raid <repo> <command>` to target a specific repo even
// when a profile-level command has the same name.
func registerRepoCommands(root *cobra.Command, repos []lib.Repo) {
	for _, repo := range repos {
		if len(repo.Commands) == 0 {
			continue
		}
		if reservedNames[repo.Name] {
			fmt.Fprintf(os.Stderr, "warning: repository '%s' conflicts with a built-in subcommand and will not be available as a command namespace\n", repo.Name)
			continue
		}
		if hasCommand(root, repo.Name) {
			fmt.Fprintf(os.Stderr, "warning: repository '%s' conflicts with a user command and will not be available as a command namespace\n", repo.Name)
			continue
		}
		repoName := repo.Name
		repoCmd := &cobra.Command{
			Use:   repoName,
			Short: fmt.Sprintf("Commands for the %s repository", repoName),
		}
		for _, cmd := range repo.Commands {
			cmdName := cmd.Name
			def := cmd
			subCmd := &cobra.Command{
				Use:   buildCommandUse(cmdName, def.Args),
				Short: cmd.Usage,
			}
			attachCommandArgsAndFlags(subCmd, def)
			subCmd.RunE = func(c *cobra.Command, args []string) error {
				named := gatherCommandValues(c, def, args)
				return raid.WithMutationLock(func() error {
					return raid.ExecuteRepoCommand(repoName, cmdName, args, named)
				})
			}
			repoCmd.AddCommand(subCmd)
		}
		root.AddCommand(repoCmd)
	}
}

// buildCommandUse renders the cobra Use string with declared positional
// args so `--help` shows the expected invocation shape, e.g.
// `patch <ticket> [comment]`.
func buildCommandUse(name string, args []lib.Arg) string {
	if len(args) == 0 {
		return name
	}
	var b strings.Builder
	b.WriteString(name)
	for _, a := range args {
		b.WriteByte(' ')
		if a.Required {
			b.WriteString("<")
			b.WriteString(a.Name)
			b.WriteString(">")
		} else {
			b.WriteString("[")
			b.WriteString(a.Name)
			b.WriteString("]")
		}
	}
	return b.String()
}

// attachCommandArgsAndFlags configures a cobra subcommand from the lib.Command
// definition: positional-arg cardinality validators and declared flags. Type
// defaults to "string" when unset; "bool" / "int" are also recognised. A
// missing or wrong-typed Default falls back to the zero value rather than
// failing — the schema's `oneOf` already guards the YAML side.
func attachCommandArgsAndFlags(co *cobra.Command, cmd lib.Command) {
	if n := len(cmd.Args); n > 0 {
		req := 0
		for _, a := range cmd.Args {
			if a.Required {
				req++
			}
		}
		switch {
		case req == n:
			co.Args = cobra.ExactArgs(n)
		case req == 0:
			co.Args = cobra.MaximumNArgs(n)
		default:
			co.Args = cobra.RangeArgs(req, n)
		}
	}

	for _, f := range cmd.Flags {
		switch f.Type {
		case "bool":
			d, _ := f.Default.(bool)
			co.Flags().BoolP(f.Name, f.Short, d, f.Usage)
		case "int":
			d := 0
			switch v := f.Default.(type) {
			case int:
				d = v
			case float64:
				// YAML numbers unmarshal into float64 through interface{}.
				d = int(v)
			}
			co.Flags().IntP(f.Name, f.Short, d, f.Usage)
		default:
			d, _ := f.Default.(string)
			co.Flags().StringP(f.Name, f.Short, d, f.Usage)
		}
		if f.Required {
			_ = co.MarkFlagRequired(f.Name)
		}
	}
}

// gatherCommandValues builds the name → value map that ExecuteCommand binds
// to env vars. Returns nil when the command has no declared args/flags so
// the legacy positional-only path stays untouched.
func gatherCommandValues(co *cobra.Command, cmd lib.Command, posArgs []string) map[string]string {
	if len(cmd.Args) == 0 && len(cmd.Flags) == 0 {
		return nil
	}
	out := make(map[string]string, len(cmd.Args)+len(cmd.Flags))
	for i, a := range cmd.Args {
		if i < len(posArgs) {
			out[a.Name] = posArgs[i]
		}
	}
	for _, f := range cmd.Flags {
		switch f.Type {
		case "bool":
			v, _ := co.Flags().GetBool(f.Name)
			if v {
				out[f.Name] = "true"
			} else {
				out[f.Name] = "false"
			}
		case "int":
			v, _ := co.Flags().GetInt(f.Name)
			out[f.Name] = strconv.Itoa(v)
		default:
			v, _ := co.Flags().GetString(f.Name)
			out[f.Name] = v
		}
	}
	return out
}

func hasCommand(root *cobra.Command, name string) bool {
	for _, c := range root.Commands() {
		if c.Name() == name {
			return true
		}
	}
	return false
}

// baseVersion strips "-preview" and anything following it from a version string
// so that preview builds compare correctly against their corresponding release tag.
func baseVersion(version string) string {
	base, _, _ := strings.Cut(version, "-preview")
	return base
}

// applyConfigFlag scans args for --config / -c so the config path is set
// before the pre-initialization call, matching cobra's later flag parsing.
// Scanning stops at the first -- end-of-flags marker.
func applyConfigFlag(args []string) {
	for i, arg := range args {
		if arg == "--" {
			return
		}
		if v, ok := strings.CutPrefix(arg, "--config="); ok {
			*raid.ConfigPath = v
			return
		}
		if v, ok := strings.CutPrefix(arg, "-c="); ok {
			*raid.ConfigPath = v
			return
		}
		if (arg == "--config" || arg == "-c") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			*raid.ConfigPath = args[i+1]
			return
		}
	}
}
