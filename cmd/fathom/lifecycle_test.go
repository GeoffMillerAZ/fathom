package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

func readRowsT(t *testing.T, ledger string) []tenancy.LifecycleRow {
	t.Helper()
	rows, skipped, err := readLifecycle(ledger)
	if err != nil || skipped != 0 {
		t.Fatalf("readLifecycle(%s): %v (%d skipped)", ledger, err, skipped)
	}
	return rows
}

func rowKinds(rows []tenancy.LifecycleRow) string {
	var ks []string
	for _, r := range rows {
		k := r.Row
		if r.Reason != "" {
			k += ":" + r.Reason
		}
		ks = append(ks, k)
	}
	return strings.Join(ks, ",")
}

// lcStart is a beginLifecycle input with every OS effect injected.
func lcStart(ledger string, pid int, now time.Time) lifecycleStart {
	return lifecycleStart{
		ledger: ledger, id: filepath.Base(filepath.Dir(ledger)), pid: pid, commit: "abc", version: "vtest", now: now,
		alive:    func(int) bool { return false },
		evidence: func(time.Time) []tenancy.EvidenceFile { return nil },
		logf:     func(string) {},
	}
}

// TestLifecycleIDFlagSharesLedgerAcrossUnmanagedStarts (RT-158, §2.3,
// §15.2): `--lifecycle-id` gives an unmanaged daemon a named, persistent
// ledger, so a second start under the same id sees the first — and names
// it unclean when it left no exit row; the id can never be a managed
// label (a scratch ledger never trips the managed breaker), nor escape
// the daemon directory; an unmanaged daemon without it gets none here
// (its per-process id is addr-<port>-<pid>, known only after the bind).
func TestLifecycleIDFlagSharesLedgerAcrossUnmanagedStarts(t *testing.T) {
	reserved := []string{"com.example.fathom"}
	for _, tc := range []struct {
		flag, label string
		managed     bool
		want        string
		err         bool
	}{
		{flag: "tenancy-stress-1", want: "tenancy-stress-1"},
		{flag: "", want: ""},
		{flag: "", managed: true, label: "com.example.fathom", want: "com.example.fathom"},
		{flag: "x", managed: true, label: "com.example.fathom", want: "com.example.fathom"}, // managed: the label wins
		{flag: defaultDaemonLabel, err: true},
		{flag: "com.example.fathom", err: true},
		{flag: "addr-7575-12", err: true},
		{flag: "../escape", err: true},
		{flag: "a/b", err: true},
	} {
		got, err := lifecycleID(tc.flag, tc.managed, tc.label, reserved)
		if (err != nil) != tc.err || got != tc.want {
			t.Errorf("lifecycleID(%q, managed=%v, %q) = %q, %v; want %q (err %v)", tc.flag, tc.managed, tc.label, got, err, tc.want, tc.err)
		}
	}

	home := t.TempDir()
	ledger, err := ledgerPathFor(home, "tenancy-stress-1")
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	// Start 1, killed (no exit row).
	if out := beginLifecycle(lcStart(ledger, 101, t0)); out.stop || out.unclean != nil {
		t.Fatalf("first start: %+v", out)
	}
	// Start 2 in a new process, same id: names start 1 unclean.
	out := beginLifecycle(lcStart(ledger, 102, t0.Add(time.Minute)))
	if out.unclean == nil || out.unclean.PID != 101 {
		t.Fatalf("second start did not name pid 101 unclean: %+v", out)
	}
	if got := rowKinds(readRowsT(t, ledger)); got != "start,exit:unclean,start" {
		t.Fatalf("ledger = %s", got)
	}
	// Starts 3 and 4, each killed: the third counted exit puts start 4 in
	// safe mode — across relaunches, in one ledger.
	beginLifecycle(lcStart(ledger, 103, t0.Add(2*time.Minute)))
	out = beginLifecycle(lcStart(ledger, 104, t0.Add(3*time.Minute)))
	if out.verdict.Action != tenancy.BreakerSafe || out.safe == nil {
		t.Fatalf("start 4 after 3 unclean exits: %+v", out.verdict)
	}
	rows := readRowsT(t, ledger)
	if last := rows[len(rows)-1]; last.Row != tenancy.RowStart || last.Mode != tenancy.ModeSafe || last.LedgerID != "tenancy-stress-1" {
		t.Fatalf("start row = %+v", last)
	}
	beginLifecycle(lcStart(ledger, 105, t0.Add(4*time.Minute)))
	// Start 6: the fifth counted exit stops it. An unmanaged daemon never
	// writes disabled.json (it would stop the operator's managed daemon).
	in := lcStart(ledger, 106, t0.Add(5*time.Minute))
	in.switchDir = filepath.Join(home, ".fathom", "daemon")
	out = beginLifecycle(in)
	if !out.stop || out.code != 0 {
		t.Fatalf("start 6: %+v", out)
	}
	if _, err := os.Stat(filepath.Join(in.switchDir, tenancy.DisabledFile)); !os.IsNotExist(err) {
		t.Fatalf("an unmanaged breaker stop wrote disabled.json: %v", err)
	}
	rows = readRowsT(t, ledger)
	if last := rows[len(rows)-1]; last.Row != tenancy.RowBreaker || last.To != "stop" {
		t.Fatalf("breaker row = %+v", last)
	}
}

