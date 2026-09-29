package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

// testHelperEnv names the role a re-exec'd test binary plays (TestMain
// dispatches it before isolation: the helper inherits the parent test's
// isolated environment).
const testHelperEnv = "FATHOM_TEST_HELPER"

// runTestHelper runs a helper role and returns the process exit code.
func runTestHelper(role string) int {
	switch role {
	case "rt20-parent":
		// Start serve-http as our child, report its pid, then wait to be
		// SIGKILLed.
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), testHelperEnv+"=rt20-serve")
		cmd.Stdout, cmd.Stderr = nil, nil
		if err := cmd.Start(); err != nil {
			fmt.Println("error", err)
			return 1
		}
		fmt.Println(cmd.Process.Pid)
		select {}
	case "rt20-serve":
		out, err := os.OpenFile(os.Getenv("RT20_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 1
		}
		return runServeHTTP([]string{"--store", os.Getenv("RT20_STORE"), "--addr", "127.0.0.1:0", "--call-log", ""}, out, out)
	case "rt42-serve":
		// RT-42: a real serve-http whose store is unavailable; the backoff
		// is shortened so the test sees it reopen.
		storeWaitFirst, storeWaitMax = 100*time.Millisecond, 200*time.Millisecond
		out, err := os.OpenFile(os.Getenv("RT42_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 1
		}
		return runServeHTTP([]string{"--store", os.Getenv("RT42_STORE"), "--addr", os.Getenv("RT42_ADDR"), "--call-log", "", "--lifecycle-id", "rt42"}, out, out)
	case "rt42-ttl-serve":
		// RT-42 with RT-20/RT-21: an unmanaged daemon waiting for its
		// store still keeps its --ttl; the lifetime tick is shortened.
		storeWaitFirst, storeWaitMax = 100*time.Millisecond, 200*time.Millisecond
		daemonLifetimeTick = 100 * time.Millisecond
		out, err := os.OpenFile(os.Getenv("RT42_LOG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return 1
		}
		return runServeHTTP([]string{"--store", os.Getenv("RT42_STORE"), "--addr", os.Getenv("RT42_ADDR"), "--call-log", "", "--lifecycle-id", "rt42ttl", "--ttl", "1s"}, out, out)
	case "rt-startfail":
		// RT-38: a managed start whose store opens and is wrong.
		return runServeHTTP([]string{"--managed", "--store", os.Getenv("RT_STORE"), "--addr", os.Getenv("RT_ADDR"), "--call-log", ""}, os.Stdout, os.Stderr)
	}
	return 2
}

func processGone(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return true
	}
	out, err := exec.Command("ps", "-o", "stat=", "-p", strconv.Itoa(pid)).Output()
	return err != nil || strings.HasPrefix(strings.TrimSpace(string(out)), "Z")
}

