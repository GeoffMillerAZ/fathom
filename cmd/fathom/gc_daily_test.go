package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// §12.1: the managed daemon runs gc's daily classes itself — session
// stamps, dead daemons' records, the $TMPDIR caches, diag captures — once
// a day, never in its first 10 minutes, never while work is in flight or
// the ladder is above normal. It never touches what §12.1 only reports
// (ledgers, the workspace classes) and never deletes the call log's
// evidence.
func TestManagedDaemonRunsDailyGC(t *testing.T) {
	home, tmp := t.TempDir(), t.TempDir()
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	old := now.Add(-40 * 24 * time.Hour)
	stamp := filepath.Join(home, ".fathom", "session-stamps", "s1.json")
	gcTouch(t, stamp, 10, old)
	cache := filepath.Join(tmp, "fathom-toplevel-cache", "a.json")
	gcTouch(t, cache, 10, old)
	ledger := filepath.Join(home, ".fathom", "session-ledger.jsonl")
	gcTouch(t, ledger, 40<<20, old)

	var logs []string
	logf := func(m string) { logs = append(logs, m) }
	st := gcDailyState{started: now}
	run := func(at time.Time) gcReport { return runGC(gcOptions{Home: home, TmpDir: tmp, Apply: true, Now: at}) }

	// The first 10 minutes: never.
	if gcDailyTick(&st, now.Add(5*time.Minute), false, run, logf) {
		t.Fatal("gc ran in the daemon's first 10 minutes")
	}
	// Busy (in flight, or the ladder above normal): not now.
	if gcDailyTick(&st, now.Add(20*time.Minute), true, run, logf) {
		t.Fatal("gc ran while the daemon was busy")
	}
	if _, err := os.Stat(stamp); err != nil {
		t.Fatal("a skipped tick deleted something")
	}
	// Idle after warmup: it runs, applying the daily classes only.
	if !gcDailyTick(&st, now.Add(30*time.Minute), false, run, logf) {
		t.Fatal("gc did not run when idle after warmup")
	}
	for _, p := range []string{stamp, cache} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s survived the daily gc", p)
		}
	}
	if _, err := os.Stat(ledger); err != nil {
		t.Fatalf("the daily gc touched a ledger (reported only): %v", err)
	}
	if len(logs) == 0 || !strings.Contains(logs[len(logs)-1], "gc (daily)") {
		t.Fatalf("the daily run is not logged: %v", logs)
	}
	// At most once a day.
	if gcDailyTick(&st, now.Add(20*time.Hour), false, run, logf) {
		t.Fatal("gc ran twice within a day")
	}
	if !gcDailyTick(&st, now.Add(25*time.Hour), false, run, logf) {
		t.Fatal("gc did not run a day later")
	}
}
