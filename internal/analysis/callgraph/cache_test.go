package callgraph

import (
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func cacheNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "s", Package: "p", Name: "A", Kind: core.KindGoFunc, FilePath: "p/a.go", Calls: []string{"B"}},
		{Service: "s", Package: "p", Name: "B", Kind: core.KindGoFunc, FilePath: "p/b.go"},
	}
}

// TestCachedCallGraphMemoizesPerWatermark pins BP-28's whole point: the
// same (service, watermark) builds once.
func TestCachedCallGraphMemoizesPerWatermark(t *testing.T) {
	c := NewCachedCallGraph(0)
	builds := 0
	build := func() *Graph { builds++; return Build(cacheNodes()) }

	g1 := c.Graph("svc", "abc@1", build)
	g2 := c.Graph("svc", "abc@1", build)
	if builds != 1 {
		t.Fatalf("built %d times, want 1", builds)
	}
	if g1 != g2 {
		t.Fatal("second call returned a different graph; the memo did not hit")
	}
	if _, hits, _ := c.Stats(); hits != 1 {
		t.Errorf("hits = %d, want 1", hits)
	}
}

// TestCachedCallGraphInvalidatesOnReIngest is I21's enforcement: a new
// watermark for the same service must rebuild AND must not leave the
// previous ingest's graph reachable.
func TestCachedCallGraphInvalidatesOnReIngest(t *testing.T) {
	c := NewCachedCallGraph(0)
	builds := 0
	build := func() *Graph { builds++; return Build(cacheNodes()) }

	c.Graph("svc", "abc@1", build)
	c.Graph("svc", "def@2", build) // re-ingest: new watermark
	if builds != 2 {
		t.Fatalf("built %d times across two watermarks, want 2", builds)
	}
	if entries, _, _ := c.Stats(); entries != 1 {
		t.Fatalf("cache holds %d entries after a re-ingest, want 1 — the stale graph survived", entries)
	}
	// Asking for the OLD watermark must not be a hit.
	c.Graph("svc", "abc@1", build)
	if builds != 3 {
		t.Errorf("the pre-ingest watermark was served from cache (builds = %d, want 3)", builds)
	}
}

// TestCachedCallGraphEmptyWatermarkNeverCaches: a caller that cannot name
// the ingest gets a fresh graph every time rather than a key that might
// later be wrong.
func TestCachedCallGraphEmptyWatermarkNeverCaches(t *testing.T) {
	c := NewCachedCallGraph(0)
	builds := 0
	build := func() *Graph { builds++; return Build(cacheNodes()) }
	c.Graph("svc", "", build)
	c.Graph("svc", "", build)
	if builds != 2 {
		t.Errorf("builds = %d, want 2 — an empty watermark must not be a cache key", builds)
	}
	if entries, _, _ := c.Stats(); entries != 0 {
		t.Errorf("cache holds %d entries, want 0", entries)
	}
}

// TestCachedCallGraphBoundedLRU: the cache is bounded, so a fleet-wide
// sweep cannot pin every service's graph in the daemon.
func TestCachedCallGraphBoundedLRU(t *testing.T) {
	c := NewCachedCallGraph(2)
	build := func() *Graph { return Build(cacheNodes()) }
	c.Graph("a", "w", build)
	c.Graph("b", "w", build)
	c.Graph("c", "w", build)
	if entries, _, _ := c.Stats(); entries != 2 {
		t.Errorf("entries = %d, want 2 (capacity)", entries)
	}
}

func TestCachedCallGraphConcurrent(t *testing.T) {
	c := NewCachedCallGraph(0)
	build := func() *Graph { return Build(cacheNodes()) }
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = c.Graph("svc", "w1", build) }()
	}
	wg.Wait()
	if entries, _, _ := c.Stats(); entries != 1 {
		t.Errorf("entries = %d, want 1", entries)
	}
}

// TestBuildResolvedCarriesOnlyTypedEdges pins BP-26/BP-27's evidence
// boundary: the resolved graph must not carry the syntax pass's edges.
func TestBuildResolvedCarriesOnlyTypedEdges(t *testing.T) {
	nodes := cacheNodes()
	syntax := Build(nodes)
	if len(CallersDetail(syntax, "B")) != 1 {
		t.Fatalf("fixture: syntax graph should give B one caller, got %d", len(CallersDetail(syntax, "B")))
	}
	resolved := BuildResolved(nodes, nil)
	if got := CallersDetail(resolved, "B"); len(got) != 0 {
		t.Errorf("BuildResolved carried %d syntax edges, want 0: %+v", len(got), got)
	}
	if resolved.HasEdges() {
		t.Error("HasEdges() true on a graph with no typed edges — the resolved-by-default guard would answer 'no callers' for a non-Go service")
	}

	byName := map[string]string{}
	for _, n := range nodes {
		byName[n.Name] = n.Key()
	}
	key := func(name string) string { return byName[name] }
	withTyped := BuildResolved(nodes, []core.Edge{
		{FromSymbol: key("A"), ToSymbol: key("B"), Kind: core.EdgeDirect},
		// An unresolved edge is heuristic and must be dropped.
		{FromSymbol: key("A"), ToSymbol: key("B"), Kind: core.EdgeUnresolved},
	})
	if !withTyped.HasEdges() {
		t.Fatal("HasEdges() false after a resolved typed edge")
	}
	got := ResolvedCallers(withTyped, "B")
	if len(got) != 1 || got[0].Kind != core.EdgeDirect {
		t.Errorf("ResolvedCallers = %+v, want one EdgeDirect row", got)
	}
}