// TestUnmanagedServeHTTPExitsWhenParentKilled (RT-20, §10.1, I52): a real
// serve-http whose parent is SIGKILLed exits within 10 s, names the
// reason, and removes its record.
func TestUnmanagedServeHTTPExitsWhenParentKilled(t *testing.T) {
	// The watch itself: no watch under pid 1; the kqueue NOTE_EXIT path
	// fires when the watched process dies even while getppid says nothing
	// changed; the poll path fires on reparenting alone.
	if watchParentExit(1, time.Hour, os.Getppid, func(string) {}) {
		t.Fatal("watching pid 1")
	}
	sleeper := exec.Command("sleep", "30")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	fired := make(chan string, 2)
	watchParentExit(sleeper.Process.Pid, time.Hour, func() int { return sleeper.Process.Pid }, func(r string) { fired <- r })
	time.Sleep(50 * time.Millisecond) // let the kqueue watch register
	_ = sleeper.Process.Kill()
	_ = sleeper.Wait()
	if runtime.GOOS == "darwin" { // off darwin only the poll path exists: nothing to wait for here
		select {
		case r := <-fired:
			if r != fmt.Sprintf("parent %d exited", sleeper.Process.Pid) {
				t.Fatalf("reason %q", r)
			}
		case <-time.After(waitSafety):
			t.Fatal("kqueue NOTE_EXIT never fired")
		}
	}
	polled := make(chan string, 1)
	var ppid atomic.Int64
	ppid.Store(int64(os.Getpid())) // a live pid: kqueue stays quiet, only the poll can fire
	watchParentExit(os.Getpid(), 10*time.Millisecond, func() int { return int(ppid.Load()) }, func(r string) { polled <- r })
	ppid.Store(1) // reparented to init
	select {
	case <-polled:
	case <-time.After(waitSafety):
		t.Fatal("getppid poll did not fire on reparenting")
	}

	if testing.Short() {
		t.Skip("spawns serve-http")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "serve.log")
	parent := exec.Command(os.Args[0])
	parent.Env = append(os.Environ(), testHelperEnv+"=rt20-parent", "RT20_LOG="+logPath,
		"RT20_STORE="+filepath.Join(dir, "scratch.sqlite"), "XPC_SERVICE_NAME=")
	stdout, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		_ = parent.Process.Kill()
		t.Fatalf("reading the serve pid: %v", err)
	}
	servePID, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		_ = parent.Process.Kill()
		t.Fatalf("parent said %q", line)
	}
	t.Cleanup(func() { _ = syscall.Kill(servePID, syscall.SIGKILL) }) // never leak it, whatever happens

	readLog := func() string { b, _ := os.ReadFile(logPath); return string(b) }
	deadline := time.Now().Add(60 * time.Second)
	for !strings.Contains(readLog(), "listening on") {
		if time.Now().After(deadline) {
			_ = parent.Process.Kill()
			t.Fatalf("serve-http never listened:\n%s", readLog())
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(readLog(), "serve-http: unmanaged") || !strings.Contains(readLog(), "auto-refresh false") {
		t.Errorf("serve-http did not start unmanaged with auto-refresh off:\n%s", readLog())
	}

	if err := parent.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = parent.Wait()
	killed := time.Now()
	for !processGone(servePID) {
		if time.Since(killed) > waitSafety {
			t.Fatalf("serve-http pid %d never exited after its parent was SIGKILLed:\n%s", servePID, readLog())
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Logf("serve-http exited %.1fs after its parent's SIGKILL", time.Since(killed).Seconds())
	if want := fmt.Sprintf("parent %d exited", parent.Process.Pid); !strings.Contains(readLog(), want) {
		t.Errorf("exit reason %q not logged:\n%s", want, readLog())
	}
	home, _ := os.UserHomeDir()
	recs, _ := listDaemonRecords(home)
	for _, r := range recs {
		if r.Rec.PID == servePID {
			t.Errorf("record %s left behind after a clean exit", r.Path)
		}
	}
}

// TestDaemonLifetimeTick: the tick heartbeats the record for every daemon,
// never exits a managed one, and exits an unmanaged one with its reason
// after removing its record.
func TestDaemonLifetimeTick(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	resetServingCallLogForTest(t)
	t0 := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	now := t0
	clock := func() time.Time { return now }
	_, path, err := recordServingDaemonAs(daemonMeta{}, "127.0.0.1:7855", "", "", "c0ffee", t0)
	if err != nil {
		t.Fatal(err)
	}
	var logged []string
	exitCode := -1
	lt := &daemonLifetime{
		bounds: tenancy.DefaultUnmanagedBounds(), startedAt: t0, act: newActivity(clock),
		recordPath: path, now: clock, logf: func(m string) { logged = append(logged, m) },
		exit: func(c int) { exitCode = c },
	}
	now = t0.Add(10 * time.Minute)
	if r := lt.tick(); r != "" {
		t.Fatalf("exited early: %s", r)
	}
	rec, err := console.ReadDaemonRecord(path)
	if err != nil || rec.HeartbeatAt != now.Format(time.RFC3339) {
		t.Fatalf("heartbeat not written: %+v %v", rec, err)
	}

	lt.managed = true
	now = t0.Add(100 * time.Hour)
	if r := lt.tick(); r != "" || exitCode != -1 {
		t.Fatalf("a managed daemon exited: %s", r)
	}
	lt.managed = false
	if r := lt.tick(); r != "ttl 8h0m0s" || exitCode != 0 {
		t.Fatalf("tick = %q exit %d, want ttl and exit 0", r, exitCode)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("record not removed on the bounded exit (err=%v)", err)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "ttl 8h0m0s") {
		t.Fatalf("exit reason not logged: %v", logged)
	}

	// A replaced executable is an unlinked one.
	exe := filepath.Join(t.TempDir(), "fathom")
	if err := os.WriteFile(exe, []byte("a"), 0o755); err != nil {
		t.Fatal(err)
	}
	id := identify(exe)
	if id.unlinked() {
		t.Fatal("a present file reads unlinked")
	}
	_ = os.Remove(exe)
	if !id.unlinked() {
		t.Fatal("a removed file does not read unlinked")
	}
}

// TestUnmanagedRefusesOperatorStore (RT-22, §10.1): an unmanaged
// serve-http refuses the managed daemon's store and call log — found
// through its record (schema 2 managed, or schema 1 at the managed
// address) — without --allow-operator-state, and serve-http exits 1 before
// opening the store.
func TestUnmanagedRefusesOperatorStore(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XPC_SERVICE_NAME", "")
	dir := t.TempDir()
	opStore := filepath.Join(dir, "fleet.sqlite")
	opLog := filepath.Join(dir, "mcp-calls.jsonl")
	legacy, _ := console.DaemonRecordPath(home, managedAddr)
	if err := console.WriteDaemonRecord(legacy, console.DaemonRecord{SchemaVersion: 1, Addr: managedAddr, PID: 1, Store: opStore, CallLog: opLog, StartedAt: "2026-09-26T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	recs, _ := listDaemonRecords(home)
	stores, logs := operatorPaths(recs, managedAddr)
	if err := refuseOperatorState(opStore, "", stores, logs); !errors.Is(err, errOperatorState) || !strings.Contains(err.Error(), "--allow-operator-state") {
		t.Fatalf("operator store not refused: %v", err)
	}
	if err := refuseOperatorState(filepath.Join(dir, "scratch.sqlite"), opLog, stores, logs); !errors.Is(err, errOperatorState) {
		t.Fatalf("operator call log not refused: %v", err)
	}
	if err := refuseOperatorState(filepath.Join(dir, "scratch.sqlite"), "", stores, logs); err != nil {
		t.Fatalf("a scratch store refused: %v", err)
	}
	// A schema-1 record at another address is a scratch daemon's, not the operator's.
	if s, l := operatorPaths([]storedRecord{{Rec: console.DaemonRecord{SchemaVersion: 1, Addr: "127.0.0.1:7856", Store: "/x"}}}, managedAddr); len(s)+len(l) != 0 {
		t.Fatalf("a scratch record counted as the operator's: %v %v", s, l)
	}

	// Through serve-http: refused before the store is opened (it is never created).
	var out, errb strings.Builder
	code := runServeHTTP([]string{"--store", opStore, "--addr", "127.0.0.1:0", "--call-log", ""}, &out, &errb)
	if code != 1 || !strings.Contains(errb.String(), "managed daemon's store") {
		t.Fatalf("serve-http on the operator store: code %d, stderr %s", code, errb.String())
	}
	if _, err := os.Stat(opStore); !os.IsNotExist(err) {
		t.Fatalf("the operator store was opened/created before the refusal (err=%v)", err)
	}
	// --managed is the operator's own daemon: not refused by this check.
	if m, _ := detectManaged(true, func(string) string { return "" }, nil); !m {
		t.Fatal("--managed not detected")
	}
	if m, label := detectManaged(false, func(k string) string {
		if k == "XPC_SERVICE_NAME" {
			return "dev.fathom.serve-http"
		}
		return ""
	}, nil); !m || label != "dev.fathom.serve-http" {
		t.Fatal("a launchd job under a fathom label (schema-1 plist, no --managed) not detected as managed")
	}
	if m, _ := detectManaged(false, func(k string) string {
		if k == "XPC_SERVICE_NAME" {
			return "application.com.mitchellh.ghostty.1"
		}
		return ""
	}, nil); m {
		t.Fatal("a terminal's XPC_SERVICE_NAME read as managed")
	}
}
