package main

// RT-7's local-work half for session-end-hook (mirrors
// TestIdentityLookupSkipsItsLocalWorkWhenTheDeadlineIsSpent in
// hookguard_localwork_test.go). session-end-hook has no daemon call ahead
// of its git reads to bound them, so its whole budget is local work:
// mechanicalSessionFacts used to fork git four times back to back, and
// under contention the cost is fork+exec LATENCY (each spawn slow to
// start, not costly in CPU) — four sequential waits where one would do,
// because the reads do not depend on each other.

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stubSessionFactsGit swaps the git seam for fn and restores it.
func stubSessionFactsGit(t *testing.T, fn func(prev func(string, ...string) string, root string, args ...string) string) {
	t.Helper()
	prev := sessionFactsGitExec
	sessionFactsGitExec = func(root string, args ...string) string { return fn(prev, root, args...) }
	t.Cleanup(func() { sessionFactsGitExec = prev })
}

// TestSessionEndFactsStopAtTheHookDeadline: once the hook's deadline is
// spent mechanicalSessionFacts forks nothing — the facts only feed a
// deposit the hook is already out of budget to make.
func TestSessionEndFactsStopAtTheHookDeadline(t *testing.T) {
	repo := hookRepo(t) // hookRepo leaves an untracked main.go: git status is dirty

	var calls atomic.Int32
	stubSessionFactsGit(t, func(prev func(string, ...string) string, root string, args ...string) string {
		calls.Add(1)
		return prev(root, args...)
	})

	if facts := mechanicalSessionFacts(repo); facts == "" {
		t.Fatal("fixture: expected mechanicalSessionFacts to find the dirty file with no guard active")
	}
	if calls.Load() == 0 {
		t.Fatal("fixture: mechanicalSessionFacts made no git calls")
	}

	calls.Store(0)
	spentHookGuard(t)
	if facts := mechanicalSessionFacts(repo); facts != "" {
		t.Fatalf("mechanicalSessionFacts with a spent deadline returned %q, want \"\" (all local work skipped)", facts)
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("git ran %d time(s) after the hook deadline was spent, want 0", n)
	}
}

// TestSessionEndFactsReadGitConcurrently: the four reads (status, stash
// list, HEAD's branch, the claimed-unproven plan diff) are independent, so
// they are all in flight at once. Every stubbed read blocks until all of
// them have been started; a sequential chain never gets the second one
// started and the barrier times out.
func TestSessionEndFactsReadGitConcurrently(t *testing.T) {
	repo := hookRepo(t)
	const reads = 4

	var started, inflight, peak atomic.Int32
	var release sync.Once
	all := make(chan struct{})
	var timedOut atomic.Bool
	stubSessionFactsGit(t, func(prev func(string, ...string) string, root string, args ...string) string {
		n := inflight.Add(1)
		defer inflight.Add(-1)
		for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
		}
		if started.Add(1) == reads {
			release.Do(func() { close(all) })
		}
		select {
		case <-all:
		case <-time.After(2 * time.Second):
			timedOut.Store(true)
			release.Do(func() { close(all) }) // let the rest through, fail once
		}
		return prev(root, args...)
	})

	facts := mechanicalSessionFacts(repo)
	if timedOut.Load() {
		t.Fatalf("at most %d of %d git reads were in flight together: the chain is sequential", peak.Load(), reads)
	}
	if got := started.Load(); got != reads {
		t.Fatalf("mechanicalSessionFacts made %d git reads, want %d", got, reads)
	}
	if facts != "1 uncommitted file(s)" {
		t.Fatalf("facts = %q, want %q (concurrency must not change the answer)", facts, "1 uncommitted file(s)")
	}
}
