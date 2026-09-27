package gitro

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		got, err := Head(d)
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
	if got, err := Head(dir); err != nil || len(got) != 40 {
		t.Errorf("without git on PATH: %q, %v", got, err)
	}
	if _, err := Head(t.TempDir()); err == nil {
		t.Error("a directory outside any repository must be an error")
	}
}
