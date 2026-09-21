package mcp

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestPatchPackageGraph_KeepsMemoFreshWithoutRebuild (SU-4): after a
// package's nodes are patched, the SAME (service, watermark) key is still a
// cache hit — no rebuild — and the graph served carries the new symbol, not
// the pre-refresh one. The resolved-only graph is dropped and rebuilds.
func TestPatchPackageGraph_KeepsMemoFreshWithoutRebuild(t *testing.T) {
	s := &Server{graphCache: callgraph.NewCachedCallGraph(0)}
	old := []core.SymbolNode{{Service: "svc", Package: "main", Name: "Original", FilePath: "target.go", Line: 3}}
	builds := 0
	build := func() *callgraph.Graph { builds++; return callgraph.BuildBoth(old, nil) }
	if g := s.graphCache.Graph(graphKeyBoth("svc"), "w1", build); g == nil || builds != 1 {
		t.Fatalf("priming build: builds=%d", builds)
	}
	s.graphCache.Graph(graphKeyResolved("svc"), "w1", func() *callgraph.Graph { return callgraph.BuildResolved(old, nil) })
	if !s.HasCachedGraph("svc") {
		t.Fatal("HasCachedGraph = false after priming")
	}

	fresh := []core.SymbolNode{{Service: "svc", Package: "main", Name: "Changed", FilePath: "target.go", Line: 3}}
	s.PatchPackageGraph("svc", "main", fresh, nil)

	g := s.graphCache.Graph(graphKeyBoth("svc"), "w1", build)
	if builds != 1 {
		t.Fatalf("the patched key rebuilt: builds=%d, want 1", builds)
	}
	// A node trivially reaches itself, so Reachable(k, k) is "k exists".
	hasChanged := callgraph.Reachable(g, fresh[0].Key(), fresh[0].Key())
	hasOriginal := callgraph.Reachable(g, old[0].Key(), old[0].Key())
	if !hasChanged || hasOriginal {
		t.Fatalf("patched graph serves Changed=%v Original=%v, want true/false", hasChanged, hasOriginal)
	}
	if s.graphCache.Has(graphKeyResolved("svc")) {
		t.Fatal("resolved-only graph should be dropped, not patched")
	}

	s.DropCachedGraphs("svc")
	if s.HasCachedGraph("svc") {
		t.Fatal("HasCachedGraph = true after DropCachedGraphs")
	}
}
