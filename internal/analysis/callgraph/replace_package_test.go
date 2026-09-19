package callgraph

// replace_package_test.go — SU-4 (docs/plans/substrate.md §A, ADR-041
// decision 3): a one-package change (typed edges reloaded, or SU-5's
// one-file re-ingest) must not force a full Build of a warm graph, and the
// patched graph must still answer "who calls this" correctly for BOTH the
// changed package and everything that calls into it.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func replaceFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkgA", Name: "A1", Kind: core.KindGoFunc, FilePath: "pkgA/a1.go", Calls: []string{"A2"}},
		{Service: "svc", Package: "pkgA", Name: "A2", Kind: core.KindGoFunc, FilePath: "pkgA/a2.go"},
		{Service: "svc", Package: "pkgB", Name: "B1", Kind: core.KindGoFunc, FilePath: "pkgB/b.go", Calls: []string{"pkgA.A1"}},
	}
}

func replaceFixtureKey(service, pkg, name string) string {
	// Matches core.SymbolNode.Key()'s own QualifiedPackage rule for the
	// "<dir>/<pkg>" shape these fixtures use (FilePath "<pkg>/<file>.go").
	return core.SymbolKey(service, pkg+"/"+pkg, name)
}

// TestInvalidatePackage_PatchesWithoutRebuild is SU-4's headline proof:
// after a one-package change, a query at the SAME watermark neither
// rebuilds the whole graph NOR keeps serving the stale pre-change edges —
// exactly the gap a plain Invalidate(service) (which forces the NEXT query
// to rebuild) does not fill.
func TestInvalidatePackage_PatchesWithoutRebuild(t *testing.T) {
	c := NewCachedCallGraph(0)
	builds := 0
	build := func() *Graph { builds++; return Build(replaceFixtureNodes()) }

	g1 := c.Graph("svc", "wm1", build)
	if builds != 1 {
		t.Fatalf("initial build count = %d, want 1", builds)
	}
	a1Key := replaceFixtureKey("svc", "pkgA", "A1")
	a2Key := replaceFixtureKey("svc", "pkgA", "A2")
	b1Key := replaceFixtureKey("svc", "pkgB", "B1")
	if !containsStr(g1.callees[a1Key], a2Key) {
		t.Fatalf("pre-patch: A1 does not call A2: %v", g1.callees[a1Key])
	}

	// Simulate a one-file change to pkgA: A1 now calls a NEW function A3
	// instead of A2; A2 is gone.
	newPkgA := []core.SymbolNode{
		{Service: "svc", Package: "pkgA", Name: "A1", Kind: core.KindGoFunc, FilePath: "pkgA/a1.go", Calls: []string{"A3"}},
		{Service: "svc", Package: "pkgA", Name: "A3", Kind: core.KindGoFunc, FilePath: "pkgA/a1.go"},
	}
	c.InvalidatePackage("svc", "pkgA", newPkgA, nil)

	g2 := c.Graph("svc", "wm1", build) // SAME watermark as before
	if builds != 1 {
		t.Fatalf("build count after InvalidatePackage = %d, want 1 (still 1 — no full rebuild)", builds)
	}
	if g2 == g1 {
		t.Fatal("Graph() returned the SAME *Graph after InvalidatePackage — the cache entry was not patched")
	}

	// g1 (still held by whoever queried it before the patch) must be
	// completely unaffected — copy-on-write, never a shared mutation.
	if !containsStr(g1.callees[a1Key], a2Key) {
		t.Errorf("the PRE-patch graph g1 changed after InvalidatePackage — it must stay exactly as it was: %v", g1.callees[a1Key])
	}

	a3Key := replaceFixtureKey("svc", "pkgA", "A3")
	if _, ok := g2.nodes[a2Key]; ok {
		t.Errorf("g2 still has A2 (%s) after it was replaced out of pkgA", a2Key)
	}
	if !containsStr(g2.callees[a1Key], a3Key) {
		t.Errorf("g2: A1 does not call A3 after the patch: %v", g2.callees[a1Key])
	}
	if containsStr(g2.callees[a1Key], a2Key) {
		t.Errorf("g2: A1 still calls the removed A2: %v", g2.callees[a1Key])
	}

	// "returns identical callers": pkgB's B1 -> pkgA.A1 edge (untouched by
	// the patch) must survive correctly in g2, in both directions.
	if !containsStr(g2.callees[b1Key], a1Key) {
		t.Errorf("g2: B1 no longer calls A1 after patching an unrelated symbol in the SAME package: %v", g2.callees[b1Key])
	}
	if !containsStr(g2.callers[a1Key], b1Key) {
		t.Errorf("g2: A1's callers no longer include B1: %v", g2.callers[a1Key])
	}
}

// TestInvalidatePackage_NoEntryIsNoOp: patching a service with nothing
// cached yet must not panic and must not fabricate an entry.
func TestInvalidatePackage_NoEntryIsNoOp(t *testing.T) {
	c := NewCachedCallGraph(0)
	c.InvalidatePackage("svc", "pkgA", replaceFixtureNodes(), nil)
	if entries, _, _ := c.Stats(); entries != 0 {
		t.Fatalf("InvalidatePackage on an empty cache created %d entries, want 0", entries)
	}
}

// TestInvalidatePackage_TypedEdgesOnly covers the internal/fusion/
// typedcalls.go wiring shape: newNodes is nil (symbols unchanged), only
// typedEdges replace pkgA's typed edges.
func TestInvalidatePackage_TypedEdgesOnly(t *testing.T) {
	c := NewCachedCallGraph(0)
	build := func() *Graph { return Build(replaceFixtureNodes()) }
	c.Graph("svc", "wm1", build)

	a1Key := replaceFixtureKey("svc", "pkgA", "A1")
	a2Key := replaceFixtureKey("svc", "pkgA", "A2")
	c.InvalidatePackage("svc", "pkgA", nil, []core.Edge{
		{FromSymbol: a1Key, ToSymbol: a2Key, Kind: core.EdgeDirect},
	})

	g2 := c.Graph("svc", "wm1", build)
	if g2.edgeKind[a1Key+"\x00"+a2Key] != core.EdgeDirect {
		t.Errorf("typed edge A1->A2 not recorded after typed-edges-only InvalidatePackage")
	}
}
