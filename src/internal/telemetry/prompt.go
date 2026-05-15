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
// Two skip tiers, by design:
//
//  1. **Persistent skip** — long-term reasons to be non-interactive:
//     `--yes` / `--headless` / `RAID_HEADLESS=1`, `DO_NOT_TRACK=1`,
//     stdin isn't a TTY (CI, pipes, agent hosts), already decided.
//     These persist `decided=off` so future runs from the same
//     machine don't re-attempt the prompt logic. Rationale: a host
//     that's non-interactive today is probably non-interactive
//     tomorrow; re-prompting on every run is noise.
//
//     Dev / no-API-key builds short-circuit before reaching this
//     tier — there's nothing to opt out of, so consent state stays
//     untouched and a later release-build run on the same machine
//     will still get a fresh prompt.
//
//  2. **Transient skip** — per-invocation reasons that don't reflect
//     the user's long-term posture: `--json` (machine-readable
//     output mode for one command). The prompt is suppressed but
//     no consent state is written, so a later interactive run
//     still gets prompted. Rationale: piping one command through
//     `--json` is a momentary tool choice, not an opt-out signal.
//
// The split exists because conflating the two caused a real bug
// where `raid context --json | jq` silently opted the user out
// forever. Callers must classify their skip signal correctly.
func MaybePromptForConsent(skipPersistent, skipTransient bool) PromptResult {
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
	// Persistent skip or non-TTY: record decided=off so the prompt
	// doesn't re-fire on subsequent runs. Checked BEFORE transient
	// so a caller passing both (e.g. `--yes --json` together) gets
	// the stronger "persist" outcome — not a state-leaving transient
	// skip that resurrects the prompt on the next run.
	if skipPersistent || !isInteractiveFn() {
		_ = SetDecidedOff()
		return PromptSkipped
	}
	// Transient skip: skip without persisting; the next interactive
	// run without the transient signal still gets prompted.
	if skipTransient {
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
			// User declined the main prompt. Ask one follow-up: may
			// we send a single anonymous "denial recorded" event so
			// the project can measure opt-out rates? Default is no.
			// If they say yes we fire under per-event consent
			// BEFORE flipping the state off (technically the bypass
			// path doesn't require the order, but firing first
			// matches the `raid telemetry off` pattern and makes
			// the intent obvious to future readers).
			if askOptOutEventConsent(reader) {
				CaptureOptOutConsented("prompt-declined")
			}
			_ = SetEnabled(false)
			return PromptDeclined
		}
	}
}

// askOptOutEventConsent renders the follow-up prompt and returns
// true iff the user explicitly accepts. Default is no (capital N)
// so a stray enter keeps the strict opt-out posture. Exists as a
// separate function so the prompt text + truthy-answer matching
// have a single test surface.
func askOptOutEventConsent(reader *bufio.Reader) bool {
	fmt.Fprint(promptOutFn(), optOutFollowUpText())
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "y", "yes":
		return true
	}
	return false
}

func optOutFollowUpText() string {
	return "" +
		"\n" +
		"Telemetry is off. May raid send a single anonymous event recording your decision?\n" +
		"This is the only event raid would ever send; it helps the project measure how\n" +
		"many users opt out vs. opt in. Same anonymity guarantees as above.\n" +
		"\n" +
		"  [y] yes, send once       [N] no, send nothing at all\n" +
		"> "
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
