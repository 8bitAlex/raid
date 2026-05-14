package lib

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// NoPrefixEnvVar toggles per-task output prefixing in concurrent runs.
// The CLI's --no-prefix persistent flag sets this var via rootCmd's
// PersistentPreRunE so a single read site in lib serves both the
// flag-driven and env-driven entry points.
const NoPrefixEnvVar = "RAID_NO_PREFIX"

// noColorEnvVar follows the https://no-color.org/ convention — when
// any non-empty value is present, ANSI color codes are stripped but
// the prefix itself is preserved. Disabling just color (not prefix)
// is useful for users who want attribution without escape sequences
// littering their terminal scrollback.
const noColorEnvVar = "NO_COLOR"

// prefixDisabledOverride lets tests force the no-prefix toggle
// deterministically without mutating os.Environ. nil means "fall
// through to the env var". Same shape as headlessOverride so callers
// of both helpers see consistent semantics.
var prefixDisabledOverride *bool

// PrefixDisabled reports whether per-task output prefixing is
// suppressed via the --no-prefix flag or RAID_NO_PREFIX env var.
// Truthy values mirror IsHeadless: "1", "true", "yes", "y", "on"
// (case-insensitive). Anything else is treated as not-disabled so a
// stray export can't silently change behavior.
func PrefixDisabled() bool {
	if prefixDisabledOverride != nil {
		return *prefixDisabledOverride
	}
	v := strings.TrimSpace(os.Getenv(NoPrefixEnvVar))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}

// SetPrefixDisabledForTest forces the prefix-disabled toggle for the
// duration of a test. Returns a restore function to defer.
func SetPrefixDisabledForTest(v bool) func() {
	prev := prefixDisabledOverride
	prefixDisabledOverride = &v
	return func() { prefixDisabledOverride = prev }
}

// colorDisabled reports whether ANSI color codes should be stripped
// from prefixed output. Honors the NO_COLOR env var convention:
// presence of any non-empty value means "no color". The prefix
// itself is still emitted; only the color codes are dropped.
func colorDisabled() bool {
	return os.Getenv(noColorEnvVar) != ""
}

// isTerminalSinkFn is the indirection point for tests — they can
// stub this to force the wrap or no-wrap path without setting up an
// actual TTY. Production callers always use isTerminalSink directly.
var isTerminalSinkFn = isTerminalSink

