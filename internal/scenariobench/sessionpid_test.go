package scenariobench

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// startSleepStandIn starts a `sleep 300` in its own process group — the
// stand-in for a session's `claude` process every test in this file uses,
// per the task's "no model spend" rule (sessionCommandMarker is
// overridden to "sleep" so the marker check matches it).
func startSleepStandIn(t *testing.T) *exec.Cmd {
	t.Helper()
	old := sessionCommandMarker
	sessionCommandMarker = "sleep"
	t.Cleanup(func() { sessionCommandMarker = old })

	cmd := exec.Command("sleep", "300")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	return cmd
}

// waitDead blocks until pid is no longer reachable by signal 0, failing the
// test if it takes longer than 5s.
func waitDead(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	waited := make(chan error, 1)
	go func() { _, err := cmd.Process.Wait(); waited <- err }()
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatalf("pid %d is still alive", cmd.Process.Pid)
	}
}

// TestWriteAndRemoveSessionPIDFiles: both the raw-attempt-dir-sibling and
// the worktree-sibling pidfiles are written on start and removed on
// completion.
func TestWriteAndRemoveSessionPIDFiles(t *testing.T) {
	dir := t.TempDir()
	rawDir := filepath.Join(dir, "raw", "guards-i1-full-t1")
	wtPath := filepath.Join(dir, "worktrees", "abc123-full")
	if err := os.MkdirAll(filepath.Dir(rawDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := writeSessionPIDFiles(rawDir, wtPath, 4242); err != nil {
		t.Fatal(err)
	}
	rawPID := sessionAttemptPIDFile(rawDir)
	wtPID := sessionWorktreePIDFile(wtPath)
	for _, p := range []string{rawPID, wtPID} {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("pidfile %s: %v", p, err)
		}
		if got, _ := strconv.Atoi(string(raw[:len(raw)-1])); got != 4242 {
			t.Errorf("pidfile %s = %q, want pid 4242", p, raw)
		}
	}

	removeSessionPIDFiles(rawDir, wtPath)
	for _, p := range []string{rawPID, wtPID} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("pidfile %s should be removed: %v", p, err)
		}
	}
}

// TestReapOrphanSession pins the O52 pattern applied to a session pidfile:
// a live, marker-matching process is killed and the pidfile removed; a
// dead or unrelated pid just gets its pidfile removed, never the process.
func TestReapOrphanSession(t *testing.T) {
	stray := startSleepStandIn(t)
	pidfile := filepath.Join(t.TempDir(), "attempt.pid")
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(stray.Process.Pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ReapOrphanSession(pidfile)

	if _, err := os.Stat(pidfile); !os.IsNotExist(err) {
		t.Errorf("pidfile survived the reap: %v", err)
	}
	waitDead(t, stray)

	// A pidfile naming an unrelated live process (this test binary itself)
	// is left alone — only the pidfile is removed.
	pidfile2 := filepath.Join(t.TempDir(), "attempt2.pid")
	me := os.Getpid()
	if err := os.WriteFile(pidfile2, []byte(strconv.Itoa(me)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ReapOrphanSession(pidfile2)
	if syscall.Kill(me, 0) != nil {
		t.Fatal("the test process was killed by a recycled-pid reap")
	}
	if _, err := os.Stat(pidfile2); !os.IsNotExist(err) {
		t.Errorf("pidfile for an unrelated pid should still be removed: %v", err)
	}
}

// TestReapOrphanSessionsUnder sweeps a raw-dir root for every leftover
// "*.pid" sibling and reaps each.
func TestReapOrphanSessionsUnder(t *testing.T) {
	old := sessionCommandMarker
	sessionCommandMarker = "sleep"
	t.Cleanup(func() { sessionCommandMarker = old })

	root := t.TempDir()
	stray1 := exec.Command("sleep", "300")
	stray1.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := stray1.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stray1.Process.Kill(); _, _ = stray1.Process.Wait() })

	pidPath := filepath.Join(root, "guards-i1-full-t1.pid")
	if err := os.WriteFile(pidPath, []byte(strconv.Itoa(stray1.Process.Pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A non-.pid file in the same dir must be left alone.
	other := filepath.Join(root, "guards-i1-full-t1.stream.jsonl")
	if err := os.WriteFile(other, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	reaped := ReapOrphanSessionsUnder(root)
	if len(reaped) != 1 || reaped[0] != stray1.Process.Pid {
		t.Errorf("ReapOrphanSessionsUnder = %v, want [%d]", reaped, stray1.Process.Pid)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Errorf("pidfile survived the sweep: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated file was removed by the sweep: %v", err)
	}
	waitDead(t, stray1)

	// A root that does not exist is not an error.
	if got := ReapOrphanSessionsUnder(filepath.Join(root, "nope")); got != nil {
		t.Errorf("ReapOrphanSessionsUnder(missing) = %v, want nil", got)
	}
}

// TestRefuseIfLiveSessionInWorktree pins O57 finding 1's second half: a
// live session pidfile beside the worktree REFUSES (returns a reason)
// rather than reaping — unlike the scratch daemon, an agent process is not
// something this driver kills on someone else's behalf.
func TestRefuseIfLiveSessionInWorktree(t *testing.T) {
	stray := startSleepStandIn(t)
	wtPath := filepath.Join(t.TempDir(), "abc123-full")
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	pidfile := sessionWorktreePIDFile(wtPath)
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(stray.Process.Pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reason := RefuseIfLiveSessionInWorktree(wtPath)
	if reason == "" {
		t.Fatal("expected a non-empty refusal reason for a live session pidfile")
	}
	// The pidfile is NOT removed and the process is NOT killed — refusal,
	// not reaping.
	if _, err := os.Stat(pidfile); err != nil {
		t.Errorf("pidfile should survive a refusal: %v", err)
	}
	if syscall.Kill(stray.Process.Pid, 0) != nil {
		t.Error("a live session must not be killed by a refusal check")
	}

	// Once the process is gone, the same worktree no longer refuses, and
	// the stale pidfile is cleared.
	_ = stray.Process.Kill()
	waitDead(t, stray)
	if reason := RefuseIfLiveSessionInWorktree(wtPath); reason != "" {
		t.Errorf("RefuseIfLiveSessionInWorktree after death = %q, want \"\"", reason)
	}
	if _, err := os.Stat(pidfile); !os.IsNotExist(err) {
		t.Errorf("stale pidfile should be cleared once the process is dead: %v", err)
	}

	// No pidfile at all: also no refusal.
	if reason := RefuseIfLiveSessionInWorktree(filepath.Join(t.TempDir(), "other")); reason != "" {
		t.Errorf("RefuseIfLiveSessionInWorktree with no pidfile = %q, want \"\"", reason)
	}
}
