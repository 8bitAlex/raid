package cmd

import (
	"testing"

	"github.com/8bitalex/raid/src/raid"
)

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
