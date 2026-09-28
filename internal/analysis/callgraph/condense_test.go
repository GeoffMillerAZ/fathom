package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// condenseFixture: A <-> B is a cycle (one SCC), C calls A, D calls C,
// TestX calls D, E calls itself, F is isolated.
func condenseFixture() []core.SymbolNode {
	fn := func(name string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: "pkg", Name: name, Kind: core.KindGoFunc, Calls: calls}
	}
	return []core.SymbolNode{
		fn("A", "svc::pkg.B"),
		fn("B", "svc::pkg.A"),
		fn("C", "svc::pkg.A"),
		fn("D", "svc::pkg.C"),
		fn("TestX", "svc::pkg.D"),
		fn("E", "svc::pkg.E"),
		fn("F"),
	}
}

// TestTransitiveCallerCounts proves the SCC-condensation count (TQ-2's
// owner decision: no per-symbol BFS): every member of a cycle is a
// transitive caller of every other member, a symbol never counts itself
// (not even through a self-loop), and a caller reached twice counts once.
func TestTransitiveCallerCounts(t *testing.T) {
	g := callgraph.Build(condenseFixture())
	got := callgraph.TransitiveCallerCounts(g)
	want := map[string]int{
		"svc::pkg.A":     4, // B, C, D, TestX
		"svc::pkg.B":     4, // A, C, D, TestX
		"svc::pkg.C":     2, // D, TestX
		"svc::pkg.D":     1, // TestX
		"svc::pkg.TestX": 0,
		"svc::pkg.E":     0,
		"svc::pkg.F":     0,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitiveCallerCounts = %v, want %v", got, want)
	}
}

// TestForwardReach proves the forward half: a source reaches every symbol
// below it, the cycle included, and never itself unless it sits on a
// cycle.
func TestForwardReach(t *testing.T) {
	g := callgraph.Build(condenseFixture())
	r := callgraph.ForwardReach(g, []string{"svc::pkg.TestX", "svc::pkg.A", "svc::pkg.E", "svc::pkg.F", "missing"})
	cases := map[string][]string{
		"svc::pkg.TestX": {"svc::pkg.A", "svc::pkg.B", "svc::pkg.C", "svc::pkg.D"},
		"svc::pkg.A":     {"svc::pkg.A", "svc::pkg.B"},
		"svc::pkg.E":     nil,
		"svc::pkg.F":     nil,
		"missing":        nil,
	}
	for src, want := range cases {
		if got := r.Keys(src); !reflect.DeepEqual(got, want) {
			t.Errorf("Keys(%s) = %v, want %v", src, got, want)
		}
	}
	if r.Fingerprint("svc::pkg.TestX") == r.Fingerprint("svc::pkg.A") {
		t.Error("different reach sets must have different fingerprints")
	}
	if r.Fingerprint("svc::pkg.E") != r.Fingerprint("svc::pkg.F") {
		t.Error("equal (empty) reach sets must share a fingerprint")
	}
}
