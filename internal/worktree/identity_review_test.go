package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func reviewClone(t *testing.T) (string, string, string) {
	t.Helper()
	repo, _ := gitRepoWithWorktree(t)
	parent := t.TempDir()
	clone := filepath.Join(parent, "clone")
	identityGit(t, parent, "clone", "-q", repo, clone)
	return repo, clone, identityGit(t, repo, "rev-parse", "HEAD")
}

func TestReviewF1RejectUnregisteredWorktree(t *testing.T) {
	repo, _, head := reviewClone(t)
	planted := t.TempDir()
	if err := os.WriteFile(filepath.Join(planted, ".git"), []byte("gitdir: "+filepath.Join(repo, ".git")+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if r, ok := Resolve(context.Background(), planted, []IndexedRoot{{Service: "svc", Root: repo, IndexedCommit: head}}); ok {
		t.Fatalf("planted pointer authorized: %+v", r)
	}
	// Even a real worktree admin directory must point back to this checkout.
	_, wt := gitRepoWithWorktree(t)
	dotgit, err := os.ReadFile(filepath.Join(wt, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planted, ".git"), dotgit, 0600); err != nil {
		t.Fatal(err)
	}
	if commonDir(planted) != "" {
		t.Fatal("borrowed registration authorized another checkout")
	}
}

func TestReviewF1RejectBroadCloneRoots(t *testing.T) {
	if cloneRootAllowed(string(filepath.Separator), nil) {
		t.Fatal("filesystem root authorized")
	}
	repo, clone, head := reviewClone(t)
	roots := []IndexedRoot{{Service: "svc", Root: repo, IndexedCommit: head}}
	t.Setenv("HOME", clone)
	if r, ok := Resolve(context.Background(), clone, roots); ok {
		t.Fatalf("home clone authorized: %+v", r)
	}
	t.Setenv("HOME", filepath.Join(clone, "home"))
	if r, ok := Resolve(context.Background(), clone, roots); ok {
		t.Fatalf("ancestor of home authorized: %+v", r)
	}
	t.Setenv("HOME", t.TempDir())
	roots = append(roots, IndexedRoot{Service: "nested", Root: filepath.Join(clone, "nested")})
	if r, ok := Resolve(context.Background(), clone, roots); ok {
		t.Fatalf("clone containing indexed root authorized: %+v", r)
	}
}

func TestReviewF4NestedCloneService(t *testing.T) {
	repo, clone, head := reviewClone(t)
	nested := filepath.Join(repo, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(clone, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	roots := []IndexedRoot{{Service: "outer", Root: repo, IndexedCommit: head}, {Service: "inner", Root: nested, IndexedCommit: head}}
	r, ok := Resolve(context.Background(), filepath.Join(clone, "nested"), roots)
	if !ok || r.Service != "inner" {
		t.Fatalf("nested clone resolution: %+v %v", r, ok)
	}
	wt := filepath.Join(t.TempDir(), "linked")
	identityGit(t, repo, "worktree", "add", "-qb", "nested-review", wt)
	if err := os.MkdirAll(filepath.Join(wt, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	r, ok = Resolve(context.Background(), filepath.Join(wt, "nested"), roots)
	if !ok || r.Service != "inner" {
		t.Fatalf("nested worktree resolution: %+v %v", r, ok)
	}
	roots = append(roots, IndexedRoot{Service: "twin", Root: nested, IndexedCommit: head})
	if _, ok = Resolve(context.Background(), filepath.Join(clone, "nested"), roots); ok {
		t.Fatal("equal clone roots must remain ambiguous")
	}
}

func TestReviewF7OriginPortsAndRelativeTarget(t *testing.T) {
	for _, raw := range []string{"ssh://git@forge.example:22/team/project.git", "https://forge.example:443/team/project", "http://forge.example:80/team/project"} {
		if got, local := NormalizeOrigin(raw); local || got != "forge.example/team/project" {
			t.Errorf("NormalizeOrigin(%q) = %q %v", raw, got, local)
		}
	}
	if got, _ := NormalizeOrigin("https://forge.example:8443/team/project"); got != "forge.example:8443/team/project" {
		t.Fatal(got)
	}
	repo, clone, _ := reviewClone(t)
	rel, err := filepath.Rel(clone, repo)
	if err != nil {
		t.Fatal(err)
	}
	identityGit(t, clone, "remote", "set-url", "origin", rel)
	class, _, _ := RelatedTarget([]string{repo}, clone)
	if class != "clone" {
		t.Fatalf("relative local origin: %q", class)
	}
}

func TestReviewF3GitFactCounters(t *testing.T) {
	repo, clone, head := reviewClone(t)
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
	roots := []IndexedRoot{{Service: "svc", Root: repo, IndexedCommit: head}}
	// Concurrent callers share the same immutable tree load too.
	r, ok := Resolve(context.Background(), clone, roots)
	if !ok {
		t.Fatal("unresolved clone")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r.MatchesBytes(context.Background(), "file.go", []byte("not the blob")) }()
	}
	wg.Wait()
	for i := 0; i < 3; i++ {
		r, ok := Resolve(context.Background(), clone, roots)
		if !ok {
			t.Fatal("unresolved clone")
		}
		if same, reason := r.MatchesBytes(context.Background(), "file.go", []byte("not the blob")); same || reason != "changed in clone" {
			t.Fatalf("blob state %v %s", same, reason)
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, line := range strings.Split(string(data), "\n") {
		for _, verb := range []string{"ls-tree ", "rev-list ", "show "} {
			if strings.HasPrefix(line, verb) {
				counts[verb]++
			}
		}
	}
	if counts["ls-tree "] != 1 || counts["show "] != 0 || counts["rev-list "] > 2 {
		t.Fatalf("git fact counters: %v\n%s", counts, data)
	}
}

func TestReviewF3NestedCommitTree(t *testing.T) {
	repo := t.TempDir()
	nested := filepath.Join(repo, "nested")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte("package nested\n")
	if err := os.WriteFile(filepath.Join(nested, "a.go"), data, 0644); err != nil {
		t.Fatal(err)
	}
	identityGit(t, repo, "init", "-q")
	identityGit(t, repo, "add", "nested/a.go")
	identityGit(t, repo, "commit", "-qm", "nested")
	head := identityGit(t, repo, "rev-parse", "HEAD")
	clone := filepath.Join(t.TempDir(), "clone")
	identityGit(t, filepath.Dir(clone), "clone", "-q", repo, clone)
	roots := []IndexedRoot{{Service: "inner", Root: nested, IndexedCommit: head}}
	r, ok := Resolve(context.Background(), filepath.Join(clone, "nested/a.go"), roots)
	if !ok {
		t.Fatal("nested identity unresolved")
	}
	if same, why := r.Unchanged(context.Background(), "a.go"); !same {
		t.Fatalf("nested tree comparison: %s", why)
	}
	visible, err := r.VisiblePaths(context.Background())
	if err != nil || !visible["a.go"] || visible["nested/a.go"] {
		t.Fatalf("nested visible paths: %v %v", visible, err)
	}
}

func TestReviewF3RepositoryIdentityKeepsGitRootShapes(t *testing.T) {
	repo, _, _ := reviewClone(t)
	want, ok := RepositoryIdentity(context.Background(), repo)
	if !ok {
		t.Fatal("base identity")
	}
	bare := filepath.Join(t.TempDir(), "bare")
	identityGit(t, filepath.Dir(bare), "clone", "-q", "--bare", repo, bare)
	if got, ok := RepositoryIdentity(context.Background(), bare); !ok || got != want {
		t.Fatalf("bare repository identity regressed: %q %v, want %q", got, ok, want)
	}
}
