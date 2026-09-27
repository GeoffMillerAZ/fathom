package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// RT-126 hygiene: the daemon's writer only renames at the bound (two
// renames under its mutex); the roll-up and gzip of the previous
// generation run in the background, off the request path, so a row write
// never waits on them; and a rotation that fails is retried at most once
// per callLogRotateRetry, not on every later row.
func TestCallLogRotationOffRequestPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-calls.jsonl")
	w, err := openRotatingCallLog(path, 256, nil)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 9, 27, 4, 0, 0, 0, time.UTC)
	w.now = func() time.Time { clock = clock.Add(time.Second); return clock }
	release := make(chan struct{})
	var archiving atomic.Int32
	w.archiveHook = func() { archiving.Add(1); <-release }
	row := func(i int) []byte {
		return []byte(fmt.Sprintf(`{"ts":"2026-09-27T04:00:%02dZ","tool":"search","seq":%d}`+"\n", i%60, i))
	}
	done := make(chan struct{})
	go func() {
		for i := 0; i < 40; i++ {
			if _, err := w.Write(row(i)); err != nil {
				t.Error(err)
			}
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("a row write waited on the previous generation's roll-up and gzip")
	}
	if archiving.Load() == 0 {
		t.Fatal("rotation past the bound never started a background archive pass")
	}
	close(release)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if p, _ := filepath.Glob(path + ".*.pending"); len(p) != 0 {
		t.Fatalf("Close waits for the archive pass; pending generations left: %v", p)
	}
	if len(callLogArchives(path)) == 0 {
		t.Fatal("no archive was written")
	}

	// A failed rotation: retried once per callLogRotateRetry, never per row.
	path2 := filepath.Join(dir, "b.jsonl")
	w2, err := openRotatingCallLog(path2, 64, nil)
	if err != nil {
		t.Fatal(err)
	}
	clock2 := time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC)
	w2.now = func() time.Time { return clock2 }
	var attempts int
	w2.rotate = func(string, time.Time) (callLogRotation, error) {
		attempts++
		return callLogRotation{}, errors.New("ENOSPC")
	}
	for i := 0; i < 50; i++ {
		if _, err := w2.Write(row(i)); err != nil {
			t.Fatal(err)
		}
	}
	if attempts != 1 {
		t.Fatalf("a failing rotation was attempted %d times in one retry window, want 1", attempts)
	}
	clock2 = clock2.Add(callLogRotateRetry + time.Second)
	_, _ = w2.Write(row(99))
	if attempts != 2 {
		t.Fatalf("after the retry window the rotation is tried again: %d attempts", attempts)
	}
	st, _ := os.Stat(path2)
	if st == nil || st.Size() == 0 {
		t.Fatal("a failed rotation keeps appending to the live log")
	}
	_ = w2.Close()
}

