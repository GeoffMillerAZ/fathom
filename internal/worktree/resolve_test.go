package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func identityGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_DATE=2000-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2000-01-01T00:00:00Z")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestResolveCloneIdentity(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	repo := filepath.Join(base, "base")
	clone := filepath.Join(base, "clone")
	wt := filepath.Join(base, "worktree")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	identityGit(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, repo, "add", "a.go")
	identityGit(t, repo, "commit", "-qm", "base")
	head := identityGit(t, repo, "rev-parse", "HEAD")
	identityGit(t, repo, "remote", "add", "origin", "HTTPS://Forge.Example/team/project.git/")
	identityGit(t, base, "clone", "-q", repo, clone)
	identityGit(t, clone, "remote", "set-url", "origin", "Git@FORGE.example:team/project")
	identityGit(t, repo, "worktree", "add", "-qb", "other", wt)
	services := []IndexedRoot{{Service: "svc", Root: repo, IndexedCommit: head}}
	for _, p := range []string{clone, wt, filepath.Join(clone, "a.go")} {
		r, ok := Resolve(ctx, p, services)
		if !ok || r.Service != "svc" || r.Note != "clone at "+head[:7]+", index at "+head[:7] {
			t.Fatalf("resolve %s: %+v %v", p, r, ok)
		}
		if same, reason := r.Unchanged(ctx, "a.go"); !same || reason != "" {
			t.Fatalf("unchanged: %v %s", same, reason)
		}
	}
	r, _ := Resolve(ctx, clone, services)
	if err := os.WriteFile(filepath.Join(clone, "a.go"), []byte("package changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if same, reason := r.Unchanged(ctx, "a.go"); same || reason != "changed in clone" {
		t.Fatalf("modified: %v %s", same, reason)
	}
	if same, reason := r.Unchanged(ctx, "new.go"); same || !strings.Contains(reason, "clone") {
		t.Fatalf("new: %v %s", same, reason)
	}
	// Local-path origins are identity by root commit, not path spelling.
	identityGit(t, clone, "remote", "set-url", "origin", filepath.Join(base, "relocated"))
	if _, ok := Resolve(ctx, clone, services); !ok {
		t.Fatal("local origin with matching root did not resolve")
	}
	identityGit(t, clone, "remote", "set-url", "origin", "https://forge.example/team/other")
	if _, ok := Resolve(ctx, clone, services); ok {
		t.Fatal("different nonlocal origins resolved")
	}
	identityGit(t, clone, "remote", "set-url", "origin", "https://forge.example/team/project")
	identityGit(t, clone, "checkout", "--orphan", "unrelated")
	identityGit(t, clone, "add", "a.go")
	identityGit(t, clone, "commit", "-qm", "unrelated root")
	if _, ok := Resolve(ctx, clone, services); ok {
		t.Fatal("different root commit resolved")
	}
}

func TestResolveCloneTieBreak(t *testing.T) {
	ctx := context.Background()
	repo, wt := gitRepoWithWorktree(t)
	base := t.TempDir()
	clone := filepath.Join(base, "clone")
	identityGit(t, repo, "remote", "add", "origin", "https://forge.example/team/project")
	identityGit(t, base, "clone", "-q", repo, clone)
	identityGit(t, clone, "remote", "set-url", "origin", "ssh://git@forge.example/team/project.git")
	candidates := []IndexedRoot{{Service: "a", Root: repo}, {Service: "b", Root: clone}}
	if r, ok := Resolve(ctx, wt, candidates); !ok || r.Service != "a" {
		t.Fatalf("common dir must win: %+v %v", r, ok)
	}
	third := filepath.Join(base, "third")
	identityGit(t, base, "clone", "-q", repo, third)
	for _, c := range [][]IndexedRoot{candidates, {candidates[1], candidates[0]}} {
		if _, ok := Resolve(ctx, third, c); ok {
			t.Fatal("ambiguous clones must be unknown")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, ok := Resolve(canceled, third, candidates); ok {
		t.Fatal("canceled resolution must fail open")
	}
}
