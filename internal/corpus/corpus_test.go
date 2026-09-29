package corpus

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// meteredCtx is a context carrying an admission ticket, so the test can
// read what corpus charged the call's meter.
func meteredCtx(t *testing.T) (context.Context, *tenancy.Ticket) {
	t.Helper()
	g := tenancy.NewGate(tenancy.GateConfig{Mode: tenancy.ModeOff})
	tk, ref := g.Admit(context.Background(), tenancy.Request{Tool: "t", Class: tenancy.ClassPoint})
	if ref != nil {
		t.Fatalf("refused: %+v", ref)
	}
	t.Cleanup(func() { tk.Release(tenancy.OutcomeOK) })
	return tk.Context(), tk
}

// TestReadFileCapped_StatsBeforeRead is RT-66 (§4.1): an oversize file and
// a non-regular file are excluded from the stat, never opened; a binary is
// excluded after an 8 KiB sniff, never read whole; a text file is read and
// its bytes charged to the call's meter.
func TestReadFileCapped_StatsBeforeRead(t *testing.T) {
	dir := t.TempDir()
	ctx, tk := meteredCtx(t)

	// Oversize and unreadable (mode 000): only a stat can name it oversize —
	// an open would fail first and name it unreadable.
	big := filepath.Join(dir, "big.txt")
	if err := os.WriteFile(big, bytes.Repeat([]byte("a"), 3<<20), 0o000); err != nil {
		t.Fatal(err)
	}
	data, ex, err := ReadFileCapped(ctx, big, DefaultCap)
	if err != nil || data != nil || ex == nil || ex.Reason != ReasonOversize || ex.Size != 3<<20 {
		t.Fatalf("oversize: data=%d ex=%+v err=%v", len(data), ex, err)
	}

	// A FIFO: opening it would block forever; the stat names it.
	fifo := filepath.Join(dir, "pipe")
	if err := syscall.Mkfifo(fifo, 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, ex, err = ReadFileCapped(ctx, fifo, DefaultCap)
	}()
	select {
	case <-done:
	case <-time.After(waitSafety):
		t.Fatal("ReadFileCapped opened a FIFO")
	}
	if err != nil || ex == nil || ex.Reason != ReasonNotRegular {
		t.Fatalf("fifo: ex=%+v err=%v", ex, err)
	}

	// Binary: NUL in the first 8 KiB of a 1 MiB file; the sniff reads at
	// most 8 KiB and charges only that.
	bin := filepath.Join(dir, "blob.bin")
	payload := append([]byte{0, 1, 2}, bytes.Repeat([]byte("x"), 1<<20)...)
	if err := os.WriteFile(bin, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	before := tk.BytesRead()
	data, ex, err = ReadFileCapped(ctx, bin, DefaultCap)
	if err != nil || data != nil || ex == nil || ex.Reason != ReasonBinary {
		t.Fatalf("binary: data=%d ex=%+v err=%v", len(data), ex, err)
	}
	if got := tk.BytesRead() - before; got > SniffSize {
		t.Fatalf("binary sniff charged %d bytes, want ≤ %d", got, SniffSize)
	}

	// Text: read whole, charged.
	txt := filepath.Join(dir, "main.go")
	src := "package main\n" + strings.Repeat("// line\n", 3000)
	if err := os.WriteFile(txt, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	before = tk.BytesRead()
	data, ex, err = ReadFileCapped(ctx, txt, DefaultCap)
	if err != nil || ex != nil || string(data) != src {
		t.Fatalf("text: len=%d ex=%+v err=%v", len(data), ex, err)
	}
	if got := tk.BytesRead() - before; got != int64(len(src)) {
		t.Fatalf("text charged %d, want %d", got, len(src))
	}

	// A missing file is an error, not an exclusion; a cancelled ctx reads
	// nothing.
	if _, _, err := ReadFileCapped(ctx, filepath.Join(dir, "nope"), DefaultCap); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing: %v", err)
	}
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err := ReadFileCapped(cctx, txt, DefaultCap); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
}

// TestScanBudgeted_StopsAndReportsLowerBound is RT-67 (§4.1): the scan
// visits one file at a time and stops at its bytes, files, wall, hits or
// ctx bound, naming the stop and reporting coverage as a lower bound;
// exclusions are named, never silently dropped.
func TestScanBudgeted_StopsAndReportsLowerBound(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for i := range 10 {
		name := filepath.Join("pkg", "f"+string(rune('0'+i))+".go")
		if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), bytes.Repeat([]byte("y"), 1000), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, name)
	}
	if err := os.WriteFile(filepath.Join(dir, "img.bin"), []byte{0, 0, 0, 1}, 0o644); err != nil {
		t.Fatal(err)
	}
	paths = append(paths, "img.bin")
	ctx := context.Background()

	// Unbounded: every file, the binary named.
	var seen int
	cov, err := ScanBudgeted(ctx, dir, paths, Budget{}, func(p string, b []byte) bool { seen++; return false })
	if err != nil || cov.Stopped != "" || cov.Scanned != 10 || cov.Total != 11 || len(cov.Excluded) != 1 || cov.Excluded[0].Reason != ReasonBinary || cov.LowerBound() {
		t.Fatalf("full scan: %+v err=%v seen=%d", cov, err, seen)
	}

	cases := []struct {
		name string
		b    Budget
		hit  int // fn stops after this many files (0 = never)
		want string
		max  int // most files scanned
	}{
		{"bytes", Budget{Bytes: 3500}, 0, StopBytes, 3},
		{"files", Budget{Files: 4}, 0, StopFiles, 4},
		{"hits", Budget{}, 2, StopHits, 2},
	}
	for _, c := range cases {
		n := 0
		cov, err := ScanBudgeted(ctx, dir, paths, c.b, func(string, []byte) bool {
			n++
			return c.hit > 0 && n >= c.hit
		})
		if err != nil || cov.Stopped != c.want || cov.Scanned > c.max || !cov.LowerBound() {
			t.Fatalf("%s: %+v err=%v", c.name, cov, err)
		}
		if !strings.Contains(cov.String(), "budget "+c.want) && c.want != StopHits {
			t.Fatalf("%s: render %q does not name the stop", c.name, cov.String())
		}
	}

	// Wall: an injected clock that jumps past the bound after one file.
	clock := time.Unix(0, 0)
	b := Budget{Wall: time.Second, Now: func() time.Time { t := clock; clock = clock.Add(700 * time.Millisecond); return t }}
	cov, _ = ScanBudgeted(ctx, dir, paths, b, func(string, []byte) bool { return false })
	if cov.Stopped != StopTime || cov.Scanned >= 10 {
		t.Fatalf("wall: %+v", cov)
	}

	// Ctx: cancelled after the first file.
	cctx, cancel := context.WithCancel(ctx)
	cov, err = ScanBudgeted(cctx, dir, paths, Budget{}, func(string, []byte) bool { cancel(); return false })
	if cov.Stopped != StopCtx || cov.Scanned != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("ctx: %+v err=%v", cov, err)
	}
}

// TestScanBudgeted_OversizeFileNeverSpendsTheByteBudget (review finding,
// RT-67): a file over the per-file cap is excluded by its stat and never
// read, so it spends none of the scan's byte budget — one large file must
// not stop a scan at zero files with Stopped=bytes.
func TestScanBudgeted_OversizeFileNeverSpendsTheByteBudget(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a_big.bin"), bytes.Repeat([]byte("a"), 400), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b_small.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	var seen []string
	cov, err := ScanBudgeted(context.Background(), dir, []string{"a_big.bin", "b_small.txt"}, Budget{Bytes: 100, FileCap: 50},
		func(p string, _ []byte) bool { seen = append(seen, p); return false })
	if err != nil {
		t.Fatal(err)
	}
	if cov.Stopped != "" || cov.Scanned != 1 || len(seen) != 1 || seen[0] != "b_small.txt" {
		t.Fatalf("coverage %+v, seen %v; want the small file scanned and no stop", cov, seen)
	}
	if len(cov.Excluded) != 1 || cov.Excluded[0].Reason != ReasonOversize {
		t.Fatalf("excluded %+v; want the big file named oversize", cov.Excluded)
	}
}
