package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/hookcache"
)

// RT-130 (design §11.3): gitToplevel is memoized in process and cached per
// cwd across processes, keyed by the cwd and <top>/.git's mtime with a
// 10-minute TTL; a `git init` under a cached "not a repository" misses.
func TestGitToplevelCachedPerCwd(t *testing.T) {
	prevDir, prevExec, prevNow := hookcache.ToplevelDir, gitToplevelExec, toplevelNow
	hookcache.ToplevelDir = t.TempDir()
	forks := 0
	gitToplevelExec = func(dir string) string { forks++; return gitToplevelUncached(dir) }
	now := time.Date(2026, 9, 27, 6, 0, 0, 0, time.UTC)
	toplevelNow = func() time.Time { return now }
	resetToplevelMemo()
	t.Cleanup(func() {
		hookcache.ToplevelDir, gitToplevelExec, toplevelNow = prevDir, prevExec, prevNow
		resetToplevelMemo()
	})

	repo, _ := filepath.EvalSymlinks(t.TempDir())
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	sub := filepath.Join(repo, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := gitToplevel(sub); got != repo || forks != 1 {
		t.Fatalf("first call: %q, %d fork(s)", got, forks)
	}
	if got := gitToplevel(sub); got != repo || forks != 1 {
		t.Fatalf("in-process memo: %q, %d fork(s)", got, forks)
	}
	// Another process: no memo, the shared cache answers.
	resetToplevelMemo()
	if got := gitToplevel(sub); got != repo || forks != 1 {
		t.Fatalf("cross-process cache: %q, %d fork(s)", got, forks)
	}
	// Per cwd: another directory is its own entry.
	if got := gitToplevel(repo); got != repo || forks != 2 {
		t.Fatalf("a second cwd forks once: %q, %d", got, forks)
	}
	// .git's mtime moves: the cached answer's evidence is gone.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(repo, ".git"), later, later); err != nil {
		t.Fatal(err)
	}
	if got := gitToplevel(sub); got != repo || forks != 3 {
		t.Fatalf(".git mtime change misses: %q, %d", got, forks)
	}
	// The TTL.
	now = now.Add(toplevelTTL)
	resetToplevelMemo()
	if gitToplevel(sub); forks != 4 {
		t.Fatalf("10 minutes on the entry expires: %d fork(s)", forks)
	}
	// A cached "not a repository" misses once a .git appears above the cwd.
	plain, _ := filepath.EvalSymlinks(t.TempDir())
	if got := gitToplevel(plain); got != "" || forks != 5 {
		t.Fatalf("outside a repo: %q, %d", got, forks)
	}
	if gitToplevel(plain); forks != 5 {
		t.Fatalf("a not-a-repo answer is cached: %d", forks)
	}
	if out, err := exec.Command("git", "init", "-q", plain).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if got := gitToplevel(plain); got != plain || forks != 6 {
		t.Fatalf("git init under a cached negative answer misses: %q, %d", got, forks)
	}
}
