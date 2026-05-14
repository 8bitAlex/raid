// Package telemetry is the cobra surface for `raid telemetry`. It
// exposes on / off / status / purge / preview sub-subcommands. The
// real behavior lives in src/internal/telemetry — this file is just
// the CLI bindings.
package telemetry

import (
	"encoding/json"
	"fmt"

	libtelemetry "github.com/8bitalex/raid/src/internal/telemetry"
	"github.com/8bitalex/raid/src/raid/errs"
	"github.com/spf13/cobra"
)

func init() {
	Command.AddCommand(onCmd)
	Command.AddCommand(offCmd)
	Command.AddCommand(statusCmd)
	Command.AddCommand(purgeCmd)
	Command.AddCommand(previewCmd)
	offCmd.Flags().String("why", "", "Optional free-text reason recorded with the opt-out event")
}

// jsonMode mirrors the helper used by sibling subcommands (env,
// doctor): reads --json off the root's persistent flag so JSON output
// stays consistent across the binary.
func jsonMode(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("json")
	return v
}

// Command is the parent `raid telemetry` group. Args are validated by
// the sub-subcommands themselves so we can give a precise error
// instead of cobra's generic one.
var Command = &cobra.Command{
	Use:   "telemetry",
	Short: "Manage anonymous CLI telemetry (off by default)",
	Long: "raid ships opt-in, anonymous CLI telemetry. Off by default; " +
		"flip it on with `raid telemetry on`, see what's stored with " +
		"`raid telemetry status`, preview a sample event with " +
		"`raid telemetry preview`, and break continuity with " +
		"`raid telemetry purge`. See https://raidcli.dev/docs/telemetry.",
	Args: cobra.NoArgs,
}

var onCmd = &cobra.Command{
	Use:   "on",
	Short: "Opt in to anonymous telemetry",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := libtelemetry.SetEnabled(true); err != nil {
			return errs.Wrap(err)
		}
		// Fire EventFirstRun synchronously so the opt-in is recorded
		// even if raid exits right after this command.
		libtelemetry.CaptureSync(libtelemetry.EventFirstRun,
			libtelemetry.FirstRunProps(""))
		cmd.Println("Telemetry: on. Run `raid telemetry status` to see what's stored.")
		return nil
	},
}

var offCmd = &cobra.Command{
	Use:   "off",
	Short: "Opt out of anonymous telemetry",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		why, _ := cmd.Flags().GetString("why")
		// Fire the opt-out BEFORE flipping consent off — otherwise
		// IsActive() would return false and the event would be
		// dropped. CaptureSync blocks on the HTTP attempt (best-effort:
		// network/non-2xx errors are silently dropped) so the event
		// gets a real chance to land before raid exits.
		libtelemetry.CaptureSync(libtelemetry.EventTelemetryOptOut,
			libtelemetry.OptOutProps(why))
		if err := libtelemetry.SetEnabled(false); err != nil {
			return errs.Wrap(err)
		}
		cmd.Println("Telemetry: off.")
		return nil
	},
}

// statusEntry is the JSON shape printed by `status --json`. Stable
// public contract — agents that script `raid telemetry status --json`
// can rely on field names.
type statusEntry struct {
	Enabled     bool   `json:"enabled"`
	Decided     bool   `json:"decided"`
	DoNotTrack  bool   `json:"doNotTrack"`
	APIKeySet   bool   `json:"apiKeySet"`
	AnonymousID string `json:"anonymousId,omitempty"`
	IDPath      string `json:"idPath"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show telemetry consent + anonymous machine ID",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		st := libtelemetry.LoadState()
		entry := statusEntry{
			Enabled:     st.Enabled,
			Decided:     st.Decided,
			DoNotTrack:  libtelemetry.DoNotTrackActive(),
			APIKeySet:   libtelemetry.HasAPIKey(),
			AnonymousID: libtelemetry.LoadIDIfExists(),
			IDPath:      libtelemetry.IDPath(),
		}
		if jsonMode(cmd) {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(entry); err != nil {
				return errs.Unknown(err)
			}
			return nil
		}
		printStatus(cmd, entry)
		return nil
	},
}

func printStatus(cmd *cobra.Command, e statusEntry) {
	state := "off"
	if e.Enabled {
		state = "on"
	}
	if !e.Decided {
		state = "off (no decision yet — will be prompted on next interactive run)"
	}
	if e.DoNotTrack {
		state = "off (DO_NOT_TRACK is set; overrides stored state)"
	}
	cmd.Println("State:", state)
	if e.APIKeySet {
		cmd.Println("API key: present (build-time configured)")
	} else {
		cmd.Println("API key: not configured for this build — events would never send")
	}
	if e.AnonymousID != "" {
		cmd.Println("Anonymous ID:", e.AnonymousID)
	} else {
		cmd.Println("Anonymous ID: none (will be generated on first opt-in)")
	}
	cmd.Println("ID file:", e.IDPath)
}

var purgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "Delete the anonymous machine ID file",
	Long: "Removes ~/.config/raid/telemetry-id so PostHog cannot link " +
		"future events to past ones. Leaves the on/off state intact — " +
		"run `raid telemetry off` to also disable sending.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := libtelemetry.PurgeID(); err != nil {
			return errs.Unknown(err)
		}
		cmd.Println("Anonymous ID purged.")
		return nil
	},
}

var previewCmd = &cobra.Command{
	Use:   "preview",
	Short: "Print a sample telemetry payload without sending it",
	Long: "Shows the full JSON body raid would post to the telemetry " +
		"endpoint for a typical command_executed event. Useful to " +
		"verify what telemetry would emit before opting in.",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		props := libtelemetry.CommandExecutedProps(
			"build",
			3,
			[]string{"shell", "shell", "print"},
			1234,
		)
		payload := libtelemetry.PreviewPayload(libtelemetry.EventCommandExecuted, props)
		if payload == "" {
			return errs.Internal("telemetry: failed to render preview payload")
		}
		fmt.Fprintln(cmd.OutOrStdout(), payload)
		return nil
	},
}
