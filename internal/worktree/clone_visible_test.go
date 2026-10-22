package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneBoundaryF2TrackedSubtree(t *testing.T) {
	repo := t.TempDir()
	subtree := "nested[scope]"
	for _, path := range []string{filepath.Join(subtree, "a.go"), "outside.go"} {
		full := filepath.Join(repo, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("package a\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	identityGit(t, repo, "init", "-q")
	identityGit(t, repo, "add", ".")
	identityGit(t, repo, "commit", "-qm", "base")
	head := identityGit(t, repo, "rev-parse", "HEAD")
	clone := filepath.Join(t.TempDir(), "clone")
	identityGit(t, filepath.Dir(clone), "clone", "-q", repo, clone)
	for _, path := range []string{"id_rsa", "keys.go"} {
		if err := os.WriteFile(filepath.Join(clone, subtree, path), []byte("private-secret\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, ok := Resolve(context.Background(), filepath.Join(clone, subtree), []IndexedRoot{{Service: "svc", Root: filepath.Join(repo, subtree), IndexedCommit: head}})
	if !ok {
		t.Fatal("subtree unresolved")
	}
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + quote(log) + "\nexec " + quote(realGit) + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	visible, err := r.VisiblePaths(context.Background())
	if err != nil || len(visible) != 1 || !visible["a.go"] {
		t.Errorf("tracked subtree only: %v, %v", visible, err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "ls-files -z --cached -- :(top,literal)"+subtree+"/\n") {
		t.Errorf("ls-files must limit enumeration to the literal service subtree: %s", calls)
	}
}
