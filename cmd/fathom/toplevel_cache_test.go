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
// cwd across processes, keyed by the cwd and the identity (device, inode) of the .git nearest
// the cwd — never its mtime, which every index refresh moves — with a
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
	// Index refreshes (git status, git diff) create and rename
	// .git/index.lock, moving .git's mtime on nearly every git command: the
	// cached answer still stands (§11.3's saving, across processes).
	if out, err := exec.Command("git", "-C", repo, "status", "--short").CombinedOutput(); err != nil {
		t.Fatalf("git status: %v %s", err, out)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(filepath.Join(repo, ".git"), later, later); err != nil {
		t.Fatal(err)
	}
	resetToplevelMemo()
	if got := gitToplevel(sub); got != repo || forks != 2 {
		t.Fatalf(".git's mtime moving is not new evidence: %q, %d fork(s)", got, forks)
	}
	// A nested `git init` between the cwd and the cached toplevel misses.
	nested := filepath.Join(repo, "a")
	if out, err := exec.Command("git", "init", "-q", nested).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if got := gitToplevel(sub); got != nested || forks != 3 {
		t.Fatalf("a nested git init misses: %q, %d fork(s)", got, forks)
	}
	if err := os.RemoveAll(filepath.Join(nested, ".git")); err != nil {
		t.Fatal(err)
	}
	if got := gitToplevel(sub); got != repo || forks != 4 {
		t.Fatalf("the nested repository removed misses: %q, %d fork(s)", got, forks)
	}
	// A re-init (the .git replaced) misses: a new .git is new evidence. The
	// old one is moved aside, not deleted: a Linux file system hands a freed
	// inode to the next file created (CS-8 measured it in a root
	// container), so a deleted-and-recreated .git can carry the identity
	// the cache recorded — at the same path that is the same answer, and at
	// another path TestGitToplevelReusedIdentityAtAnotherPathMisses pins
	// the miss. Kept allocated, the old inode cannot be reused here.
	if err := os.Rename(filepath.Join(repo, ".git"), filepath.Join(repo, "old.git")); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if got := gitToplevel(sub); got != repo || forks != 5 {
		t.Fatalf("a re-initialized .git misses: %q, %d fork(s)", got, forks)
	}
	// The TTL.
	now = now.Add(toplevelTTL)
	resetToplevelMemo()
	if gitToplevel(sub); forks != 6 {
		t.Fatalf("10 minutes on the entry expires: %d fork(s)", forks)
	}
	// A cached "not a repository" misses once a .git appears above the cwd.
	plain, _ := filepath.EvalSymlinks(t.TempDir())
	if got := gitToplevel(plain); got != "" || forks != 7 {
		t.Fatalf("outside a repo: %q, %d", got, forks)
	}
	if gitToplevel(plain); forks != 7 {
		t.Fatalf("a not-a-repo answer is cached: %d", forks)
	}
	if out, err := exec.Command("git", "init", "-q", plain).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	if got := gitToplevel(plain); got != plain || forks != 8 {
		t.Fatalf("git init under a cached negative answer misses: %q, %d", got, forks)
	}
}

// CS-8 (found in a root ubuntu:24.04 container): the cached evidence is
// <top>/.git itself, not merely a file with its identity. A Linux file
// system hands a freed inode to the next file created, so after
// <top>/.git is removed a nested `git init` between the cwd and <top> can
// carry the very (device, inode) the cache recorded for <top>/.git. The
// entry is staged with the nested .git's identity, as that reuse leaves
// it, and must miss: the answer is the nested repository, not <top>.
func TestGitToplevelReusedIdentityAtAnotherPathMisses(t *testing.T) {
	prevDir, prevExec, prevNow := hookcache.ToplevelDir, gitToplevelExec, toplevelNow
	hookcache.ToplevelDir = t.TempDir()
	forks := 0
	gitToplevelExec = func(dir string) string { forks++; return gitToplevelUncached(dir) }
	now := time.Date(2026, 10, 3, 6, 0, 0, 0, time.UTC)
	toplevelNow = func() time.Time { return now }
	resetToplevelMemo()
	t.Cleanup(func() {
		hookcache.ToplevelDir, gitToplevelExec, toplevelNow = prevDir, prevExec, prevNow
		resetToplevelMemo()
	})

	repo, _ := filepath.EvalSymlinks(t.TempDir())
	nested := filepath.Join(repo, "a")
	sub := filepath.Join(nested, "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{repo, nested} {
		if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
			t.Fatalf("git init %s: %v %s", dir, err, out)
		}
	}
	// <top>/.git removed; the nested .git holds the identity the entry
	// recorded for it.
	if err := os.RemoveAll(filepath.Join(repo, ".git")); err != nil {
		t.Fatal(err)
	}
	reused := identify(filepath.Join(nested, ".git"))
	if !reused.ok {
		t.Fatal("identify the nested .git")
	}
	toplevelMemo.Store(sub, toplevelEntry{Cwd: sub, Top: repo, GitDev: reused.dev, GitIno: reused.ino, At: now})

	if got := gitToplevel(sub); got != nested || forks != 1 {
		t.Fatalf("an entry whose identity now names another .git must miss: %q, %d fork(s); want %q after one fork", got, forks, nested)
	}
	// The answer it wrote back stands.
	if got := gitToplevel(sub); got != nested || forks != 1 {
		t.Fatalf("the re-forked answer is cached: %q, %d fork(s)", got, forks)
	}
}
