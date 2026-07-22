package impact_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/impact"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// dispatchNodes mirrors the ADR-021 §3 O9 residue shape: RunAdapter has no
// resolvable Calls entry for "Complete" at all (an ambiguous method name
// across two receivers — the syntax-only resolver refuses it entirely, so
// there isn't even a raw unresolved string to point at). AdapterA.Complete
// changing is only in RunAdapter's blast radius via a typed edge.
var dispatchNodes = []core.SymbolNode{
	{Service: "svc", Package: "AdapterA", Name: "AdapterA.Complete", FilePath: "adaptera.go"},
	{Service: "svc", Package: "AdapterB", Name: "AdapterB.Complete", FilePath: "adapterb.go"},
	{Service: "svc", Package: "pkg", Name: "RunAdapter", FilePath: "run.go"},
	{Service: "svc", Package: "pkg", Name: "TestRunAdapter", FilePath: "run_test.go", Calls: []string{"svc::pkg.RunAdapter"}},
}

var dispatchTyped = []core.Edge{
	{FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::AdapterA.AdapterA.Complete", Kind: core.EdgeIfaceDispatch},
	{FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::AdapterB.AdapterB.Complete", Kind: core.EdgeIfaceDispatch},
}

func TestBlastRadius_WithoutTypedEdges_MissesDispatchCaller(t *testing.T) {
	r := impact.BlastRadius(dispatchNodes, []string{"adaptera.go"})
	for _, s := range r.Symbols {
		if s == "svc::pkg.RunAdapter" || s == "svc::pkg.TestRunAdapter" {
			t.Fatalf("plain BlastRadius unexpectedly reached %s via the syntax-only graph — the residue this test needs to be real", s)
		}
	}
}

func TestBlastRadiusWithTypedEdges_ReachesDispatchCaller(t *testing.T) {
	r := impact.BlastRadiusWithTypedEdges(dispatchNodes, []string{"adaptera.go"}, dispatchTyped)
	want := map[string]bool{
		"svc::AdapterA.AdapterA.Complete": true,
		"svc::pkg.RunAdapter":             true,
		"svc::pkg.TestRunAdapter":         true,
	}
	got := map[string]bool{}
	for _, s := range r.Symbols {
		got[s] = true
	}
	for k := range want {
		if !got[k] {
			t.Errorf("BlastRadiusWithTypedEdges missing %s; got %v", k, r.Symbols)
		}
	}
	var sawTest bool
	for _, s := range r.Tests {
		if s == "svc::pkg.TestRunAdapter" {
			sawTest = true
		}
	}
	if !sawTest {
		t.Errorf("BlastRadiusWithTypedEdges.Tests missing TestRunAdapter; got %v", r.Tests)
	}
}

func TestBlastRadiusWithTypedEdges_NilTypedIdenticalToPlain(t *testing.T) {
	plain := impact.BlastRadius(dispatchNodes, []string{"run.go"})
	withNil := impact.BlastRadiusWithTypedEdges(dispatchNodes, []string{"run.go"}, nil)
	if len(plain.Symbols) != len(withNil.Symbols) {
		t.Fatalf("BlastRadiusWithTypedEdges(nil) = %v, want identical to BlastRadius = %v", withNil.Symbols, plain.Symbols)
	}
	for i := range plain.Symbols {
		if plain.Symbols[i] != withNil.Symbols[i] {
			t.Errorf("Symbols[%d]: plain=%q withNil=%q", i, plain.Symbols[i], withNil.Symbols[i])
		}
	}
}

func TestDiffImpactWithTypedEdges_SameAsBlastRadiusWithTypedEdges(t *testing.T) {
	a := impact.BlastRadiusWithTypedEdges(dispatchNodes, []string{"adaptera.go"}, dispatchTyped)
	b := impact.DiffImpactWithTypedEdges(dispatchNodes, []string{"adaptera.go"}, dispatchTyped)
	if len(a.Symbols) != len(b.Symbols) {
		t.Fatalf("DiffImpactWithTypedEdges diverged from BlastRadiusWithTypedEdges: %v vs %v", b.Symbols, a.Symbols)
	}
}
