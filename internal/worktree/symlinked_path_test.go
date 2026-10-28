package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

// A repository addressed through a symlinked directory is the same
// directory under two spellings, not a worktree of itself (issue 118).
func TestRelatedTargetSymlinkedPath(t *testing.T) {
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	repo := filepath.Join(link, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	identityGit(t, repo, "init", "-q")
	identityGit(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	file := filepath.Join(repo, "widget.go")
	if err := os.WriteFile(file, []byte("package w\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if class, _, _ := RelatedTarget([]string{repo}, file); class != "" {
		t.Fatalf("a file of the repository itself classed %q, want none", class)
	}
	wt := filepath.Join(link, "wt")
	identityGit(t, repo, "worktree", "add", "-q", wt)
	if class, _, _ := RelatedTarget([]string{repo}, filepath.Join(wt, "x.go")); class != "worktree" {
		t.Fatalf("a linked worktree under a symlinked path classed %q, want worktree", class)
	}
}
