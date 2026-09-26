package main

import (
	"bytes"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

var censusNow = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

func censusFixture() ([]storedRecord, []psProc, func(string) (uint64, bool)) {
	ts := func(d time.Duration) string { return censusNow.Add(-d).Format(time.RFC3339) }
	recs := []storedRecord{
		{Path: "/h/.fathom/daemon/dev.fathom.serve-http/record.json", Rec: console.DaemonRecord{SchemaVersion: 2, ID: "dev.fathom.serve-http", Managed: true, PID: 100, Addr: "127.0.0.1:7575", Exe: "/bin/fathom", ExeInode: 1, LastActivityAt: ts(5 * time.Hour)}},
		{Path: "/h/.fathom/daemon/addr-7851-200/record.json", Rec: console.DaemonRecord{SchemaVersion: 2, PID: 200, PPID: 50, Addr: "127.0.0.1:7851", Exe: "/w/fathom", ExeInode: 7, LastActivityAt: ts(time.Minute)}},
		{Path: "/h/.fathom/daemon/addr-7852-300/record.json", Rec: console.DaemonRecord{SchemaVersion: 2, PID: 300, PPID: 51, Addr: "127.0.0.1:7852", Exe: "/w/fathom", ExeInode: 7, LastActivityAt: ts(time.Minute)}},
		{Path: "/h/.fathom/daemon/addr-7853-400/record.json", Rec: console.DaemonRecord{SchemaVersion: 2, PID: 400, PPID: 52, Addr: "127.0.0.1:7853", Exe: "/w/fathom", ExeInode: 7, LastActivityAt: ts(2 * time.Hour)}},
		{Path: "/h/.fathom/daemon/addr-7854-500/record.json", Rec: console.DaemonRecord{SchemaVersion: 2, PID: 500, PPID: 53, Addr: "127.0.0.1:7854", Exe: "/w/old-fathom", ExeInode: 9, LastActivityAt: ts(time.Minute)}},
		{Path: "/h/.fathom/daemons/127.0.0.1-7855.json", Legacy: true, Rec: console.DaemonRecord{SchemaVersion: 1, PID: 600, Addr: "127.0.0.1:7855"}},
	}
	procs := []psProc{
		{PID: 100, PPID: 1, Elapsed: 9 * time.Hour, RSSKiB: 2 << 20, Command: "/bin/fathom serve-http --store /s/fleet.sqlite"},
		{PID: 200, PPID: 50, Elapsed: time.Hour, RSSKiB: 1 << 10, Command: "/w/fathom serve-http --store /t/a.sqlite --addr 127.0.0.1:7851"},
		{PID: 300, PPID: 1, Elapsed: 22 * time.Hour, RSSKiB: 1 << 10, Command: "/w/fathom serve-http --store /t/b.sqlite --addr 127.0.0.1:7852"},
		{PID: 400, PPID: 52, Elapsed: 3 * time.Hour, RSSKiB: 1 << 10, Command: "/w/fathom serve-http --store /t/c.sqlite --addr 127.0.0.1:7853"},
		{PID: 500, PPID: 53, Elapsed: time.Hour, RSSKiB: 1 << 10, Command: "/w/old-fathom serve-http --store /t/d.sqlite -addr=127.0.0.1:7854"},
		{PID: 700, PPID: 60, Elapsed: 23 * time.Hour, RSSKiB: 1 << 10, Command: "/x/fathom serve-http --store /t/e.sqlite --addr 127.0.0.1:7856"},
		{PID: 800, PPID: 1, Elapsed: 23 * time.Hour, RSSKiB: 1 << 10, Command: "/x/fathom serve-http --store /s/fleet.sqlite"},
	}
	stat := func(p string) (uint64, bool) {
		switch p {
		case "/bin/fathom":
			return 1, true
		case "/w/fathom", "/x/fathom":
			return 7, true
		}
		return 0, false // /w/old-fathom was deleted
	}
	return recs, procs, stat
}

// TestDaemonCensusVerdicts (RT-23, §10.2): records joined with the live
// census name ok, orphaned, idle, unlinked, no-record and stale-record
// daemons; the managed daemon (by record, or a record-less one on the
// managed address) is never judged by the scratch bounds.
func TestDaemonCensusVerdicts(t *testing.T) {
	recs, procs, stat := censusFixture()
	rows := daemonCensus(recs, procs, censusNow, "127.0.0.1:7575", stat)
	got := map[int]string{}
	managed := map[int]bool{}
	for _, r := range rows {
		got[r.PID] = r.Verdict
		managed[r.PID] = r.Managed
	}
	want := map[int]string{
		100: "ok",           // managed; idle 5 h is not a verdict for it
		200: "ok",           // parent alive, active
		300: "orphaned",     // ppid 1
		400: "idle",         // 2 h without a request
		500: "unlinked",     // its executable is gone
		600: "stale-record", // legacy record, no such process
		700: "no-record",    // a scratch daemon with no record
		800: "no-record",    // no record, default address = the managed one: managed, never reaped
	}
	for pid, v := range want {
		if got[pid] != v {
			t.Errorf("pid %d: verdict %q, want %q", pid, got[pid], v)
		}
	}
	if !managed[100] || !managed[800] || managed[300] {
		t.Errorf("managed flags wrong: %v", managed)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i-1].PID > rows[i].PID {
			t.Fatalf("rows not sorted by pid: %v", rows)
		}
	}
	var out bytes.Buffer
	renderDaemonCensus(&out, rows, censusNow)
	for _, s := range []string{"PID", "orphaned", "stale-record", "no-record", "127.0.0.1:7852"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("table lacks %q:\n%s", s, out.String())
		}
	}

	procLine := "  300     1 22:01:02   1024 /w/fathom serve-http --store /t/b.sqlite --addr 127.0.0.1:7852"
	p, ok := parsePsLine(procLine)
	if !ok || p.PID != 300 || p.PPID != 1 || p.Elapsed != 22*time.Hour+time.Minute+2*time.Second || p.RSSKiB != 1024 {
		t.Fatalf("parsePsLine: %+v ok=%v", p, ok)
	}
	if _, ok := parsePsLine("  301 1 01:00 12 /usr/bin/vim notes-about-serve-http.md"); ok {
		t.Fatal("a non-fathom process mentioning serve-http was counted")
	}
	if d := parseEtime("2-03:04:05"); d != 2*24*time.Hour+3*time.Hour+4*time.Minute+5*time.Second {
		t.Fatalf("parseEtime days: %v", d)
	}
}