// RT-125/RT-126 hygiene: `fathom gc --apply` never renames a live call log
// a daemon holds open (the daemon's writer rotates it); it archives the
// pending generations under the archive lock the daemon's pass shares, and
// its archive accounting counts `fathom metrics rotate`'s dated archives
// too.
func TestGCArchivesPendingNeverRenamesLiveCallLog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-calls.jsonl")
	w, err := openRotatingCallLog(path, callLogRotateBytes, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if err := os.Truncate(path, callLogRotateBytes+1); err != nil {
		t.Fatal(err)
	}
	w.size = callLogRotateBytes + 1
	pending := path + ".20260926T000000Z.pending"
	if err := os.WriteFile(pending, []byte(`{"ts":"2026-09-26T00:00:00Z","tool":"search"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	metricsArchive := path + ".archive-2026-01-01"
	if err := os.WriteFile(metricsArchive, make([]byte, 1000), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(path)
	rep := runGC(gcOptions{CallLog: path, Apply: true, Now: time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)})
	if len(rep.Errors) > 0 {
		t.Fatalf("gc errors: %v", rep.Errors)
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("gc renamed the live call log a daemon holds open: %v", err)
	}
	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Fatalf("gc created a rotated generation from outside the daemon: %v", err)
	}
	if _, err := os.Stat(pending); !os.IsNotExist(err) {
		t.Fatalf("the pending generation was not archived: %v", err)
	}
	if _, err := os.Stat(path + ".20260926T000000Z.jsonl.gz"); err != nil {
		t.Fatalf("no archive for the pending generation: %v", err)
	}
	if n := callLogArchiveBytes(path); n < 1000 {
		t.Fatalf("archive accounting omits `metrics rotate`'s archive: %d bytes", n)
	}
	var live bool
	for _, it := range rep.Items {
		if it.Class == "call-log" && it.Path == path {
			live = true
			if it.Action != "report" {
				t.Fatalf("the live log over the bound is reported, not acted on: %+v", it)
			}
		}
	}
	if !live {
		t.Fatalf("the live log over the bound is not reported: %+v", rep.Items)
	}
}

// RT-126 hygiene (I38): a rotation hides no history from a usage reader.
// classifyCallLogRead (every metrics, instrument, flywheel, doctor, fleet
// and research reader) and the discoveries pointer rows read every
// generation — gzip archives, pending generations, <log>.1 and the live
// log — so a windowed answer counts rows the rotation moved.
func TestCallLogReadersReadEveryGeneration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-calls.jsonl")
	w, err := openRotatingCallLog(path, 512, nil)
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 9, 27, 4, 0, 0, 0, time.UTC)
	w.now = func() time.Time { clock = clock.Add(time.Minute); return clock }
	const n = 60
	for i := 0; i < n; i++ {
		row := fmt.Sprintf(`{"schema_version":3,"ts":"2026-09-27T04:%02d:00Z","tool":"search","service":"svc","surface":"agent","outcome":"ok","pointers":["p%d"]}`+"\n", i, i)
		if _, err := w.Write([]byte(row)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if len(callLogArchives(path)) == 0 {
		t.Fatal("the fixture must reach the archive generation")
	}
	// A pending generation (an archive pass not yet run) is read too.
	if err := os.WriteFile(path+".20260926T000000Z.pending", []byte(`{"schema_version":3,"ts":"2026-09-26T23:59:00Z","tool":"search","service":"svc","pointers":["old"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _, err := classifyCallLogRead(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != n+1 {
		t.Fatalf("classifyCallLogRead saw %d rows, want %d across every generation", len(entries), n+1)
	}
	if rows := readCallLogPointerRows(path, "svc", time.Time{}); len(rows) != n+1 {
		t.Fatalf("readCallLogPointerRows saw %d rows, want %d across every generation", len(rows), n+1)
	}
	// Absent everywhere is still the honest ENOENT.
	if _, _, err := classifyCallLogRead(filepath.Join(dir, "none.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("no generation at all = %v, want ENOENT", err)
	}
}

// RT-126 hygiene: the console's tail never copies the rotated generation
// when a live row arrives — the rotated rows are parsed once per rotation
// and returned as their own segment, shared, beside the live rows.
func TestConsoleTailSharesRotatedGeneration(t *testing.T) {
	resetCallLogTail()
	t.Cleanup(resetCallLogTail)
	path := filepath.Join(t.TempDir(), "mcp-calls.jsonl")
	line := func(i int) string {
		return fmt.Sprintf(`{"ts":"2026-09-27T04:00:%02dZ","tool":"search","service":"svc"}`+"\n", i%60)
	}
	var gen string
	for i := 0; i < 50; i++ {
		gen += line(i)
	}
	mustWrite(t, path+".1", gen)
	mustWrite(t, path, line(0))
	rot1, live1, err := callLogRowSegments(path)
	if err != nil || len(rot1) != 50 || len(live1) != 1 {
		t.Fatalf("segments = %d rotated, %d live, %v", len(rot1), len(live1), err)
	}
	fh, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	_, _ = fh.WriteString(line(1))
	fh.Close()
	rot2, live2, err := callLogRowSegments(path)
	if err != nil || len(live2) != 2 {
		t.Fatalf("after a live append: %d live, %v", len(live2), err)
	}
	if &rot2[0] != &rot1[0] {
		t.Fatal("a live append copied the rotated generation")
	}
	if got := callEntriesFor("svc", time.Time{}, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), rot2, live2); len(got) != 52 {
		t.Fatalf("callEntriesFor over both segments = %d, want 52", len(got))
	}
}
