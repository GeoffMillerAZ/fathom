package main

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

var stopT0 = time.Date(2026, 9, 26, 21, 7, 0, 0, time.UTC)

// stopFixture: a scratch HOME with a daemon record for addr naming pid,
// and a fake process table holding that pid as a fathom serve-http.
func stopFixture(t *testing.T, addr string, pid int) (string, *fakeDaemonOps) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	p, err := console.DaemonRecordPath(home, addr)
	if err != nil {
		t.Fatal(err)
	}
	if err := console.WriteDaemonRecord(p, console.NewDaemonRecord(addr, pid, "", "/s.sqlite", "abc", stopT0)); err != nil {
		t.Fatal(err)
	}
	ops := newFakeDaemonOps(stopT0.Add(time.Hour))
	ops.procs[pid] = procInfo{PID: pid, Start: stopT0.Add(-time.Second), Command: "/opt/fathom/fathom serve-http --managed --store /s.sqlite"}
	ops.open[addr] = true
	return home, ops
}

// RT-4 (§9): stop writes the switch, kills with a pid-reuse guard, and
// reports success only once the process is gone AND the port is closed. A
// process that dies while the port stays open is NOT a stop.
func TestStopProofRequiresPortClosed(t *testing.T) {
	const addr = "127.0.0.1:7575"
	home, ops := stopFixture(t, addr, 4242)
	// The process exits on SIGTERM but something still holds the port.
	ops.onSignal = func(f *fakeDaemonOps, pid int, sig syscall.Signal) { f.kill(pid, "") }
	var out, errBuf bytes.Buffer
	rc := runDaemonStop([]string{"--url", "http://" + addr, "--reason", "test"}, &out, &errBuf, ops)
	if rc == 0 || !strings.Contains(out.String(), "NOT STOPPED") || !strings.Contains(out.String(), ":7575 is still open") {
		t.Fatalf("an open port must fail the stop: rc=%d\n%s", rc, out.String())
	}
	if strings.Contains(out.String(), "gone after") {
		t.Fatalf("no success proof may be printed when the port is open:\n%s", out.String())
	}
	sw, err := os.ReadFile(filepath.Join(home, ".fathom", "daemon", tenancy.DisabledFile))
	if err != nil {
		t.Fatalf("stop must write the kill switch first: %v", err)
	}
	if rec, _ := tenancy.ParseSwitch(sw); rec.Reason != "test" || rec.Since.IsZero() {
		t.Fatalf("the switch must record by/reason/since: %s", sw)
	}

	// Process gone and port closed: success, with the proof.
	_, ops = stopFixture(t, addr, 4242)
	ops.onSignal = func(f *fakeDaemonOps, pid int, sig syscall.Signal) { f.kill(pid, addr) }
	out.Reset()
	if rc := runDaemonStop([]string{"--url", "http://" + addr}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("stop: rc=%d\n%s", rc, out.String())
	}
	for _, want := range []string{"pid 4242 gone after", "(SIGTERM)", ":7575 closed", "launchd will not relaunch: disabled.json"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("the proof must carry %q:\n%s", want, out.String())
		}
	}

	// Pid reuse: the record's pid is now someone else's process, started
	// after the record — it must never be signalled.
	_, ops = stopFixture(t, addr, 4242)
	ops.procs[4242] = procInfo{PID: 4242, Start: stopT0.Add(30 * time.Minute), Command: "/opt/fathom/fathom serve-http"}
	ops.open[addr] = false
	out.Reset()
	if rc := runDaemonStop([]string{"--url", "http://" + addr}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("rc=%d\n%s", rc, out.String())
	}
	if len(ops.signals) != 0 || !strings.Contains(out.String(), "pid reused") {
		t.Fatalf("a reused pid must not be signalled: %v\n%s", ops.signals, out.String())
	}
	_, ops = stopFixture(t, addr, 4242)
	ops.procs[4242] = procInfo{PID: 4242, Start: stopT0.Add(-time.Minute), Command: "/usr/bin/vim notes.txt"}
	ops.open[addr] = false
	out.Reset()
	runDaemonStop([]string{"--url", "http://" + addr}, &out, &errBuf, ops)
	if len(ops.signals) != 0 {
		t.Fatalf("a pid that is no longer a serve-http must not be signalled: %v", ops.signals)
	}
}

