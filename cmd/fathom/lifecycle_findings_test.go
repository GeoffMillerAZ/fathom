package main

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// TestStoreWaitRecoversErrnoFromRealOpen (RT-42, RT-165): the SQLite VFS
// drops the errno ("unable to open database file (14)"), so the wait
// probes the path itself to recover it — through the real client open, not
// a hand-made *os.PathError. A refused file is EACCES, not CANTOPEN; an
// EPERM on a path that stats (INC-23's TCC wedge) is labelled tcc-wedge and
// gets the daemon binary's remedy.
func TestStoreWaitRecoversErrnoFromRealOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a mode-0 file")
	}
	path := filepath.Join(t.TempDir(), "s.sqlite")
	c, err := client.NewConstantClient(client.ModeLocal, path)
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
	if err := os.Chmod(path, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	_, openErr := client.NewConstantClient(client.ModeLocal, path)
	if openErr == nil {
		t.Fatal("a mode-0 store opened")
	}
	if !storeUnavailable(openErr) {
		t.Fatalf("a refused store is not unavailable: %v", openErr)
	}
	errno, label := classifyStoreWait(openErr, path, statPath, probeStoreOpen)
	if errno != "EACCES" || label != storeWaitUnavailable {
		t.Fatalf("real open of a refused store = %s/%s, want EACCES/unavailable (the errno recovered by the probe)", errno, label)
	}

	// The TCC wedge: stat works, open is EPERM — seen only by the probe.
	eperm := func(string) error { return &os.PathError{Op: "open", Path: path, Err: syscall.EPERM} }
	errno, label = classifyStoreWait(errors.New("store.NewSQLiteStore schema: unable to open database file (14)"), path, statPath, eperm)
	if errno != "EPERM" || label != storeWaitTCC {
		t.Fatalf("an EPERM behind a VFS error = %s/%s, want EPERM/tcc-wedge", errno, label)
	}
	// openStoreWithWait uses the probe too.
	var seen storeWaitState
	ctx, cancel := context.WithCancel(context.Background())
	_, _ = openStoreWithWait(ctx, storeWaitDeps[int]{
		path:  path,
		open:  func() (int, error) { return 0, errors.New("unable to open database file (14)") },
		stat:  statPath,
		probe: eperm,
		sleep: func(context.Context, time.Duration) error { cancel(); return context.Canceled },
		now:   time.Now,
		exe:   "/opt/fathom/bin/fathom",
		codesign: func(exe string) signingIdentity {
			return signingIdentity{Binary: exe, AdHoc: true}
		},
		onWait: func(s storeWaitState) { seen = s },
	})
	if seen.Label != storeWaitTCC || !strings.Contains(seen.Remedy, "/opt/fathom/bin/fathom") || !strings.Contains(seen.Remedy, "Full Disk Access") {
		t.Fatalf("waiting state = %+v", seen)
	}
}

// TestUnmanagedStoreWaitHonoursTTL (RT-20, RT-21, RT-42; I47, I48): an
// unmanaged serve-http waiting for an unavailable store still keeps its
// --ttl (and --idle-exit, --exit-with-parent): a mistyped --store never
// leaves a scratch daemon holding its port and record for good.
func TestUnmanagedStoreWaitHonoursTTL(t *testing.T) {
	home, _ := withIsolatedConfigHome(t)
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	addr := freeAddr(t)
	logPath := filepath.Join(t.TempDir(), "serve.log")
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), testHelperEnv+"=rt42-ttl-serve", "RT42_STORE="+filepath.Join(locked, "db", "s.sqlite"), "RT42_ADDR="+addr, "RT42_LOG="+logPath)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	select {
	case err := <-exited:
		if err != nil {
			b, _ := os.ReadFile(logPath)
			t.Fatalf("exit: %v\n%s", err, b)
		}
	case <-time.After(waitSafety):
		b, _ := os.ReadFile(logPath)
		t.Fatalf("an unmanaged daemon waiting for its store outlived --ttl 1s:\n%s", b)
	}
	ledger, _ := ledgerPathFor(home, "rt42ttl")
	rows := readRowsT(t, ledger)
	if last := rows[len(rows)-1]; last.Row != tenancy.RowExit || last.Reason != tenancy.ExitTTL {
		t.Fatalf("ledger = %s, want a closing exit:ttl", rowKinds(rows))
	}
	if recs, _ := filepath.Glob(filepath.Join(home, ".fathom", "daemon", "*", "record*.json")); len(recs) != 0 {
		t.Fatalf("records left behind: %v", recs)
	}
}

