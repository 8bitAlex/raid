package doctor

import (
	"fmt"
	"os"

	"github.com/8bitalex/raid/src/raid"
	"github.com/spf13/cobra"
)

// Command is the doctor subcommand that checks the raid configuration for issues.
var Command = &cobra.Command{
	Use:   "doctor",
	Short: "Check the raid configuration and report any issues",
	Args:  cobra.NoArgs,
	Run:   runDoctor,
}

func runDoctor(_ *cobra.Command, _ []string) {
	findings := raid.Doctor()

	warnings, errors := 0, 0
	for _, f := range findings {
		switch f.Severity {
		case raid.SeverityOK:
			fmt.Printf("  [ok]    %s: %s\n", f.Check, f.Message)
		case raid.SeverityWarn:
			warnings++
			fmt.Printf("  [warn]  %s: %s\n", f.Check, f.Message)
			if f.Suggestion != "" {
				fmt.Printf("          → %s\n", f.Suggestion)
			}
		case raid.SeverityError:
			errors++
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
