package doctor

import (
	"encoding/json"
	"fmt"

	"github.com/8bitalex/raid/src/raid"
	"github.com/8bitalex/raid/src/raid/errs"
	"github.com/spf13/cobra"
)

// Command is the doctor subcommand that checks the raid configuration for issues.
// JSON output is controlled by the persistent --json flag on rootCmd.
var Command = &cobra.Command{
	Use:   "doctor",
	Short: "Check the raid configuration and report any issues",
	Args:  cobra.NoArgs,
	RunE:  runDoctor,
}

// jsonModeFromRoot resolves --json against the root's persistent flag so the
// read always reflects the current invocation, even when the package-level
// Command var is reused across tests. GetBool returns false (its zero
// value) plus an ignored error when the flag isn't registered, so a bare
// cmd with no parent and no --json flag yields false naturally.
func jsonModeFromRoot(cmd *cobra.Command) bool {
	v, _ := cmd.Root().PersistentFlags().GetBool("json")
	return v
}

// findingJSON is the stable JSON shape for a single doctor finding. Severity
// is encoded as a string ("ok" | "warn" | "error") so the JSON output is
// self-describing without consulting documentation.
type findingJSON struct {
	Severity   string `json:"severity"`
	Check      string `json:"check"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type doctorSummary struct {
	OK       int `json:"ok"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

type doctorOutput struct {
	Findings []findingJSON `json:"findings"`
	Summary  doctorSummary `json:"summary"`
}

func severityString(s raid.Severity) string {
	switch s {
	case raid.SeverityOK:
		return "ok"
	case raid.SeverityWarn:
		return "warn"
	case raid.SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	// Doctor runs verify task sequences whose onFail remediation can mutate
	// files, so it must serialize against other raid processes (CLI and MCP
	// server) via the cross-process mutation lock.
	var findings []raid.Finding
	if lockErr := raid.WithMutationLock(func() error {
		findings = raid.Doctor()
		return nil
	}); lockErr != nil {
		return errs.Wrap(lockErr)
	}

	oks, warnings, errorCount := 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case raid.SeverityOK:
			oks++
		case raid.SeverityWarn:
			warnings++
		case raid.SeverityError:
			errorCount++
		}
	}

	jsonOutput := jsonModeFromRoot(cmd)
	if jsonOutput {
		out := doctorOutput{
			Findings: make([]findingJSON, 0, len(findings)),
			Summary:  doctorSummary{OK: oks, Warnings: warnings, Errors: errorCount},
		}
		for _, f := range findings {
			out.Findings = append(out.Findings, findingJSON{
				Severity:   severityString(f.Severity),
				Check:      f.Check,
				Message:    f.Message,
				Suggestion: f.Suggestion,
			})
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return errs.Unknown(err)
		}
		if errorCount > 0 {
			// Return a structured error so the central handler sets exit
			// code = CategoryConfig (2). Message is suppressed because the
			// JSON findings already carry the detail.
			return errs.ConfigInvalid(fmt.Errorf("%d doctor finding(s) at error severity", errorCount))
		}
		return nil
	}

	out := cmd.OutOrStdout()
	for _, f := range findings {
		switch f.Severity {
		case raid.SeverityOK:
			fmt.Fprintf(out, "  [ok]    %s: %s\n", f.Check, f.Message)
		case raid.SeverityWarn:
			fmt.Fprintf(out, "  [warn]  %s: %s\n", f.Check, f.Message)
			if f.Suggestion != "" {
				fmt.Fprintf(out, "          → %s\n", f.Suggestion)
			}
		case raid.SeverityError:
			fmt.Fprintf(out, "  [error] %s: %s\n", f.Check, f.Message)
			if f.Suggestion != "" {
				fmt.Fprintf(out, "          → %s\n", f.Suggestion)
			}
		}
	}

	fmt.Fprintln(out)
	switch {
	case errorCount > 0:
		fmt.Fprintf(out, "%d error(s) detected.\n", errorCount)
		return errs.ConfigInvalid(fmt.Errorf("%d doctor finding(s) at error severity", errorCount))
	case warnings > 0:
		fmt.Fprintf(out, "%d warning(s).\n", warnings)
	default:
		fmt.Fprintln(out, "No issues found.")
	}
	return nil
}
