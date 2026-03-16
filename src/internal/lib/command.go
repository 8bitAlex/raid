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
	Tasks []Task   `json:"tasks"`
	Out   *Output  `json:"out,omitempty"`
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

// ExecuteCommand runs the tasks for the named command, applying any output configuration.
// Args are exposed as RAID_ARG_1, RAID_ARG_2, ... environment variables for the duration
// of the command and are unset afterwards.
func ExecuteCommand(name string, args []string) error {
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

	clearRaidArgs()
	defer clearRaidArgs()
	for i, arg := range args {
		os.Setenv(fmt.Sprintf("RAID_ARG_%d", i+1), arg)
	}

	return runCommand(found)
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
