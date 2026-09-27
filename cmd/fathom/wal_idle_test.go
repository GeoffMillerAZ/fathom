package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// RT-128 (design §12.2): the WAL is truncated after 5 idle minutes — no
// request and nothing in flight — and at most hourly; activity resets the
// idle clock; and the checkpoint really truncates the -wal file.
func TestWALTruncatedWhenIdle(t *testing.T) {
	var st walIdleState
	t0 := time.Date(2026, 9, 27, 5, 0, 0, 0, time.UTC)
	runs := 0
	cp := func() error { runs++; return nil }
	var logs []string
	logf := func(m string) { logs = append(logs, m) }
	tick := func(at time.Duration, activity int64, busy bool) bool {
		return checkpointWhenIdle(&st, t0.Add(at), activity, busy, cp, logf)
	}
	tick(0, 10, false)
	if tick(4*time.Minute, 10, false) {
		t.Fatal("4 idle minutes is not idle enough")
	}
	if !tick(5*time.Minute, 10, false) || runs != 1 {
		t.Fatalf("5 idle minutes checkpoints: runs %d", runs)
	}
	if tick(20*time.Minute, 10, false) || runs != 1 {
		t.Fatal("at most hourly")
	}
	// Activity resets the idle clock; in-flight work is activity.
	tick(61*time.Minute, 11, false)
	if tick(65*time.Minute, 11, false) {
		t.Fatal("4 minutes after activity is not idle")
	}
	tick(65*time.Minute+30*time.Second, 11, true) // busy
	if tick(69*time.Minute, 11, false) {
		t.Fatal("in-flight work resets the idle clock")
	}
	if !tick(71*time.Minute, 11, false) || runs != 2 {
		t.Fatalf("idle again for 5 min, an hour after the last: runs %d", runs)
	}
	if len(logs) != 2 || !strings.Contains(logs[0], "checkpoint(TRUNCATE) ran") {
		t.Fatalf("each checkpoint is logged: %v", logs)
	}

	// The real store: the checkpoint the loop runs truncates the -wal.
	path := filepath.Join(t.TempDir(), "s.sqlite")
	s, err := store.NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.PutObservations(context.Background(), []core.Observation{{ServiceID: "svc", SourceRef: "a.go:1", Symbol: "A", Claim: "func A()"}}); err != nil {
		t.Fatalf("seeding the store: %v", err)
	}
	if st, err := os.Stat(path + "-wal"); err != nil || st.Size() == 0 {
		t.Fatalf("a write leaves a -wal to truncate: %v", err)
	}
	var st2 walIdleState
	checkpointWhenIdle(&st2, t0, 0, false, func() error { return nil }, logf)
	if !checkpointWhenIdle(&st2, t0.Add(walIdleAfter), 0, false, func() error { return s.Checkpoint(context.Background()) }, logf) {
		t.Fatal("idle checkpoint did not run")
	}
	if st, err := os.Stat(path + "-wal"); err == nil && st.Size() != 0 {
		t.Fatalf("-wal is %d bytes after the idle checkpoint, want truncated", st.Size())
	}
}
