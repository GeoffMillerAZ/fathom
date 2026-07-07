package depgraph_test

import (
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/depgraph"
)

// helpers

func sortedStrings(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}

// ─── AddNode / Nodes ──────────────────────────────────────────────────────────

func TestNodes_empty(t *testing.T) {
	g := depgraph.NewGraph()
	if got := g.Nodes(); len(got) != 0 {
		t.Fatalf("expected 0 nodes, got %v", got)
	}
}

func TestNodes_addNode(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddNode("a")
	g.AddNode("b")
	g.AddNode("a") // duplicate — idempotent
	got := sortedStrings(g.Nodes())
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNodes_addEdgeImpliesNodes(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("x", "y")
	nodes := sortedStrings(g.Nodes())
	if !reflect.DeepEqual(nodes, []string{"x", "y"}) {
		t.Fatalf("AddEdge should register both endpoints as nodes, got %v", nodes)
	}
}

// ─── Deps / Dependents ───────────────────────────────────────────────────────

func TestDeps_basic(t *testing.T) {
	g := depgraph.NewGraph()
	// a → b, a → c
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")
	got := sortedStrings(g.Deps("a"))
	if !reflect.DeepEqual(got, []string{"b", "c"}) {
		t.Fatalf("got %v", got)
	}
}

func TestDeps_noEdges(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddNode("a")
	if got := g.Deps("a"); len(got) != 0 {
		t.Fatalf("expected empty deps, got %v", got)
	}
}

func TestDeps_unknownNode(t *testing.T) {
	g := depgraph.NewGraph()
	if got := g.Deps("z"); got != nil {
		t.Fatalf("expected nil for unknown node, got %v", got)
	}
}

func TestDependents_basic(t *testing.T) {
	g := depgraph.NewGraph()
	// a → c, b → c
	g.AddEdge("a", "c")
	g.AddEdge("b", "c")
	got := sortedStrings(g.Dependents("c"))
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("got %v", got)
	}
}

func TestDependents_unknownNode(t *testing.T) {
	g := depgraph.NewGraph()
	if got := g.Dependents("z"); got != nil {
		t.Fatalf("expected nil for unknown node, got %v", got)
	}
}

// ─── HasCycle / Cycles ────────────────────────────────────────────────────────

func TestHasCycle_acyclic(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	if g.HasCycle() {
		t.Fatal("expected no cycle")
	}
}

func TestHasCycle_selfLoop(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "a")
	if !g.HasCycle() {
		t.Fatal("expected cycle (self-loop)")
	}
}

func TestHasCycle_simpleCycle(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "a")
	if !g.HasCycle() {
		t.Fatal("expected cycle")
	}
}

func TestHasCycle_twoComponentsOneCyclic(t *testing.T) {
	g := depgraph.NewGraph()
	// acyclic component
	g.AddEdge("x", "y")
	// cyclic component
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")
	if !g.HasCycle() {
		t.Fatal("expected cycle detected across components")
	}
}

func TestCycles_noCycles(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	if cycles := g.Cycles(); len(cycles) != 0 {
		t.Fatalf("expected no cycles, got %v", cycles)
	}
}

func TestCycles_simpleCycle(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "a")
	cycles := g.Cycles()
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle, got %v", cycles)
	}
	got := sortedStrings(cycles[0])
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cycle members: got %v want %v", got, want)
	}
}

func TestCycles_twoCycles(t *testing.T) {
	g := depgraph.NewGraph()
	// cycle 1: a→b→a
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")
	// cycle 2: x→y→z→x
	g.AddEdge("x", "y")
	g.AddEdge("y", "z")
	g.AddEdge("z", "x")
	cycles := g.Cycles()
	if len(cycles) != 2 {
		t.Fatalf("expected 2 cycles, got %v", cycles)
	}
}

func TestCycles_selfLoop(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "a")
	cycles := g.Cycles()
	if len(cycles) != 1 {
		t.Fatalf("expected 1 cycle for self-loop, got %v", cycles)
	}
	if len(cycles[0]) != 1 || cycles[0][0] != "a" {
		t.Fatalf("expected [a], got %v", cycles[0])
	}
}

// Cycles() must be deterministic across multiple calls.
func TestCycles_deterministic(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("c", "b")
	g.AddEdge("b", "a")
	g.AddEdge("a", "c")
	first := g.Cycles()
	for i := 0; i < 10; i++ {
		subsequent := g.Cycles()
		if !reflect.DeepEqual(first, subsequent) {
			t.Fatalf("non-deterministic Cycles(): first=%v subsequent=%v", first, subsequent)
		}
	}
}

// ─── TopoSort ─────────────────────────────────────────────────────────────────

func TestTopoSort_acyclic(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	order, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isTopoOrder(g, order) {
		t.Fatalf("not a valid topological order: %v", order)
	}
}

func TestTopoSort_disconnected(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddNode("x")
	g.AddNode("y")
	g.AddEdge("a", "b")
	order, err := g.TopoSort()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// all nodes must appear
	seen := map[string]bool{}
	for _, n := range order {
		seen[n] = true
	}
	for _, n := range g.Nodes() {
		if !seen[n] {
			t.Fatalf("node %q missing from topo order", n)
		}
	}
}

func TestTopoSort_cyclic_returnsError(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	g.AddEdge("b", "a")
	if _, err := g.TopoSort(); err == nil {
		t.Fatal("expected error for cyclic graph")
	}
}

func TestTopoSort_deterministic(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("d", "a")
	g.AddEdge("d", "b")
	g.AddEdge("d", "c")
	g.AddEdge("a", "b")
	first, _ := g.TopoSort()
	for i := 0; i < 10; i++ {
		next, _ := g.TopoSort()
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("non-deterministic TopoSort: %v vs %v", first, next)
		}
	}
}

// isTopoOrder verifies that for every edge u→v, u appears before v.
func isTopoOrder(g *depgraph.Graph, order []string) bool {
	pos := map[string]int{}
	for i, n := range order {
		pos[n] = i
	}
	for _, u := range g.Nodes() {
		for _, v := range g.Deps(u) {
			if pos[u] > pos[v] {
				return false
			}
		}
	}
	return true
}

// ─── Transitive ───────────────────────────────────────────────────────────────

func TestTransitive_direct(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	got := g.Transitive("a")
	if !reflect.DeepEqual(got, []string{"b"}) {
		t.Fatalf("got %v", got)
	}
}

func TestTransitive_chain(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "d")
	got := g.Transitive("a")
	want := []string{"b", "c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTransitive_diamond(t *testing.T) {
	g := depgraph.NewGraph()
	//   a
	//  / \
	// b   c
	//  \ /
	//   d
	g.AddEdge("a", "b")
	g.AddEdge("a", "c")
	g.AddEdge("b", "d")
	g.AddEdge("c", "d")
	got := g.Transitive("a")
	want := []string{"b", "c", "d"} // sorted, no duplicates
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestTransitive_noDeps(t *testing.T) {
	g := depgraph.NewGraph()
	g.AddNode("a")
	if got := g.Transitive("a"); len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestTransitive_unknownNode(t *testing.T) {
	g := depgraph.NewGraph()
	if got := g.Transitive("z"); got != nil {
		t.Fatalf("expected nil for unknown node, got %v", got)
	}
}

func TestTransitive_cycleDoesNotLoop(t *testing.T) {
	// Transitive must not loop infinitely on a cyclic graph.
	g := depgraph.NewGraph()
	g.AddEdge("a", "b")
	g.AddEdge("b", "c")
	g.AddEdge("c", "a")
	got := g.Transitive("a")
	// a's transitive includes b and c (not a itself)
	want := []string{"b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
