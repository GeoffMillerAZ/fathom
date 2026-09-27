package gitro

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// TestHead_FallbackAnswersAreCached is RT-101 (design §7.1, the audit's
// autorefresh-git-fork-per-read hazard): a layout Head cannot read from the
// files forks git once, not on every read. The fallback's answer is kept
// while the files it was asked about stand — a failure at most
// failureCacheTTL — and an unborn branch that git confirms has no commit is
// ErrUnbornHead (the caller's content-manifest case), not freshness unknown.
func TestHead_FallbackAnswersAreCached(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	ctx := context.Background()
	forks := countForks(t)

	repo := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", "main", repo)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	for i := 0; i < 5; i++ {
		got, err := Head(ctx, repo)
		if got != "" || !errors.Is(err, ErrUnbornHead) {
			t.Fatalf("unborn repo read %d: Head = %q, %v; want \"\", ErrUnbornHead", i, got, err)
		}
	}
	if n := atomic.LoadInt64(forks); n != 1 {
		t.Fatalf("5 reads of an unborn repository forked git %d times; want 1", n)
	}

	// The first commit writes the branch's loose ref: the cached verdict
	// is stale by its stamps, and the files answer without a fork.
	writeFile(t, filepath.Join(repo, ".git", "refs", "heads", "main"), shaA+"\n")
	if got, err := Head(ctx, repo); err != nil || got != shaA {
		t.Fatalf("after the first commit: Head = %q, %v; want %s", got, err, shaA)
	}
	if n := atomic.LoadInt64(forks); n != 1 {
		t.Fatalf("a readable ref still forked (total %d)", n)
	}

	// A fallback failure that is not a confirmed unborn branch (here git
	// fails on a HEAD it cannot parse) is cached for failureCacheTTL only.
	odd := t.TempDir()
	writeFile(t, filepath.Join(odd, ".git", "HEAD"), "garbage\n")
	var oddForks int64
	headFallback = func(ctx context.Context, root string) (string, error) {
		atomic.AddInt64(&oddForks, 1)
		return "", errors.New("exit status 128")
	}
	now := time.Unix(1_700_000_000, 0)
	prevNow := headNow
	headNow = func() time.Time { return now }
	t.Cleanup(func() { headNow = prevNow })
	for i := 0; i < 5; i++ {
		if _, err := Head(ctx, odd); !errors.Is(err, ErrFreshnessUnknown) {
			t.Fatalf("unparseable HEAD: %v; want ErrFreshnessUnknown", err)
		}
	}
	if n := atomic.LoadInt64(&oddForks); n != 1 {
		t.Fatalf("5 reads within the failure TTL forked %d times; want 1", n)
	}
	now = now.Add(failureCacheTTL + time.Second)
	_, _ = Head(ctx, odd)
	if n := atomic.LoadInt64(&oddForks); n != 2 {
		t.Fatalf("a failure past its TTL must be asked again: %d forks, want 2", n)
	}

	// A fallback that times out is never confirmation of anything: it is
	// freshness unknown (cached briefly too).
	slow := t.TempDir()
	writeFile(t, filepath.Join(slow, ".git", "HEAD"), "ref: refs/heads/main\n")
	headFallback = func(ctx context.Context, root string) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	}
	prevDeadline := fallbackDeadline
	fallbackDeadline = 10 * time.Millisecond
	t.Cleanup(func() { fallbackDeadline = prevDeadline })
	if _, err := Head(ctx, slow); !errors.Is(err, ErrFreshnessUnknown) || errors.Is(err, ErrUnbornHead) {
		t.Fatalf("timed-out fallback: %v; want ErrFreshnessUnknown, never unborn", err)
	}
	_ = os.Remove(filepath.Join(slow, ".git", "HEAD"))
}
