package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

// CS-8: a held port names its holder on a Linux host with no lsof (the
// cloud session VMs have none) — procListenerPID reads the kernel's socket
// tables and the processes' fd links instead. A fake procfs pins the
// parsing on every platform: only a LISTEN row (state 0A) on the port
// counts, IPv6 too, and the lowest pid holding the socket wins.
func TestProcListenerPIDReadsTheSocketTables(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fd := func(pid, n int, target string) {
		t.Helper()
		dir := filepath.Join(root, strconv.Itoa(pid), "fd")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, strconv.Itoa(n))); err != nil {
			t.Fatal(err)
		}
	}
	const header = "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	// 7575 = 0x1D97. Row 0: a connection on the port (01, ESTABLISHED) —
	// not a listener. Row 1: the listener on 127.0.0.1:7575. Row 2: a
	// listener on another port.
	write("net/tcp", header+
		"   0: 0100007F:1D97 0100007F:C350 01 00000000:00000000 00:00000000 00000000   501        0 111 1 0000000000000000 20 4 30 10 -1\n"+
		"   1: 0100007F:1D97 00000000:0000 0A 00000000:00000000 00:00000000 00000000   501        0 222 1 0000000000000000 100 0 0 10 0\n"+
		"   2: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000   501        0 333 1 0000000000000000 100 0 0 10 0\n")
	write("net/tcp6", header+
		"   0: 00000000000000000000000000000000:2328 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000   501        0 444 1 0000000000000000 100 0 0 10 0\n")
	fd(10, 3, "socket:[111]") // the connection's peer, not the listener
	fd(20, 4, "socket:[333]") // listens on another port
	fd(30, 5, "/dev/null")
	fd(30, 6, "socket:[222]") // the listener
	fd(40, 7, "socket:[222]") // a forked child sharing it
	fd(50, 8, "socket:[444]") // the IPv6 listener on 9000
	write("self/status", "not a pid directory\n")

	if got := procListenerPID(root, 7575); got != 30 {
		t.Errorf("port 7575: pid %d, want 30 (the lowest pid holding the LISTEN socket)", got)
	}
	if got := procListenerPID(root, 9000); got != 50 {
		t.Errorf("port 9000 (tcp6): pid %d, want 50", got)
	}
	if got := procListenerPID(root, 8080); got != 20 {
		t.Errorf("port 8080: pid %d, want 20", got)
	}
	if got := procListenerPID(root, 50000); got != 0 {
		t.Errorf("a port only a connection uses: pid %d, want 0", got)
	}
	if got := procListenerPID(filepath.Join(root, "absent"), 7575); got != 0 {
		t.Errorf("no procfs: pid %d, want 0", got)
	}
}

// On Linux the real procfs names this process as the holder of a port it
// listens on, lsof or not.
func TestProcListenerPIDFindsThisProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("procfs socket tables are Linux's; TestProcListenerPIDReadsTheSocketTables pins the parsing everywhere")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if got := procListenerPID("/proc", port); got != os.Getpid() {
		t.Fatalf("procListenerPID(%d) = %d, want this process %d", port, got, os.Getpid())
	}
	if got := (realDaemonOps{}).ListenerPID(fmt.Sprintf("127.0.0.1:%d", port)); got != os.Getpid() {
		t.Fatalf("ListenerPID = %d, want this process %d", got, os.Getpid())
	}
}
