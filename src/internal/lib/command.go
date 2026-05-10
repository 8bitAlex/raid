package lib

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/8bitalex/raid/src/internal/sys"
)

// Command is a named, user-defined CLI command that can be invoked via 'raid <name>'.
type Command struct {
	Name  string   `json:"name"`
	Usage string   `json:"usage"`
	Args  []Arg    `json:"args,omitempty"`
	Flags []Flag   `json:"flags,omitempty"`
	Tasks []Task   `json:"tasks"`
	Out   *Output  `json:"out,omitempty"`
}

// Arg declares a positional argument for a custom command. The supplied value
// is bound to the env var named after Name (uppercased) for the duration of
// the command, so tasks can reference it as `$NAME`. Required args without a
// matching positional value cause cobra to reject the invocation.
type Arg struct {
	Name     string `json:"name"`
	Usage    string `json:"usage,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// Flag declares a long-form (--name) and optional short-form (-x) flag for a
// custom command. Type is one of "string" (default), "bool", or "int".
// Required flags are enforced by cobra. Default supplies the value when the
// flag is omitted; bool flags default to false unless overridden.
type Flag struct {
	Name     string `json:"name"`
	Short    string `json:"short,omitempty"`
	Usage    string `json:"usage,omitempty"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
	Default  any    `json:"default,omitempty"`
}

// Output configures how a command's task output is handled.
// Stdout and Stderr default to true when Out is nil.
// When Out is set, only streams explicitly set to true are shown.
type Output struct {
	Stdout bool   `json:"stdout"`
	Stderr bool   `json:"stderr"`
	File   string `json:"file,omitempty"`
}

// IsZero reports whether the command is uninitialized.
func (c Command) IsZero() bool {
	return c.Name == ""
}

// GetCommands returns all commands available in the active profile.
func GetCommands() []Command {
	if context == nil {
		return nil
	}
	return context.Profile.Commands
}

// GetRepos returns the repositories in the active profile.
func GetRepos() []Repo {
	if context == nil {
		return nil
	}
	return context.Profile.Repositories
}

// ExecuteCommand runs the tasks for the named command, applying any output configuration.
// Positional `args` are exposed as RAID_ARG_1, RAID_ARG_2, ... environment
// variables. When `named` is non-nil, each entry is also exported as a env
// var with the key uppercased — this is how cobra-parsed named arguments and
// flags reach task scripts. All bindings are unset after the command exits.
func ExecuteCommand(name string, args []string, named map[string]string) error {
	var found Command
	for _, cmd := range GetCommands() {
		if cmd.Name == name {
			found = cmd
			break
		}
	}
	if found.IsZero() {
		return fmt.Errorf("command '%s' not found", name)
	}

	cleanup := setCommandArgs(args, named)
	defer cleanup()

	startSession()
	defer endSession()

	startedAt := RecordRecentStart(found.Name)
	err := runCommand(found)
	RecordRecentEnd(found.Name, err, startedAt)
	return err
}

// ExecuteRepoCommand runs a command defined in a specific repository's raid.yaml.
// See ExecuteCommand for how `args` and `named` are bound to env vars.
func ExecuteRepoCommand(repoName, cmdName string, args []string, named map[string]string) error {
	repos := GetRepos()
	var repo *Repo
	for i := range repos {
		if repos[i].Name == repoName {
			repo = &repos[i]
			break
		}
	}
	if repo == nil {
		return fmt.Errorf("repository '%s' not found", repoName)
	}

	var found Command
	for _, cmd := range repo.Commands {
		if cmd.Name == cmdName {
			found = cmd
			break
		}
	}
	if found.IsZero() {
		return fmt.Errorf("command '%s' not found in repository '%s'", cmdName, repoName)
	}

	cleanup := setCommandArgs(args, named)
	defer cleanup()

	startSession()
	defer endSession()

	recentName := repoName + ":" + found.Name
	startedAt := RecordRecentStart(recentName)
	err := runCommand(found)
	RecordRecentEnd(recentName, err, startedAt)
	return err
}

// setCommandArgs binds positional args to RAID_ARG_N and named args/flags to
// sanitised, uppercased env vars for the lifetime of a command run. Returns
// a cleanup closure that restores any pre-existing values raid overwrote
// (or unsets entries that didn't exist) so a command declaring e.g.
// `name: PATH` doesn't permanently clobber the parent process's PATH.
//
// Names are normalised via sanitizeEnvName: lowercase → uppercase, anything
// outside [A-Za-z0-9_] becomes '_'. Names that sanitise to a non-identifier
// (empty / all-underscores) are skipped — the schema rejects these
// up-front via the `pattern` constraint, this is defence-in-depth for
// callers that construct lib.Command directly (tests, future MCP hooks).
func setCommandArgs(args []string, named map[string]string) func() {
	clearRaidArgs()
	for i, arg := range args {
		os.Setenv(fmt.Sprintf("RAID_ARG_%d", i+1), arg)
	}
	type prev struct {
		key      string
		oldValue string
		hadValue bool
	}
	snapshots := make([]prev, 0, len(named))
	for k, v := range named {
		key := sanitizeEnvName(k)
		if key == "" {
			continue
		}
		old, had := os.LookupEnv(key)
		snapshots = append(snapshots, prev{key, old, had})
		os.Setenv(key, v)
	}
	return func() {
		clearRaidArgs()
		for _, p := range snapshots {
			if p.hadValue {
				os.Setenv(p.key, p.oldValue)
			} else {
				os.Unsetenv(p.key)
			}
		}
	}
}

// sanitizeEnvName normalises an arg/flag name to a valid env var identifier.
// Mirrors sanitizeRepoVarName so RAID_REPO_* and command-arg/flag names use
// the same scheme. Returns "" for inputs that produce only underscores
// (which would expand to a meaningless empty/underscore env var).
func sanitizeEnvName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r - ('a' - 'A'))
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if strings.Trim(out, "_") == "" {
		return ""
	}
	return out
}

// clearRaidArgs unsets all RAID_ARG_* environment variables.
func clearRaidArgs() {
	for _, kv := range os.Environ() {
		key, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(key, "RAID_ARG_") {
			os.Unsetenv(key)
		}
	}
}

func runCommand(cmd Command) error {
	if cmd.Out == nil {
		return ExecuteTasks(cmd.Tasks)
	}

	origOut, origErr := commandStdout, commandStderr
	defer func() {
		commandStdout = origOut
		commandStderr = origErr
	}()

	if !cmd.Out.Stdout {
		commandStdout = io.Discard
	}
	if !cmd.Out.Stderr {
		commandStderr = io.Discard
	}

	if cmd.Out.File != "" {
		expanded := sys.ExpandPath(cmd.Out.File)
		if err := os.MkdirAll(filepath.Dir(expanded), 0755); err != nil {
			return fmt.Errorf("failed to create output directory for '%s': %w", cmd.Out.File, err)
		}
		f, err := os.Create(expanded)
		if err != nil {
			return fmt.Errorf("failed to open output file '%s': %w", cmd.Out.File, err)
		}
		defer f.Close()
		commandStdout = io.MultiWriter(commandStdout, f)
		commandStderr = io.MultiWriter(commandStderr, f)
	}

	return ExecuteTasks(cmd.Tasks)
}

// mergeCommands merges additional into base. On name conflicts, base takes priority.
func mergeCommands(base, additional []Command) []Command {
	existing := make(map[string]bool, len(base))
	for _, c := range base {
		existing[c.Name] = true
	}
	result := append([]Command(nil), base...)
	for _, c := range additional {
		if !existing[c.Name] {
			result = append(result, c)
		}
	}
	return result
}