// TestManagedBreakerStopWritesDisabledJSON (RT-38, §2.3, §9): the managed
// daemon's breaker stop writes disabled.json {reason: breaker, detail,
// since, addr} and exits 0 — launchd stays down under schema 2.
func TestManagedBreakerStopWritesDisabledJSON(t *testing.T) {
	home := t.TempDir()
	ledger, _ := ledgerPathFor(home, "com.example.fathom")
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	for i := range 5 {
		in := lcStart(ledger, 200+i, t0.Add(time.Duration(i)*time.Minute))
		in.managed = true
		beginLifecycle(in)
	}
	in := lcStart(ledger, 300, t0.Add(6*time.Minute))
	in.managed, in.addr = true, "127.0.0.1:7575"
	in.switchDir = filepath.Join(home, ".fathom", "daemon")
	var logged []string
	in.logf = func(m string) { logged = append(logged, m) }
	in.relaunch = func(tenancy.SwitchRecord) []string { return []string{"launchd will not relaunch: disabled.json"} }
	out := beginLifecycle(in)
	if !out.stop || out.code != 0 {
		t.Fatalf("managed breaker stop: %+v", out)
	}
	raw, err := os.ReadFile(filepath.Join(in.switchDir, tenancy.DisabledFile))
	if err != nil {
		t.Fatal(err)
	}
	rec, err := tenancy.ParseSwitch(raw)
	if err != nil || rec.Reason != "breaker" || rec.Addr != "127.0.0.1:7575" || !strings.Contains(rec.Detail, "counted exits") || rec.Since.IsZero() {
		t.Fatalf("disabled.json = %s (%v)", raw, err)
	}
	if !strings.Contains(strings.Join(logged, "\n"), "launchd will not relaunch") {
		t.Fatalf("the relaunch clause is not logged: %q", logged)
	}
}