type fakeReap struct {
	rows    [][]censusRow // successive census answers
	calls   int
	signals []string
	alive   map[int]bool
	pruned  []string
}

func (f *fakeReap) env() reapEnv {
	return reapEnv{
		census: func() ([]censusRow, error) {
			i := f.calls
			if i >= len(f.rows) {
				i = len(f.rows) - 1
			}
			f.calls++
			return f.rows[i], nil
		},
		signal: func(pid int, sig syscall.Signal) error {
			f.signals = append(f.signals, sig.String()+" "+strconv.Itoa(pid))
			if sig == syscall.SIGKILL {
				f.alive[pid] = false
			}
			return nil
		},
		alive:        func(pid int) bool { return f.alive[pid] },
		removeRecord: func(p string) { f.pruned = append(f.pruned, p) },
		sleep:        func(time.Duration) {},
		self:         42,
		selfParent:   41,
	}
}

// TestDaemonReapDryRunKillsNothing (RT-24, §10.2): reap is a dry run by
// default — no signal, no record removed — and with --apply it kills only
// unmanaged daemons whose verdict is not ok (SIGTERM, grace, SIGKILL),
// never the managed daemon, never itself, re-checks the pid before each
// kill (pid reuse), and prunes stale records.
func TestDaemonReapDryRunKillsNothing(t *testing.T) {
	recs, procs, stat := censusFixture()
	rows := daemonCensus(recs, procs, censusNow, "127.0.0.1:7575", stat)
	// The managed daemon also reads "unlinked" after a deploy replaced its
	// binary: still never touched.
	for i := range rows {
		if rows[i].PID == 100 {
			rows[i].Verdict = "unlinked"
		}
		if rows[i].PID == 700 { // a record-less scratch daemon whose parent has since died
			rows[i].Verdict, rows[i].ParentAlive = "orphaned", false
		}
	}
	rows = append(rows, censusRow{PID: 42, Verdict: "orphaned", Command: "fathom serve-http"}) // ourselves, somehow

	f := &fakeReap{rows: [][]censusRow{rows}, alive: map[int]bool{300: true, 400: true, 500: true, 700: true}}
	var out, errb bytes.Buffer
	if code := runDaemonReap(nil, &out, &errb, f.env()); code != 0 {
		t.Fatalf("dry run exit %d: %s", code, errb.String())
	}
	if len(f.signals) != 0 || len(f.pruned) != 0 {
		t.Fatalf("dry run signalled %v and pruned %v", f.signals, f.pruned)
	}
	for _, s := range []string{"would kill pid 300", "would kill pid 400", "would kill pid 500", "would kill pid 700", "would prune", "dry run"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("dry-run output lacks %q:\n%s", s, out.String())
		}
	}
	if strings.Contains(out.String(), "kill pid 100") || strings.Contains(out.String(), "kill pid 800") || strings.Contains(out.String(), "kill pid 42") {
		t.Fatalf("the managed daemon or reap itself is in the plan:\n%s", out.String())
	}

	// --apply: pid 400 exits on SIGTERM; 300, 500, 700 need SIGKILL; pid
	// 700's pid was reused by the time we get to it (the re-census shows a
	// different command) and is skipped.
	reused := make([]censusRow, len(rows))
	copy(reused, rows)
	for i := range reused {
		if reused[i].PID == 700 {
			reused[i].Command = "/usr/bin/other"
		}
	}
	f = &fakeReap{rows: [][]censusRow{rows, rows, rows, rows, reused}, alive: map[int]bool{300: true, 400: true, 500: true, 700: true}}
	env := f.env()
	baseSignal := env.signal
	env.signal = func(pid int, sig syscall.Signal) error {
		if pid == 400 && sig == syscall.SIGTERM {
			f.alive[400] = false
		}
		return baseSignal(pid, sig)
	}
	out.Reset()
	if code := runDaemonReap([]string{"--apply"}, &out, &errb, env); code != 0 {
		t.Fatalf("apply exit %d: %s", code, errb.String())
	}
	sigs := strings.Join(f.signals, ",")
	for _, s := range []string{"terminated 300", "killed 300", "terminated 400", "terminated 500", "killed 500"} {
		if !strings.Contains(sigs, s) {
			t.Errorf("signals %s lack %q", sigs, s)
		}
	}
	for _, pid := range []string{"100", "800", "42"} {
		if strings.Contains(sigs+",", " "+pid+",") {
			t.Errorf("signalled pid %s (managed or reap itself): %s", pid, sigs)
		}
	}
	if strings.Contains(sigs, "killed 400") {
		t.Errorf("SIGKILLed a daemon that exited on SIGTERM: %s", sigs)
	}
	if strings.Contains(sigs, " 700") || !strings.Contains(out.String(), "pid 700 changed") {
		t.Errorf("a reused pid was signalled (%s) or not named:\n%s", sigs, out.String())
	}
	if !strings.Contains(strings.Join(f.pruned, ","), "127.0.0.1-7855.json") {
		t.Errorf("stale record not pruned: %v", f.pruned)
	}
}
