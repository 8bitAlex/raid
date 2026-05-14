package lib

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// --- prefixedWriter ---

func TestPrefixedWriter_singleLine(t *testing.T) {
	var sink bytes.Buffer
	w := newPrefixedWriter(&sink, "[a] ")
	n, err := w.Write([]byte("hello\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len("hello\n") {
		t.Errorf("Write n = %d, want %d", n, len("hello\n"))
	}
	if got := sink.String(); got != "[a] hello\n" {
		t.Errorf("sink = %q, want %q", got, "[a] hello\n")
	}
}

func TestPrefixedWriter_multipleLinesInOneWrite(t *testing.T) {
	var sink bytes.Buffer
	w := newPrefixedWriter(&sink, "[x] ")
	_, err := w.Write([]byte("one\ntwo\nthree\n"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	want := "[x] one\n[x] two\n[x] three\n"
	if got := sink.String(); got != want {
		t.Errorf("sink = %q, want %q", got, want)
	}
}

func TestPrefixedWriter_splitWritesAcrossNewline(t *testing.T) {
	// A subprocess that writes a line in chunks must still produce
	// exactly one prefix+line emission. Buffered bytes don't leak to
	// the sink until the newline arrives.
	var sink bytes.Buffer
	w := newPrefixedWriter(&sink, "[p] ")
	for _, chunk := range []string{"hel", "lo\nwo", "rld\n"} {
		if _, err := w.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write(%q): %v", chunk, err)
		}
	}
	want := "[p] hello\n[p] world\n"
	if got := sink.String(); got != want {
		t.Errorf("sink = %q, want %q", got, want)
	}
}

func TestPrefixedWriter_bufferedBytesNotVisibleUntilNewline(t *testing.T) {
	// Mid-stream check: after writing only "partial", the sink should
	// still be empty. The whole point of buffering partial lines is
	// to avoid emitting a bare "[p] partial" without its full content.
	var sink bytes.Buffer
	w := newPrefixedWriter(&sink, "[p] ")
	if _, err := w.Write([]byte("partial")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := sink.String(); got != "" {
		t.Errorf("sink = %q, want empty (no newline yet)", got)
	}
}

func TestPrefixedWriter_flushEmitsTrailingPartialLine(t *testing.T) {
	var sink bytes.Buffer
	w := newPrefixedWriter(&sink, "[t] ")
	if _, err := w.Write([]byte("no-newline")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	want := "[t] no-newline\n"
	if got := sink.String(); got != want {
		t.Errorf("sink = %q, want %q", got, want)
	}
}

func TestPrefixedWriter_flushNoOpWhenBufferEmpty(t *testing.T) {
	var sink bytes.Buffer
	w := newPrefixedWriter(&sink, "[n] ")
	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	before := sink.Len()
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if sink.Len() != before {
		t.Errorf("Flush wrote extra bytes when buffer was empty: before=%d after=%d", before, sink.Len())
	}
}

func TestPrefixedWriter_zeroByteWriteIsNoOp(t *testing.T) {
	var sink bytes.Buffer
	w := newPrefixedWriter(&sink, "[z] ")
	n, err := w.Write(nil)
	if err != nil || n != 0 {
		t.Errorf("Write(nil) = (%d, %v), want (0, nil)", n, err)
	}
	if sink.Len() != 0 {
		t.Errorf("zero-byte write produced output: %q", sink.String())
	}
}

func TestPrefixedWriter_mutexSerializesConcurrentWrites(t *testing.T) {
	// Two writers driving the same sink from goroutines must never
	// produce a sink line that mixes both prefixes mid-line. This is
	// the load-bearing concurrency guarantee — without the shared
	// outputMu, exec.Cmd's internal pump could call Write on writer
	// A in the middle of a partially-flushed line from writer B.
	var sink bytes.Buffer
	a := newPrefixedWriter(&sink, "[A] ")
	b := newPrefixedWriter(&sink, "[B] ")

	const lines = 500
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < lines; i++ {
			_, _ = a.Write([]byte("line from a\n"))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < lines; i++ {
			_, _ = b.Write([]byte("line from b\n"))
		}
	}()
	wg.Wait()

	// Every output line must start with exactly one prefix and end
	// with a newline. Any mid-line interleave would show as a line
	// containing both "[A]" and "[B]" tokens.
	re := regexp.MustCompile(`^\[[AB]\] line from [ab]$`)
	for i, line := range strings.Split(strings.TrimSuffix(sink.String(), "\n"), "\n") {
		if !re.MatchString(line) {
			t.Fatalf("line %d not single-prefix attributed: %q", i, line)
		}
	}
	got := strings.Count(sink.String(), "\n")
	if got != 2*lines {
		t.Errorf("line count = %d, want %d", got, 2*lines)
	}
}

// errorSink is an io.Writer that returns an error on the Nth Write
// call, used to exercise the Write/Flush error paths.
type errorSink struct {
	failOn  int
	calls   int
	written []byte
}

func (e *errorSink) Write(p []byte) (int, error) {
	e.calls++
	if e.calls == e.failOn {
		return 0, io.ErrShortWrite
	}
	e.written = append(e.written, p...)
	return len(p), nil
}

func TestPrefixedWriter_writeErrorClearsBuffer(t *testing.T) {
	// If the sink errors mid-emit, the buffer must be cleared so the
	// next call doesn't replay stale bytes. Otherwise an error on
	// one task could "poison" subsequent writes from peers.
	sink := &errorSink{failOn: 1} // fail on first Write (the prefix)
	w := newPrefixedWriter(sink, "[e] ")
	_, err := w.Write([]byte("line\n"))
	if err == nil {
		t.Fatal("expected error from failing sink")
	}
	if w.buf != nil {
		t.Errorf("buffer not cleared after error: %q", w.buf)
	}
}

func TestPrefixedWriter_writeErrorOnLine(t *testing.T) {
	// The prefix write succeeds (call 1) but the line write fails
	// (call 2). The buffer must still be cleared.
	sink := &errorSink{failOn: 2}
	w := newPrefixedWriter(sink, "[e] ")
	_, err := w.Write([]byte("line\n"))
	if err == nil {
		t.Fatal("expected error from failing sink on line write")
	}
	if w.buf != nil {
		t.Errorf("buffer not cleared after line write error: %q", w.buf)
	}
}

func TestPrefixedWriter_flushErrorOnPrefix(t *testing.T) {
	sink := &errorSink{failOn: 1}
	w := newPrefixedWriter(sink, "[e] ")
	w.buf = []byte("partial")
	if err := w.Flush(); err == nil {
		t.Fatal("expected error from failing prefix write")
	}
}

func TestPrefixedWriter_flushErrorOnContent(t *testing.T) {
	sink := &errorSink{failOn: 2}
	w := newPrefixedWriter(sink, "[e] ")
	w.buf = []byte("partial")
	if err := w.Flush(); err == nil {
		t.Fatal("expected error from failing content write")
	}
}

func TestPrefixedWriter_flushErrorOnNewline(t *testing.T) {
	sink := &errorSink{failOn: 3}
	w := newPrefixedWriter(sink, "[e] ")
	w.buf = []byte("partial")
	if err := w.Flush(); err == nil {
		t.Fatal("expected error from failing newline write")
	}
}

// --- colorForName / palette ---

func TestColorForName_deterministic(t *testing.T) {
	first := colorForName("deploy")
	for i := 0; i < 100; i++ {
		if got := colorForName("deploy"); got != first {
			t.Fatalf("call %d: colorForName drifted (%q vs %q)", i, got, first)
		}
	}
}

func TestColorForName_palette(t *testing.T) {
	got := colorForName("anything")
	for _, c := range prefixColors {
		if got == c {
			return
		}
	}
	t.Errorf("color %q not in palette", got)
}

func TestColorForName_distributesAcrossPalette(t *testing.T) {
	// A handful of representative task names should hit at least 4
	// distinct colors out of 6. Hash collisions on a small palette
	// are possible, but a wholesale "all five names get the same
	// color" failure would mean colorForName is broken.
	names := []string{"build", "test", "lint", "deploy", "migrate", "format", "release"}
	seen := map[string]bool{}
	for _, n := range names {
		seen[colorForName(n)] = true
	}
	if len(seen) < 4 {
		t.Errorf("only %d distinct colors across %d names: %v", len(seen), len(names), seen)
	}
}

// --- buildPrefix ---

func TestBuildPrefix_withColor(t *testing.T) {
	got := buildPrefix("foo", "\033[31m")
	want := "\033[31m[foo]\033[0m "
	if got != want {
		t.Errorf("buildPrefix(color) = %q, want %q", got, want)
	}
}

func TestBuildPrefix_noColor(t *testing.T) {
	got := buildPrefix("foo", "")
	want := "[foo] "
	if got != want {
		t.Errorf("buildPrefix(no color) = %q, want %q", got, want)
	}
}

// --- isTerminalSink ---

func TestIsTerminalSink_bytesBuffer(t *testing.T) {
	if isTerminalSink(&bytes.Buffer{}) {
		t.Error("bytes.Buffer reported as TTY")
	}
}

func TestIsTerminalSink_devNull(t *testing.T) {
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Skipf("can't open %s: %v", os.DevNull, err)
	}
	defer f.Close()
	if isTerminalSink(f) {
		t.Errorf("%s reported as TTY", os.DevNull)
	}
}

func TestIsTerminalSink_pipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	if isTerminalSink(w) {
		t.Error("os.Pipe writer reported as TTY")
	}
}

func TestIsTerminalSink_regularFile(t *testing.T) {
	// File on disk is not a char device.
	f, err := os.CreateTemp("", "raid-prefix-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if isTerminalSink(f) {
		t.Error("regular file reported as TTY")
	}
}

// --- env-var toggles ---

func withPrefixEnv(t *testing.T, v string) {
	t.Helper()
	prev, had := os.LookupEnv(NoPrefixEnvVar)
	if v == "" {
		os.Unsetenv(NoPrefixEnvVar)
	} else {
		os.Setenv(NoPrefixEnvVar, v)
	}
	t.Cleanup(func() {
		if had {
			os.Setenv(NoPrefixEnvVar, prev)
		} else {
			os.Unsetenv(NoPrefixEnvVar)
		}
	})
	prefixDisabledOverride = nil
}

func TestPrefixDisabled_unsetReturnsFalse(t *testing.T) {
	withPrefixEnv(t, "")
	if PrefixDisabled() {
		t.Error("expected false when RAID_NO_PREFIX is unset")
	}
}

func TestPrefixDisabled_truthyValues(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "yes", "Y", "on", "  1  "} {
		t.Run(v, func(t *testing.T) {
			withPrefixEnv(t, v)
			if !PrefixDisabled() {
				t.Errorf("RAID_NO_PREFIX=%q should disable prefix", v)
			}
		})
	}
}

func TestPrefixDisabled_falsyValues(t *testing.T) {
	for _, v := range []string{"0", "false", "no", "off", "maybe", " "} {
		t.Run(v, func(t *testing.T) {
			withPrefixEnv(t, v)
			if PrefixDisabled() {
				t.Errorf("RAID_NO_PREFIX=%q should not disable prefix", v)
			}
		})
	}
}

func TestSetPrefixDisabledForTest_overridesEnv(t *testing.T) {
	withPrefixEnv(t, "")
	restore := SetPrefixDisabledForTest(true)
	if !PrefixDisabled() {
		t.Error("SetPrefixDisabledForTest(true) did not enable disable")
	}
	restore()
	if PrefixDisabled() {
		t.Error("restore did not clear override")
	}
}

func TestSetPrefixDisabledForTest_overridesTruthyEnv(t *testing.T) {
	withPrefixEnv(t, "1")
	restore := SetPrefixDisabledForTest(false)
	defer restore()
	if PrefixDisabled() {
		t.Error("override should win over truthy env")
	}
}

func TestColorDisabled_envVar(t *testing.T) {
	prev, had := os.LookupEnv("NO_COLOR")
	t.Cleanup(func() {
		if had {
			os.Setenv("NO_COLOR", prev)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	})

	os.Unsetenv("NO_COLOR")
	if colorDisabled() {
		t.Error("NO_COLOR unset should keep color enabled")
	}

	os.Setenv("NO_COLOR", "1")
	if !colorDisabled() {
		t.Error("NO_COLOR=1 should disable color")
	}

	// Any non-empty value disables per the spec.
	os.Setenv("NO_COLOR", "anything")
	if !colorDisabled() {
		t.Error("NO_COLOR=anything should disable color")
	}
}

// --- shouldPrefix ---

func TestShouldPrefix_sequentialNeverPrefixes(t *testing.T) {
	defer SetPrefixDisabledForTest(false)()
	defer stubTerminalSink(true)()
	if shouldPrefix(Task{Concurrent: false}, commandStdout) {
		t.Error("sequential task should not be prefixed")
	}
}

func TestShouldPrefix_concurrentOnTTYWithFlagDisabled(t *testing.T) {
	defer SetPrefixDisabledForTest(true)()
	defer stubTerminalSink(true)()
	if shouldPrefix(Task{Concurrent: true}, commandStdout) {
		t.Error("PrefixDisabled should suppress prefixing even on TTY")
	}
}

func TestShouldPrefix_concurrentNonTTY(t *testing.T) {
	defer SetPrefixDisabledForTest(false)()
	defer stubTerminalSink(false)()
	if shouldPrefix(Task{Concurrent: true}, commandStdout) {
		t.Error("non-TTY sink should suppress prefixing")
	}
}

func TestShouldPrefix_concurrentOnTTY(t *testing.T) {
	defer SetPrefixDisabledForTest(false)()
	defer stubTerminalSink(true)()
	if !shouldPrefix(Task{Concurrent: true}, commandStdout) {
		t.Error("concurrent task on TTY should be prefixed")
	}
}

// When stdout is a TTY but stderr is redirected to a non-TTY (or
// vice versa), each sink is judged independently. Verifies the fix
// for the original "decision-from-stdout-applies-to-stderr" bug.
func TestShouldPrefix_perSinkIndependent(t *testing.T) {
	defer SetPrefixDisabledForTest(false)()
	prev := isTerminalSinkFn
	defer func() { isTerminalSinkFn = prev }()
	tty := &taggedWriter{tag: "tty"}
	pipe := &taggedWriter{tag: "pipe"}
	isTerminalSinkFn = func(w io.Writer) bool {
		tw, _ := w.(*taggedWriter)
		return tw != nil && tw.tag == "tty"
	}
	if !shouldPrefix(Task{Concurrent: true}, tty) {
		t.Error("TTY sink should be prefixed")
	}
	if shouldPrefix(Task{Concurrent: true}, pipe) {
		t.Error("non-TTY sink should not be prefixed even if peer is TTY")
	}
}

type taggedWriter struct{ tag string }

func (t *taggedWriter) Write(p []byte) (int, error) { return len(p), nil }

// stubTerminalSink overrides the TTY detector for the duration of a
// test. Production callers always read isTerminalSink directly.
func stubTerminalSink(v bool) func() {
	prev := isTerminalSinkFn
	isTerminalSinkFn = func(io.Writer) bool { return v }
	return func() { isTerminalSinkFn = prev }
}