// TestDeployKickstartNotCountedByBreaker (RT-156, §8.5, §2.3): `task
// deploy` writes a deploy intent row (the replaced pid and the new
// binary's hash) before its kickstart, so a kickstart whose SIGTERM did not
// reach a drained exit is classified not counted at the next start — three
// agent deploys in 30 minutes leave the daemon in normal mode.
func TestDeployKickstartNotCountedByBreaker(t *testing.T) {
	home := t.TempDir()
	ledger, _ := ledgerPathFor(home, "com.example.fathom")
	bin := filepath.Join(home, "fathom")
	if err := os.WriteFile(bin, []byte("new binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	pid := 400
	in := lcStart(ledger, pid, t0)
	in.managed = true
	beginLifecycle(in)
	for i := 1; i <= 4; i++ {
		deployAt := t0.Add(time.Duration(i) * 8 * time.Minute)
		if err := recordDeployIntent(ledger, pid, bin, deployAt); err != nil {
			t.Fatal(err)
		}
		pid++
		in := lcStart(ledger, pid, deployAt.Add(3*time.Second))
		in.managed = true
		out := beginLifecycle(in)
		if out.unclean == nil || out.unclean.NotCounted == "" {
			t.Fatalf("deploy %d: predecessor not classified as a kickstart: %+v", i, out.unclean)
		}
		if out.verdict.Action != tenancy.BreakerRun || out.verdict.Counted != 0 {
			t.Fatalf("deploy %d moved the breaker: %+v", i, out.verdict)
		}
	}
	rows := readRowsT(t, ledger)
	var deploy *tenancy.LifecycleRow
	for i := range rows {
		if rows[i].Row == tenancy.RowDeploy {
			deploy = &rows[i]
		}
	}
	if deploy == nil || deploy.ReplacedPID != 403 || len(deploy.BinaryHash) != 12 {
		t.Fatalf("deploy row = %+v", deploy)
	}

	// The CLI resolves the replaced pid from the daemon record.
	t.Setenv("HOME", home)
	recPath, _ := daemonRecordPath(home, "com.example.fathom")
	if err := os.MkdirAll(filepath.Dir(recPath), 0o700); err != nil {
		t.Fatal(err)
	}
	recJSON, _ := json.Marshal(map[string]any{"schema_version": 2, "addr": "127.0.0.1:7575", "pid": 4242, "started_at": t0.Format(time.RFC3339)})
	if err := os.WriteFile(recPath, recJSON, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if rc := runDaemonDeployIntent([]string{"--label", "com.example.fathom", "--binary", bin}, &stdout, &stderr); rc != 0 {
		t.Fatalf("deploy-intent rc=%d: %s %s", rc, stdout.String(), stderr.String())
	}
	rows = readRowsT(t, ledger)
	if last := rows[len(rows)-1]; last.Row != tenancy.RowDeploy || last.ReplacedPID != 4242 {
		t.Fatalf("CLI deploy row = %+v (%s)", last, stdout.String())
	}
	// No record: nothing to replace, said so, exit 0 (a deploy never fails
	// on its intent row).
	_ = os.Remove(recPath)
	stdout.Reset()
	if rc := runDaemonDeployIntent([]string{"--label", "com.example.fathom", "--binary", bin}, &stdout, &stderr); rc != 0 || !strings.Contains(stdout.String(), "no running daemon") {
		t.Fatalf("deploy-intent without a record: rc=%d %q", rc, stdout.String())
	}
}

// TestLedgerRotatesKeepingOneGeneration (§2.3): the ledger rotates at 1
// MiB keeping one generation, and readers see both generations in order.
func TestLedgerRotatesKeepingOneGeneration(t *testing.T) {
	dir := t.TempDir()
	ledger := filepath.Join(dir, "lifecycle.jsonl")
	big := strings.Repeat("x", 4096)
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	n := 0
	for range 600 { // ~2.4 MiB: rotates twice
		r := tenancy.NewLifecycleRow(tenancy.RowMode, t0.Add(time.Duration(n)*time.Second), 1, "v")
		r.Detail = big
		if err := appendLifecycle(ledger, r); err != nil {
			t.Fatal(err)
		}
		n++
	}
	fi, err := os.Stat(ledger)
	if err != nil || fi.Size() > tenancy.LedgerRotateBytes+8192 {
		t.Fatalf("current generation %v, %v", fi, err)
	}
	if _, err := os.Stat(ledger + ".1"); err != nil {
		t.Fatalf("no rotated generation: %v", err)
	}
	if _, err := os.Stat(ledger + ".2"); !os.IsNotExist(err) {
		t.Fatal("more than one generation kept")
	}
	rows := readRowsT(t, ledger)
	for i := 1; i < len(rows); i++ {
		if !rows[i].Time().After(rows[i-1].Time()) {
			t.Fatalf("rows out of order at %d", i)
		}
	}
	if rows[len(rows)-1].Time() != t0.Add(599*time.Second) || len(rows) < 250 {
		t.Fatalf("%d rows, last %s", len(rows), rows[len(rows)-1].At)
	}
}

// TestUncleanEvidenceScanBounded (RT-37, §2.3): the scan reads only
// JetsamEvent-* and fathom_*.diag reports modified since the start, newest
// first, at most 20.
func TestUncleanEvidenceScanBounded(t *testing.T) {
	dir := t.TempDir()
	since := time.Now().Add(-time.Hour)
	write := func(name string, mod time.Time) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("PID: 1\nEvent: disk writes\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	write("fathom_old.diag", since.Add(-time.Minute))
	write("other_new.diag", since.Add(time.Minute))
	write("JetsamEvent-a.ips", since.Add(2*time.Minute))
	for i := range 25 {
		write("fathom_"+strings.Repeat("n", i+1)+".diag", since.Add(time.Duration(3+i)*time.Minute))
	}
	got := scanUncleanEvidence([]string{dir, filepath.Join(dir, "missing")}, since)
	if len(got) != tenancy.MaxEvidenceFiles {
		t.Fatalf("%d files, want %d", len(got), tenancy.MaxEvidenceFiles)
	}
	for _, f := range got {
		if f.Name == "fathom_old.diag" || f.Name == "other_new.diag" {
			t.Fatalf("scanned %s", f.Name)
		}
	}
	if got[0].Name != "fathom_"+strings.Repeat("n", 25)+".diag" {
		t.Fatalf("newest first, got %s", got[0].Name)
	}
}
