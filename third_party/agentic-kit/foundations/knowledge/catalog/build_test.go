package catalog

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// TestBuildBaseAndTwoOverlaysSecondOverridesFirst covers the union-loader
// case: a base layer, an overlay that overrides one field, and a second
// overlay that overrides the same field again — the later layer must win,
// and the node must record both layers as contributors plus the LAST one
// as OverriddenBy.
func TestBuildBaseAndTwoOverlaysSecondOverridesFirst(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "widgets", Title: "Widgets (base)", Provenance: Provenance{Source: "base-scan"}},
	}}
	overlay1 := Layer{Name: "overlay-1", Nodes: []Node{
		{Slug: "widgets", Title: "Widgets (overlay-1)"},
	}}
	overlay2 := Layer{Name: "overlay-2", Nodes: []Node{
		{Slug: "widgets", Title: "Widgets (overlay-2)"},
	}}

	cat, err := Build(base, overlay1, overlay2)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	n, ok := cat.Get("widgets")
	if !ok {
		t.Fatal("widgets not found")
	}
	if n.Title != "Widgets (overlay-2)" {
		t.Errorf("Title = %q, want last-layer-wins %q", n.Title, "Widgets (overlay-2)")
	}
	if !n.Provenance.Overridden {
		t.Error("expected Overridden = true")
	}
	if n.Provenance.OverriddenBy != "overlay-2" {
		t.Errorf("OverriddenBy = %q, want overlay-2", n.Provenance.OverriddenBy)
	}
	want := []string{"base", "overlay-1", "overlay-2"}
	if !reflect.DeepEqual(n.Provenance.ContributingLayers, want) {
		t.Errorf("ContributingLayers = %v, want %v", n.Provenance.ContributingLayers, want)
	}
	// Provenance.Source from base must survive since neither overlay set it.
	if n.Provenance.Source != "base-scan" {
		t.Errorf("Source = %q, want base-scan to survive", n.Provenance.Source)
	}
}

// TestBuildOverlayExtendsFacetsWithoutOverride: adding facet values via an
// overlay must NOT set Overridden — extend is additive, not a lie about
// what an earlier layer said.
func TestBuildOverlayExtendsFacetsWithoutOverride(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "hero", Facets: map[string][]string{"framework": {"react"}}},
	}}
	overlay := Layer{Name: "overlay", Nodes: []Node{
		{Slug: "hero", Facets: map[string][]string{"framework": {"vue"}, "mode": {"dark"}}},
	}}

	cat, err := Build(base, overlay)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	n, _ := cat.Get("hero")
	wantFramework := []string{"react", "vue"}
	if !reflect.DeepEqual(n.Facets["framework"], wantFramework) {
		t.Errorf("Facets[framework] = %v, want %v", n.Facets["framework"], wantFramework)
	}
	if !reflect.DeepEqual(n.Facets["mode"], []string{"dark"}) {
		t.Errorf("Facets[mode] = %v, want [dark]", n.Facets["mode"])
	}
	if n.Provenance.Overridden {
		t.Error("extending facets must not set Overridden")
	}
}

// TestBuildOverlayThreeDeepUnderTwoDeepBase: an overlay introduces a path
// segment the base never had — arbitrary depth applies to overlays too.
func TestBuildOverlayThreeDeepUnderTwoDeepBase(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "foundations"},
		{ParentID: "foundations", Slug: "knowledge"},
	}}
	overlay := Layer{Name: "overlay", Nodes: []Node{
		// New intermediate group the base never declared, plus a leaf under it.
		{ParentID: "foundations/knowledge", Slug: "catalog"},
		{ParentID: "foundations/knowledge/catalog", Slug: "node-go"},
	}}

	cat, err := Build(base, overlay)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	leaf, ok := cat.Get("foundations/knowledge/catalog/node-go")
	if !ok {
		t.Fatal("expected the overlay-introduced 3-deep-under-base leaf to exist")
	}
	if Depth(leaf.ID()) != 4 {
		t.Errorf("Depth(leaf) = %d, want 4", Depth(leaf.ID()))
	}
	if origin := originLayer(leaf); origin != "overlay" {
		t.Errorf("origin layer = %q, want overlay", origin)
	}
}