// RT-157 (§9): under an installed schema-1 plist (KeepAlive true) stop says
// launchd will relaunch every 10 s and each start exits in < 50 ms, and
// prints the bootout and --reconcile lines instead of claiming the daemon
// stays down; it never runs launchctl itself; and the relaunched starts'
// kill-switch checks write no lifecycle row.
func TestStopUnderV1PlistNamesRelaunchAndBootout(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd plists are darwin-only")
	}
	const addr = "127.0.0.1:7575"
	home, ops := stopFixture(t, addr, 4242)
	t.Setenv("XPC_SERVICE_NAME", "")
	agents := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agents, "com.example.mine.plist"), []byte(v1PlistFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	ops.onSignal = func(f *fakeDaemonOps, pid int, sig syscall.Signal) { f.kill(pid, addr) }
	var out, errBuf bytes.Buffer
	if rc := runDaemonStop([]string{"--url", "http://" + addr}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("rc=%d\n%s%s", rc, out.String(), errBuf.String())
	}
	for _, want := range []string{
		"launchd will relaunch every 10 s and each start exits in < 50 ms",
		"launchctl bootout gui/501/com.example.mine",
		"fathom daemon install --reconcile --label com.example.mine",
	} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stop under a v1 plist must say %q:\n%s", want, out.String())
		}
	}
	if strings.Contains(out.String(), "will not relaunch") {
		t.Fatalf("under v1 stop must not claim launchd stays down:\n%s", out.String())
	}
	for _, c := range ops.calls {
		if strings.HasPrefix(c, "launchctl") {
			t.Fatalf("stop must never run launchctl: %v", ops.calls)
		}
	}
	// launchd's relaunches: each exits 0 on the switch and writes nothing.
	for range 3 {
		if code, stop := checkKillSwitch([]string{"--managed"}, os.Getenv, time.Now(), &errBuf); !stop || code != 0 {
			t.Fatalf("a relaunch after stop must exit 0: %d %v", code, stop)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(home, ".fathom", "daemon"))
	for _, e := range entries {
		if e.Name() != tenancy.DisabledFile {
			t.Fatalf("stop and the relaunched checks must write no lifecycle row or other file: found %s", e.Name())
		}
	}
}

// TestHelperDaemonIgnoresSIGTERM is not a test: it is the child process
// TestStopKillsDaemonIgnoringSIGTERM starts — a listener that ignores
// SIGTERM, as a wedged daemon does.
func TestHelperDaemonIgnoresSIGTERM(t *testing.T) {
	addr := os.Getenv("FATHOM_TEST_HELPER_IGNORE_TERM")
	if addr == "" {
		t.Skip("helper process only")
	}
	signal.Ignore(syscall.SIGTERM)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		os.Exit(3)
	}
	defer l.Close()
	time.Sleep(time.Minute) // SIGKILL ends it; a minute bounds a leak
	os.Exit(4)
}

