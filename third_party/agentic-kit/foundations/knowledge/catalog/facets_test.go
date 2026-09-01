package catalog

import (
	"reflect"
	"testing"
)

// TestVariantsThreeAxesNoTreeLevels proves the design's central facets
// claim: one item with 3 facet axes yields the full expected variant set
// (the Cartesian product) WITHOUT creating any child Node for an axis —
// Variants is computed purely from Node.Facets, and the item stays a
// single Node in the catalog throughout.
func TestVariantsThreeAxesNoTreeLevels(t *testing.T) {
	item := Node{
		Slug:  "hero-section",
		Title: "Hero section",
		Facets: map[string][]string{
			"framework": {"react", "html", "vue"},
			"version":   {"4", "3"},
			"mode":      {"light", "dark", "system"},
		},
	}
	cat, err := New([]Node{item})
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.Nodes) != 1 {
		t.Fatalf("expected exactly one Node in the catalog, got %d", len(cat.Nodes))
	}

	variants := Variants(item)
	wantCount := 3 * 2 * 3
	if len(variants) != wantCount {
		t.Fatalf("len(Variants) = %d, want %d (3 frameworks x 2 versions x 3 modes)", len(variants), wantCount)
	}

	seen := map[string]bool{}
	for _, v := range variants {
		if v.NodeID != item.ID() {
			t.Errorf("variant.NodeID = %q, want %q", v.NodeID, item.ID())
		}
		if len(v.Values) != 3 {
			t.Errorf("variant.Values has %d axes, want 3", len(v.Values))
		}
		key := v.Values["framework"] + "|" + v.Values["version"] + "|" + v.Values["mode"]
		if seen[key] {
			t.Errorf("duplicate variant %s", key)
		}
		seen[key] = true
	}
	if !seen["react|4|light"] || !seen["vue|3|system"] {
		t.Errorf("expected specific combinations present, got %v", seen)
	}

	// Deterministic ordering: same call twice, same order.
	again := Variants(item)
	if !reflect.DeepEqual(variants, again) {
		t.Error("Variants must be deterministic across repeated calls")
	}
}

func TestVariantsNoFacets(t *testing.T) {
	item := Node{Slug: "plain-item", Title: "Plain"}
	variants := Variants(item)
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant for a facet-less item, got %d", len(variants))
	}
	if variants[0].NodeID != item.ID() || len(variants[0].Values) != 0 {
		t.Errorf("unexpected variant: %+v", variants[0])
	}
}

func TestVariantsEmptyAxisYieldsNoVariants(t *testing.T) {
	item := Node{Slug: "broken", Facets: map[string][]string{"framework": {}}}
	if got := Variants(item); got != nil {
		t.Errorf("expected nil variants when an axis has zero values, got %v", got)
	}
}
