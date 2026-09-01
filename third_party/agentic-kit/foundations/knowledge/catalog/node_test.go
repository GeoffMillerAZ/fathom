package catalog

import (
	"testing"
	"time"
)

func TestJoinID(t *testing.T) {
	cases := []struct {
		parentID, slug, want string
	}{
		{"", "foundations", "foundations"},
		{"foundations", "knowledge", "foundations/knowledge"},
		{"foundations/knowledge", "catalog", "foundations/knowledge/catalog"},
	}
	for _, c := range cases {
		if got := JoinID(c.parentID, c.slug); got != c.want {
			t.Errorf("JoinID(%q, %q) = %q, want %q", c.parentID, c.slug, got, c.want)
		}
	}
}

func TestNodeID(t *testing.T) {
	n := Node{ParentID: "foundations/knowledge", Slug: "catalog"}
	if got, want := n.ID(), "foundations/knowledge/catalog"; got != want {
		t.Errorf("ID() = %q, want %q", got, want)
	}
}

func TestDepth(t *testing.T) {
	cases := []struct {
		id   string
		want int
	}{
		{"", 0},
		{"foundations", 1},
		{"foundations/knowledge", 2},
		{"foundations/knowledge/catalog", 3},
	}
	for _, c := range cases {
		if got := Depth(c.id); got != c.want {
			t.Errorf("Depth(%q) = %d, want %d", c.id, got, c.want)
		}
	}
}

func TestValidateSlug(t *testing.T) {
	valid := []string{"a", "abc", "foo-bar", "foo-bar-baz", "html", "3", "v3-4"}
	for _, s := range valid {
		if err := ValidateSlug(s); err != nil {
			t.Errorf("ValidateSlug(%q) = %v, want nil", s, err)
		}
	}
	invalid := []string{"", "Foo", "foo_bar", "foo bar", "-foo", "foo-", "foo--bar", "/foo"}
	for _, s := range invalid {
		if err := ValidateSlug(s); err == nil {
			t.Errorf("ValidateSlug(%q) = nil, want error", s)
		}
	}
}

func TestNodeHashStability(t *testing.T) {
	n1 := Node{
		ParentID: "a", Slug: "b", Title: "Widget",
		Facets:     map[string][]string{"framework": {"react", "html"}, "version": {"4", "3"}},
		Provenance: Provenance{Source: "src", Extractor: "ext", Watermark: "wm"},
	}
	n2 := Node{
		ParentID: "a", Slug: "b", Title: "Widget",
		// Same facets, different construction/iteration order.
		Facets:     map[string][]string{"version": {"3", "4"}, "framework": {"html", "react"}},
		Provenance: Provenance{Source: "src", Extractor: "ext", Watermark: "wm"},
	}
	if n1.Hash() != n2.Hash() {
		t.Errorf("Hash should be independent of facet map/slice construction order: %s != %s", n1.Hash(), n2.Hash())
	}
}

func TestNodeHashChangesOnIdentityFields(t *testing.T) {
	base := Node{ParentID: "a", Slug: "b", Title: "Widget", Provenance: Provenance{Source: "src"}}

	titleChanged := base
	titleChanged.Title = "Gadget"
	if base.Hash() == titleChanged.Hash() {
		t.Error("Hash must change when Title changes")
	}

	facetChanged := base
	facetChanged.Facets = map[string][]string{"framework": {"react"}}
	if base.Hash() == facetChanged.Hash() {
		t.Error("Hash must change when Facets change")
	}

	sourceChanged := base
	sourceChanged.Provenance.Source = "other"
	if base.Hash() == sourceChanged.Hash() {
		t.Error("Hash must change when Provenance.Source changes")
	}
}

func TestNodeHashIgnoresPresentationAndMergeBookkeeping(t *testing.T) {
	base := Node{ParentID: "a", Slug: "b", Title: "Widget", Provenance: Provenance{Source: "src"}}

	synopsisChanged := base
	synopsisChanged.Synopsis = "a fine widget"
	if base.Hash() != synopsisChanged.Hash() {
		t.Error("Hash must NOT change when only Synopsis changes")
	}

	orderChanged := base
	orderChanged.Order = 5
	if base.Hash() != orderChanged.Hash() {
		t.Error("Hash must NOT change when only Order changes")
	}

	hiddenChanged := base
	hiddenChanged.Hidden = true
	if base.Hash() != hiddenChanged.Hash() {
		t.Error("Hash must NOT change when only Hidden changes")
	}

	observedChanged := base
	observedChanged.Provenance.ObservedAt = time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	if base.Hash() != observedChanged.Hash() {
		t.Error("Hash must NOT change when only Provenance.ObservedAt changes")
	}

	layerBookkeepingChanged := base
	layerBookkeepingChanged.Provenance.ContributingLayers = []string{"base", "overlay"}
	layerBookkeepingChanged.Provenance.Overridden = true
	layerBookkeepingChanged.Provenance.OverriddenBy = "overlay"
	if base.Hash() != layerBookkeepingChanged.Hash() {
		t.Error("Hash must NOT change when only merge bookkeeping changes")
	}
}

func TestProvenanceFresh(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	fresh := Provenance{ObservedAt: now.Add(-1 * time.Hour)}
	if !fresh.Fresh(now, 2*time.Hour) {
		t.Error("expected Fresh to be true within maxAge")
	}
	stale := Provenance{ObservedAt: now.Add(-3 * time.Hour)}
	if stale.Fresh(now, 2*time.Hour) {
		t.Error("expected Fresh to be false beyond maxAge")
	}
	unset := Provenance{}
	if unset.Fresh(now, 100*time.Hour) {
		t.Error("expected Fresh to be false when ObservedAt is zero")
	}
}
