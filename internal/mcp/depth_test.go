package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memo"
)

// depthFixture: Target ← Direct1 (syntax) ← Deep (syntax) ← TestDeep;
// Target ← Direct2 (syntax edge ALSO confirmed by a typed EdgeDirect).
func depthFixture() ([]core.SymbolNode, []core.Edge) {
	n := func(name, file string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "pkg", Name: name, Kind: core.KindGoFunc, FilePath: file, Line: 1, Calls: calls}
	}
	nodes := []core.SymbolNode{
		n("Target", "a.go"),
		n("Direct1", "b.go", "svc::pkg.Target"),
		n("Direct2", "b.go", "svc::pkg.Target"),
		n("Deep", "c.go", "svc::pkg.Direct1"),
		n("TestDeep", "c_test.go", "svc::pkg.Deep"),
	}
	typed := []core.Edge{{FromService: "svc", FromSymbol: "svc::pkg.Direct2", ToSymbol: "svc::pkg.Target", Kind: core.EdgeDirect}}
	return nodes, typed
}

func TestCallerDepthsAndBuckets(t *testing.T) {
	nodes, typed := depthFixture()
	g := callgraph.Build(nodes)
	callgraph.AddTypedEdges(g, typed)

	got := callerDepths(g, "svc::pkg.Target", 0)
	want := map[string]depthInfo{
		"svc::pkg.Direct1":  {1, core.EdgeConfidenceHeuristic},
		"svc::pkg.Direct2":  {1, core.EdgeConfidenceResolved},
		"svc::pkg.Deep":     {2, core.EdgeConfidenceHeuristic},
		"svc::pkg.TestDeep": {3, core.EdgeConfidenceHeuristic},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("callerDepths = %v, want %v", got, want)
	}
	if bounded := callerDepths(g, "Target", 1); len(bounded) != 2 {
		t.Fatalf("callerDepths(maxDepth=1) = %v, want the two direct callers", bounded)
	}
	if callerDepths(g, "NoSuchSymbol", 0) != nil {
		t.Fatal("callerDepths on an unknown symbol must be nil")
	}

	rows := sortedDepthRows(got)
	buckets := withDepthBuckets([]depthInfo{got[rows[0].Key], got[rows[1].Key], got[rows[2].Key], got[rows[3].Key]})
	wantBuckets := []depthBucket{{1, 2, 1, 1}, {2, 1, 0, 1}, {3, 1, 0, 1}}
	if !reflect.DeepEqual(buckets, wantBuckets) {
		t.Fatalf("withDepthBuckets = %+v, want %+v", buckets, wantBuckets)
	}
	if withDepthBuckets(nil) == nil {
		t.Fatal("withDepthBuckets(nil) must be an empty slice, never nil (marshals as [])")
	}
	if rows[0].Key != "svc::pkg.Direct1" || rows[1].Key != "svc::pkg.Direct2" || rows[3].Key != "svc::pkg.TestDeep" {
		t.Fatalf("sortedDepthRows order = %+v (want depth, then key)", rows)
	}
}

// bothGraphOf builds nodes' syntax+typed graph through the graph store,
// unscoped (never cached): the graph a service's snapshot would get.
func bothGraphOf(t *testing.T, nodes []core.SymbolNode, typed []core.Edge) *callgraph.Graph {
	t.Helper()
	g, err := memo.GraphFor(context.Background(), memo.NewSnapshot(nodes), memo.GraphBoth, func(context.Context) ([]core.Edge, error) { return typed, nil })
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestBlastRadiusByDepth(t *testing.T) {
	nodes, typed := depthFixture()
	g := bothGraphOf(t, nodes, typed)
	buckets := blastRadiusByDepth(g, nodes, []string{"a.go"})
	if len(buckets) != 4 {
		t.Fatalf("buckets = %+v, want depths 0..3", buckets)
	}
	if buckets[0].Depth != 0 || buckets[0].Count != 1 || buckets[0].Symbols[0].Key != "svc::pkg.Target" || buckets[0].Symbols[0].EdgeConfidence != "" {
		t.Errorf("depth-0 bucket = %+v: want the changed symbol with no edge_confidence", buckets[0])
	}
	if buckets[1].Depth != 1 || buckets[1].Resolved != 1 || buckets[1].Heuristic != 1 {
		t.Errorf("depth-1 bucket = %+v: want one resolved (typed) and one heuristic caller", buckets[1])
	}
	if buckets[1].Symbols[0].Key != "svc::pkg.Direct1" || buckets[1].Symbols[1].EdgeConfidence != "resolved" {
		t.Errorf("depth-1 symbols = %+v: want sorted by key, Direct2 resolved", buckets[1].Symbols)
	}

	// The union of the buckets is exactly the flat blast radius the tool
	// already reported — by_depth regroups, it never adds or drops.
	flat := impact.BlastRadiusWithTypedEdges(nodes, []string{"a.go"}, typed).Symbols
	var union []string
	for _, b := range buckets {
		for _, s := range b.Symbols {
			union = append(union, s.Key)
		}
	}
	if len(union) != len(flat) {
		t.Fatalf("by_depth union %v != blast radius %v", union, flat)
	}

	// Determinism: two runs marshal byte-identically.
	a, _ := json.Marshal(buckets)
	b, _ := json.Marshal(blastRadiusByDepth(bothGraphOf(t, nodes, typed), nodes, []string{"a.go"}))
	if string(a) != string(b) {
		t.Fatalf("blastRadiusByDepth is not deterministic:\n%s\n%s", a, b)
	}
	if empty := blastRadiusByDepth(bothGraphOf(t, nodes, nil), nodes, []string{"nope.go"}); len(empty) != 0 || empty == nil {
		t.Fatalf("no changed symbols must yield an empty, non-nil slice, got %#v", empty)
	}
}
