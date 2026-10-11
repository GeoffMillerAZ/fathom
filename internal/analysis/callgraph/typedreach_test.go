package callgraph_test

// typedreach_test.go — RF-5 (ADR-067 D3, I57 extended to call edges):
// verify's reach never rests on a guess. ReachableTyped and ProductReach
// follow only edges of resolution tier typed; a reach that exists only
// through a heuristic edge is refused, and the refusal names that edge
// and its tier. The edge itself stays in the graph, served with its tier.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// guardFixture:
//
//	cmd/app main -> lib.Wire      (typed: direct)
//	lib.Wire     -> lib.Typed     (typed: direct)
//	cmd/app main -> lib.Guess     (syntax only: heuristic)
//	lib.Guess    -> lib.Deep      (typed, but behind the heuristic edge)
//	main         -> reg.Use       (syntax only), reg init -> reg.Handler (typed)
//	TestReach    -> lib.Typed     (typed) ; TestReach -> lib.Guess (syntax only)
func guardFixture() ([]core.SymbolNode, []core.Edge) {
	fn := func(file, pkg, name string, calls ...string) core.SymbolNode {
		return core.SymbolNode{Service: "svc", Package: pkg, Name: name, Kind: core.KindGoFunc, FilePath: file, Calls: calls}
	}
	nodes := []core.SymbolNode{
		fn("cmd/app/main.go", "main", "main", "lib.Wire", "lib.Guess", "reg.Use"),
		fn("internal/lib/lib.go", "lib", "Wire", "Typed"),
		fn("internal/lib/lib.go", "lib", "Typed"),
		fn("internal/lib/guess.go", "lib", "Guess", "Deep"),
		fn("internal/lib/guess.go", "lib", "Deep"),
		fn("internal/reg/reg.go", "reg", "Use"),
		fn("internal/reg/reg.go", "reg", "init", "Handler"),
		fn("internal/reg/reg.go", "reg", "Handler"),
		fn("internal/lib/lib_test.go", "lib", "TestReach", "Typed", "Guess"),
	}
	e := func(from, to string) core.Edge {
		return core.Edge{FromService: "svc", FromSymbol: "svc::" + from, ToSymbol: "svc::" + to, Kind: core.EdgeDirect}
	}
	typed := []core.Edge{
		e("cmd/app/main.main", "internal/lib/lib.Wire"),
		e("internal/lib/lib.Wire", "internal/lib/lib.Typed"),
		e("internal/lib/lib.Guess", "internal/lib/lib.Deep"),
		e("internal/reg/reg.init", "internal/reg/reg.Handler"),
		e("internal/lib/lib.TestReach", "internal/lib/lib.Typed"),
	}
	return nodes, typed
}

func TestReachableTyped_RefusesAHeuristicEdgeAndNamesIt(t *testing.T) {
	nodes, typed := guardFixture()
	g := callgraph.BuildBoth(nodes, typed)

	if ok, via := callgraph.ReachableTyped(g, "TestReach", "Typed"); !ok || via != nil {
		t.Errorf("TestReach -> Typed over a typed edge: got (%v, %v), want (true, nil)", ok, via)
	}
	// Deep is reached from TestReach only through TestReach -> Guess, a
	// syntax-only edge: refused, and the refusal names that edge.
	ok, via := callgraph.ReachableTyped(g, "TestReach", "Deep")
	if ok {
		t.Fatal("TestReach -> Deep passes only through a heuristic edge; ReachableTyped must refuse it")
	}
	if via == nil || via.From != "svc::internal/lib/lib.TestReach" || via.To != "svc::internal/lib/lib.Guess" {
		t.Fatalf("via = %+v, want the heuristic edge TestReach -> Guess", via)
	}
	if got := via.Tier().String(); got != "heuristic" {
		t.Errorf("via tier = %q, want heuristic", got)
	}
	if via.Kind != "" {
		t.Errorf("via kind = %q, want \"\" (a syntax-only edge)", via.Kind)
	}
	// No path at all: no edge to name.
	if ok, via := callgraph.ReachableTyped(g, "Typed", "Deep"); ok || via != nil {
		t.Errorf("Typed -> Deep has no path: got (%v, %+v), want (false, nil)", ok, via)
	}
	// The tier-blind walk still sees the heuristic reach: the edge is
	// served, only verify refuses to pass on it.
	if !callgraph.Reachable(g, "TestReach", "Deep") {
		t.Error("Reachable (tier-blind) must still follow the heuristic edge")
	}
}

func TestProductReach_TypedOnlyAndHeuristicNamed(t *testing.T) {
	nodes, typed := guardFixture()
	r := callgraph.ProductEntryReach(callgraph.BuildBoth(nodes, typed))

	if e, ok := r.ReachedBy("Typed"); !ok || e != "svc::cmd/app/main.main" {
		t.Errorf("Typed: ReachedBy = (%q, %v), want main over typed edges", e, ok)
	}
	for _, c := range []struct{ symbol, from, to string }{
		{"Guess", "svc::cmd/app/main.main", "svc::internal/lib/lib.Guess"},
		// Deep sits behind the same heuristic edge, though its own edge is typed.
		{"Deep", "svc::cmd/app/main.main", "svc::internal/lib/lib.Guess"},
		// reg is linked only through main -> reg.Use (heuristic), so its
		// init — and what the init reaches — rests on that edge.
		{"Handler", "svc::cmd/app/main.main", "svc::internal/reg/reg.Use"},
	} {
		if e, ok := r.ReachedBy(c.symbol); ok {
			t.Errorf("%s: ReachedBy = %q, want refused (only a heuristic edge reaches it)", c.symbol, e)
		}
		entry, via, ok := r.HeuristicReachedBy(c.symbol)
		if !ok {
			t.Errorf("%s: HeuristicReachedBy found nothing, want the edge %s -> %s", c.symbol, c.from, c.to)
			continue
		}
		if entry != "svc::cmd/app/main.main" || via.From != c.from || via.To != c.to || via.Tier().String() != "heuristic" {
			t.Errorf("%s: HeuristicReachedBy = (%q, %+v), want main via %s -> %s (heuristic)", c.symbol, entry, via, c.from, c.to)
		}
	}
	if _, _, ok := r.HeuristicReachedBy("Typed"); ok {
		t.Error("Typed is reached over typed edges; it has no heuristic reach to name")
	}
	if got := r.Entries(); len(got) != 1 || got[0] != "svc::cmd/app/main.main" {
		t.Errorf("Entries = %v, want main alone (reg's init is linked only through a heuristic edge)", got)
	}
}
