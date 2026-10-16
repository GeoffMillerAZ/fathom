package livegraph_test

import (
	"reflect"
	"testing"

	lg "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/topology/live-graph"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// buildGraph is a convenience helper for tests.
func buildGraph(t *testing.T, nodes []string, edges [][2]string) *lg.Graph {
	t.Helper()
	g := &lg.Graph{}
	for _, id := range nodes {
		if err := g.AddNode(lg.Node{ID: id}); err != nil {
			t.Fatalf("AddNode(%q): %v", id, err)
		}
	}
	for _, e := range edges {
		if err := g.AddEdge(lg.Edge{From: e[0], To: e[1]}); err != nil {
			t.Fatalf("AddEdge(%v→%v): %v", e[0], e[1], err)
		}
	}
	return g
}

// ── AddNode ───────────────────────────────────────────────────────────────────

func TestAddNode_emptyID_rejected(t *testing.T) {
	g := &lg.Graph{}
	err := g.AddNode(lg.Node{ID: ""})
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestAddNode_idempotent(t *testing.T) {
	g := &lg.Graph{}
	g.AddNode(lg.Node{ID: "a"}) //nolint:errcheck
	g.AddNode(lg.Node{ID: "a"}) //nolint:errcheck
	nodes := g.Nodes()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
}

// ── AddEdge ───────────────────────────────────────────────────────────────────

func TestAddEdge_unknownFrom_rejected(t *testing.T) {
	g := &lg.Graph{}
	g.AddNode(lg.Node{ID: "b"}) //nolint:errcheck
	err := g.AddEdge(lg.Edge{From: "unknown", To: "b"})
	if err == nil {
		t.Fatal("expected error for unknown From node")
	}
}

func TestAddEdge_unknownTo_rejected(t *testing.T) {
	g := &lg.Graph{}
	g.AddNode(lg.Node{ID: "a"}) //nolint:errcheck
	err := g.AddEdge(lg.Edge{From: "a", To: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown To node")
	}
}

func TestAddEdge_emptyGraph_rejected(t *testing.T) {
	g := &lg.Graph{}
	err := g.AddEdge(lg.Edge{From: "a", To: "b"})
	if err == nil {
		t.Fatal("expected error when graph has no nodes")
	}
}

// ── Cycles ────────────────────────────────────────────────────────────────────

func TestCycles_acyclic(t *testing.T) {
	// a → b → c
	g := buildGraph(t, []string{"a", "b", "c"}, [][2]string{{"a", "b"}, {"b", "c"}})
	if g.Cycles() {
		t.Fatal("expected no cycles in linear DAG")
	}
}

func TestCycles_selfLoop(t *testing.T) {
	g := buildGraph(t, []string{"a"}, [][2]string{{"a", "a"}})
	if !g.Cycles() {
		t.Fatal("expected cycle for self-loop")
	}
}

func TestCycles_directCycle(t *testing.T) {
	// a → b → a
	g := buildGraph(t, []string{"a", "b"}, [][2]string{{"a", "b"}, {"b", "a"}})
	if !g.Cycles() {
		t.Fatal("expected cycle for a↔b")
	}
}

func TestCycles_longerCycle(t *testing.T) {
	// a → b → c → a
	g := buildGraph(t, []string{"a", "b", "c"}, [][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}})
	if !g.Cycles() {
		t.Fatal("expected cycle for a→b→c→a")
	}
}

func TestCycles_diamond_noCycle(t *testing.T) {
	// a → b, a → c, b → d, c → d
	g := buildGraph(
		t,
		[]string{"a", "b", "c", "d"},
		[][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}},
	)
	if g.Cycles() {
		t.Fatal("expected no cycle in diamond DAG")
	}
}

func TestCycles_emptyGraph(t *testing.T) {
	g := &lg.Graph{}
	if g.Cycles() {
		t.Fatal("empty graph should have no cycles")
	}
}

// ── CriticalPath ──────────────────────────────────────────────────────────────

func TestCriticalPath_linear(t *testing.T) {
	// a → b → c: critical path is [a, b, c]
	g := buildGraph(t, []string{"a", "b", "c"}, [][2]string{{"a", "b"}, {"b", "c"}})
	path, err := g.CriticalPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(path, want) {
		t.Errorf("CriticalPath = %v, want %v", path, want)
	}
}

func TestCriticalPath_diamond(t *testing.T) {
	// a → b → d, a → c → d
	// Two paths of length 3: [a,b,d] and [a,c,d]; lex first is [a,b,d].
	g := buildGraph(
		t,
		[]string{"a", "b", "c", "d"},
		[][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}},
	)
	path, err := g.CriticalPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The longest path in the diamond is 3 nodes (a→b→d or a→c→d).
	if len(path) != 3 {
		t.Errorf("expected path length 3, got %d: %v", len(path), path)
	}
	// Deterministic: lexicographically first path is [a, b, d].
	want := []string{"a", "b", "d"}
	if !reflect.DeepEqual(path, want) {
		t.Errorf("CriticalPath = %v, want %v", path, want)
	}
}

