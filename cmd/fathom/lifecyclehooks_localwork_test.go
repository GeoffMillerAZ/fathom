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
	"sync/atomic"
	"testing"
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

// TestSessionEndFactsBarrierCountsProgress (EW-2): the four reads (status,
// stash list, HEAD's branch, the claimed-unproven plan diff) are independent,
// so they are all in flight at once. Every stubbed read waits at a barrier
// that counts arrivals and releases when the fourth has started; a
// sequential chain never gets the second one started, so the count stalls at
// one. The barrier waits on that count with a large safety bound (a hang
// guard, barrierSafety), never a two-second timer a loaded machine can miss
// while its goroutines are merely slow to start.
func TestSessionEndFactsBarrierCountsProgress(t *testing.T) {
	repo := hookRepo(t)
	const reads = 4

	barrier := newProgressBarrier(reads, barrierSafety)
	stubSessionFactsGit(t, func(prev func(string, ...string) string, root string, args ...string) string {
		barrier.Arrive()
		return prev(root, args...)
	})

	facts := mechanicalSessionFacts(repo)
	if barrier.Stalled() {
		t.Fatalf("the git reads did not start together (%s): the chain is sequential", barrier.Describe())
	}
	if got := barrier.Arrived(); got != reads {
		t.Fatalf("mechanicalSessionFacts made %d git reads, want %d", got, reads)
	}
	if facts != "1 uncommitted file(s)" {
		t.Fatalf("facts = %q, want %q (concurrency must not change the answer)", facts, "1 uncommitted file(s)")
	}
}