// TestBuildHiddenNodeStillCountedByVerify: an overlay marks a base node
// Hidden; the merged catalog still contains it, and Verify still checks
// it (rendering-out is not deleting).
func TestBuildHiddenNodeStillCountedByVerify(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "deprecated-tool", Title: "Deprecated tool"},
	}}
	overlay := Layer{Name: "overlay", Nodes: []Node{
		{Slug: "deprecated-tool", Hidden: true},
	}}

	built, err := Build(base, overlay)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	n, ok := built.Get("deprecated-tool")
	if !ok || !n.Hidden {
		t.Fatalf("expected deprecated-tool to still be present and Hidden, got %+v ok=%v", n, ok)
	}

	// Verify against a "published" catalog that never saw the hide: the
	// node is present in both by ID with the same Hash (Hidden is
	// excluded from Hash, deliberately — hiding is presentation, not
	// identity), so it must NOT show up as any kind of drift, but it
	// still counted toward CheckedBuilt/CheckedPublished.
	published, err := New([]Node{{Slug: "deprecated-tool", Title: "Deprecated tool"}})
	if err != nil {
		t.Fatal(err)
	}
	drift, err := Verify(built, published)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !drift.Clean() {
		t.Errorf("expected clean drift (Hidden is not an identity change), got %+v", drift)
	}
	if drift.CheckedBuilt != 1 || drift.CheckedPublished != 1 {
		t.Errorf("expected both sides to count the hidden node, got built=%d published=%d", drift.CheckedBuilt, drift.CheckedPublished)
	}

	// And RenderMarkdown, by default, renders it out.
	md := RenderMarkdown(built, MarkdownOptions{})
	if strings.Contains(md, "deprecated-tool") {
		t.Error("expected Hidden node to be rendered out by default")
	}
	mdWithHidden := RenderMarkdown(built, MarkdownOptions{IncludeHidden: true})
	if !strings.Contains(mdWithHidden, "deprecated-tool") {
		t.Error("expected Hidden node to appear when IncludeHidden is set")
	}
}

// TestBuildConflictingFacetValuesResolveDeterministicallyByOrder: two
// overlays contribute different values for the same axis; the union must
// be identical (sorted) regardless of which order the layers list their
// own values in.
func TestBuildConflictingFacetValuesResolveDeterministicallyByOrder(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "hero", Facets: map[string][]string{"mode": {"light"}}},
	}}
	overlayA := Layer{Name: "overlay-a", Nodes: []Node{
		{Slug: "hero", Facets: map[string][]string{"mode": {"dark", "system"}}},
	}}
	overlayB := Layer{Name: "overlay-b", Nodes: []Node{
		{Slug: "hero", Facets: map[string][]string{"mode": {"system", "dark"}}}, // same values, reversed
	}}

	cat1, err := Build(base, overlayA, overlayB)
	if err != nil {
		t.Fatal(err)
	}
	cat2, err := Build(base, overlayB, overlayA)
	if err != nil {
		t.Fatal(err)
	}
	n1, _ := cat1.Get("hero")
	n2, _ := cat2.Get("hero")
	want := []string{"dark", "light", "system"}
	if !reflect.DeepEqual(n1.Facets["mode"], want) {
		t.Errorf("cat1 Facets[mode] = %v, want %v", n1.Facets["mode"], want)
	}
	if !reflect.DeepEqual(n2.Facets["mode"], want) {
		t.Errorf("cat2 Facets[mode] = %v, want %v", n2.Facets["mode"], want)
	}
}

func TestBuildRejectsEmptyLayerName(t *testing.T) {
	_, err := Build(Layer{Name: "", Nodes: []Node{{Slug: "x"}}})
	if err == nil {
		t.Fatal("expected error for empty layer Name")
	}
}