// isTerminalSink reports whether w is an *os.File backed by an
// interactive terminal. Anything that isn't an *os.File
// (bytes.Buffer, syncBuffer, io.MultiWriter, exec.Pipe) returns
// false. Uses golang.org/x/term.IsTerminal under the hood — a
// raw os.ModeCharDevice check is too permissive (it accepts
// /dev/null and other non-terminal character devices), so we
// defer to the proper isatty check on POSIX (and the analogous
// console-handle check on Windows).
func isTerminalSink(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// ANSI palette for the per-task prefix. Six high-contrast colors
// chosen to read cleanly on both light and dark terminal themes;
// white and gray are intentionally excluded because they collide
// with default terminal foregrounds. Order matters only for
// determinism: colorForName indexes into this slice via a hash.
var prefixColors = []string{
	"\033[31m", // red
	"\033[32m", // green
	"\033[33m", // yellow
	"\033[34m", // blue
	"\033[35m", // magenta
	"\033[36m", // cyan
}

// ansiReset is the terminator that turns colored prefix output back
// to the terminal's default. Empty when no color was applied so we
// don't litter output with reset codes that have nothing to reset.
const ansiReset = "\033[0m"

// colorForName picks a stable ANSI color for a task label. FNV-1a
// is used (not crypto-grade — we just want a quick deterministic
// hash with reasonable distribution across the 6-color palette).
func colorForName(name string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return prefixColors[h.Sum32()%uint32(len(prefixColors))]
}

// buildPrefix renders the per-task prefix string ("[name] ") with
// optional color. When color is empty, no ANSI codes are emitted.
func buildPrefix(label string, color string) string {
	if color == "" {
		return fmt.Sprintf("[%s] ", label)
	}
	return fmt.Sprintf("%s[%s]%s ", color, label, ansiReset)
}

// prefixedWriter wraps a sink so every newline-terminated line
// written through it is preceded by a per-task prefix. Partial
// lines (bytes without a trailing '\n') are buffered until a
// newline arrives or Flush is called. Writes to the underlying
// sink are serialized through a shared mutex so two concurrent
// tasks never produce a line that mixes both prefixes.
//
// The buffer is reused across Write calls so a long line spanning
// many short writes still emits as one prefix+line+newline triple.
type prefixedWriter struct {
	sink   io.Writer
	prefix string
	mu     *sync.Mutex // shared across all prefixedWriters in the same process
	buf    []byte      // partial-line buffer; never holds bytes after a '\n'
}

// outputMu is the package-level mutex shared by every
// prefixedWriter. Concurrent tasks (and nested Group invocations)
// write to the same underlying sink, so a single mutex serializing
// line-grained writes is correct. Writes are short — prefix +
// line + newline — so contention is acceptable for typical
// workloads.
var outputMu sync.Mutex

// newPrefixedWriter wires a sink, prefix, and the shared mutex.
// Callers must invoke Flush() (or defer it) after the subprocess
// exits to surface any trailing partial line.
func newPrefixedWriter(sink io.Writer, prefix string) *prefixedWriter {
	return &prefixedWriter{sink: sink, prefix: prefix, mu: &outputMu}
}

// Write satisfies io.Writer. The byte count returned is len(p) on
// success so callers (and exec.Cmd's internal pump) don't see a
// short write — the bytes are either flushed to the sink or held
// in the partial-line buffer; in both cases they're accepted.
func (w *prefixedWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	// Find all newline-terminated lines. Anything after the last
	// '\n' is a partial line that stays in w.buf for the next
	// Write or Flush.
	w.mu.Lock()
	defer w.mu.Unlock()
	combined := append(w.buf, p...)
	i := 0
	for {
		j := bytes.IndexByte(combined[i:], '\n')
		if j < 0 {
			break
		}
		// Emit prefix + line (including the trailing '\n') as a
		// single Write to the sink. This is the atomicity
		// guarantee — two competing prefixedWriters won't
		// interleave mid-line because each holds outputMu for
		// the duration of its own emit.
		if _, err := io.WriteString(w.sink, w.prefix); err != nil {
			w.buf = nil
			return 0, err
		}
		if _, err := w.sink.Write(combined[i : i+j+1]); err != nil {
			w.buf = nil
			return 0, err
		}
		i += j + 1
	}
	// Stash any trailing bytes (no '\n' yet) for the next call.
	if i < len(combined) {
		// Copy so we don't keep a reference to p's backing array.
		tail := make([]byte, len(combined)-i)
		copy(tail, combined[i:])
		w.buf = tail
	} else {
		w.buf = nil
	}
	return len(p), nil
}

// Flush emits any buffered partial line with a synthesized newline
// so a subprocess that exits without a final '\n' still gets its
// last line attributed. No-op when the buffer is empty.
func (w *prefixedWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) == 0 {
		return nil
	}
	if _, err := io.WriteString(w.sink, w.prefix); err != nil {
		return err
	}
	if _, err := w.sink.Write(w.buf); err != nil {
		return err
	}
	if _, err := io.WriteString(w.sink, "\n"); err != nil {
		return err
	}
	w.buf = nil
	return nil
}

// shouldPrefix decides whether task output should be wrapped.
// All three conditions must hold:
//   - the task is opted into concurrent execution (sequential
//     output is unambiguous by ordering, so prefixing it is just
//     visual noise);
//   - the underlying sink is a TTY (pipes, file redirects, CI
//     logs, and the MCP server's syncBuffer all return false so
//     machine-readable output stays byte-identical to today);
//   - the user hasn't disabled prefixing via --no-prefix or
//     RAID_NO_PREFIX.
func shouldPrefix(task Task) bool {
	if !task.Concurrent {
		return false
	}
	if PrefixDisabled() {
		return false
	}
	return isTerminalSinkFn(commandStdout)
}
