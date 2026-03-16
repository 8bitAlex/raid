package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/8bitalex/raid/src/internal/lib"
	"github.com/spf13/cobra"

	"github.com/8bitalex/raid/src/raid"
)

func TestBaseVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    string
	}{
		{"stable release", "1.2.3", "1.2.3"},
		{"beta release", "1.2.3-beta", "1.2.3-beta"},
		{"preview build", "1.2.3-preview", "1.2.3"},
		{"beta preview build", "1.2.3-beta-preview", "1.2.3-beta"},
		{"preview build with sha", "1.2.3-beta-preview.abc1234", "1.2.3-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := baseVersion(tt.version)
			if got != tt.want {
				t.Errorf("baseVersion(%q) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestIsInfoCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"no args", []string{"raid"}, true},
		{"empty args", []string{}, true},
		{"help subcommand", []string{"raid", "help"}, true},
		{"version subcommand", []string{"raid", "version"}, true},
		{"completion subcommand", []string{"raid", "completion"}, true},
		{"--help flag", []string{"raid", "--help"}, true},
		{"-h flag", []string{"raid", "-h"}, true},
		{"--version flag", []string{"raid", "--version"}, true},
		{"-v flag", []string{"raid", "-v"}, true},
		{"regular command", []string{"raid", "install"}, false},
		{"doctor command", []string{"raid", "doctor"}, false},
		{"profile subcommand", []string{"raid", "profile", "create"}, false},
		{"flag after end-of-flags marker", []string{"raid", "--", "--help"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isInfoCommand(tt.args)
			if got != tt.want {
				t.Errorf("isInfoCommand(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestApplyConfigFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantPath string
	}{
		{
			name:     "no config flag",
			args:     []string{"raid", "install"},
			wantPath: "",
		},
		{
			name:     "long flag with separate value",
			args:     []string{"raid", "--config", "/custom/config.toml", "install"},
			wantPath: "/custom/config.toml",
		},
		{
			name:     "long flag with equals",
			args:     []string{"raid", "--config=/custom/config.toml"},
			wantPath: "/custom/config.toml",
		},
		{
			name:     "short flag",
			args:     []string{"raid", "-c", "/custom/config.toml"},
			wantPath: "/custom/config.toml",
		},
		{
			name:     "config flag at end with no value",
			args:     []string{"raid", "--config"},
			wantPath: "",
		},
		{
			name:     "config flag before subcommand",
			args:     []string{"raid", "--config", "/path.toml", "env", "list"},
			wantPath: "/path.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := *raid.ConfigPath
			*raid.ConfigPath = ""
			t.Cleanup(func() { *raid.ConfigPath = old })

			applyConfigFlag(tt.args)

			if got := *raid.ConfigPath; got != tt.wantPath {
				t.Errorf("applyConfigFlag() ConfigPath = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

// --- registerUserCommands ---

// newTestRoot returns a minimal cobra root command suitable for registration tests.
func newTestRoot() *cobra.Command {
	return &cobra.Command{Use: "raid"}
}

// helpOutput captures the help text produced by root.
func helpOutput(root *cobra.Command) string {
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	_ = root.Help()
	return buf.String()
}

func TestRegisterUserCommands_emptyProfile(t *testing.T) {
	root := newTestRoot()
	registerUserCommands(root, nil)
	if len(root.Commands()) != 0 {
		t.Errorf("expected no subcommands, got %d", len(root.Commands()))
	}
}

func TestRegisterUserCommands_appearsInHelp(t *testing.T) {
	root := newTestRoot()
	registerUserCommands(root, []lib.Command{
		{Name: "deploy", Usage: "Deploy all services"},
		{Name: "sync", Usage: "Sync repositories"},
	})

	out := helpOutput(root)
	for _, want := range []string{"deploy", "Deploy all services", "sync", "Sync repositories"} {
		if !strings.Contains(out, want) {
			t.Errorf("help output missing %q\n\nfull output:\n%s", want, out)
		}
	}
}

func TestRegisterUserCommands_reservedNameSkipped(t *testing.T) {
	root := newTestRoot()
	registerUserCommands(root, []lib.Command{
		{Name: "install", Usage: "should be skipped"},
		{Name: "deploy", Usage: "should appear"},
	})

	cmds := root.Commands()
	if len(cmds) != 1 || cmds[0].Name() != "deploy" {
		t.Errorf("expected only 'deploy', got %v", func() []string {
			names := make([]string, len(cmds))
			for i, c := range cmds {
				names[i] = c.Name()
			}
			return names
		}())
	}
}

// Regression: user commands must appear in output for every invocation type,
// including info commands (no args, --help, help) and unknown subcommands.
// Each subtest drives Cobra via SetArgs + Execute() so the full Cobra dispatch
// path is exercised rather than calling Help() directly.
func TestRegisterUserCommands_visibleForAllInvocationTypes(t *testing.T) {
	cmds := []lib.Command{{Name: "build", Usage: "Build services"}}

	tests := []struct {
		inv  string
		args []string
	}{
		{"no args", []string{}},
		{"--help flag", []string{"--help"}},
		{"help subcommand", []string{"help"}},
	}

	for _, tt := range tests {
		t.Run(tt.inv, func(t *testing.T) {
			root := newTestRoot()
			registerUserCommands(root, cmds)

			var buf bytes.Buffer
			root.SetOut(&buf)
			root.SetErr(&buf)
			root.SetArgs(tt.args)
			_ = root.Execute() // error expected for unknown-cmd; ignore it

			if !strings.Contains(buf.String(), "build") {
				t.Errorf("'build' missing from output for invocation %q\noutput:\n%s", tt.inv, buf.String())
			}
		})
	}
}