// RT-5 (§9): stop kills a daemon that ignores SIGTERM — SIGTERM, the wait,
// then SIGKILL — and proves the port closed. A real child process, a real
// signal, a real port; the child is this test's own.
func TestStopKillsDaemonIgnoringSIGTERM(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	addr := freeAddr(t)
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperDaemonIgnoresSIGTERM$", "--", "serve-http")
	cmd.Env = append(os.Environ(), "FATHOM_TEST_HELPER_IGNORE_TERM="+addr)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(waited) }()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-waited
	})
	ops := realDaemonOps{}
	deadline := time.Now().Add(10 * time.Second)
	for !ops.PortOpen(addr) {
		if time.Now().After(deadline) {
			t.Fatal("helper never listened")
		}
		time.Sleep(20 * time.Millisecond)
	}
	p, _ := console.DaemonRecordPath(home, addr)
	if err := console.WriteDaemonRecord(p, console.NewDaemonRecord(addr, cmd.Process.Pid, "", "", "", time.Now())); err != nil {
		t.Fatal(err)
	}
	oldTerm := daemonStopTermWait
	daemonStopTermWait = 700 * time.Millisecond
	t.Cleanup(func() { daemonStopTermWait = oldTerm })

	var out, errBuf bytes.Buffer
	rc := runDaemonStop([]string{"--url", "http://" + addr}, &out, &errBuf, ops)
	if rc != 0 {
		t.Fatalf("stop of a SIGTERM-ignoring daemon: rc=%d\n%s%s", rc, out.String(), errBuf.String())
	}
	for _, want := range []string{"ignored SIGTERM", "(SIGKILL)", "closed"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("stop output must say %q:\n%s", want, out.String())
		}
	}
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("the helper is still running after stop reported success")
	}
	if ops.PortOpen(addr) {
		t.Fatal("the port is still open after stop reported success")
	}
}

// RT-6 (§9): `daemon start` removes the switch, resets the breaker window
// (an operator_reset row) and prints — never runs — the kickstart from an
// agent shell; `pause`/`resume` write and remove paused.json, which the
// managed daemon's tick reads as safe mode.
func TestDaemonStartPauseResume(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ops := newFakeDaemonOps(stopT0)
	ops.env["CLAUDECODE"] = "1"
	dir := filepath.Join(home, ".fathom", "daemon")
	var out, errBuf bytes.Buffer
	if rc := runDaemonStop([]string{"--url", "http://127.0.0.1:1"}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("stop with nothing running: rc=%d %s", rc, out.String())
	}
	if rc := runDaemonStart([]string{"--label", "com.example.mine"}, &out, &errBuf, ops); rc != 0 {
		t.Fatalf("start: rc=%d %s", rc, errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(dir, tenancy.DisabledFile)); !os.IsNotExist(err) {
		t.Fatal("start must remove disabled.json")
	}
	ledger, err := os.ReadFile(filepath.Join(dir, "com.example.mine", "lifecycle.jsonl"))
	if err != nil || !strings.Contains(string(ledger), `"row":"operator_reset"`) {
		t.Fatalf("start must reset the breaker window: %v %s", err, ledger)
	}
	for _, c := range ops.calls {
		if strings.HasPrefix(c, "launchctl") {
			t.Fatalf("an agent shell must never run launchctl: %v", ops.calls)
		}
	}

	out.Reset()
	if rc := runDaemonPause(true, nil, &out, &errBuf, ops); rc != 0 {
		t.Fatal("pause")
	}
	// RT-39: pause names what safe mode does — and only that.
	if !strings.Contains(out.String(), "enters safe mode") || !strings.Contains(out.String(), "tree and fleet calls are refused") {
		t.Fatalf("pause must name safe mode's effects:\n%s", out.String())
	}
	t.Cleanup(tenancy.ResetLifecycleStateForTest)
	daemonSafeMode.Store(false)
	stopTick := make(chan struct{})
	done := make(chan struct{})
	go func() { watchPauseSwitch(dir, 5*time.Millisecond, time.Now, nil, stopTick); close(done) }()
	waitFor := func(want bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for safeModeActive() != want {
			if time.Now().After(deadline) {
				t.Fatalf("safe mode never became %v", want)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
	waitFor(true)
	if rc := runDaemonPause(false, nil, &out, &errBuf, ops); rc != 0 {
		t.Fatal("resume")
	}
	waitFor(false)
	close(stopTick)
	<-done
}
