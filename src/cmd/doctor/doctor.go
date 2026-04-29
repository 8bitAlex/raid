package doctor

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

var jsonOutput bool

func init() {
	Command.Flags().BoolVar(&jsonOutput, "json", false, "Emit machine-readable JSON output")
}

// Command is the doctor subcommand that checks the raid configuration for issues.
var Command = &cobra.Command{
	Use:   "doctor",
	Short: "Check the raid configuration and report any issues",
	Args:  cobra.NoArgs,
	Run:   runDoctor,
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

func runDoctor(cmd *cobra.Command, _ []string) {
	findings := raid.Doctor()

	oks, warnings, errors := 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case raid.SeverityOK:
			oks++
		case raid.SeverityWarn:
			warnings++
		case raid.SeverityError:
			errors++
		}
	}

	if jsonOutput {
		out := doctorOutput{
			Findings: make([]findingJSON, 0, len(findings)),
			Summary:  doctorSummary{OK: oks, Warnings: warnings, Errors: errors},
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
		_ = enc.Encode(out)
		if errors > 0 {
			os.Exit(1)
		}
		return
	}

	for _, f := range findings {
		switch f.Severity {
		case raid.SeverityOK:
			fmt.Printf("  [ok]    %s: %s\n", f.Check, f.Message)
		case raid.SeverityWarn:
			fmt.Printf("  [warn]  %s: %s\n", f.Check, f.Message)
			if f.Suggestion != "" {
				fmt.Printf("          → %s\n", f.Suggestion)
			}
		case raid.SeverityError:
			fmt.Printf("  [error] %s: %s\n", f.Check, f.Message)
			if f.Suggestion != "" {
				fmt.Printf("          → %s\n", f.Suggestion)
			}
		}
	}

	fmt.Println()
	switch {
	case errors > 0:
		fmt.Printf("%d error(s) detected.\n", errors)
		os.Exit(1)
	case warnings > 0:
		fmt.Printf("%d warning(s).\n", warnings)
	default:
		fmt.Println("No issues found.")
	}
}
