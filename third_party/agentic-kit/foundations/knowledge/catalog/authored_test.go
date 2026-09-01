package catalog

import "testing"

func TestFlattenNestedTree(t *testing.T) {
	roots := []AuthoredNode{
		{
			Slug: "foundations",
			Children: []AuthoredNode{
				{
					Slug: "knowledge",
					Children: []AuthoredNode{
						{Slug: "catalog", Title: "catalog", Facets: map[string][]string{"tier": {"stable"}}},
					},
				},
			},
		},
	}
	cat, err := Flatten(roots, "")
	if err != nil {
		t.Fatal(err)
	}
	leaf, ok := cat.Get("foundations/knowledge/catalog")
	if !ok {
		t.Fatal("expected flattened leaf to exist")
	}
	if leaf.Title != "catalog" {
		t.Errorf("Title = %q, want catalog", leaf.Title)
	}
	if len(cat.Nodes) != 3 {
		t.Errorf("expected 3 flattened nodes, got %d", len(cat.Nodes))
	}
}

func TestFlattenWithLayerNameGoesThroughBuild(t *testing.T) {
	roots := []AuthoredNode{{Slug: "hero", Title: "Hero"}}
	cat, err := Flatten(roots, "authored-layer")
	if err != nil {
		t.Fatal(err)
	}
	n, ok := cat.Get("hero")
	if !ok {
		t.Fatal("expected hero to exist")
	}
	if len(n.Provenance.ContributingLayers) != 1 || n.Provenance.ContributingLayers[0] != "authored-layer" {
		t.Errorf("expected origin layer authored-layer, got %v", n.Provenance.ContributingLayers)
	}
}

func TestLoadAuthoredJSONRoundTrip(t *testing.T) {
	data := []byte(`[
		{
			"slug": "foundations",
			"children": [
				{"slug": "knowledge", "title": "Knowledge", "children": [
					{"slug": "catalog", "title": "Catalog", "synopsis": "Catalogs that cannot lie."}
				]}
			]
		}
	]`)
	cat, err := LoadAuthoredJSON(data, "")
	if err != nil {
		t.Fatal(err)
	}
	n, ok := cat.Get("foundations/knowledge/catalog")
	if !ok || n.Synopsis != "Catalogs that cannot lie." {
		t.Errorf("unexpected node: %+v ok=%v", n, ok)
	}
}

func TestLoadAuthoredJSONInvalid(t *testing.T) {
	if _, err := LoadAuthoredJSON([]byte("not json"), ""); err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}
