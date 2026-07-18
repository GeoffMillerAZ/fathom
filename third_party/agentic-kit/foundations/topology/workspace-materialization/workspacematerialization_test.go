package workspacematerialization_test

import (
	"reflect"
	"testing"

	wm "github.com/geoffmilleraz/agentic-kit/foundations/topology/workspace-materialization"
)

// ── stubGraph — test double for Graph interface ───────────────────────────────

// stubGraph implements workspacematerialization.Graph for testing.
type stubGraph struct {
	nodes []string            // sorted list of all node IDs
	preds map[string][]string // predecessors (dependencies) of each node
}

func (s *stubGraph) Nodes() []string { return append([]string(nil), s.nodes...) }
func (s *stubGraph) Predecessors(id string) []string {
	p, ok := s.preds[id]
	if !ok {
		return []string{}
	}
	return append([]string(nil), p...)
}

// buildStub constructs a stubGraph from a list of node IDs and directed edges
// where edge[0] → edge[1] means "edge[1] depends on edge[0]".
func buildStub(nodes []string, edges [][2]string) *stubGraph {
	preds := make(map[string][]string)
	for _, n := range nodes {
		preds[n] = []string{}
	}
	for _, e := range edges {
		preds[e[1]] = append(preds[e[1]], e[0])
	}
	return &stubGraph{nodes: nodes, preds: preds}
}

// ── Materialize ───────────────────────────────────────────────────────────────

func TestMaterialize_singleNode(t *testing.T) {
	g := buildStub([]string{"app"}, [][2]string{})
	result, err := wm.Materialize(g, "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"app"}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("Materialize = %v, want %v", result, want)
	}
}

func TestMaterialize_linearChain(t *testing.T) {
	// infra → db → app → ui; target=ui  → expect all four
	g := buildStub(
		[]string{"app", "db", "infra", "ui"},
		[][2]string{{"infra", "db"}, {"db", "app"}, {"app", "ui"}},
	)
	result, err := wm.Materialize(g, "ui")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"app", "db", "infra", "ui"}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("Materialize = %v, want %v", result, want)
	}
}

func TestMaterialize_partialDeps(t *testing.T) {
	// infra → db → app, infra → cache → app, unrelated → other
	// target=app: should include infra, db, cache, app but NOT other or unrelated.
	g := buildStub(
		[]string{"app", "cache", "db", "infra", "other", "unrelated"},
		[][2]string{
			{"infra", "db"},
			{"infra", "cache"},
			{"db", "app"},
			{"cache", "app"},
			{"unrelated", "other"},
		},
	)
	result, err := wm.Materialize(g, "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"app", "cache", "db", "infra"}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("Materialize = %v, want %v", result, want)
	}
}

func TestMaterialize_targetWithNoDeps(t *testing.T) {
	g := buildStub([]string{"a", "b", "c"}, [][2]string{{"a", "b"}, {"b", "c"}})
	// target=a has no predecessors
	result, err := wm.Materialize(g, "a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a"}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("Materialize = %v, want %v", result, want)
	}
}

func TestMaterialize_emptyTarget_error(t *testing.T) {
	g := buildStub([]string{"a"}, [][2]string{})
	_, err := wm.Materialize(g, "")
	if err == nil {
		t.Fatal("expected error for empty target")
	}
}

func TestMaterialize_unknownTarget_error(t *testing.T) {
	g := buildStub([]string{"a", "b"}, [][2]string{{"a", "b"}})
	_, err := wm.Materialize(g, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent target")
	}
}

func TestMaterialize_deterministic(t *testing.T) {
	// Run twice and confirm same sorted output.
	g := buildStub(
		[]string{"a", "b", "c", "d", "e"},
		[][2]string{{"a", "c"}, {"b", "c"}, {"c", "d"}, {"c", "e"}},
	)
	r1, err1 := wm.Materialize(g, "d")
	r2, err2 := wm.Materialize(g, "d")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v %v", err1, err2)
	}
	if !reflect.DeepEqual(r1, r2) {
		t.Errorf("non-deterministic: %v vs %v", r1, r2)
	}
}

func TestMaterialize_diamondDeps(t *testing.T) {
	// a → b, a → c, b → d, c → d; target=d: all four nodes
	g := buildStub(
		[]string{"a", "b", "c", "d"},
		[][2]string{{"a", "b"}, {"a", "c"}, {"b", "d"}, {"c", "d"}},
	)
	result, err := wm.Materialize(g, "d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a", "b", "c", "d"}
	if !reflect.DeepEqual(result, want) {
		t.Errorf("Materialize = %v, want %v", result, want)
	}
}