func TestCriticalPath_singleNode(t *testing.T) {
	g := buildGraph(t, []string{"solo"}, [][2]string{})
	path, err := g.CriticalPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 1 || path[0] != "solo" {
		t.Errorf("expected [solo], got %v", path)
	}
}

func TestCriticalPath_emptyGraph(t *testing.T) {
	g := &lg.Graph{}
	path, err := g.CriticalPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(path) != 0 {
		t.Errorf("expected empty path, got %v", path)
	}
}

func TestCriticalPath_cycleReturnsError(t *testing.T) {
	g := buildGraph(t, []string{"a", "b"}, [][2]string{{"a", "b"}, {"b", "a"}})
	_, err := g.CriticalPath()
	if err == nil {
		t.Fatal("expected error for cyclic graph")
	}
}

func TestCriticalPath_deterministic(t *testing.T) {
	// Run twice; must produce identical result.
	nodes := []string{"a", "b", "c", "d", "e"}
	edges := [][2]string{{"a", "c"}, {"a", "d"}, {"b", "c"}, {"b", "e"}, {"c", "e"}, {"d", "e"}}
	g := buildGraph(t, nodes, edges)
	p1, err1 := g.CriticalPath()
	p2, err2 := g.CriticalPath()
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v %v", err1, err2)
	}
	if !reflect.DeepEqual(p1, p2) {
		t.Errorf("non-deterministic: %v vs %v", p1, p2)
	}
}

// ── Waves ─────────────────────────────────────────────────────────────────────

func TestWaves_linear(t *testing.T) {
	// a → b → c: wave 0=[a], wave 1=[b], wave 2=[c]
	g := buildGraph(t, []string{"a", "b", "c"}, [][2]string{{"a", "b"}, {"b", "c"}})
	waves, err := g.Waves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := [][]string{{"a"}, {"b"}, {"c"}}
	if !reflect.DeepEqual(waves, want) {
		t.Errorf("Waves = %v, want %v", waves, want)
	}
}

func TestWaves_diamond(t *testing.T) {
	// a → b, a → c, b → d, c → d
	// wave 0=[a], wave 1=[b,c], wave 2=[d]
	g := buildGraph(
		t,
		[]string{"a", "b", "c", "d"},
		[][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}},
	)
	waves, err := g.Waves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves, got %d: %v", len(waves), waves)
	}
	if !reflect.DeepEqual(waves[0], []string{"a"}) {
		t.Errorf("wave 0 = %v, want [a]", waves[0])
	}
	if !reflect.DeepEqual(waves[1], []string{"b", "c"}) {
		t.Errorf("wave 1 = %v, want [b,c]", waves[1])
	}
	if !reflect.DeepEqual(waves[2], []string{"d"}) {
		t.Errorf("wave 2 = %v, want [d]", waves[2])
	}
}

func TestWaves_isolated_nodes(t *testing.T) {
	// a, b, c — no edges: all in wave 0, sorted.
	g := buildGraph(t, []string{"c", "a", "b"}, [][2]string{})
	waves, err := g.Waves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 1 {
		t.Fatalf("expected 1 wave for disconnected nodes, got %d: %v", len(waves), waves)
	}
	if !reflect.DeepEqual(waves[0], []string{"a", "b", "c"}) {
		t.Errorf("wave 0 = %v, want [a,b,c]", waves[0])
	}
}

func TestWaves_emptyGraph(t *testing.T) {
	g := &lg.Graph{}
	waves, err := g.Waves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 0 {
		t.Errorf("expected empty waves, got %v", waves)
	}
}

func TestWaves_cycleReturnsError(t *testing.T) {
	g := buildGraph(t, []string{"a", "b", "c"}, [][2]string{{"a", "b"}, {"b", "c"}, {"c", "a"}})
	_, err := g.Waves()
	if err == nil {
		t.Fatal("expected error for cyclic graph")
	}
}

func TestWaves_deterministic(t *testing.T) {
	nodes := []string{"a", "b", "c", "d", "e"}
	edges := [][2]string{{"a", "c"}, {"b", "c"}, {"c", "d"}, {"c", "e"}}
	g := buildGraph(t, nodes, edges)
	w1, err1 := g.Waves()
	w2, err2 := g.Waves()
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v %v", err1, err2)
	}
	if !reflect.DeepEqual(w1, w2) {
		t.Errorf("non-deterministic waves: %v vs %v", w1, w2)
	}
}