// TestManagedStartFailuresTripTheBreaker (RT-38, §2.3): a managed start
// that fails after its start row — here a store that opens and is not a
// database — writes exit{start_failed, code 1}, removes its record, and is
// counted: launchd relaunches it every throttle interval, so the third
// puts it in safe mode and the fifth stops it (exit 0, disabled.json).
func TestManagedStartFailuresTripTheBreaker(t *testing.T) {
	home, _ := withIsolatedConfigHome(t)
	bad := filepath.Join(t.TempDir(), "bad.sqlite")
	junk := make([]byte, 8192)
	_, _ = rand.Read(junk)
	if err := os.WriteFile(bad, junk, 0o600); err != nil {
		t.Fatal(err)
	}
	run := func() int {
		t.Helper()
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), testHelperEnv+"=rt-startfail", "RT_STORE="+bad, "RT_ADDR="+freeAddr(t))
		out, err := cmd.CombinedOutput()
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		if err != nil {
			t.Fatalf("run: %v\n%s", err, out)
		}
		return 0
	}
	for i := 1; i <= 5; i++ {
		if rc := run(); rc != 1 {
			t.Fatalf("start %d: rc %d, want 1", i, rc)
		}
	}
	ledgers, _ := filepath.Glob(filepath.Join(home, ".fathom", "daemon", "*", "lifecycle.jsonl"))
	if len(ledgers) != 1 {
		t.Fatalf("ledgers = %v", ledgers)
	}
	rows := readRowsT(t, ledgers[0])
	if n := strings.Count(rowKinds(rows), "exit:start_failed"); n != 5 {
		t.Fatalf("ledger = %s, want 5 exit:start_failed rows", rowKinds(rows))
	}
	if !strings.Contains(rowKinds(rows), "breaker") {
		t.Fatalf("no breaker row after 5 failed starts: %s", rowKinds(rows))
	}
	if recs, _ := filepath.Glob(filepath.Join(home, ".fathom", "daemon", "*", "record*.json")); len(recs) != 0 {
		t.Fatalf("a failed start left its record: %v", recs)
	}
	if rc := run(); rc != 0 {
		t.Fatalf("sixth start: rc %d, want 0 (breaker stop)", rc)
	}
	if _, err := os.Stat(filepath.Join(home, ".fathom", "daemon", tenancy.DisabledFile)); err != nil {
		t.Fatalf("breaker stop wrote no disabled.json: %v", err)
	}
}

// TestBatchCannotBypassLifecycleModes (RT-39, RT-41, RT-44) was retired:
// batch (RT-60) was unregistered by SR-19 (surface reckoning, F-77), and
// with it classWeight/rpcBatchClasses and the batch sub-call-class fan-out
// this test exercised.

// TestBeginLifecycleNamesEveryDeadUnclosedStart (RT-37, §2.3): an alive
// incumbent's unclosed start (a duplicate start found the port served)
// never hides an older dead pid's unclean death, nor does a duplicate's
// closed start hide the incumbent's later death.
func TestBeginLifecycleNamesEveryDeadUnclosedStart(t *testing.T) {
	ledger := filepath.Join(t.TempDir(), "d", "lifecycle.jsonl")
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	dead := tenancy.NewLifecycleRow(tenancy.RowStart, t0, 101, "v")
	live := tenancy.NewLifecycleRow(tenancy.RowStart, t0.Add(time.Minute), 102, "v")
	dup := tenancy.NewLifecycleRow(tenancy.RowStart, t0.Add(2*time.Minute), 103, "v")
	dupx := tenancy.NewLifecycleRow(tenancy.RowExit, t0.Add(2*time.Minute), 103, "v")
	dupx.Reason, dupx.Code = tenancy.ExitPortHeld, tenancy.IntPtr(0)
	for _, r := range []tenancy.LifecycleRow{dead, live, dup, dupx} {
		if err := appendLifecycle(ledger, r); err != nil {
			t.Fatal(err)
		}
	}
	in := lcStart(ledger, 200, t0.Add(3*time.Minute))
	in.alive = func(pid int) bool { return pid == 102 }
	beginLifecycle(in)
	rows := readRowsT(t, ledger)
	var named []int
	for _, r := range rows {
		if r.Row == tenancy.RowExit && r.Reason == tenancy.ExitUnclean {
			named = append(named, r.PID)
		}
	}
	if len(named) != 1 || named[0] != 101 {
		t.Fatalf("unclean exits for %v, want [101] (102 is alive): %s", named, rowKinds(rows))
	}
	// Now the incumbent dies too: the next start names it.
	in = lcStart(ledger, 201, t0.Add(4*time.Minute))
	beginLifecycle(in)
	rows = readRowsT(t, ledger)
	named = named[:0]
	for _, r := range rows {
		if r.Row == tenancy.RowExit && r.Reason == tenancy.ExitUnclean {
			named = append(named, r.PID)
		}
	}
	if len(named) != 3 || named[1] != 102 || named[2] != 200 {
		t.Fatalf("unclean exits for %v, want [101 102 200]: %s", named, rowKinds(rows))
	}
}

