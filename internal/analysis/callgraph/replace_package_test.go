package callgraph

// replace_package_test.go — SU-4 (docs/plans/substrate.md §A, ADR-041
// decision 3): a one-package change (typed edges reloaded, or SU-5's
// one-file re-ingest) must not force a full Build of a warm graph, and the
// patched graph must still answer "who calls this" correctly for BOTH the
// changed package and everything that calls into it.

import (
	"slices"
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

// A refresh of either file declaring Config replays the whole owner package.
// The field remains queryable by its full key but cannot hide the method from
// bare-name or package-name call resolution.
func TestReplacePackageGoFieldCollisionMatchesBuild(t *testing.T) {
	method := core.SymbolNode{Service: "svc", Package: "Config", Name: "Config.Load", Kind: core.KindGoMethod, FilePath: "a/config.go"}
	field := core.SymbolNode{Service: "svc", Package: "Config", Name: "Config.Load", Kind: core.KindGoField, FilePath: "b/config.go"}
	caller := core.SymbolNode{Service: "svc", Package: "app", Name: "Call", Kind: core.KindGoFunc, FilePath: "app/main.go", Calls: []string{"Config.Load"}}
	nodes := []core.SymbolNode{method, field, caller}
	fresh := Build(nodes)
	patched := fresh.ReplacePackage("Config", []core.SymbolNode{method, field}, nil)
	for _, name := range []string{"Config.Load", "Config.Config.Load"} {
		want := Canonical(fresh, name)
		if want != method.Key() {
			t.Fatalf("Build canonical %q = %q, want method %q", name, want, method.Key())
		}
		if got := Canonical(patched, name); got != want {
			t.Errorf("patched canonical %q = %q, Build = %q", name, got, want)
		}
	}
	if got := Canonical(patched, field.Key()); got != field.Key() {
		t.Errorf("field full-key lookup = %q, want %q", got, field.Key())
	}
	if got, want := patched.callees[caller.Key()], fresh.callees[caller.Key()]; !slices.Equal(got, want) {
		t.Errorf("patched call edges = %v, Build = %v", got, want)
	}
}

func TestReplacePackageGoFieldNeverSyntaxCallTarget(t *testing.T) {
	field := core.SymbolNode{Service: "svc", Package: "Owner", Name: "Owner.Run", Kind: core.KindGoField, FilePath: "pkg/owner.go"}
	caller := core.SymbolNode{Service: "svc", Package: "sample", Name: "Call", Kind: core.KindGoFunc, FilePath: "pkg/call.go", Calls: []string{"Run", "x.Run", "sample.Owner.Run", field.Key(), "example.com/pkg#.Run"}}
	nodes := []core.SymbolNode{field, caller}
	fresh := Build(nodes)
	patched := fresh.ReplacePackage("sample", []core.SymbolNode{caller}, nil)
	if got, want := patched.EdgeCount(), fresh.EdgeCount(); got != want {
		t.Errorf("patched edge count = %d, Build = %d", got, want)
	}
	if got := patched.EdgeCount(); got != 0 {
		t.Errorf("field fabricated %d call edges after refresh", got)
	}
	patched = fresh.ReplacePackage("Owner", []core.SymbolNode{field}, nil)
	if key, ok := patched.byPkgName["Owner.Owner.Run"]; ok {
		t.Errorf("field entered package call index after refresh: %q", key)
	}
	if key, ok := patched.byName["Owner.Run"]; ok {
		t.Errorf("field entered bare-name call index after refresh: %q", key)
	}
}

func TestReplacePackageMethodFieldKindTransitionMatchesBuild(t *testing.T) {
	method := core.SymbolNode{Service: "svc", Package: "Owner", Name: "Owner.Run", Kind: core.KindGoMethod, FilePath: "pkg/owner.go"}
	field := method
	field.Kind = core.KindGoField // same full key, different call-target class
	localCaller := core.SymbolNode{Service: "svc", Package: "Owner", Name: "Call", Kind: core.KindGoFunc, FilePath: "pkg/call.go", Calls: []string{"x.Run"}}
	otherCaller := core.SymbolNode{Service: "svc", Package: "sample", Name: "Other", Kind: core.KindGoFunc, FilePath: "sample/call.go", Calls: []string{method.Key()}}

	methodNodes := []core.SymbolNode{method, localCaller, otherCaller}
	fieldNodes := []core.SymbolNode{field, localCaller, otherCaller}
	methodGraph := Build(methodNodes)
	fieldGraph := Build(fieldNodes)
	if methodGraph.EdgeCount() != 2 || fieldGraph.EdgeCount() != 0 {
		t.Fatalf("fixture edges: method = %d, field = %d; want 2 and 0", methodGraph.EdgeCount(), fieldGraph.EdgeCount())
	}

	toField := methodGraph.ReplacePackage("Owner", []core.SymbolNode{field, localCaller}, nil)
	if got, want := toField.EdgeCount(), fieldGraph.EdgeCount(); got != want {
		t.Errorf("method to field edges = %d, Build = %d", got, want)
	}
	for _, name := range []string{"Owner.Run", "Owner.Owner.Run", field.Key()} {
		if got, want := Canonical(toField, name), Canonical(fieldGraph, name); got != want {
			t.Errorf("method to field canonical %q = %q, Build = %q", name, got, want)
		}
	}
	if got, want := toField.callees[otherCaller.Key()], fieldGraph.callees[otherCaller.Key()]; !slices.Equal(got, want) {
		t.Errorf("method to field other-package calls = %v, Build = %v", got, want)
	}
	if key, ok := toField.bySuffix["Run"]; ok && key.key == field.Key() {
		t.Errorf("field retained suffix call alias %q", key.key)
	}

	// A caller inside the patched package is re-resolved when the field
	// becomes a method again. Other packages' newly resolvable calls follow
	// ReplacePackage's documented bounded-staleness contract.
	reverseBefore := Build([]core.SymbolNode{field, localCaller})
	reverseFresh := Build([]core.SymbolNode{method, localCaller})
	toMethod := reverseBefore.ReplacePackage("Owner", []core.SymbolNode{method, localCaller}, nil)
	if got, want := toMethod.EdgeCount(), reverseFresh.EdgeCount(); got != want {
		t.Errorf("field to method edges = %d, Build = %d", got, want)
	}
	for _, name := range []string{"Owner.Run", "Owner.Owner.Run", method.Key()} {
		if got, want := Canonical(toMethod, name), Canonical(reverseFresh, name); got != want {
			t.Errorf("field to method canonical %q = %q, Build = %q", name, got, want)
		}
	}
}
