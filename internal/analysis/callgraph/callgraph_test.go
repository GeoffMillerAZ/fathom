package callgraph_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// fixture builds a small, representative call graph:
//
//	A  -> B -> C          (plain call chain)
//	TestFoo -> A          (test reaching the chain)
//	TestBar -> B          (second test, shorter reach)
//	Orphan               (no callers, no callees — dead code candidate)
//
// SymbolKey format: "svc::pkg.Name"
// Nodes also record bare-name Calls entries to exercise tolerant matching.
func fixture() []core.SymbolNode {
	return []core.SymbolNode{
		{
			Service: "svc", Package: "pkg", Name: "A",
			Kind:  core.KindGoFunc,
			Calls: []string{"svc::pkg.B"}, // full key reference
		},
		{
			Service: "svc", Package: "pkg", Name: "B",
			Kind:  core.KindGoFunc,
			Calls: []string{"C"}, // bare-name reference — tolerant matching
		},
		{
			Service: "svc", Package: "pkg", Name: "C",
			Kind:  core.KindGoFunc,
			Calls: nil,
		},
		{
			Service: "svc", Package: "pkg", Name: "TestFoo",
			Kind:  core.KindGoFunc,
			Calls: []string{"svc::pkg.A"}, // TestFoo -> A -> B -> C
		},
		{
			Service: "svc", Package: "pkg", Name: "TestBar",
			Kind:  core.KindGoFunc,
			Calls: []string{"B"}, // bare name, TestBar -> B -> C
		},
		{
			Service: "svc", Package: "pkg", Name: "Orphan",
			Kind:  core.KindGoFunc,
			Calls: nil, // no callers, no callees
		},
	}
}

// ─── Build ───────────────────────────────────────────────────────────────────

func TestBuild_NodeCount(t *testing.T) {
	nodes := fixture()
	g := callgraph.Build(nodes)
	if g == nil {
		t.Fatal("Build returned nil")
	}
	if got := g.NodeCount(); got != len(nodes) {
		t.Errorf("NodeCount() = %d, want %d", got, len(nodes))
	}
}

func TestBuild_EdgeCount(t *testing.T) {
	g := callgraph.Build(fixture())
	// A->B, B->C, TestFoo->A, TestBar->B = 4 edges
	if got := g.EdgeCount(); got != 4 {
		t.Errorf("EdgeCount() = %d, want 4", got)
	}
}

// ─── RenameImpact ────────────────────────────────────────────────────────────

// RenameImpact(B) should return all direct callers of B: A, TestBar.
func TestRenameImpact_DirectCallers(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.RenameImpact(g, "svc::pkg.B")
	want := []string{"svc::pkg.A", "svc::pkg.TestBar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RenameImpact(B) = %v, want %v", got, want)
	}
}

// RenameImpact on a leaf with no callers should return empty (not nil).
func TestRenameImpact_NoCallers(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.RenameImpact(g, "svc::pkg.C")
	// B is the only direct caller
	want := []string{"svc::pkg.B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RenameImpact(C) = %v, want %v", got, want)
	}
}

func TestRenameImpact_Orphan(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.RenameImpact(g, "svc::pkg.Orphan")
	if len(got) != 0 {
		t.Errorf("RenameImpact(Orphan) = %v, want []", got)
	}
}

// RenameImpact using a bare name should still resolve correctly.
func TestRenameImpact_BareName(t *testing.T) {
	g := callgraph.Build(fixture())
	// "B" resolves to "svc::pkg.B"
	got := callgraph.RenameImpact(g, "B")
	want := []string{"svc::pkg.A", "svc::pkg.TestBar"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("RenameImpact(bare B) = %v, want %v", got, want)
	}
}

// ─── Reachable ───────────────────────────────────────────────────────────────

func TestReachable_DirectEdge(t *testing.T) {
	g := callgraph.Build(fixture())
	if !callgraph.Reachable(g, "svc::pkg.A", "svc::pkg.B") {
		t.Error("A should reach B directly")
	}
}

func TestReachable_Transitive(t *testing.T) {
	g := callgraph.Build(fixture())
	if !callgraph.Reachable(g, "svc::pkg.A", "svc::pkg.C") {
		t.Error("A should transitively reach C via B")
	}
}

func TestReachable_NoPath(t *testing.T) {
	g := callgraph.Build(fixture())
	if callgraph.Reachable(g, "svc::pkg.C", "svc::pkg.A") {
		t.Error("C cannot reach A (reverse direction)")
	}
}

func TestReachable_Self(t *testing.T) {
	g := callgraph.Build(fixture())
	// a node trivially reaches itself
	if !callgraph.Reachable(g, "svc::pkg.A", "svc::pkg.A") {
		t.Error("A should reach itself")
	}
}

func TestReachable_BareName(t *testing.T) {
	g := callgraph.Build(fixture())
	if !callgraph.Reachable(g, "A", "C") {
		t.Error("bare name A should reach bare name C")
	}
}

