package main

// daemonstdio_test.go — DG-6: the daemon's own launchd-piped stdout/stderr
// must be truncated IN PLACE (never renamed out from under the fd launchd
// already holds), the write offset must reset to 0 (never leave a
// sparse-file hole of NUL bytes), and one prior generation must be kept.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/dlog"
)

// openSameFile opens path for read/write and returns the *os.File — used so
// the test's "f" and "path" are provably os.SameFile, exactly the launchd
// shape truncateStdioIfNeeded exists for.
func openSameFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestTruncateStdioIfNeeded_UnderCap_NoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fathom-http.log")
	f := openSameFile(t, path)
	if _, err := f.WriteString("small"); err != nil {
		t.Fatal(err)
	}
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	truncateStdioIfNeeded(f, path, 1000, log)

	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != int64(len("small")) {
		t.Errorf("size = %d after a no-op rotation, want unchanged (%d)", fi.Size(), len("small"))
	}
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Error("expected no backup file for a rotation that never fired")
	}
}

// TestTruncateStdioIfNeeded_OverCap_ResetsOffsetWithoutSparseHole is the
// load-bearing proof: after truncation, a subsequent write through the SAME
// still-open handle must land at byte 0, not at the old (pre-truncate)
// length — a Truncate without a Seek reset would leave the write offset
// where it was, and the next write would create a sparse hole of NUL bytes
// from 0 up to the old offset before the new bytes ever appeared.
func TestTruncateStdioIfNeeded_OverCap_ResetsOffsetWithoutSparseHole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fathom-http.log")
	f := openSameFile(t, path)

	original := strings.Repeat("x", 200)
	if _, err := f.WriteString(original); err != nil {
		t.Fatal(err)
	}

	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	truncateStdioIfNeeded(f, path, 100, log)

	// The prior generation must be preserved byte-for-byte.
	backup, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backup) != original {
		t.Errorf("backup = %q, want the original %d bytes", backup, len(original))
	}

	// The live file must now be empty...
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != 0 {
		t.Fatalf("size after truncate = %d, want 0", fi.Size())
	}

	// ...and the NEXT write through the SAME handle must land at byte 0,
	// not at the old length — this is the sparse-file-hole regression this
	// test exists to catch.
	next := "new-generation"
	if _, err := f.WriteString(next); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != next {
		t.Errorf("file content after the post-truncate write = %q (len %d), want exactly %q with no leading NUL padding",
			got, len(got), next)
	}
}

// TestTruncateStdioIfNeeded_UnrelatedFile_LeavesItAlone pins the
// os.SameFile guard: a handle whose path does not resolve to the SAME file
// (a manual run with stdout redirected elsewhere, or no launchd file at
// that path at all) must never be truncated or backed up — there is
// nothing there to safely restore, and truncating an unrelated file wipe
// out unrelated log content.
func TestTruncateStdioIfNeeded_UnrelatedFile_LeavesItAlone(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "actual-stdout.log")
	f := openSameFile(t, actual)
	content := strings.Repeat("z", 200)
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}

	// unrelated is a DIFFERENT file that happens to share the "expected"
	// launchd path name but is not the file f is writing to.
	unrelated := filepath.Join(dir, "fathom-http.log")
	if err := os.WriteFile(unrelated, []byte("unrelated content"), 0o644); err != nil {
		t.Fatal(err)
	}

	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	truncateStdioIfNeeded(f, unrelated, 100, log)

	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != int64(len(content)) {
		t.Errorf("f was mutated (size %d, want %d) despite not being os.SameFile as the rotation target", fi.Size(), len(content))
	}
	if _, err := os.Stat(unrelated + ".1"); err == nil {
		t.Error("an unrelated file must never be backed up as if it were the daemon's own log")
	}
}

func TestRotateDaemonStdio_NoHomeDir_NoOp(t *testing.T) {
	// HOME resolution failing (os.UserHomeDir) must degrade to a silent
	// no-op, never a panic — same discipline as every other best-effort
	// beat-log operation in this codebase.
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "") // windows' os.UserHomeDir fallback
	log := dlog.New(io.Discard, dlog.LevelDebug, time.Now)
	rotateDaemonStdio(nil, nil, 100, log, log) // must not panic
}

// TestTruncateStdioIfNeeded_ConcurrentWriterNeverProducesSparseHole is the
// finding [1] regression test: the daemon's *dlog.Logger writes to the SAME
// *os.File this function truncates (servehttp.go's flog/outLog share their
// underlying os.Stdout/os.Stderr with rotateDaemonStdio's out/errW), from
// many concurrent request-handling goroutines, 24/7. Before
// Logger.WithWriteLock existed, a Write landing between Truncate(0) and
// Seek(0, io.SeekStart) would land at the pre-truncate offset, padding the
// file with NUL bytes up to there — this test drives real concurrent
// writers THROUGH THE LOGGER against the exact file being rotated and
// proves no such hole appears, and that no logged line is torn/corrupted.
func TestTruncateStdioIfNeeded_ConcurrentWriterNeverProducesSparseHole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fathom-http.log")
	f := openSameFile(t, path)

	log := dlog.New(f, dlog.LevelDebug, time.Now)

	// Get the file over the rotation threshold up front so the very first
	// truncateStdioIfNeeded call fires immediately, maximizing overlap with
	// the concurrent writers below instead of racing a size check.
	if _, err := f.WriteString(strings.Repeat("x", 200)); err != nil {
		t.Fatal(err)
	}

	const writers = 16
	const linesPerWriter = 50
	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < linesPerWriter; j++ {
				log.Info(fmt.Sprintf("writer %d line %d", id, j))
			}
		}(i)
	}

	// Fire rotation repeatedly WHILE the writers above are active, exactly
	// like the daemon's scavenge ticker racing live request-handling
	// goroutines.
	var rotateWG sync.WaitGroup
	rotateWG.Add(1)
	go func() {
		defer rotateWG.Done()
		for {
			select {
			case <-stop:
				return
			default:
				truncateStdioIfNeeded(f, path, 100, log)
			}
		}
	}()

	wg.Wait()
	close(stop)
	rotateWG.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if idx := strings.IndexByte(string(data), 0); idx >= 0 {
		t.Fatalf("found a NUL byte at offset %d — a concurrent write raced Truncate+Seek and left a sparse hole", idx)
	}
	// Every non-empty line must be a COMPLETE, well-formed log line (never
	// a torn fragment of two interleaved writes) — slog's text handler
	// always starts a line with "time=".
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "time=") {
			t.Fatalf("corrupted/torn log line (does not start with \"time=\"): %q", line)
		}
	}
}

