package scenariobench

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// TestReapStaleScratchDaemon pins O52: a daemon a previous cell left
// running beside the shared worktree is killed by pidfile before the next
// cell resets that worktree; a pidfile naming a dead or unrelated pid is a
// no-op, and the pidfile is removed either way.
func TestReapStaleScratchDaemon(t *testing.T) {
	old := scratchDaemonCommandMarker
	scratchDaemonCommandMarker = "sleep"
	t.Cleanup(func() { scratchDaemonCommandMarker = old })

	wt := filepath.Join(t.TempDir(), "1b8765d-deadbeef")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	stray := exec.Command("sleep", "300")
	stray.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := stray.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stray.Process.Kill(); _, _ = stray.Process.Wait() })
	pidfile := scratchDaemonPIDFile(wt)
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(stray.Process.Pid)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	reapStaleScratchDaemon(wt)

	if _, err := os.Stat(pidfile); !os.IsNotExist(err) {
		t.Errorf("pidfile survived the reap: %v", err)
	}
	// The stray is this test's child, so it lingers as a zombie until
	// waited on (a real orphan is reaped by init); Wait returning is the
	// proof it was killed.
	waited := make(chan error, 1)
	go func() { _, err := stray.Process.Wait(); waited <- err }()
	select {
	case <-waited:
	case <-time.After(waitSafety):
		t.Fatalf("stray daemon pid %d is still alive after the reap", stray.Process.Pid)
	}

	// A pidfile naming an unrelated live process is left alone.
	me := os.Getpid()
	if err := os.WriteFile(pidfile, []byte(strconv.Itoa(me)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reapStaleScratchDaemon(wt)
	if syscall.Kill(me, 0) != nil {
		t.Fatal("the test process was killed by a recycled-pid reap")
	}
	if _, err := os.Stat(pidfile); !os.IsNotExist(err) {
		t.Errorf("pidfile for an unrelated pid should still be removed: %v", err)
	}
}

// TestRemoveStaleIndexLock: an old index.lock is cleared, a fresh one kept.
func TestRemoveStaleIndexLock(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "init", "--no-verify"}} {
		if out, err := runIn(ctx, repo, "git", args...); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	lock := filepath.Join(repo, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	removeStaleIndexLock(ctx, repo)
	if _, err := os.Stat(lock); err != nil {
		t.Fatal("a fresh index.lock must be left alone")
	}
	old := time.Now().Add(-time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	removeStaleIndexLock(ctx, repo)
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatal("a minute-old index.lock must be removed")
	}
}