func TestBuildRejectsDuplicateWithinOneLayer(t *testing.T) {
	_, err := Build(Layer{Name: "l", Nodes: []Node{{Slug: "x"}, {Slug: "x"}}})
	if err == nil {
		t.Fatal("expected error for duplicate node within one layer")
	}
}

// TestBuildRefusesStableKeyCollisionAcrossLayers is the realistic path
// the coordinator flagged: an overlay adds a node whose StableKey already
// exists in the base, under a DIFFERENT ID. Left unchecked, this would
// silently corrupt Verify's StableKey index (one of the two nodes would
// vanish from lookup, and a Moved entry could be reported against the
// wrong node). Build must refuse instead, naming both node IDs and both
// layers.
func TestBuildRefusesStableKeyCollisionAcrossLayers(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "original-widget", Title: "Widget", StableKey: "widget-key"},
	}}
	overlay := Layer{Name: "overlay", Nodes: []Node{
		// A different node (unrelated path) that happens to reuse the same
		// StableKey — an authoring mistake, not a rename (a rename would
		// keep it as one Verify-side Moved concern; this is two nodes
		// simultaneously claiming to BE the same stable identity).
		{Slug: "unrelated-widget", Title: "Unrelated widget", StableKey: "widget-key"},
	}}

	_, err := Build(base, overlay)
	if err == nil {
		t.Fatal("expected Build to refuse a cross-layer StableKey collision")
	}
	var collErr *LayerStableKeyCollisionError
	if !errors.As(err, &collErr) {
		t.Fatalf("expected a *LayerStableKeyCollisionError, got %T: %v", err, err)
	}
	if collErr.StableKey != "widget-key" {
		t.Errorf("StableKey = %q, want widget-key", collErr.StableKey)
	}
	if collErr.ExistingID != "original-widget" || collErr.ExistingLayer != "base" {
		t.Errorf("existing = %q/%q, want original-widget/base", collErr.ExistingID, collErr.ExistingLayer)
	}
	if collErr.IncomingID != "unrelated-widget" || collErr.IncomingLayer != "overlay" {
		t.Errorf("incoming = %q/%q, want unrelated-widget/overlay", collErr.IncomingID, collErr.IncomingLayer)
	}
	// Both layer names must actually appear in the error text, per the ask.
	msg := err.Error()
	if !strings.Contains(msg, "base") || !strings.Contains(msg, "overlay") {
		t.Errorf("expected both layer names in the error message, got: %s", msg)
	}
}

// TestBuildRefusesStableKeyCollisionWithinOneLayer: the same ambiguity is
// reachable purely from a hand-authored base layer with an authoring
// mistake — no overlay required.
func TestBuildRefusesStableKeyCollisionWithinOneLayer(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "a", StableKey: "dup"},
		{Slug: "b", StableKey: "dup"},
	}}
	_, err := Build(base)
	if err == nil {
		t.Fatal("expected Build to refuse a same-layer StableKey collision")
	}
	var collErr *LayerStableKeyCollisionError
	if !errors.As(err, &collErr) {
		t.Fatalf("expected a *LayerStableKeyCollisionError, got %T: %v", err, err)
	}
}

// TestBuildAllowsSameLayerReassertingItsOwnStableKey: re-listing the same
// node (same ID) with an unchanged StableKey across a layer boundary — an
// overlay that EXTENDs a base node's facets while also repeating its
// StableKey — is not a collision.
func TestBuildAllowsSameLayerReassertingItsOwnStableKey(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "widget", Title: "Widget", StableKey: "widget-key", Facets: map[string][]string{"framework": {"react"}}},
	}}
	overlay := Layer{Name: "overlay", Nodes: []Node{
		{Slug: "widget", StableKey: "widget-key", Facets: map[string][]string{"framework": {"vue"}}},
	}}
	cat, err := Build(base, overlay)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	n, _ := cat.Get("widget")
	if n.StableKey != "widget-key" {
		t.Errorf("StableKey = %q, want widget-key", n.StableKey)
	}
}
