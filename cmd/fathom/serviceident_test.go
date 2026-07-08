package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestServiceCandidates_PlainDir(t *testing.T) {
	dir := t.TempDir() // not a git repo
	got := serviceCandidates(dir + "/")
	if len(got) != 1 || got[0].name != filepath.Base(dir) || got[0].root != dir {
		t.Fatalf("candidates = %+v, want just the basename", got)
	}
}

func TestServiceCandidates_SubdirOfGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git unavailable")
	}
	root := t.TempDir()
	sub := filepath.Join(root, "deploy")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	got := serviceCandidates(sub)
	if len(got) != 2 {
		t.Fatalf("candidates = %+v, want basename then git-toplevel", got)
	}
	if got[0].name != "deploy" || got[0].root != sub {
		t.Errorf("candidate[0] = %+v", got[0])
	}
	// macOS TempDir may be behind a /private symlink; compare resolved paths.
	wantRoot, _ := filepath.EvalSymlinks(root)
	gotRoot, _ := filepath.EvalSymlinks(got[1].root)
	if got[1].name != filepath.Base(root) || gotRoot != wantRoot {
		t.Errorf("candidate[1] = %+v, want repo root %s", got[1], root)
	}
}

func TestWithNameAliases(t *testing.T) {
	if got := withNameAliases("sextant"); len(got) != 2 || got[1] != "fathom" {
		t.Errorf("sextant aliases = %v", got)
	}
	if got := withNameAliases("deploy"); len(got) != 1 {
		t.Errorf("deploy aliases = %v", got)
	}
}
