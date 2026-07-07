package cluster_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/cluster"
)

// ---------------------------------------------------------------------------
// Cluster tests
// ---------------------------------------------------------------------------

func TestCluster_Empty(t *testing.T) {
	got := cluster.Cluster(nil)
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestCluster_SingleItem(t *testing.T) {
	items := []cluster.Item{{ID: "a", Hash: "h1"}}
	got := cluster.Cluster(items)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	if len(got[0]) != 1 || got[0][0] != "a" {
		t.Errorf("expected [[a]], got %v", got)
	}
}

func TestCluster_AllSameHash(t *testing.T) {
	items := []cluster.Item{
		{ID: "c", Hash: "same"},
		{ID: "a", Hash: "same"},
		{ID: "b", Hash: "same"},
	}
	got := cluster.Cluster(items)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	want := []string{"a", "b", "c"}
	if len(got[0]) != 3 {
		t.Fatalf("expected 3 IDs in group, got %d", len(got[0]))
	}
	for i, id := range want {
		if got[0][i] != id {
			t.Errorf("group[0][%d]: want %q, got %q", i, id, got[0][i])
		}
	}
}

func TestCluster_DistinctHashes(t *testing.T) {
	items := []cluster.Item{
		{ID: "x", Hash: "h1"},
		{ID: "y", Hash: "h2"},
		{ID: "z", Hash: "h3"},
	}
	got := cluster.Cluster(items)
	if len(got) != 3 {
		t.Fatalf("expected 3 groups (one per item), got %d", len(got))
	}
	// Each group should be a singleton.
	for _, g := range got {
		if len(g) != 1 {
			t.Errorf("expected singleton group, got %v", g)
		}
	}
}

func TestCluster_MixedHashes(t *testing.T) {
	items := []cluster.Item{
		{ID: "b", Hash: "h1"},
		{ID: "d", Hash: "h2"},
		{ID: "a", Hash: "h1"},
		{ID: "c", Hash: "h2"},
		{ID: "e", Hash: "h3"},
	}
	got := cluster.Cluster(items)
	if len(got) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(got))
	}
	// Groups sorted by first element: a < d < e.
	if got[0][0] != "a" || len(got[0]) != 2 {
		t.Errorf("first group: want [a b], got %v", got[0])
	}
	if got[1][0] != "c" || len(got[1]) != 2 {
		t.Errorf("second group: want [c d], got %v", got[1])
	}
	if got[2][0] != "e" || len(got[2]) != 1 {
		t.Errorf("third group: want [e], got %v", got[2])
	}
}

func TestCluster_Deterministic(t *testing.T) {
	items := []cluster.Item{
		{ID: "z", Hash: "h1"},
		{ID: "a", Hash: "h2"},
		{ID: "m", Hash: "h1"},
		{ID: "b", Hash: "h2"},
	}
	first := cluster.Cluster(items)
	second := cluster.Cluster(items)
	if len(first) != len(second) {
		t.Fatalf("non-deterministic: %v vs %v", first, second)
	}
	for i := range first {
		if len(first[i]) != len(second[i]) {
			t.Errorf("group[%d] length mismatch: %v vs %v", i, first[i], second[i])
		}
		for j := range first[i] {
			if first[i][j] != second[i][j] {
				t.Errorf("non-deterministic at [%d][%d]: %q vs %q", i, j, first[i][j], second[i][j])
			}
		}
	}
}

func TestCluster_EmptyHashItems(t *testing.T) {
	// Items with empty hash each get their own group keyed by "".
	items := []cluster.Item{
		{ID: "a", Hash: ""},
		{ID: "b", Hash: ""},
	}
	got := cluster.Cluster(items)
	// Both share hash "" → grouped together.
	if len(got) != 1 {
		t.Fatalf("expected 1 group for items sharing empty hash, got %d", len(got))
	}
	if len(got[0]) != 2 {
		t.Errorf("expected 2 items in group, got %v", got[0])
	}
}

// ---------------------------------------------------------------------------
// NormalizedHash tests
// ---------------------------------------------------------------------------

func TestNormalizedHash_Same(t *testing.T) {
	a := cluster.NormalizedHash("func foo() { return 1 }")
	b := cluster.NormalizedHash("func foo() { return 1 }")
	if a != b {
		t.Errorf("same input produced different hashes: %q vs %q", a, b)
	}
}

func TestNormalizedHash_WhitespaceInsensitive(t *testing.T) {
	a := cluster.NormalizedHash("func foo()  {  return  1  }")
	b := cluster.NormalizedHash("func foo() { return 1 }")
	if a != b {
		t.Errorf("whitespace difference should not affect hash: %q vs %q", a, b)
	}
}

func TestNormalizedHash_IdentifierInsensitive(t *testing.T) {
	// Different identifiers → same structural hash.
	a := cluster.NormalizedHash("func foo(x int) int { return x + 1 }")
	b := cluster.NormalizedHash("func bar(y int) int { return y + 1 }")
	if a != b {
		t.Errorf("identifier rename should not affect hash: %q vs %q", a, b)
	}
}

func TestNormalizedHash_StructuralDifference(t *testing.T) {
	a := cluster.NormalizedHash("func foo() { return 1 }")
	b := cluster.NormalizedHash("func foo() { return 2 }")
	if a == b {
		t.Errorf("structurally different code should produce different hashes")
	}
}

func TestNormalizedHash_NotEmpty(t *testing.T) {
	h := cluster.NormalizedHash("anything")
	if h == "" {
		t.Error("expected non-empty hash")
	}
}

func TestNormalizedHash_Length(t *testing.T) {
	h := cluster.NormalizedHash("func main() {}")
	// SHA-256 hex = 64 chars.
	if len(h) != 64 {
		t.Errorf("expected 64-char hex hash, got len=%d: %q", len(h), h)
	}
}

func TestNormalizedHash_Deterministic(t *testing.T) {
	src := "func process(data []byte) error { return nil }"
	a := cluster.NormalizedHash(src)
	b := cluster.NormalizedHash(src)
	if a != b {
		t.Errorf("non-deterministic hash: %q vs %q", a, b)
	}
}
