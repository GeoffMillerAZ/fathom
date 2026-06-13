package graph

import "testing"

func TestTraverse_BasicOut(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "B", Kind: KindCalls},
		{From: "B", To: "C", Kind: KindCalls},
		{From: "C", To: "D", Kind: KindSpecRef},
	}
	res := Traverse(edges, "A", nil, 3, "out")
	if res.Truncated {
		t.Error("should not be truncated")
	}
	// Should find A→B, A→B→C, A→B→C→D (3 paths)
	if len(res.Paths) != 3 {
		t.Errorf("expected 3 paths, got %d", len(res.Paths))
	}
}

func TestTraverse_DepthCap(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "B", Kind: KindCalls},
		{From: "B", To: "C", Kind: KindCalls},
		{From: "C", To: "D", Kind: KindCalls},
	}
	res := Traverse(edges, "A", nil, 1, "out")
	// depth=1: only 1-hop paths
	if len(res.Paths) != 1 {
		t.Errorf("depth=1: expected 1 path, got %d", len(res.Paths))
	}
	if res.Paths[0].Edges[0].To != "B" {
		t.Errorf("expected first hop to B, got %s", res.Paths[0].Edges[0].To)
	}
}

func TestTraverse_DirectionIn(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "C", Kind: KindCalls},
		{From: "B", To: "C", Kind: KindCalls},
	}
	res := Traverse(edges, "C", nil, 2, "in")
	if len(res.Paths) != 2 {
		t.Errorf("direction=in from C: expected 2 paths (A→C and B→C), got %d", len(res.Paths))
	}
}

func TestTraverse_DirectionBoth(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "B", Kind: KindCalls},
		{From: "B", To: "C", Kind: KindCalls},
	}
	// Start at B: out→C, in←A
	res := Traverse(edges, "B", nil, 1, "both")
	if len(res.Paths) != 2 {
		t.Errorf("direction=both from B depth=1: expected 2 paths, got %d", len(res.Paths))
	}
}

func TestTraverse_CycleSafe(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "B", Kind: KindCalls},
		{From: "B", To: "A", Kind: KindCalls}, // cycle
	}
	res := Traverse(edges, "A", nil, 10, "out")
	// With cycle guard: A→B is 1 path (B→A would cycle back, so stops)
	if len(res.Paths) != 1 {
		t.Errorf("cycle: expected 1 path, got %d", len(res.Paths))
	}
}

func TestTraverse_KindFilter(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "B", Kind: KindCalls},
		{From: "A", To: "spec1", Kind: KindSpecRef},
	}
	res := Traverse(edges, "A", []string{KindCalls}, 2, "out")
	for _, p := range res.Paths {
		for _, e := range p.Edges {
			if e.Kind != KindCalls {
				t.Errorf("kind filter: got kind %q, want %q", e.Kind, KindCalls)
			}
		}
	}
}

func TestTraverse_TruncationFlag(t *testing.T) {
	// Create a star graph: center → 201 nodes
	edges := make([]Edge, 0, 201)
	for i := 0; i < 201; i++ {
		edges = append(edges, Edge{
			From: "center",
			To:   "node" + string(rune('A'+i%26)) + string(rune('0'+i/26)),
			Kind: KindCalls,
		})
	}
	res := Traverse(edges, "center", nil, 1, "out")
	if !res.Truncated {
		t.Error("expected Truncated=true for 201 one-hop paths")
	}
	if len(res.Paths) != maxPaths {
		t.Errorf("expected %d paths, got %d", maxPaths, len(res.Paths))
	}
}

func TestTraverse_MixedKinds(t *testing.T) {
	// 2-hop: call edge then spec-ref edge
	edges := []Edge{
		{From: "funcA", To: "funcB", Kind: KindCalls, File: "a.go", Line: 10},
		{From: "funcB", To: "US-001", Kind: KindSpecRef, File: "b.go", Line: 5},
	}
	res := Traverse(edges, "funcA", nil, 3, "out")
	// Path 1: funcA→funcB (1 edge)
	// Path 2: funcA→funcB→US-001 (2 edges)
	if len(res.Paths) != 2 {
		t.Errorf("mixed kinds: expected 2 paths, got %d", len(res.Paths))
	}
	if len(res.Paths[1].Edges) != 2 {
		t.Errorf("mixed kinds: 2nd path should have 2 edges, got %d", len(res.Paths[1].Edges))
	}
	if res.Paths[1].Edges[1].Kind != KindSpecRef {
		t.Errorf("2nd edge should be spec-ref, got %q", res.Paths[1].Edges[1].Kind)
	}
}

func TestTraverse_EdgeCounts(t *testing.T) {
	edges := []Edge{
		{From: "A", To: "B", Kind: KindCalls},
		{From: "A", To: "C", Kind: KindCalls},
		{From: "A", To: "key1", Kind: KindEnvKey},
	}
	res := Traverse(edges, "A", nil, 1, "out")
	if res.EdgeCounts[KindCalls] != 2 {
		t.Errorf("edge_counts[calls] = %d, want 2", res.EdgeCounts[KindCalls])
	}
	if res.EdgeCounts[KindEnvKey] != 1 {
		t.Errorf("edge_counts[env-key] = %d, want 1", res.EdgeCounts[KindEnvKey])
	}
}
