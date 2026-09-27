package main

// daemon_ops.go — the operating-system interactions the daemon verbs need
// (stop, start, reconcile, status), behind one interface so every launchd,
// signal and process interaction is tested through a fake. The tests never
// call launchctl and never signal a process they did not start.

import (
	"context"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// procInfo is what the pid-reuse guard needs about a live process.
type procInfo struct {
	PID     int
	Start   time.Time
	Command string // full command line
}

// daemonOps is every side effect a daemon verb performs besides files.
type daemonOps interface {
	Now() time.Time
	Getenv(key string) string
	// AgentShell reports whether this is an agent's shell (CLAUDECODE or
	// any CODEX_* set): launchctl run from one wedges (design §8.3), so
	// launchd commands are printed, never run.
	AgentShell() bool
	UID() int
	// Run executes an external command and returns its combined output.
	Run(name string, args ...string) ([]byte, error)
	// Proc reports a live process, or false when pid is not running.
	Proc(pid int) (procInfo, bool)
	Signal(pid int, sig syscall.Signal) error
	// ListenerPID is the pid listening on addr's port, or 0.
	ListenerPID(addr string) int
	// PortOpen reports whether something accepts connections on addr.
	PortOpen(addr string) bool
	Sleep(d time.Duration)
}

// realDaemonOps is daemonOps on this machine.
type realDaemonOps struct{}

func (realDaemonOps) Now() time.Time         { return time.Now() }
func (realDaemonOps) Getenv(k string) string { return os.Getenv(k) }
func (realDaemonOps) UID() int               { return os.Getuid() }
func (realDaemonOps) Sleep(d time.Duration)  { time.Sleep(d) }
func (realDaemonOps) AgentShell() bool       { return isAgentShell(os.Environ()) }
func (realDaemonOps) Signal(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

func (realDaemonOps) Run(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return cmd.CombinedOutput()
}

// Proc reads the process's start time and command line from ps (LC_ALL=C
// so lstart parses). A zombie or vanished pid reads as not running.
func (o realDaemonOps) Proc(pid int) (procInfo, bool) {
	if pid <= 0 {
		return procInfo{}, false
	}
	out, err := o.Run("ps", "-o", "stat=,lstart=,command=", "-p", strconv.Itoa(pid))
	if err != nil {
		return procInfo{}, false
	}
	return parsePSLine(pid, strings.TrimSpace(string(out)))
}

// parsePSLine parses `ps -o stat=,lstart=,command=` output: a state, a
// 5-field lstart ("Sat Sep 26 21:07:01 2026"), then the command.
func parsePSLine(pid int, line string) (procInfo, bool) {
	f := strings.Fields(line)
	if len(f) < 7 {
		return procInfo{}, false
	}
	if strings.HasPrefix(f[0], "Z") {
		return procInfo{}, false // a zombie is gone for every purpose here
	}
	start, err := time.ParseInLocation("Mon Jan 2 15:04:05 2006", strings.Join(f[1:6], " "), time.Local)
	if err != nil {
		return procInfo{}, false
	}
	return procInfo{PID: pid, Start: start, Command: strings.Join(f[6:], " ")}, true
}

func (o realDaemonOps) ListenerPID(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	out, err := o.Run("lsof", "-nP", "-iTCP:"+port, "-sTCP:LISTEN", "-t")
	if err != nil {
		return 0
	}
	for _, l := range strings.Fields(string(out)) {
		if pid, err := strconv.Atoi(l); err == nil {
			return pid
		}
	}
	return 0
}

func (realDaemonOps) PortOpen(addr string) bool {
	c, err := net.DialTimeout("tcp", dialableAddr(addr), 300*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// dialableAddr turns a listen address (":7575", "0.0.0.0:7575") into one a
// client can dial.
func dialableAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::", "localhost":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// isAgentShell reports whether env belongs to an agent's shell.
func isAgentShell(env []string) bool {
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if (k == "CLAUDECODE" && v != "") || (strings.HasPrefix(k, "CODEX_") && v != "") {
			return true
		}
	}
	return false
}
