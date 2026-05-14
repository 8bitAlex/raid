package telemetry

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// PromptResult describes what the first-run prompt resolved to. Used
// by the caller (cmd/raid.go) to decide whether to fire the
// first_run event and to surface a short post-prompt confirmation.
type PromptResult int

const (
	// PromptSkipped means we never showed the prompt (non-interactive
	// context, DO_NOT_TRACK, already decided, no API key). For most
	// skip reasons consent is also marked decided=off so we won't try
	// again. Exception: the "no API key" branch (dev builds where
	// telemetry is dead code) returns PromptSkipped without persisting
	// any consent state — there's nothing useful to remember.
	PromptSkipped PromptResult = iota
	// PromptDeclined means the user explicitly chose no.
	PromptDeclined
	// PromptAccepted means the user explicitly chose yes. The caller
	// should fire EventFirstRun (with install_method if known).
	PromptAccepted
)

// promptInFn is the prompt's stdin reader, indirected for tests. We
// don't reuse lib.getStdinReader because that's behind the task-
// execution mutex and creates a bufio.Reader tied to os.Stdin —
// which we don't want to lock here.
var promptInFn = func() io.Reader { return os.Stdin }

// promptOutFn is where the prompt's text lands. Stderr by default so
// stdout stays clean for piped consumers (`raid context --json |
// jq`).
var promptOutFn = func() io.Writer { return os.Stderr }

// isInteractiveFn reports whether stdin is a TTY. Tests stub this so
// they don't depend on the runner's stdin state.
var isInteractiveFn = func() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

// MaybePromptForConsent runs the first-run consent flow when
// appropriate. Returns the resolved outcome so the caller can fire
// follow-up events.
//
// Skip conditions (each → PromptSkipped + consent persisted off):
//   - DO_NOT_TRACK env var set
//   - Consent already decided in a prior invocation
//   - Build has no API key (telemetry is dead code anyway)
//   - Stdin isn't a TTY (CI, pipes, agent hosts)
//   - skipInteractive is true (raid invoked with --yes/--headless,
//     or for an info command like `--help`, or for a subcommand
//     that itself manages telemetry like `raid telemetry on`)
//
// The skip-and-persist behavior matches the user-confirmed scope:
// non-interactive contexts get the same "off forever, never
// prompt again" treatment so a later interactive run also stays
// quiet unless the user runs `raid telemetry on` explicitly.
func MaybePromptForConsent(skipInteractive bool) PromptResult {
	if APIKey == "" {
		return PromptSkipped
	}
	if isDoNotTrack() {
		_ = SetDecidedOff()
		return PromptSkipped
	}
	if LoadState().Decided {
		return PromptSkipped
	}
	if skipInteractive || !isInteractiveFn() {
		_ = SetDecidedOff()
		return PromptSkipped
	}

	// One bufio reader threaded through the explainer loop so a fresh
	// bufio per attempt can't strand input buffered after the first
	// newline.
	reader := bufio.NewReader(promptInFn())
	for {
		answer := readPromptAnswer(reader)
		switch answer {
		case "y", "yes":
			if err := SetEnabled(true); err != nil {
				return PromptSkipped
			}
			return PromptAccepted
		case "?":
			fmt.Fprint(promptOutFn(), explainerText())
			continue
		default:
			_ = SetEnabled(false)
			return PromptDeclined
		}
	}
}

// readPromptAnswer renders the prompt and reads a single line of
// input. Returns the trimmed, lowercased answer; empty string means
// the user just hit enter (treated as the capital-N default).
func readPromptAnswer(reader *bufio.Reader) string {
	fmt.Fprint(promptOutFn(), promptText())
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return ""
	}
	return strings.TrimSpace(strings.ToLower(line))
}

func promptText() string {
	return "" +
		"raid would like to send anonymous usage telemetry to help prioritize features.\n" +
		"We never collect: file paths, command contents, env values, or anything that could identify you.\n" +
		"See: https://raidcli.dev/docs/telemetry\n" +
		"\n" +
		"  [y] yes, send telemetry      [N] no, leave it off       [?] what's collected\n" +
		"> "
}

func explainerText() string {
	return "" +
		"\n" +
		"raid would send:\n" +
		"  - which built-in commands you run (install, env, doctor, …)\n" +
		"  - which custom-command names you run (the name only — never the cmd body)\n" +
		"  - which task types ran (Shell, Script, Wait, …) — never the cmd body, paths, or env values\n" +
		"  - command success/failure + structured error code (e.g. TASK_SHELL_FAILED)\n" +
		"  - raid version, OS, architecture\n" +
		"  - an anonymous machine ID (UUIDv4 — purgeable via `raid telemetry purge`)\n" +
		"\n" +
		"raid never collects:\n" +
		"  - cmd bodies, paths, URLs, env values, or anything else you typed\n" +
		"  - stdout/stderr of your tasks\n" +
		"  - your username, hostname, IP, or any system identifier beyond OS+arch\n" +
		"\n" +
		"Source: https://github.com/8bitalex/raid/tree/main/src/internal/telemetry\n" +
		"\n"
}
