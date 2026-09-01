package catalog

import (
	"reflect"
	"strconv"
	"testing"
)

func TestNewSortsAndValidates(t *testing.T) {
	nodes := []Node{
		{ParentID: "", Slug: "b"},
		{ParentID: "", Slug: "a"},
		{ParentID: "a", Slug: "child"},
	}
	cat, err := New(nodes)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var ids []string
	for _, n := range cat.Nodes {
		ids = append(ids, n.ID())
	}
	want := []string{"a", "a/child", "b"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("Nodes order = %v, want %v", ids, want)
	}
}

func TestNewRejectsInvalidSlug(t *testing.T) {
	_, err := New([]Node{{Slug: "Bad Slug"}})
	if err == nil {
		t.Fatal("expected error for invalid slug")
	}
}

func TestCatalogGet(t *testing.T) {
	cat, err := New([]Node{{Slug: "a", Title: "A"}, {ParentID: "a", Slug: "b", Title: "B"}})
	if err != nil {
		t.Fatal(err)
	}
	n, ok := cat.Get("a/b")
	if !ok || n.Title != "B" {
		t.Errorf("Get(a/b) = %+v, %v", n, ok)
	}
	if _, ok := cat.Get("nope"); ok {
		t.Error("Get(nope) should not be found")
	}
}

func TestCatalogChildrenOrdering(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "z", Order: 1},
		{Slug: "a", Order: 1},
		{Slug: "m", Order: 0},
	})
	if err != nil {
		t.Fatal(err)
	}
	children := cat.Children("")
	var slugs []string
	for _, n := range children {
		slugs = append(slugs, n.Slug)
	}
	// Order 0 ("m") sorts before Order 1 group; within Order 1, "a" before "z".
	want := []string{"m", "a", "z"}
	if !reflect.DeepEqual(slugs, want) {
		t.Errorf("Children order = %v, want %v", slugs, want)
	}
}

func TestCatalogAncestors(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "a"},
		{ParentID: "a", Slug: "b"},
		{ParentID: "a/b", Slug: "c"},
	})
	if err != nil {
		t.Fatal(err)
	}
	anc := cat.Ancestors("a/b/c")
	var ids []string
	for _, n := range anc {
		ids = append(ids, n.ID())
	}
	want := []string{"a", "a/b"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("Ancestors = %v, want %v", ids, want)
	}
	if got := cat.Ancestors("a"); got != nil {
		t.Errorf("Ancestors(top-level) = %v, want nil", got)
	}
}

func TestCatalogDescendants(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "a"},
		{ParentID: "a", Slug: "b"},
		{ParentID: "a/b", Slug: "c"},
		{Slug: "other"},
	})
	if err != nil {
		t.Fatal(err)
	}
	desc := cat.Descendants("a")
	var ids []string
	for _, n := range desc {
		ids = append(ids, n.ID())
	}
	want := []string{"a/b", "a/b/c"}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("Descendants = %v, want %v", ids, want)
	}
}

func TestCatalogByDepth(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "a"},
		{ParentID: "a", Slug: "b"},
		{ParentID: "a/b", Slug: "c"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := cat.ByDepth(2, 2)
	if len(got) != 1 || got[0].ID() != "a/b" {
		t.Errorf("ByDepth(2,2) = %v, want [a/b]", got)
	}
}

func TestCatalogByFacet(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "with-facet", Facets: map[string][]string{"framework": {"react"}}},
		{Slug: "without-facet"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := cat.ByFacet(func(f map[string][]string) bool { return len(f) > 0 })
	if len(got) != 1 || got[0].Slug != "with-facet" {
		t.Errorf("ByFacet = %v, want [with-facet]", got)
	}
}

// TestArbitraryDepthNoSpecialCasing builds catalogs of two very different
// depths through the exact same loop and the exact same New/Children/
// Descendants/ByDepth API, proving depth is data, not a type or a
// constant (design §2.1).
func TestArbitraryDepthNoSpecialCasing(t *testing.T) {
	for _, depth := range []int{2, 10} {
		t.Run(depthName(depth), func(t *testing.T) {
			nodes := buildChain(depth)
			cat, err := New(nodes)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if len(cat.Nodes) != depth {
				t.Fatalf("len(Nodes) = %d, want %d", len(cat.Nodes), depth)
			}

			leafID := chainID(depth)
			leaf, ok := cat.Get(leafID)
			if !ok {
				t.Fatalf("leaf %q not found", leafID)
			}
			if got := Depth(leaf.ID()); got != depth {
				t.Errorf("Depth(leaf) = %d, want %d", got, depth)
			}

			// The root's descendant count is every other node in the chain.
			root := cat.Children("")
			if len(root) != 1 {
				t.Fatalf("expected exactly one root, got %d", len(root))
			}
			if got := len(cat.Descendants(root[0].ID())); got != depth-1 {
				t.Errorf("Descendants(root) = %d, want %d", got, depth-1)
			}

			// ByDepth at the deepest level returns exactly the leaf.
			deepest := cat.ByDepth(depth, depth)
			if len(deepest) != 1 || deepest[0].ID() != leafID {
				t.Errorf("ByDepth(%d,%d) = %v, want [%s]", depth, depth, deepest, leafID)
			}
		})
	}
}

// buildChain builds a straight-line chain of n nodes, each the sole child
// of the previous one: level-1, level-1/level-2, ... — via one generic
// loop, regardless of n.
func buildChain(n int) []Node {
	var nodes []Node
	parentID := ""
	for i := 1; i <= n; i++ {
		slug := levelSlug(i)
		nodes = append(nodes, Node{ParentID: parentID, Slug: slug, Title: slug})
		parentID = JoinID(parentID, slug)
	}
	return nodes
}

func chainID(n int) string {
	id := ""
	for i := 1; i <= n; i++ {
		id = JoinID(id, levelSlug(i))
	}
	return id
}

func levelSlug(i int) string {
	return "level-" + strconv.Itoa(i)
}

func depthName(d int) string {
	return "depth-" + strconv.Itoa(d)
}
