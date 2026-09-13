package indexbench

import (
	"path/filepath"
	"testing"
)

func TestLoadGraphifyEdges_KeepsCallsDropsRest(t *testing.T) {
	set, err := LoadGraphifyEdges(filepath.Join("testdata", "graphify-fixture.json"))
	if err != nil {
		t.Fatalf("LoadGraphifyEdges: %v", err)
	}
	// n2->n1 (calls) and n3->n1 (calls) resolve; n3->n2 (references) is
	// dropped with its count — in v0.9.61 "references" is function→type,
	// not a call the truth can judge; the n2->n9 "calls" edge names a
	// target node id not in the node table. Sorted by (To, From).
	want := []Edge{
		{From: "Bar", To: "Foo", Kind: EdgeKindCall},
		{From: "Baz", To: "Foo", Kind: EdgeKindCall},
	}
	if set.DroppedKinds["references"] != 1 {
		t.Errorf("references must be dropped with a count, got %v", set.DroppedKinds)
	}
	if len(set.Edges) != len(want) {
		t.Fatalf("got %d edges, want %d: %+v", len(set.Edges), len(want), set.Edges)
	}
	for i, e := range want {
		if set.Edges[i] != e {
			t.Errorf("edge %d = %+v, want %+v", i, set.Edges[i], e)
		}
	}
}

func TestLoadGraphifyEdges_NamesDroppedKinds(t *testing.T) {
	set, err := LoadGraphifyEdges(filepath.Join("testdata", "graphify-fixture.json"))
	if err != nil {
		t.Fatalf("LoadGraphifyEdges: %v", err)
	}
	if set.DroppedKinds["imports_from"] != 1 {
		t.Errorf("DroppedKinds[imports_from] = %d, want 1", set.DroppedKinds["imports_from"])
	}
	if set.DroppedKinds["contains"] != 1 {
		t.Errorf("DroppedKinds[contains] = %d, want 1", set.DroppedKinds["contains"])
	}
	if set.DroppedKinds["unresolved_target"] != 1 {
		t.Errorf("DroppedKinds[unresolved_target] = %d, want 1 (the n2->n9 edge)", set.DroppedKinds["unresolved_target"])
	}
}

func TestLoadGraphifyEdges_MissingFile(t *testing.T) {
	if _, err := LoadGraphifyEdges(filepath.Join("testdata", "does-not-exist.json")); err == nil {
		t.Fatal("want an error for a missing graph.json, got nil")
	}
}

func TestLoadGraphifyEdges_ScoresAgainstTruth(t *testing.T) {
	// Fit the fixture's edges against a truth where Foo really is called by
	// Bar and Baz, but Bar is not truly referenced by Baz's file — an
	// end-to-end check that LoadGraphifyEdges' output is consumable by
	// CompareEdges without adaptation.
	set, err := LoadGraphifyEdges(filepath.Join("testdata", "graphify-fixture.json"))
	if err != nil {
		t.Fatalf("LoadGraphifyEdges: %v", err)
	}
	truth := smallTruth()
	got := CompareEdges(set.Edges, truth)
	if got.Queries == 0 {
		t.Fatal("want at least one adjudicable query from the fixture graph")
	}
}
