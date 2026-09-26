package main

import (
	"fmt"
	"strings"
	"sync"
	"syscall"
	"time"
)

// fakeDaemonOps is daemonOps with a fake clock, a fake process table and a
// recorder: launchctl, codesign and signals never reach the machine.
type fakeDaemonOps struct {
	mu       sync.Mutex
	now      time.Time
	env      map[string]string
	runOut   map[string][]byte
	runErr   map[string]error
	calls    []string
	procs    map[int]procInfo
	signals  []string
	listener map[string]int
	open     map[string]bool
	// onSignal simulates the process's reaction to a signal.
	onSignal func(f *fakeDaemonOps, pid int, sig syscall.Signal)
}

func newFakeDaemonOps(now time.Time) *fakeDaemonOps {
	return &fakeDaemonOps{
		now: now, env: map[string]string{}, runOut: map[string][]byte{}, runErr: map[string]error{},
		procs: map[int]procInfo{}, listener: map[string]int{}, open: map[string]bool{},
	}
}

func (f *fakeDaemonOps) Now() time.Time         { f.mu.Lock(); defer f.mu.Unlock(); return f.now }
func (f *fakeDaemonOps) Getenv(k string) string { return f.env[k] }
func (f *fakeDaemonOps) UID() int               { return 501 }
func (f *fakeDaemonOps) AgentShell() bool {
	var env []string
	for k, v := range f.env {
		env = append(env, k+"="+v)
	}
	return isAgentShell(env)
}
func (f *fakeDaemonOps) Sleep(d time.Duration) { f.mu.Lock(); f.now = f.now.Add(d); f.mu.Unlock() }

func (f *fakeDaemonOps) Run(name string, args ...string) ([]byte, error) {
	key := strings.TrimSpace(name + " " + strings.Join(args, " "))
	f.mu.Lock()
	f.calls = append(f.calls, key)
	f.mu.Unlock()
	if out, ok := f.runOut[key]; ok {
		return out, f.runErr[key]
	}
	return nil, fmt.Errorf("fake: no such command %q", key)
}

func (f *fakeDaemonOps) Proc(pid int) (procInfo, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, ok := f.procs[pid]
	return p, ok
}

func (f *fakeDaemonOps) Signal(pid int, sig syscall.Signal) error {
	f.mu.Lock()
	f.signals = append(f.signals, fmt.Sprintf("%d:%s", pid, sigName(sig)))
	_, alive := f.procs[pid]
	f.mu.Unlock()
	if !alive {
		return syscall.ESRCH
	}
	if f.onSignal != nil {
		f.onSignal(f, pid, sig)
	}
	return nil
}

func (f *fakeDaemonOps) ListenerPID(addr string) int { return f.listener[addr] }
func (f *fakeDaemonOps) PortOpen(addr string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.open[addr]
}

func (f *fakeDaemonOps) kill(pid int, addr string) {
	f.mu.Lock()
	delete(f.procs, pid)
	if addr != "" {
		f.open[addr] = false
	}
	f.mu.Unlock()
}

func sigName(s syscall.Signal) string {
	switch s {
	case syscall.SIGTERM:
		return "TERM"
	case syscall.SIGKILL:
		return "KILL"
	}
	return s.String()
}
