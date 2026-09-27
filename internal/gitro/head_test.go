package gitro

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const (
	shaA = "1111111111111111111111111111111111111111"
	shaB = "2222222222222222222222222222222222222222"
	shaC = "3333333333333333333333333333333333333333"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// bump moves path's mtime forward so the per-root cache sees the change
// even on a filesystem with coarse timestamps.
func bump(t *testing.T, path string, d time.Duration) {
	t.Helper()
	at := time.Now().Add(d)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// countForks replaces the fallback with a counter that fails the test if
// it is reached (a fork is exactly what Head must not do for a readable
// repository).
func countForks(t *testing.T) *int64 {
	t.Helper()
	var n int64
	prev := headFallback
	headFallback = func(ctx context.Context, root string) (string, error) {
		atomic.AddInt64(&n, 1)
		return prev(ctx, root)
	}
	t.Cleanup(func() { headFallback = prev; ResetHeadCache() })
	ResetHeadCache()
	return &n
}

// TestHead_ReadsRefsWithoutFork is RT-101 (design §7.1): HEAD is read from
// .git/HEAD and the ref files — a loose ref, packed-refs, a detached HEAD
// and a linked worktree's gitdir/commondir — with no git process, cached
// per root and invalidated when a file's mtime moves. Only an unreadable
// layout takes the contexted fallback, and a fallback that times out reads
// as freshness unknown.
func TestHead_ReadsRefsWithoutFork(t *testing.T) {
	forks := countForks(t)
	ctx := context.Background()

	// A loose ref.
	repo := t.TempDir()
	gd := filepath.Join(repo, ".git")
	writeFile(t, filepath.Join(gd, "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(gd, "refs", "heads", "main"), shaA+"\n")
	if got, err := Head(ctx, repo); err != nil || got != shaA {
		t.Fatalf("loose ref: Head = %q, %v; want %s", got, err, shaA)
	}
	// Cached: a second read is answered without re-reading (still no fork),
	// and a moved ref (new mtime) is seen.
	writeFile(t, filepath.Join(gd, "refs", "heads", "main"), shaB+"\n")
	bump(t, filepath.Join(gd, "refs", "heads", "main"), 2*time.Second)
	if got, _ := Head(ctx, repo); got != shaB {
		t.Fatalf("after the ref moved: Head = %q, want %s (cache must invalidate on mtime)", got, shaB)
	}

	// packed-refs only (the loose ref removed, as `git pack-refs` does).
	if err := os.Remove(filepath.Join(gd, "refs", "heads", "main")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(gd, "packed-refs"), "# pack-refs with: peeled fully-peeled sorted\n"+shaC+" refs/heads/main\n")
	bump(t, filepath.Join(gd, "packed-refs"), 3*time.Second)
	if got, err := Head(ctx, repo); err != nil || got != shaC {
		t.Fatalf("packed ref: Head = %q, %v; want %s", got, err, shaC)
	}

	// A detached HEAD.
	det := t.TempDir()
	writeFile(t, filepath.Join(det, ".git", "HEAD"), shaA+"\n")
	if got, err := Head(ctx, det); err != nil || got != shaA {
		t.Fatalf("detached: Head = %q, %v; want %s", got, err, shaA)
	}

	// A linked worktree: .git is a file naming the per-worktree gitdir,
	// whose commondir points back at the main repository's refs.
	main := t.TempDir()
	mgd := filepath.Join(main, ".git")
	writeFile(t, filepath.Join(mgd, "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(mgd, "refs", "heads", "feature"), shaB+"\n")
	wtgd := filepath.Join(mgd, "worktrees", "wt")
	writeFile(t, filepath.Join(wtgd, "HEAD"), "ref: refs/heads/feature\n")
	writeFile(t, filepath.Join(wtgd, "commondir"), "../..\n")
	wt := t.TempDir()
	writeFile(t, filepath.Join(wt, ".git"), "gitdir: "+wtgd+"\n")
	if got, err := Head(ctx, wt); err != nil || got != shaB {
		t.Fatalf("worktree: Head = %q, %v; want %s", got, err, shaB)
	}
	// A subdirectory of a work tree resolves to the enclosing repository.
	sub := filepath.Join(repo, "internal", "x")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got, _ := Head(ctx, sub); got != shaC {
		t.Fatalf("subdirectory: Head = %q, want %s", got, shaC)
	}

	if n := atomic.LoadInt64(forks); n != 0 {
		t.Fatalf("Head forked git %d time(s) for readable repositories; want 0", n)
	}

	// A root that does not exist is in no work tree, though the directory
	// it would sit in is one (git -C fails there too).
	if got, err := Head(ctx, filepath.Join(repo, "no-such-dir")); got != "" || !errors.Is(err, ErrNotRepository) {
		t.Fatalf("missing root: Head = %q, %v; want \"\", ErrNotRepository", got, err)
	}

	// Not a repository: no fork either — "" with ErrNotRepository.
	plain := t.TempDir()
	if got, err := Head(ctx, plain); got != "" || !errors.Is(err, ErrNotRepository) {
		t.Fatalf("plain dir: Head = %q, %v; want \"\", ErrNotRepository", got, err)
	}

	// An unreadable layout (a symbolic ref naming a ref that exists
	// nowhere readable, e.g. a reftable repository) takes the fallback,
	// and a fallback that exceeds its deadline is freshness unknown.
	odd := t.TempDir()
	writeFile(t, filepath.Join(odd, ".git", "HEAD"), "ref: refs/heads/.invalid\n")
	headFallback = func(ctx context.Context, root string) (string, error) {
		atomic.AddInt64(forks, 1)
		<-ctx.Done()
		return "", ctx.Err()
	}
	prevDeadline := fallbackDeadline
	fallbackDeadline = 20 * time.Millisecond
	t.Cleanup(func() { fallbackDeadline = prevDeadline })
	got, err := Head(ctx, odd)
	if got != "" || !errors.Is(err, ErrFreshnessUnknown) {
		t.Fatalf("slow fallback: Head = %q, %v; want \"\", ErrFreshnessUnknown", got, err)
	}
	if !strings.Contains(err.Error(), "freshness unknown") {
		t.Fatalf("the error must read as freshness unknown: %v", err)
	}
	if n := atomic.LoadInt64(forks); n != 1 {
		t.Fatalf("fallback ran %d time(s); want exactly 1", n)
	}
}

// TestHead_MatchesGitOnARealRepository cross-checks Head against git itself
// on a real repository: a commit, a packed ref after `git pack-refs`, and a
// linked worktree.
func TestHead_MatchesGitOnARealRepository(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	ResetHeadCache()
	t.Cleanup(ResetHeadCache)
	ctx := context.Background()
	repo := t.TempDir()
	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run(repo, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(repo, "a.txt"), "a")
	run(repo, "add", "a.txt")
	run(repo, "commit", "-q", "-m", "one")
	want := run(repo, "rev-parse", "HEAD")
	if got, err := Head(ctx, repo); err != nil || got != want {
		t.Fatalf("loose: Head = %q, %v; git says %s", got, err, want)
	}
	run(repo, "pack-refs", "--all")
	if got, err := Head(ctx, repo); err != nil || got != want {
		t.Fatalf("packed: Head = %q, %v; git says %s", got, err, want)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	run(repo, "worktree", "add", "-q", "-b", "side", wt)
	writeFile(t, filepath.Join(wt, "b.txt"), "b")
	run(wt, "add", "b.txt")
	run(wt, "commit", "-q", "-m", "two")
	wantWT := run(wt, "rev-parse", "HEAD")
	if got, err := Head(ctx, wt); err != nil || got != wantWT {
		t.Fatalf("worktree: Head = %q, %v; git says %s", got, err, wantWT)
	}
	if got, _ := Head(ctx, repo); got != want {
		t.Fatalf("main checkout after a worktree commit: Head = %q, want %s", got, want)
	}
}

// TestHead_ReadsRefsWithoutGit (RT-115): Head answers `git rev-parse HEAD`
// from the repository's files — a loose ref, a packed ref, a detached HEAD,
// a subdirectory of the work tree and a linked worktree — without running
// git, and names a directory that is not in a repository.
func TestHead_ReadsRefsWithoutGit(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644)
	os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o755)
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-qm", "one")
	revParse := func(d string) string { return strings.TrimSpace(git(t, d, "rev-parse", "HEAD")) }
	check := func(label, d string) {
		t.Helper()
		want := revParse(d)
		got, err := Head(context.Background(), d)
		if err != nil || got != want {
			t.Errorf("%s: Head = %q, %v; want %q", label, got, err, want)
		}
	}
	check("loose ref", dir)
	check("subdirectory", filepath.Join(dir, "sub", "deep"))
	git(t, dir, "pack-refs", "--all")
	check("packed ref", dir)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("b\n"), 0o644)
	git(t, dir, "commit", "-qam", "two")
	check("loose over packed", dir)
	git(t, dir, "checkout", "-q", "--detach", "HEAD~1")
	check("detached", dir)
	git(t, dir, "checkout", "-q", "main")
	wt := filepath.Join(t.TempDir(), "wt")
	git(t, dir, "worktree", "add", "-q", "-b", "side", wt, "HEAD~1")
	check("linked worktree", wt)
	check("main after worktree", dir)

	// PATH without git: Head still answers.
	t.Setenv("PATH", t.TempDir())
	if got, err := Head(context.Background(), dir); err != nil || len(got) != 40 {
		t.Errorf("without git on PATH: %q, %v", got, err)
	}
	if _, err := Head(context.Background(), t.TempDir()); err == nil {
		t.Error("a directory outside any repository must be an error")
	}
}
