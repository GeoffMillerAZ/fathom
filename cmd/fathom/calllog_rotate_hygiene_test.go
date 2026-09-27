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
