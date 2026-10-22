package main

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestReviewF2DirectRootRanking(t *testing.T) {
	root := t.TempDir()
	gitT(t, root, "init", "-q")
	for _, tc := range []struct {
		name               string
		symbolsA, symbolsB int
		want               string
	}{
		{"symbols", 10, 20, "b"}, {"name affinity", 10, 10, "a"}, {"lexical", 10, 10, "a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidateName := "other"
			if tc.name == "name affinity" {
				candidateName = "a"
			}
			services := map[string]svcInfo{"a": {path: root, symbols: tc.symbolsA}, "b": {path: root, symbols: tc.symbolsB}}
			cands := []serviceCandidate{{name: candidateName, root: root}}
			old, ok := resolveFromServicesCands(services, cands)
			if !ok || old.name != tc.want {
				t.Fatalf("fixture ranking: %+v %v", old, ok)
			}
			got := resolveIdentity(services, cands, context.Background())
			if got.Service != old.name {
				t.Fatalf("direct ranking changed: %+v, want %s", got, old.name)
			}
		})
	}
}
func TestReviewF2PathlessRegistryBasename(t *testing.T) {
	root := filepath.Join(t.TempDir(), "svc")
	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}
	gitT(t, root, "init", "-q")
	got := resolveIdentity(map[string]svcInfo{"svc": {symbols: 5}}, []serviceCandidate{{name: "svc", root: root}}, context.Background())
	if got.Service != "svc" || got.MatchedBy != identityByBasename {
		t.Fatalf("pathless basename fallback: %+v", got)
	}
}
func TestReviewF8IdentityCallerRootSpelling(t *testing.T) {
	repo := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(repo, alias); err != nil {
		t.Fatal(err)
	}
	got := resolveIdentity(map[string]svcInfo{"svc": {path: repo}}, []serviceCandidate{{name: "svc", root: alias}}, context.Background())
	if got.Root != alias {
		t.Fatalf("root spelling %q, want %q", got.Root, alias)
	}
	both := resolveIdentity(map[string]svcInfo{"svc": {path: repo}}, []serviceCandidate{{name: "svc", root: alias}, {name: "svc", root: repo}}, context.Background())
	if both.Root != alias {
		t.Fatalf("canonical candidate displaced caller spelling: %+v", both)
	}
	cands := mergeCandidates(serviceCandidate{name: "svc", root: got.Root}, []serviceCandidate{{name: "svc", root: alias}})
	if len(cands) != 1 {
		t.Fatalf("duplicate root injection: %+v", cands)
	}
}
func TestReviewF7ThreadDefaultPorts(t *testing.T) {
	for _, origin := range []string{"ssh://git@FORGE.example:22/team/project.git", "https://forge.example:443/team/project", "http://forge.example:80/team/project"} {
		if got := normalizeOriginURL(origin); got != "forge.example/team/project" {
			t.Errorf("%s = %s", origin, got)
		}
	}
}
func TestReviewF3CloneIdentityCacheRefreshesOnlyNote(t *testing.T) {
	oldDir := identCacheDir
	identCacheDir = t.TempDir()
	t.Cleanup(func() { identCacheDir = oldDir; resetIdentityMemo() })
	resetIdentityMemo()
	repo := t.TempDir()
	clone := filepath.Join(t.TempDir(), "clone")
	gitT(t, repo, "init", "-q")
	writeRepoFile(t, repo, "a.go", "package a\n")
	gitT(t, repo, "add", "a.go")
	gitT(t, repo, "commit", "-qm", "base")
	gitT(t, filepath.Dir(clone), "clone", "-q", repo, clone)
	alias := filepath.Join(t.TempDir(), "clone-alias")
	if err := os.Symlink(clone, alias); err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(reviewGitOutput(t, repo, "rev-parse", "HEAD"))
	var calls atomic.Int64
	const baseURL = "http://127.0.0.1:1"
	oldReg := inProcessReg.Load()
	inProcessReg.Store(&inProcessRegistry{self: baseURL, services: func(context.Context) (map[string]svcInfo, error) {
		calls.Add(1)
		return map[string]svcInfo{"svc": {path: repo, indexedCommit: head}}, nil
	}})
	t.Cleanup(func() { inProcessReg.Store(oldReg) })
	first, err := lookupIdentityCached(&http.Client{}, baseURL, alias, context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Root != alias {
		t.Fatalf("clone caller spelling: %q, want %q", first.Root, alias)
	}
	gitT(t, clone, "commit", "--allow-empty", "-qm", "advance")
	nextHead := strings.TrimSpace(reviewGitOutput(t, clone, "rev-parse", "HEAD"))
	for _, disk := range []bool{false, true} {
		if disk {
			resetIdentityMemo()
		}
		next, err := lookupIdentityCached(&http.Client{}, baseURL, alias, context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if next.Service != first.Service || next.FreshnessNote != "clone at "+nextHead[:7]+", index at "+head[:7] {
			t.Fatalf("cached identity note: %+v", next)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("identity RPC count=%d, want 1", calls.Load())
	}
}

func reviewGitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}