// ─── ReachableAny (TC-22: one BFS against a whole candidate set) ───────────

func TestReachableAny_MatchesOneOfMany(t *testing.T) {
	g := callgraph.Build(fixture())
	// TestFoo -> A -> B -> C; "svc::pkg.Z" names nothing in the fixture, so
	// only the "svc::pkg.C" candidate can match — proving the whole-set
	// membership test, not just "first candidate wins".
	if !callgraph.ReachableAny(g, "svc::pkg.TestFoo", []string{"svc::pkg.Z", "svc::pkg.C"}) {
		t.Error("TestFoo should reach C, one of the two candidates")
	}
}

func TestReachableAny_NoneMatch(t *testing.T) {
	g := callgraph.Build(fixture())
	// TestBar -> B -> C; Orphan and "Z" are both unreached (Orphan has no
	// callers, Z names nothing at all).
	if callgraph.ReachableAny(g, "svc::pkg.TestBar", []string{"svc::pkg.Orphan", "svc::pkg.Z"}) {
		t.Error("TestBar reaches neither Orphan nor the nonexistent Z")
	}
}

func TestReachableAny_EmptyCandidatesFalse(t *testing.T) {
	g := callgraph.Build(fixture())
	if callgraph.ReachableAny(g, "svc::pkg.TestFoo", nil) {
		t.Error("an empty candidate set can never be reached")
	}
}

func TestReachableAny_UnknownFromFalse(t *testing.T) {
	g := callgraph.Build(fixture())
	if callgraph.ReachableAny(g, "svc::pkg.NoSuchSymbol", []string{"svc::pkg.C"}) {
		t.Error("a from that canonicalizes to nothing can never reach anything")
	}
}

func TestReachableAny_SelfMatch(t *testing.T) {
	g := callgraph.Build(fixture())
	if !callgraph.ReachableAny(g, "svc::pkg.A", []string{"svc::pkg.A"}) {
		t.Error("A is trivially in a candidate set containing itself")
	}
}

// ─── CallPath ────────────────────────────────────────────────────────────────

func TestCallPath_Direct(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.CallPath(g, "svc::pkg.A", "svc::pkg.B")
	want := []string{"svc::pkg.A", "svc::pkg.B"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CallPath(A,B) = %v, want %v", got, want)
	}
}

func TestCallPath_Transitive(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.CallPath(g, "svc::pkg.A", "svc::pkg.C")
	want := []string{"svc::pkg.A", "svc::pkg.B", "svc::pkg.C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CallPath(A,C) = %v, want %v", got, want)
	}
}

func TestCallPath_NoPath(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.CallPath(g, "svc::pkg.C", "svc::pkg.A")
	if got != nil {
		t.Errorf("CallPath(C,A) = %v, want nil", got)
	}
}

func TestCallPath_Self(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.CallPath(g, "svc::pkg.A", "svc::pkg.A")
	want := []string{"svc::pkg.A"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CallPath(A,A) = %v, want %v", got, want)
	}
}

func TestCallPath_BareName(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.CallPath(g, "A", "C")
	want := []string{"svc::pkg.A", "svc::pkg.B", "svc::pkg.C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CallPath(bare A, bare C) = %v, want %v", got, want)
	}
}

// ─── TestImpact ──────────────────────────────────────────────────────────────

// TestImpact(A) should return TestFoo (which calls A directly) — TestBar
// does not reach A.
func TestTestImpact_DirectTestCaller(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.TestImpact(g, "svc::pkg.A")
	want := []string{"svc::pkg.TestFoo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(A) = %v, want %v", got, want)
	}
}

// TestImpact(C) — C is reachable from TestFoo (via A->B->C) and TestBar (via B->C).
func TestTestImpact_Transitive(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.TestImpact(g, "svc::pkg.C")
	want := []string{"svc::pkg.TestBar", "svc::pkg.TestFoo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(C) = %v, want %v", got, want)
	}
}

// TestImpact(Orphan) should be empty — no test reaches Orphan.
func TestTestImpact_Orphan(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.TestImpact(g, "svc::pkg.Orphan")
	if len(got) != 0 {
		t.Errorf("TestImpact(Orphan) = %v, want []", got)
	}
}

func TestTestImpact_BareName(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.TestImpact(g, "C")
	want := []string{"svc::pkg.TestBar", "svc::pkg.TestFoo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TestImpact(bare C) = %v, want %v", got, want)
	}
}

// ─── DeadCode ────────────────────────────────────────────────────────────────

// With entrypoints = [TestFoo, TestBar]:
// TestFoo -> A -> B -> C
// TestBar -> B -> C
// Reachable from entrypoints: TestFoo, TestBar, A, B, C
// Orphan is never reached and has no callers => dead code.
func TestDeadCode_Orphan(t *testing.T) {
	g := callgraph.Build(fixture())
	eps := []string{"svc::pkg.TestFoo", "svc::pkg.TestBar"}
	got := callgraph.DeadCode(g, eps)
	want := []string{"svc::pkg.Orphan"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DeadCode = %v, want %v", got, want)
	}
}

