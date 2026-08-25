package callgraph_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestSameNamedMethodsDoNotFlipReachability is VS-5 (docs/plans/
// verify-spine.md): a method call resolves by receiver type, or refuses
// loudly when same-named methods on different receivers are ambiguous — so
// adding a test to a package can never flip another row's reachability
// verdict (the 2026-08-23 cpp-toolchain CT-1/CT-2 flip). Build is called at
// this test's top level, exercising the same bare-name-collision path the
// 2026-08-23 incident hit: a syntax-recorded ambiguous selector call
// ("a.Extract") must resolve to NOTHING, in EITHER node order — never
// first-match-wins.
func TestSameNamedMethodsDoNotFlipReachability(t *testing.T) {
	const svc = "vs5fix"

	caller := core.SymbolNode{
		Service: svc, Package: svc, Name: "Caller", Kind: core.KindGoFunc,
		FilePath: "caller.go", Calls: []string{"a.Extract"},
	}
	aExtract := core.SymbolNode{
		Service: svc, Package: "A", Name: "A.Extract", Kind: core.KindGoMethod,
		FilePath: "a.go",
	}
	bExtract := core.SymbolNode{
		Service: svc, Package: "B", Name: "B.Extract", Kind: core.KindGoMethod,
		FilePath: "b.go",
	}

	for _, tc := range []struct {
		name  string
		nodes []core.SymbolNode
	}{
		{"A-B-Caller order", []core.SymbolNode{aExtract, bExtract, caller}},
		{"B-A-Caller order", []core.SymbolNode{bExtract, aExtract, caller}},
		{"Caller-A-B order", []core.SymbolNode{caller, aExtract, bExtract}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := callgraph.Build(tc.nodes)
			if callgraph.Reachable(g, caller.Key(), aExtract.Key()) {
				t.Errorf("Reachable(Caller, A.Extract) = true, want false — ambiguous same-named method call must refuse, never first-match-wins")
			}
			if callgraph.Reachable(g, caller.Key(), bExtract.Key()) {
				t.Errorf("Reachable(Caller, B.Extract) = true, want false — ambiguous same-named method call must refuse, never first-match-wins")
			}
			if impact := callgraph.TestImpact(g, aExtract.Key()); len(impact) != 0 {
				t.Errorf("TestImpact(A.Extract) = %v, want empty", impact)
			}
			if impact := callgraph.TestImpact(g, bExtract.Key()); len(impact) != 0 {
				t.Errorf("TestImpact(B.Extract) = %v, want empty", impact)
			}
		})
	}

	// Precise contrast: a caller whose Calls entry is a full, unambiguous
	// SymbolKey (exactly what the typed pass records) resolves to THAT
	// receiver only, regardless of the sibling same-named method existing.
	preciseCaller := core.SymbolNode{
		Service: svc, Package: svc, Name: "PreciseCaller", Kind: core.KindGoFunc,
		FilePath: "precise.go", Calls: []string{aExtract.Key()},
	}
	g := callgraph.Build([]core.SymbolNode{aExtract, bExtract, preciseCaller})
	if !callgraph.Reachable(g, preciseCaller.Key(), aExtract.Key()) {
		t.Error("Reachable(PreciseCaller, A.Extract) = false, want true — a full SymbolKey call must resolve precisely")
	}
	if callgraph.Reachable(g, preciseCaller.Key(), bExtract.Key()) {
		t.Error("Reachable(PreciseCaller, B.Extract) = true, want false — a precise call to A.Extract must never reach B.Extract")
	}
}
