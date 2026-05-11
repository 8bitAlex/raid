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
	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/8bitalex/raid/src/internal/sys"
	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

// reservedNames are built-in cobra subcommands that custom commands cannot shadow.
var reservedNames = map[string]bool{
	"profile":    true,
	"install":    true,
	"env":        true,
	"doctor":     true,
	"context":    true,
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
	// Subcommands
	rootCmd.AddCommand(profile.Command)
	rootCmd.AddCommand(install.Command)
	rootCmd.AddCommand(env.Command)
	rootCmd.AddCommand(doctor.Command)
	rootCmd.AddCommand(contextcmd.Command)
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

	if err := rootCmd.Execute(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintln(os.Stderr, "raid:", err)
		return 1
	}
	return 0
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