// TestScavengeTick_FlushesDedupeOnBothLoggers is the finding [2]/[7]
// regression test: Logger.FlushDedupe (DG-2) was fully implemented and
// unit-tested in internal/dlog but never called from any production code
// path. scavengeTick is what servehttp.go's scavenge-ticker closure now
// calls every interval — this proves it actually flushes BOTH loggers'
// deduped counts (each Logger owns its own independent dedupeState), not
// just one.
func TestScavengeTick_FlushesDedupeOnBothLoggers(t *testing.T) {
	var outBuf, errBuf strings.Builder
	fixed := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	outLog := dlog.New(&outBuf, dlog.LevelDebug, func() time.Time { return fixed })
	flog := dlog.New(&errBuf, dlog.LevelDebug, func() time.Time { return fixed })

	// Seed a repeating condition on EACH logger so a flush has something to
	// report.
	for i := 0; i < 3; i++ {
		outLog.Dedupe(dlog.LevelWarn, "out-key", "out condition: %d", i)
		flog.Dedupe(dlog.LevelWarn, "err-key", "fleet: sources for demo: %d", i)
	}
	outBuf.Reset()
	errBuf.Reset()

	// nil files: rotateDaemonStdio degrades to a no-op for those (see
	// TestRotateDaemonStdio_NoHomeDir_NoOp) — this test is only about the
	// dedupe-flush wiring, not rotation itself.
	scavengeTick(outLog, flog, func() time.Time { return fixed.Add(time.Minute) })

	if !strings.Contains(outBuf.String(), "repeated 2 times") {
		t.Errorf("scavengeTick did not flush outLog's dedupe summary; got %q", outBuf.String())
	}
	if !strings.Contains(errBuf.String(), "repeated 2 times") {
		t.Errorf("scavengeTick did not flush flog's dedupe summary; got %q", errBuf.String())
	}
}

// RT-127 (design §12.2): the stdio logs rotate on their own ticker (with
// FATHOM_SCAVENGE=off too), the prior generation is streamed to .1, and
// above the hard cap the log is truncated without a copy and one line
// names the discarded bytes.
func TestStdioRotationStreamsAndHardCaps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FATHOM_SCAVENGE", "off")
	dir := filepath.Join(home, ".fathom", "logs")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fathom-http.log")
	f := openSameFile(t, path)
	var logBuf strings.Builder
	var mu sync.Mutex
	log := dlog.New(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return logBuf.Write(p) }), dlog.LevelDebug, time.Now)
	original := strings.Repeat("line of daemon output\n", 20)
	if _, err := f.WriteString(original); err != nil {
		t.Fatal(err)
	}
	prev := daemonStdioRotateBytes
	daemonStdioRotateBytes = 100
	t.Cleanup(func() { daemonStdioRotateBytes = prev })

	// Its own ticker, scavenging off.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { runStdioRotation(ctx, 5*time.Millisecond, f, nil, log, log); close(done) }()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if b, err := os.ReadFile(path + ".1"); err == nil && string(b) == original {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the stdio ticker did not rotate with FATHOM_SCAVENGE=off")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done
	if st, _ := f.Stat(); st.Size() != 0 {
		t.Fatalf("rotated log is %d bytes, want truncated", st.Size())
	}
	if _, err := os.Stat(path + ".1.tmp"); !os.IsNotExist(err) {
		t.Fatal("the streamed copy's temp file was left behind")
	}
	if _, err := f.WriteString("next\n"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != "next\n" {
		t.Fatalf("after rotation the next write lands at 0: %q", b)
	}

	// Hard cap: no copy, the discarded bytes named.
	if err := os.WriteFile(path+".1", []byte("previous generation"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(strings.Repeat("y", 500)); err != nil {
		t.Fatal(err)
	}
	rotateStdioLog(f, path, 100, 400, log)
	if b, _ := os.ReadFile(path + ".1"); string(b) != "previous generation" {
		t.Fatalf("over the hard cap nothing is copied; .1 = %q", b)
	}
	if st, _ := f.Stat(); st.Size() != 0 {
		t.Fatalf("over the hard cap the log is truncated; size %d", st.Size())
	}
	mu.Lock()
	out := logBuf.String()
	mu.Unlock()
	if !strings.Contains(out, "505 bytes discarded") || !strings.Contains(out, "WITHOUT a prior generation") {
		t.Fatalf("the hard-cap line must name the discarded bytes: %q", out)
	}
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }
