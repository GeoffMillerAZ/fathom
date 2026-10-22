package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/corpus"
)

type clonePathBackend struct {
	QueryInterface
	root, commit string
	nodes        []core.SymbolNode
}

func (b clonePathBackend) Services(context.Context) ([]string, error) { return []string{"svc"}, nil }
func (b clonePathBackend) ServiceMeta(context.Context, string) (string, string, bool, error) {
	return b.root, b.commit, true, nil
}
func (b clonePathBackend) ListAllSymbols(context.Context, string) ([]core.SymbolNode, error) {
	return b.nodes, nil
}
func (b clonePathBackend) FindSymbol(_ context.Context, _, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range b.nodes {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}
func (b clonePathBackend) TypedEdges(context.Context, string) ([]core.Edge, error)   { return nil, nil }
func (b clonePathBackend) ListCommits(context.Context, string) ([]CommitInfo, error) { return nil, nil }
func (b clonePathBackend) QuerySymbols(context.Context, core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (b clonePathBackend) ScanSources(context.Context, string, corpus.Budget, func(core.SourceFile) bool) (corpus.Coverage, error) {
	return corpus.Coverage{}, nil
}

func cloneGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
	c.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, b)
	}
	return strings.TrimSpace(string(b))
}
func TestCloneMCPPaths(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	base := t.TempDir()
	clone := filepath.Join(base, "clone")
	cloneGit(t, repo, "init", "-q")
	writeFlagFixture(t, repo, "a.go", "package a\nfunc Stable() {}\n")
	writeFlagFixture(t, repo, "b.go", "package a\nfunc Changed() {}\n")
	cloneGit(t, repo, "add", "a.go", "b.go")
	cloneGit(t, repo, "commit", "-qm", "base")
	commit := cloneGit(t, repo, "rev-parse", "HEAD")
	cloneGit(t, repo, "remote", "add", "origin", "https://forge.example/team/project.git")
	cloneGit(t, base, "clone", "-q", repo, clone)
	writeFlagFixture(t, clone, "b.go", "package a\nfunc Changed() { println(\"clone only\") }\n")
	writeFlagFixture(t, clone, "new.go", "package a\nfunc New() {}\n")
	b := clonePathBackend{root: repo, commit: commit, nodes: []core.SymbolNode{
		{Service: "svc", Name: "Stable", Kind: core.KindGoFunc, FilePath: "a.go", Line: 2, EndLine: 2},
		{Service: "svc", Name: "Changed", Kind: core.KindGoFunc, FilePath: "b.go", Line: 2, EndLine: 2},
	}}
	s := NewServer(b)
	call := func(name string, args map[string]any) map[string]any {
		t.Helper()
		tool, ok := s.findTool(name)
		if !ok {
			t.Fatal(name)
		}
		raw, _ := json.Marshal(args)
		v, e := tool.handler(ctx, raw)
		if e != nil {
			t.Fatalf("%s: %+v", name, e)
		}
		data, _ := json.Marshal(v)
		var out map[string]any
		if err := json.Unmarshal(data, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	search := call("search", map[string]any{"root": clone, "pattern": "func"})
	if !strings.Contains(search["freshness_note"].(string), "clone at ") {
		t.Fatal(search)
	}
	for _, item := range search["hits"].([]any) {
		hit := item.(map[string]any)
		if strings.HasPrefix(hit["loc"].(string), "a.go:") {
			if hit["sym"] != "Stable" {
				t.Fatalf("unchanged receipt: %v", hit)
			}
		} else if hit["sym"] != nil {
			t.Fatalf("changed/new receipt: %v", hit)
		}
	}
	read := call("read_source", map[string]any{"service": "svc", "file": filepath.Join(clone, "b.go")})
	if !strings.Contains(read["lines"].([]any)[1].(map[string]any)["text"].(string), "clone only") || read["status"] != "unknown" || read["reason"] != "changed in clone" {
		t.Fatalf("clone read: %v", read)
	}
	for _, name := range []string{"find_symbol", "symbol_context"} {
		out := call(name, map[string]any{"service": "svc", "name": "Changed", "symbol": "Changed", "path": filepath.Join(clone, "b.go"), "snippet": false})
		if out["status"] != "unknown" || out["reason"] != "changed in clone" {
			t.Fatalf("%s: %v", name, out)
		}
	}
	out := call("find_symbol", map[string]any{"service": "svc", "name": "New", "path": filepath.Join(clone, "new.go")})
	if out["status"] != "unknown" {
		t.Fatalf("new symbol must be unknown: %v", out)
	}
	changedReach := call("blast_radius", map[string]any{"service": "svc", "path": clone, "changed_files": []string{"b.go"}})
	unknownSymbols, _ := changedReach["unknown_symbols"].([]any)
	if len(unknownSymbols) != 1 || unknownSymbols[0].(map[string]any)["reason"] != "changed in clone" {
		t.Fatalf("changed symbol reach must be marked: %v", changedReach)
	}
	// New clone reach is unknown, even when the base has no such file.
	reach := call("blast_radius", map[string]any{"service": "svc", "path": clone, "changed_files": []string{filepath.Join(clone, "new.go")}})
	if reach["status"] != "unknown" || reach["reason"] != "changed in clone" {
		t.Fatalf("new clone reach: %v", reach)
	}
	unknown := call("symbol_context", map[string]any{"service": "svc", "symbol": "New", "path": filepath.Join(clone, "new.go"), "snippet": false})
	if unknown["status"] != "unknown" || unknown["not_found"] == true {
		t.Fatalf("new symbol reported absent: %v", unknown)
	}
	if err := os.Remove(filepath.Join(clone, "b.go")); err != nil {
		t.Fatal(err)
	}
	deleted := call("read_source", map[string]any{"service": "svc", "file": filepath.Join(clone, "b.go")})
	if deleted["status"] != "unknown" {
		t.Fatalf("deleted clone file: %v", deleted)
	}
	writeFlagFixture(t, clone, "b.go", "package a\nfunc Changed() {}\n")
	// A changed checkout may not bypass the base service's exclusions.
	writeFlagFixture(t, repo, ".fathomignore", "b.go\n")
	tool, _ := s.findTool("read_source")
	raw, _ := json.Marshal(map[string]any{"service": "svc", "file": filepath.Join(clone, "b.go")})
	if _, e := tool.handler(ctx, raw); e == nil {
		t.Fatal("base exclusion bypassed through clone")
	}
	search = call("search", map[string]any{"root": clone, "pattern": "func"})
	for _, item := range search["hits"].([]any) {
		if strings.HasPrefix(item.(map[string]any)["loc"].(string), "b.go:") {
			t.Fatal("base search exclusion bypassed")
		}
	}
	raw, _ = json.Marshal(map[string]any{"service": "svc", "file": filepath.Join(clone, "b.go"), "ref": commit})
	if _, e := tool.handler(ctx, raw); e == nil {
		t.Fatal("base exclusion bypassed at a clone ref")
	}

}

func TestCloneReceiptsCheckStoredFingerprint(t *testing.T) {
	ctx := context.Background()
	c, repo := citationFixture(t)
	base := t.TempDir()
	clone := filepath.Join(base, "clone")
	cloneGit(t, base, "clone", "-q", repo, clone)
	server := NewServer(c)
	tool, _ := server.findTool("search")
	search := func() map[string]any {
		t.Helper()
		args, _ := json.Marshal(map[string]any{"root": clone, "pattern": "Alpha"})
		out, e := tool.handler(ctx, args)
		if e != nil {
			t.Fatalf("search: %+v", e)
		}
		raw, _ := json.Marshal(out)
		var result map[string]any
		_ = json.Unmarshal(raw, &result)
		return result
	}
	result := search()
	hits := result["hits"].([]any)
	if len(hits) != 1 || hits[0].(map[string]any)["sym"] != "Alpha" {
		t.Fatalf("indexed receipt: %v", result)
	}
	// A dirty ingest may change indexed bytes without changing its commit.
	// Equality to the commit alone must not attach a receipt for old bytes.
	writeFlagFixture(t, repo, "sample.go", "package sample\n\nfunc Alpha() { println(2) }\n")
	if err := c.IngestServiceAt(ctx, "sample", repo); err != nil {
		t.Fatal(err)
	}
	result = search()
	hits = result["hits"].([]any)
	if len(hits) != 1 || hits[0].(map[string]any)["sym"] != nil {
		t.Fatalf("mismatched stored fingerprint got a receipt: %v", result)
	}
}