// TestPausedSafeModeWritesNoStepDownRow (RT-39, §2.3): paused.json's safe
// mode is the operator's, not the breaker's — 30 min at normal never
// writes a safe→normal mode row while the daemon is still paused.
func TestPausedSafeModeWritesNoStepDownRow(t *testing.T) {
	resetLifecycleModes(t)
	ledger := filepath.Join(t.TempDir(), "d", "lifecycle.jsonl")
	tenancy.SetPausedSafeMode(true)
	sm := newSafeModeController(lifecycleOutcome{}, ledger, 1, "v", time.Minute, nil)
	t0 := time.Date(2026, 9, 26, 21, 0, 0, 0, time.UTC)
	sm.step(t0, tenancy.LevelNormal)
	sm.step(t0.Add(31*time.Minute), tenancy.LevelNormal)
	if _, err := os.Stat(ledger); err == nil {
		t.Fatalf("a paused daemon wrote a step-down row: %s", rowKinds(readRowsT(t, ledger)))
	}
	if _, on := tenancy.SafeModeNow(); !on {
		t.Fatal("paused safe mode ended")
	}
	// The breaker's safe mode does step down, with its row.
	tenancy.SetPausedSafeMode(false)
	info := &tenancy.SafeModeInfo{Since: t0, Reason: "3 counted exits"}
	applySafeMode(info, nil)
	sm = newSafeModeController(lifecycleOutcome{safe: info}, ledger, 1, "v", time.Minute, nil)
	sm.step(t0, tenancy.LevelNormal)
	sm.step(t0.Add(31*time.Minute), tenancy.LevelNormal)
	if got := rowKinds(readRowsT(t, ledger)); got != "mode:30 min at normal" {
		t.Fatalf("breaker safe mode step-down rows = %q", got)
	}
}

// TestWarmupHoldsUnderObserve (RT-40, §2.3): warm-up is a lifecycle bound,
// like safe mode's refusals — under the default admission = "observe" a
// second heavy call in the window is refused (503 + Retry-After, a hook's
// breaker reads it as busy, not a failure), not merely logged; only
// admission = "off" skips it.
func TestWarmupHoldsUnderObserve(t *testing.T) {
	resetLifecycleModes(t)
	now := time.Now()
	tenancy.SetWarmup(now, time.Minute)
	g := newWarmupGate(func() string { return "observe" }, 10*time.Millisecond)
	rel, ref := g.acquire(context.Background(), tenancy.ClassFleet, "list_services", "agent", now)
	if ref != nil {
		t.Fatalf("first fleet call refused: %+v", ref)
	}
	defer rel()
	if _, ref := g.acquire(context.Background(), tenancy.ClassFleet, "fleet_drift", "hook", now); ref == nil || !strings.Contains(ref.Reason, "warm-up") {
		t.Fatalf("second fleet call during warm-up under observe: %+v", ref)
	}
	off := newWarmupGate(func() string { return "off" }, 10*time.Millisecond)
	ra, _ := off.acquire(context.Background(), tenancy.ClassFleet, "a", "agent", now)
	if _, ref := off.acquire(context.Background(), tenancy.ClassFleet, "b", "hook", now); ref != nil {
		t.Fatalf("admission off refused: %+v", ref)
	}
	ra()
}

// TestListServicesCountErrorNeverReadsZero (RT-41, I38): verify's
// list_services probe and the primer name an unreadable symbol count
// instead of reading it as 0.
func TestListServicesCountErrorNeverReadsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"services\":[{\"service\":\"svc\",\"languages\":[],\"symbol_count_error\":\"store distress: disk full\"}],\"count\":1}"}]}}`))
	}))
	defer srv.Close()
	ok, err := daemonServiceIndexedViaListServices(srv.Client(), srv.URL, "svc")
	if err == nil || ok || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("an unreadable count = %v, %v; want an error naming it", ok, err)
	}
	info := svcInfo{countErr: "store distress: disk full"}
	if got := info.countText(); strings.Contains(got, "0 symbols") || !strings.Contains(got, "disk full") {
		t.Fatalf("countText = %q", got)
	}
	if got := (svcInfo{symbols: 12}).countText(); got != "12 symbols" {
		t.Fatalf("countText = %q", got)
	}
}
