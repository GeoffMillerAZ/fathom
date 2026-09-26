package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// TestDaemonReapSparesBoundedRecordlessDaemons (RT-24 review): a daemon
// whose record lives under another HOME (stranger-check, a
// testisolate'd suite, scenariobench) reads "no-record" from the
// operator's HOME. With its parent alive and its own bounds in force it is
// not a kill target — reaping it during `task validate` killed the gate's
// own daemon. Record-less daemons whose parent is gone are "orphaned" and
// still reaped.
func TestDaemonReapSparesBoundedRecordlessDaemons(t *testing.T) {
	rows := []censusRow{
		{PID: 700, PPID: 60, ParentAlive: true, Record: "none", Verdict: "no-record", Addr: "127.0.0.1:7856", Command: "/x/fathom serve-http --addr 127.0.0.1:7856"},
		{PID: 710, PPID: 1, ParentAlive: false, Record: "none", Verdict: "orphaned", Addr: "127.0.0.1:7857", Command: "/x/fathom serve-http --addr 127.0.0.1:7857"},
	}
	f := &fakeReap{rows: [][]censusRow{rows}, alive: map[int]bool{700: true, 710: true}}
	var out, errb bytes.Buffer
	if code := runDaemonReap([]string{"--apply"}, &out, &errb, f.env()); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	sigs := strings.Join(f.signals, ",")
	if strings.Contains(sigs, " 700") {
		t.Fatalf("a record-less daemon with a live parent was signalled: %s\n%s", sigs, out.String())
	}
	if !strings.Contains(out.String(), "keep pid 700") {
		t.Errorf("the spared daemon is not named:\n%s", out.String())
	}
	if !strings.Contains(sigs, "terminated 710") {
		t.Errorf("the record-less orphan was not reaped: %s", sigs)
	}
}

// TestDaemonReapRechecksBeforePruning (RT-24 review): a daemon that binds
// and writes its record between the census's ps and its record read shows
// as stale-record; the prune re-checks the pid and keeps a live one's.
func TestDaemonReapRechecksBeforePruning(t *testing.T) {
	rows := []censusRow{
		{PID: 900, Record: "stale", Verdict: "stale-record", RecordPath: "/h/.fathom/daemon/addr-7001-900/record.json"},
		{PID: 901, Record: "stale", Verdict: "stale-record", RecordPath: "/h/.fathom/daemon/addr-7002-901/record.json"},
	}
	f := &fakeReap{rows: [][]censusRow{rows}, alive: map[int]bool{900: true}}
	var out, errb bytes.Buffer
	if code := runDaemonReap([]string{"--apply"}, &out, &errb, f.env()); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	pruned := strings.Join(f.pruned, ",")
	if strings.Contains(pruned, "addr-7001-900") {
		t.Fatalf("a live daemon's record was pruned: %s", pruned)
	}
	if !strings.Contains(pruned, "addr-7002-901") {
		t.Fatalf("a dead daemon's record was not pruned: %s", pruned)
	}
}

// TestHeartbeatCountsOpenStreamsAsActivity (RT-23 review): an open /events
// stream is a request in flight; the heartbeat records the daemon as
// active now while one is open, so the census never reads a daemon
// feeding a console as idle and reap never kills it for that.
func TestHeartbeatCountsOpenStreamsAsActivity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	resetServingCallLogForTest(t)
	t0 := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	now := t0
	clock := func() time.Time { return now }
	_, path, err := recordServingDaemonAs(daemonMeta{}, "127.0.0.1:7857", "", "", "c0ffee", t0)
	if err != nil {
		t.Fatal(err)
	}
	act := newActivity(clock)
	act.inFlight.Add(1) // an /events subscriber, open since t0
	lt := &daemonLifetime{
		bounds: tenancy.DefaultUnmanagedBounds(), startedAt: t0, act: act,
		recordPath: path, now: clock, logf: func(string) {}, exit: func(int) {},
	}
	now = t0.Add(2 * time.Hour)
	if r := lt.tick(); r != "" {
		t.Fatalf("exited while serving a stream: %s", r)
	}
	rec, err := console.ReadDaemonRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	if rec.LastActivityAt != now.Format(time.RFC3339) {
		t.Fatalf("last_activity_at = %s with a stream open, want %s", rec.LastActivityAt, now.Format(time.RFC3339))
	}
	rows := daemonCensus([]storedRecord{{Path: path, Rec: rec}}, []psProc{{PID: rec.PID, PPID: 50, Command: "/w/fathom serve-http --addr 127.0.0.1:7857"}}, now, "127.0.0.1:7575", func(string) (uint64, bool) { return rec.ExeInode, true })
	if len(rows) != 1 || rows[0].Verdict == "idle" {
		t.Fatalf("census reads a streaming daemon idle: %+v", rows)
	}
}

// TestHeartbeatNeverRecreatesARemovedRecord (RT-20 review): a heartbeat
// that read the record before a concurrent shutdown (the parent-exit
// watch) removed it must not write it back — the process then exits and
// the record would be stale for good.
func TestHeartbeatNeverRecreatesARemovedRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	resetServingCallLogForTest(t)
	t0 := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return t0 }
	_, path, err := recordServingDaemonAs(daemonMeta{}, "127.0.0.1:7858", "", "", "c0ffee", t0)
	if err != nil {
		t.Fatal(err)
	}
	lt := &daemonLifetime{
		managed: true, // no exit bounds: only the heartbeat runs
		act:     newActivity(clock), recordPath: path, now: clock,
		logf: func(string) {}, exit: func(int) {},
	}
	lt.readRecord = func(p string) (console.DaemonRecord, error) {
		rec, err := console.ReadDaemonRecord(p)
		lt.shutdown("parent 1 exited") // lands between the read and the write
		return rec, err
	}
	lt.tick()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the heartbeat recreated the record shutdown removed (err=%v)", err)
	}
}