// If we include Orphan as an entrypoint, nothing should be dead.
func TestDeadCode_NoneWhenAllReachable(t *testing.T) {
	g := callgraph.Build(fixture())
	eps := []string{"svc::pkg.TestFoo", "svc::pkg.TestBar", "svc::pkg.Orphan"}
	got := callgraph.DeadCode(g, eps)
	if len(got) != 0 {
		t.Errorf("DeadCode (all reachable) = %v, want []", got)
	}
}

// With no entrypoints everything unreachable from an entry counts as dead code.
// All nodes have no incoming edges from non-test callers except B (called by A
// and TestBar) and C (called by B). So nodes with no callers at all:
// A (called by TestFoo), Orphan — but TestFoo/TestBar also have no callers.
// With empty entrypoints, ALL nodes are not reachable, so all are dead.
func TestDeadCode_EmptyEntrypoints(t *testing.T) {
	g := callgraph.Build(fixture())
	got := callgraph.DeadCode(g, nil)
	// With no entrypoints, all nodes are unreachable AND have callers or not.
	// Only nodes with zero callers qualify per spec ("no callers").
	// Nodes with zero callers: A (called by TestFoo), no — TestFoo calls A, so A has a caller.
	// Actually with no entrypoints AND no callers:
	//   - TestFoo: no callers => dead
	//   - TestBar: no callers => dead
	//   - Orphan: no callers => dead
	//   - A: has caller TestFoo => not dead (has callers)
	//   - B: has callers A, TestBar => not dead
	//   - C: has caller B => not dead
	wantSet := map[string]bool{
		"svc::pkg.TestFoo": true,
		"svc::pkg.TestBar": true,
		"svc::pkg.Orphan":  true,
	}
	gotSet := make(map[string]bool, len(got))
	for _, k := range got {
		gotSet[k] = true
	}
	if !reflect.DeepEqual(gotSet, wantSet) {
		t.Errorf("DeadCode(nil entrypoints) = %v, want %v", got, wantSet)
	}
}

// ─── Determinism ─────────────────────────────────────────────────────────────

// All analysis functions must return identical results on repeated calls.
func TestDeterminism_RenameImpact(t *testing.T) {
	g := callgraph.Build(fixture())
	a := callgraph.RenameImpact(g, "svc::pkg.B")
	b := callgraph.RenameImpact(g, "svc::pkg.B")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("RenameImpact not deterministic: %v vs %v", a, b)
	}
}

func TestDeterminism_CallPath(t *testing.T) {
	g := callgraph.Build(fixture())
	a := callgraph.CallPath(g, "svc::pkg.A", "svc::pkg.C")
	b := callgraph.CallPath(g, "svc::pkg.A", "svc::pkg.C")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("CallPath not deterministic: %v vs %v", a, b)
	}
}

func TestDeterminism_TestImpact(t *testing.T) {
	g := callgraph.Build(fixture())
	a := callgraph.TestImpact(g, "svc::pkg.C")
	b := callgraph.TestImpact(g, "svc::pkg.C")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("TestImpact not deterministic: %v vs %v", a, b)
	}
}

func TestDeterminism_DeadCode(t *testing.T) {
	g := callgraph.Build(fixture())
	eps := []string{"svc::pkg.TestFoo", "svc::pkg.TestBar"}
	a := callgraph.DeadCode(g, eps)
	b := callgraph.DeadCode(g, eps)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("DeadCode not deterministic: %v vs %v", a, b)
	}
}

// ─── Cycle tolerance ─────────────────────────────────────────────────────────

// A graph with a cycle must not cause infinite loops in any analysis.
func TestCycleTolerance(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "X", Calls: []string{"s::p.Y"}},
		{Service: "s", Package: "p", Name: "Y", Calls: []string{"s::p.X"}}, // cycle X <-> Y
	}
	g := callgraph.Build(nodes)

	if !callgraph.Reachable(g, "s::p.X", "s::p.Y") {
		t.Error("X should reach Y")
	}
	// no infinite loop
	callgraph.CallPath(g, "s::p.X", "s::p.Y")
	callgraph.RenameImpact(g, "s::p.X")
	callgraph.TestImpact(g, "s::p.X")
	callgraph.DeadCode(g, []string{"s::p.X"})
}

// ─── Unknown symbol tolerance ─────────────────────────────────────────────────

// Calls to a symbol not present in the node list must not panic; they are simply
// ignored (no node to add to the graph).
func TestBuild_UnknownCallee(t *testing.T) {
	nodes := []core.SymbolNode{
		{Service: "s", Package: "p", Name: "F", Calls: []string{"external.Func"}},
	}
	g := callgraph.Build(nodes)
	if g == nil {
		t.Fatal("Build returned nil")
	}
	// F has no known callee node; should still register F's node.
	if g.NodeCount() != 1 {
		t.Errorf("NodeCount = %d, want 1", g.NodeCount())
	}
}
