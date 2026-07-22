package callgraph_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// dispatchFixture builds a graph mirroring the ADR-021 §3 O9 residue shape:
// TestRunAdapter -> RunAdapter (syntax-derived direct edge) plus a typed
// interface-dispatch fan-out RunAdapter -> {AdapterA.Complete, AdapterB.Complete}
// that the OLD syntax-only resolver could never emit (an ambiguous method
// name shared by two receivers refuses, per graph.go:218-240).
func dispatchFixture() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "TestRunAdapter", Kind: core.KindGoFunc, Calls: []string{"svc::pkg.RunAdapter"}},
		{Service: "svc", Package: "pkg", Name: "RunAdapter", Kind: core.KindGoFunc},
		{Service: "svc", Package: "AdapterA", Name: "AdapterA.Complete", Kind: core.KindGoMethod},
		{Service: "svc", Package: "AdapterB", Name: "AdapterB.Complete", Kind: core.KindGoMethod},
		{Service: "svc", Package: "pkg", Name: "Orphan", Kind: core.KindGoFunc}, // untouched by typed edges
	}
}

func TestAddTypedEdges_DispatchReachability(t *testing.T) {
	g := callgraph.Build(dispatchFixture())

	// Before typed edges: RunAdapter has no outgoing edges at all (the
	// ambiguous "Complete" call was never resolved by the syntax pass).
	if reached := callgraph.TestImpact(g, "svc::AdapterA.AdapterA.Complete"); len(reached) != 0 {
		t.Fatalf("before AddTypedEdges: TestImpact(AdapterA.Complete) = %v, want empty", reached)
	}

	typed := []core.Edge{
		{FromService: "svc", FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::AdapterA.AdapterA.Complete", Kind: core.EdgeIfaceDispatch},
		{FromService: "svc", FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::AdapterB.AdapterB.Complete", Kind: core.EdgeIfaceDispatch},
	}
	callgraph.AddTypedEdges(g, typed)

	// After typed edges: TestRunAdapter reaches BOTH implementers.
	for _, target := range []string{"svc::AdapterA.AdapterA.Complete", "svc::AdapterB.AdapterB.Complete"} {
		reached := callgraph.TestImpact(g, target)
		if len(reached) != 1 || reached[0] != "svc::pkg.TestRunAdapter" {
			t.Errorf("TestImpact(%s) = %v, want [svc::pkg.TestRunAdapter]", target, reached)
		}
	}

	// Reachable() also picks up the typed edges (RunAdapter -> AdapterA.Complete directly).
	if !callgraph.Reachable(g, "svc::pkg.RunAdapter", "svc::AdapterA.AdapterA.Complete") {
		t.Error("Reachable(RunAdapter, AdapterA.Complete) = false, want true")
	}

	// TestImpactDetail reports the dispatch modality and implementer count.
	details := callgraph.TestImpactDetail(g, "svc::AdapterA.AdapterA.Complete")
	if len(details) != 1 {
		t.Fatalf("TestImpactDetail = %d entries, want 1: %+v", len(details), details)
	}
	d := details[0]
	if d.Key != "svc::pkg.TestRunAdapter" {
		t.Errorf("detail.Key = %q, want svc::pkg.TestRunAdapter", d.Key)
	}
	if !d.ViaDispatch {
		t.Error("detail.ViaDispatch = false, want true (path crosses RunAdapter's dispatch edge)")
	}
	if d.Implementers != 2 {
		t.Errorf("detail.Implementers = %d, want 2 (AdapterA + AdapterB)", d.Implementers)
	}

	// A plain (non-dispatch) reach still reports ViaDispatch=false.
	directDetails := callgraph.TestImpactDetail(g, "svc::pkg.RunAdapter")
	if len(directDetails) != 1 || directDetails[0].ViaDispatch {
		t.Errorf("TestImpactDetail(RunAdapter) = %+v, want one non-dispatch entry", directDetails)
	}

	// Orphan is untouched: no typed edges reference it.
	if reached := callgraph.TestImpact(g, "svc::pkg.Orphan"); len(reached) != 0 {
		t.Errorf("TestImpact(Orphan) = %v, want empty", reached)
	}
}

// TestAddTypedEdges_SkipsUnknownEndpoints confirms AddTypedEdges never
// invents a graph vertex: an edge referencing a symbol absent from the
// syntax-built node set (stale row, or an EdgeUnresolved callsite marker) is
// silently dropped, never added to the adjacency lists.
func TestAddTypedEdges_SkipsUnknownEndpoints(t *testing.T) {
	g := callgraph.Build(dispatchFixture())
	before := g.EdgeCount()

	callgraph.AddTypedEdges(g, []core.Edge{
		{FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::pkg.DoesNotExist", Kind: core.EdgeDirect},
		{FromSymbol: "svc::pkg.DoesNotExist", ToSymbol: "svc::pkg.RunAdapter", Kind: core.EdgeDirect},
		{FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "<unresolved:42>", Kind: core.EdgeUnresolved},
		{FromSymbol: "", ToSymbol: "svc::pkg.RunAdapter", Kind: core.EdgeDirect},
	})

	if after := g.EdgeCount(); after != before {
		t.Errorf("EdgeCount changed from %d to %d; every typed edge above should have been skipped", before, after)
	}
}

// TestAddTypedEdges_DedupesAgainstSyntaxEdge confirms a typed edge that
// duplicates an already-resolved syntax edge doesn't inflate the adjacency
// list.
func TestAddTypedEdges_DedupesAgainstSyntaxEdge(t *testing.T) {
	g := callgraph.Build(dispatchFixture())
	before := g.EdgeCount()

	// TestRunAdapter -> RunAdapter is ALREADY a syntax edge (dispatchFixture's Calls).
	callgraph.AddTypedEdges(g, []core.Edge{
		{FromSymbol: "svc::pkg.TestRunAdapter", ToSymbol: "svc::pkg.RunAdapter", Kind: core.EdgeDirect},
	})

	if after := g.EdgeCount(); after != before {
		t.Errorf("EdgeCount changed from %d to %d; a duplicate (caller,callee) pair must not be re-added", before, after)
	}
}

// TestCallersDetail_KindLabeled is the CT-6 MCP-surfacing building block:
// CallersDetail/CalleesDetail label each edge with its typed Kind (or ""
// for syntax-only), letting tool handlers distinguish "reached, but how" at
// a glance.
func TestCallersDetail_KindLabeled(t *testing.T) {
	g := callgraph.Build(dispatchFixture())
	callgraph.AddTypedEdges(g, []core.Edge{
		{FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::AdapterA.AdapterA.Complete", Kind: core.EdgeIfaceDispatch},
		{FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::AdapterB.AdapterB.Complete", Kind: core.EdgeIfaceDispatch},
	})

	callers := callgraph.CallersDetail(g, "svc::AdapterA.AdapterA.Complete")
	if len(callers) != 1 || callers[0].Key != "svc::pkg.RunAdapter" || callers[0].Kind != core.EdgeIfaceDispatch {
		t.Errorf("CallersDetail(AdapterA.Complete) = %+v, want one EdgeIfaceDispatch entry from RunAdapter", callers)
	}

	callees := callgraph.CalleesDetail(g, "svc::pkg.RunAdapter")
	if len(callees) != 2 {
		t.Fatalf("CalleesDetail(RunAdapter) = %d entries, want 2", len(callees))
	}
	for _, c := range callees {
		if c.Kind != core.EdgeIfaceDispatch {
			t.Errorf("CalleesDetail(RunAdapter) entry %+v: Kind = %q, want %q", c, c.Kind, core.EdgeIfaceDispatch)
		}
	}

	// The pre-existing syntax edge (TestRunAdapter -> RunAdapter) has no
	// typed confirmation — Kind must be "".
	syntaxCallers := callgraph.CallersDetail(g, "svc::pkg.RunAdapter")
	if len(syntaxCallers) != 1 || syntaxCallers[0].Kind != "" {
		t.Errorf("CallersDetail(RunAdapter) = %+v, want one syntax-only (Kind=\"\") entry", syntaxCallers)
	}

	// Unknown symbol resolves to nil.
	if got := callgraph.CallersDetail(g, "svc::pkg.NoSuchSymbol"); got != nil {
		t.Errorf("CallersDetail(unknown) = %+v, want nil", got)
	}
}
