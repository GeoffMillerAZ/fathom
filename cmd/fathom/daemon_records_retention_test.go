package main

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// deadPID returns the pid of a process that has exited and been reaped.
func deadPID(t *testing.T) int {
	t.Helper()
	c := exec.Command("true")
	if err := c.Run(); err != nil {
		t.Fatal(err)
	}
	return c.Process.Pid
}

// TestDaemonRecordRetentionIsBounded (RT-166 review, I48): a scratch
// daemon's record id is addr-<port>-<pid>, so records are never
// overwritten. Records of scratch daemons whose pid is gone are pruned
// when a daemon records itself (a managed one's are kept — verbs name its
// call log through it while it restarts), and a lookup for an address
// reads only the record directories that can be that address's (the
// managed label's, and addr-<port>-*), not every record ever written.
func TestDaemonRecordRetentionIsBounded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	resetServingCallLogForTest(t)
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

	gone := deadPID(t)
	write := func(id string, rec console.DaemonRecord) string {
		t.Helper()
		p, err := daemonRecordPath(home, id)
		if err != nil {
			t.Fatal(err)
		}
		rec.SchemaVersion, rec.ID, rec.StartedAt = 2, id, now.Format(time.RFC3339)
		if err := console.WriteDaemonRecord(p, rec); err != nil {
			t.Fatal(err)
		}
		return p
	}
	staleScratch := write("addr-7001-"+strconv.Itoa(gone), console.DaemonRecord{Addr: "127.0.0.1:7001", PID: gone})
	managedDown := write("dev.fathom.serve-http", console.DaemonRecord{Addr: "127.0.0.1:7581", PID: gone, Managed: true})
	liveOther := write("addr-7002-"+strconv.Itoa(os.Getppid()), console.DaemonRecord{Addr: "127.0.0.1:7002", PID: os.Getppid()})

	if _, _, err := recordServingDaemonAs(daemonMeta{PPID: 1}, "127.0.0.1:7003", "", "", "c0ffee", now); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(staleScratch); !os.IsNotExist(err) {
		t.Fatalf("a dead scratch daemon's record survived a new daemon's start (err=%v)", err)
	}
	if _, err := os.Stat(filepath.Dir(staleScratch)); !os.IsNotExist(err) {
		t.Fatalf("its empty record directory survived (err=%v)", err)
	}
	if _, err := os.Stat(managedDown); err != nil {
		t.Fatalf("the managed daemon's record was pruned while it is down: %v", err)
	}
	if _, err := os.Stat(liveOther); err != nil {
		t.Fatalf("a live daemon's record was pruned: %v", err)
	}

	// The lookup reads by directory name: a record filed under another
	// port's directory is not read for this address.
	write("addr-7999-1", console.DaemonRecord{Addr: "127.0.0.1:7004", PID: 1})
	if rec, ok := findDaemonRecord(home, "127.0.0.1:7004"); ok {
		t.Fatalf("findDaemonRecord read a directory that cannot hold 127.0.0.1:7004: %+v", rec)
	}
	if rec, ok := findDaemonRecord(home, "localhost:7581"); !ok || !rec.Managed {
		t.Fatalf("the managed daemon's labelled record not found for its address: %+v ok=%v", rec, ok)
	}
	if rec, ok := findDaemonRecord(home, "127.0.0.1:7002"); !ok || rec.PID != os.Getppid() {
		t.Fatalf("addr-7002-* not found for 127.0.0.1:7002: %+v ok=%v", rec, ok)
	}
}

// TestUnmanagedServeHTTPRemovesRecordOnSIGTERM (RT-166 review): kill,
// pkill, `daemon reap` and a script's trap all stop a scratch daemon with
// SIGTERM; before, only the bounded exits removed the record, so every
// SIGTERM left one behind for good. The daemon exits 143 (128+SIGTERM)
// having removed it.
func TestUnmanagedServeHTTPRemovesRecordOnSIGTERM(t *testing.T) {
	if testing.Short() {
		t.Skip("spawns serve-http")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "serve.log")
	serve := exec.Command(os.Args[0])
	serve.Env = append(os.Environ(), testHelperEnv+"=rt20-serve", "RT20_LOG="+logPath,
		"RT20_STORE="+filepath.Join(dir, "scratch.sqlite"), "XPC_SERVICE_NAME=")
	if err := serve.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serve.Process.Kill(); _ = serve.Wait() })
	readLog := func() string { b, _ := os.ReadFile(logPath); return string(b) }
	deadline := time.Now().Add(60 * time.Second)
	for !strings.Contains(readLog(), "daemon record:") {
		if time.Now().After(deadline) {
			t.Fatalf("serve-http never wrote its record:\n%s", readLog())
		}
		time.Sleep(100 * time.Millisecond)
	}
	var recordPath string
	sc := bufio.NewScanner(strings.NewReader(readLog()))
	for sc.Scan() {
		if _, after, ok := strings.Cut(sc.Text(), "daemon record: "); ok {
			recordPath, _, _ = strings.Cut(after, " (call log")
		}
	}
	if _, err := os.Stat(recordPath); err != nil {
		t.Fatalf("record %q not on disk: %v", recordPath, err)
	}
	if err := serve.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	err := serve.Wait()
	ee, ok := err.(*exec.ExitError)
	if !ok || ee.ExitCode() != 143 {
		t.Fatalf("serve-http on SIGTERM: %v, want exit 143\n%s", err, readLog())
	}
	if _, err := os.Stat(recordPath); !os.IsNotExist(err) {
		t.Fatalf("record %s left behind after SIGTERM (err=%v)\n%s", recordPath, err, readLog())
	}
	if !strings.Contains(readLog(), "signal terminated") {
		t.Errorf("exit reason not logged:\n%s", readLog())
	}
}
