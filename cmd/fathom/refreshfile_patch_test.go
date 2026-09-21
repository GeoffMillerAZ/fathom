package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// recordingPatcher stands in for the MCP server's graph memo.
type recordingPatcher struct {
	has     bool
	patched []string // packages patched, in order
	nodes   []core.SymbolNode
	dropped int
}

func (p *recordingPatcher) HasCachedGraph(string) bool { return p.has }
func (p *recordingPatcher) PatchPackageGraph(_, pkg string, nodes []core.SymbolNode, _ []core.Edge) {
	p.patched = append(p.patched, pkg)
	p.nodes = append(p.nodes, nodes...)
}
func (p *recordingPatcher) DropCachedGraphs(string) { p.dropped++ }

// TestRefreshFile_PatchesCachedGraph (SU-4): a refreshed file patches the
// memoized graph for its package with the package's CURRENT nodes — the
// changed symbol in, the old one out — only when a graph is memoized at all;
// a file that now defines nothing drops the memo instead of guessing.
func TestRefreshFile_PatchesCachedGraph(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package main\n\nfunc Original() int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := client.New(connector.NewLocalConnector(), store.NewMemoryStore())
	t.Cleanup(func() { _ = c.Close() })
	ctx := context.Background()
	if _, err := c.Ingest(ctx, client.ServiceRef{ID: "refresh-svc", Path: dir}); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	// Memo present: the package is patched with the fresh node set.
	if err := os.WriteFile(target, []byte("package main\n\nfunc Changed() int { return 2 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := &recordingPatcher{has: true}
	out := buildRefreshFileResponse(ctx, c, p, "refresh-svc", "target.go")
	if !out.Refreshed || !out.GraphPatched {
		t.Fatalf("response = %+v, want Refreshed and GraphPatched", out)
	}
	if len(p.patched) != 1 || p.patched[0] != "main" {
		t.Fatalf("patched packages = %v, want [main]", p.patched)
	}
	var sawChanged, sawOriginal bool
	for _, n := range p.nodes {
		sawChanged = sawChanged || n.Name == "Changed"
		sawOriginal = sawOriginal || n.Name == "Original"
	}
	if !sawChanged || sawOriginal {
		t.Fatalf("patched nodes Changed=%v Original=%v, want true/false", sawChanged, sawOriginal)
	}

	// No memo: nothing is listed or patched, and the response says so.
	p2 := &recordingPatcher{has: false}
	out2 := buildRefreshFileResponse(ctx, c, p2, "refresh-svc", "target.go")
	if out2.GraphPatched || len(p2.patched) != 0 || p2.dropped != 0 {
		t.Fatalf("no-memo refresh patched anyway: %+v %+v", out2, p2)
	}

	// The file now defines nothing: no package to patch, memo dropped.
	if err := os.WriteFile(target, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p3 := &recordingPatcher{has: true}
	out3 := buildRefreshFileResponse(ctx, c, p3, "refresh-svc", "target.go")
	if out3.GraphPatched || p3.dropped != 1 || len(p3.patched) != 0 {
		t.Fatalf("empty refresh: response=%+v patcher=%+v, want dropped once and nothing patched", out3, p3)
	}
}
