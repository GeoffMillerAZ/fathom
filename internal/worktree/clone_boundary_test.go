package worktree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func caseAlias(t *testing.T, path string) string {
	t.Helper()
	alias := strings.ToUpper(path)
	a, err := os.Stat(alias)
	if err != nil {
		t.Skip("filesystem does not resolve case aliases")
	}
	b, err := os.Stat(path)
	if err != nil || !os.SameFile(a, b) {
		t.Skip("filesystem does not resolve case aliases")
	}
	return alias
}

func TestCloneBoundaryF1HomeIdentity(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	if err := os.MkdirAll(filepath.Join(home, "child"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	if cloneRootAllowed(caseAlias(t, home), nil) {
		t.Fatal("respelled home authorized as a clone root")
	}
	t.Setenv("HOME", filepath.Join(home, "child"))
	if cloneRootAllowed(caseAlias(t, home), nil) {
		t.Fatal("respelled ancestor of home authorized as a clone root")
	}
}

func TestCloneBoundaryF1IndexedIdentity(t *testing.T) {
	repo, _, head := reviewClone(t)
	roots := []IndexedRoot{{Service: "svc", Root: repo, IndexedCommit: head}}
	alias := caseAlias(t, repo)
	if cloneRootAllowed(alias, roots) || cloneRootAllowed(filepath.Dir(alias), roots) {
		t.Fatal("respelled indexed root or its ancestor authorized as a clone root")
	}
	for _, target := range []string{alias, filepath.Join(alias, "new.go")} {
		r, ok := Resolve(context.Background(), target, roots)
		if !ok || r.Service != "svc" || r.Class != "" || r.Note != "" {
			t.Fatalf("indexed path must resolve directly: %+v, %v", r, ok)
		}
	}
	roots = append(roots, IndexedRoot{Service: "alias", Root: alias, IndexedCommit: head})
	if r, ok := Resolve(context.Background(), repo, roots); ok {
		t.Fatalf("same directory registered twice must remain ambiguous: %+v", r)
	}
}

func TestCloneBoundaryF3MissingHome(t *testing.T) {
	repo, clone, head := reviewClone(t)
	t.Setenv("HOME", "")
	roots := []IndexedRoot{{Service: "svc", Root: repo, IndexedCommit: head}}
	r, ok := Resolve(context.Background(), clone, roots)
	if ok || !strings.Contains(r.Note, "home directory unavailable") {
		t.Fatalf("clone must refuse and explain missing home: %+v, %v", r, ok)
	}
	if r, ok := Resolve(context.Background(), repo, roots); !ok || r.Class != "" {
		t.Fatalf("direct lookup must remain available: %+v, %v", r, ok)
	}
}
