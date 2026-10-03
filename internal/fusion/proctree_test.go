//go:build unix

package fusion

// RT-178: a child the typed pass starts runs in its own process group, and
// cancelling the pass's context kills the whole group — the child and
// anything it forked — not only the direct child.
//
// CS-8: "killed" is asserted as "no process of the group is still
// running", never as "the pid is gone". The killed grandchild is orphaned
// (its parent, the child, dies with it) and reparented to PID 1, and
// whether PID 1 reaps it is the host's business: an init does, while a
// container whose PID 1 is not one (`docker run` without --init, a
// `sleep infinity` keeping the container up) leaves it a zombie forever —
// and a zombie still answers signal 0.

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCommandTreeKillsGrandchildOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := commandTree(ctx, "sh", "-c", "sleep 3600 & echo $!; wait")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pgid := cmd.Process.Pid // Setpgid: the child leads its own group
	line, err := bufio.NewReader(out).ReadString('\n')
	if err != nil {
		t.Fatalf("read grandchild pid: %v", err)
	}
	grandchild, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("grandchild pid %q: %v", line, err)
	}
	// The grandchild sleeps for an hour and holds the pipe Wait drains, so a
	// tree that was not killed leaves Wait blocked for good: returning is
	// "the tree was killed promptly". Never leak the sleep, whatever happens.
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })
	cancel()
	returnsUnaided(t, "cmd.Wait after cancel with a grandchild holding the pipe", func() { _ = cmd.Wait() })
	deadline := time.Now().Add(waitSafety)
	for groupRunning(pgid) || pidRunning(grandchild) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(grandchild, syscall.SIGKILL)
			t.Fatalf("process group %d (grandchild %d) still running after its context was cancelled", pgid, grandchild)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// pidRunning reports whether pid names a process that is running — alive
// and not a zombie waiting for a reaper.
func pidRunning(pid int) bool {
	if err := syscall.Kill(pid, 0); err != nil && !errors.Is(err, syscall.EPERM) {
		return false
	}
	state, _, ok := procStat(pid)
	return !ok || !zombie(state) // no procfs (macOS): signal 0 is the answer
}

// groupRunning reports whether any process of group pgid is running. With
// procfs (Linux) it reads every process's group and state, so members that
// are only zombies count as gone; without one, signal 0 to the group is the
// answer (ESRCH: the group has no member left).
func groupRunning(pgid int) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		err := syscall.Kill(-pgid, 0)
		return err == nil || errors.Is(err, syscall.EPERM)
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if state, pgrp, ok := procStat(pid); ok && pgrp == pgid && !zombie(state) {
			return true
		}
	}
	return false
}

// procStat reads a process's state and process group from
// /proc/<pid>/stat; ok is false without procfs or once the pid is gone.
// The fields after the parenthesised command (which may itself hold
// spaces or parentheses) are: state, ppid, pgrp, …
func procStat(pid int) (state byte, pgrp int, ok bool) {
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0, 0, false
	}
	i := bytes.LastIndexByte(b, ')')
	if i < 0 {
		return 0, 0, false
	}
	f := strings.Fields(string(b[i+1:]))
	if len(f) < 3 || len(f[0]) != 1 {
		return 0, 0, false
	}
	pgrp, err = strconv.Atoi(f[2])
	if err != nil {
		return 0, 0, false
	}
	return f[0][0], pgrp, true
}

// zombie: Z (exited, unreaped) or X (dead) — not running.
func zombie(state byte) bool { return state == 'Z' || state == 'X' }
