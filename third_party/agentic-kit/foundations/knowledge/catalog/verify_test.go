package catalog

import (
	"errors"
	"testing"
)

func mustVerify(t *testing.T, built, published Catalog) Drift {
	t.Helper()
	d, err := Verify(built, published)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	return d
}

func TestVerifyClean(t *testing.T) {
	cat, err := New([]Node{{Slug: "a", Title: "A"}})
	if err != nil {
		t.Fatal(err)
	}
	d := mustVerify(t, cat, cat)
	if !d.Clean() {
		t.Errorf("expected clean, got %+v", d)
	}
	if d.CheckedBuilt != 1 || d.CheckedPublished != 1 {
		t.Errorf("expected both sides checked, got %+v", d)
	}
}

func TestVerifyMissing(t *testing.T) {
	built, _ := New(nil)
	published, _ := New([]Node{{Slug: "gone", Title: "Gone"}})
	d := mustVerify(t, built, published)
	if len(d.Missing) != 1 || d.Missing[0].ID != "gone" {
		t.Errorf("expected one Missing(gone), got %+v", d.Missing)
	}
	if len(d.Extra) != 0 || len(d.Changed) != 0 || len(d.Moved) != 0 {
		t.Errorf("expected only Missing, got %+v", d)
	}
}

func TestVerifyExtra(t *testing.T) {
	built, _ := New([]Node{{Slug: "new", Title: "New"}})
	published, _ := New(nil)
	d := mustVerify(t, built, published)
	if len(d.Extra) != 1 || d.Extra[0].ID != "new" {
		t.Errorf("expected one Extra(new), got %+v", d.Extra)
	}
	if len(d.Missing) != 0 || len(d.Changed) != 0 || len(d.Moved) != 0 {
		t.Errorf("expected only Extra, got %+v", d)
	}
}

func TestVerifyChanged(t *testing.T) {
	built, _ := New([]Node{{Slug: "a", Title: "New title"}})
	published, _ := New([]Node{{Slug: "a", Title: "Old title"}})
	d := mustVerify(t, built, published)
	if len(d.Changed) != 1 || d.Changed[0].ID != "a" {
		t.Errorf("expected one Changed(a), got %+v", d.Changed)
	}
	if len(d.Missing) != 0 || len(d.Extra) != 0 || len(d.Moved) != 0 {
		t.Errorf("expected only Changed, got %+v", d)
	}
}

// TestVerifyMovedNodeNotDoubleReported is the drift-kind test the design
// calls out for the most attention: a node renamed since publish (same
// StableKey, different ID) must be reported as exactly one Moved entry
// — never as a Missing (old path) PLUS an Extra (new path). This is also
// the regression the duplicate-StableKey refusal must not break: a
// single, legitimate shared key must still resolve to exactly one Moved.
func TestVerifyMovedNodeNotDoubleReported(t *testing.T) {
	built, err := New([]Node{{Slug: "new-name", Title: "Thing", StableKey: "thing-key"}})
	if err != nil {
		t.Fatal(err)
	}
	published, err := New([]Node{{Slug: "old-name", Title: "Thing", StableKey: "thing-key"}})
	if err != nil {
		t.Fatal(err)
	}
	d := mustVerify(t, built, published)

	if len(d.Moved) != 1 {
		t.Fatalf("expected exactly one Moved entry, got %d: %+v", len(d.Moved), d.Moved)
	}
	m := d.Moved[0]
	if m.StableKey != "thing-key" || m.From != "old-name" || m.To != "new-name" {
		t.Errorf("unexpected Moved entry: %+v", m)
	}
	if len(d.Missing) != 0 {
		t.Errorf("moved node must NOT also appear in Missing, got %+v", d.Missing)
	}
	if len(d.Extra) != 0 {
		t.Errorf("moved node must NOT also appear in Extra, got %+v", d.Extra)
	}
	if d.Clean() {
		t.Error("a Moved entry means Drift is not Clean")
	}
}

// TestVerifyUnrelatedDeleteAndAddStillReportBoth: without a shared
// StableKey, an unrelated deletion and an unrelated addition must NOT be
// collapsed into a Moved entry — Moved requires the shared key.
func TestVerifyUnrelatedDeleteAndAddStillReportBoth(t *testing.T) {
	built, _ := New([]Node{{Slug: "brand-new", Title: "Brand new"}})
	published, _ := New([]Node{{Slug: "long-gone", Title: "Long gone"}})
	d := mustVerify(t, built, published)
	if len(d.Moved) != 0 {
		t.Errorf("expected no Moved without a shared StableKey, got %+v", d.Moved)
	}
	if len(d.Missing) != 1 || d.Missing[0].ID != "long-gone" {
		t.Errorf("expected Missing(long-gone), got %+v", d.Missing)
	}
	if len(d.Extra) != 1 || d.Extra[0].ID != "brand-new" {
		t.Errorf("expected Extra(brand-new), got %+v", d.Extra)
	}
}

// TestDriftForLayerDistinguishesBaseFromOverlay proves per-project
// extension's source-aware Verify: base drift and overlay drift are
// reported as distinguishable findings, never one aggregate.
func TestDriftForLayerDistinguishesBaseFromOverlay(t *testing.T) {
	base := Layer{Name: "base", Nodes: []Node{
		{Slug: "base-item", Title: "Base item"},
	}}
	overlay := Layer{Name: "overlay", Nodes: []Node{
		{Slug: "overlay-item", Title: "Overlay item"},
	}}
	built, err := Build(base, overlay)
	if err != nil {
		t.Fatal(err)
	}

	// Published only ever knew about the base item — the overlay item is
	// "extra" and must be attributed to the overlay layer, not the base.
	published, err := New([]Node{{Slug: "base-item", Title: "Base item"}})
	if err != nil {
		t.Fatal(err)
	}

	d := mustVerify(t, built, published)
	if !d.Clean() && len(d.Extra) != 1 {
		t.Fatalf("expected exactly one Extra finding, got %+v", d)
	}

	baseDrift := d.ForLayer("base")
	if !baseDrift.Clean() {
		t.Errorf("expected the base layer's drift to be clean, got %+v", baseDrift)
	}
	overlayDrift := d.ForLayer("overlay")
	if overlayDrift.Clean() {
		t.Error("expected the overlay layer's drift to be non-clean")
	}
	if len(overlayDrift.Extra) != 1 || overlayDrift.Extra[0].ID != "overlay-item" {
		t.Errorf("expected overlay-item attributed to the overlay layer, got %+v", overlayDrift.Extra)
	}
}

// TestVerifyRefusesDuplicateStableKeyInBuilt: two built nodes sharing a
// StableKey makes the built-side key index ambiguous. Verify must refuse
// (a *DuplicateStableKeyError naming the key and both IDs) rather than
// pick one node's identity for the other, which could report a Moved
// entry against the wrong node or hide a genuine rename.
func TestVerifyRefusesDuplicateStableKeyInBuilt(t *testing.T) {
	built := Catalog{Nodes: []Node{
		{Slug: "a", Title: "A", StableKey: "dup-key"},
		{Slug: "b", Title: "B", StableKey: "dup-key"},
	}}
	published, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Verify(built, published)
	if err == nil {
		t.Fatal("expected Verify to refuse on a duplicate StableKey in built")
	}
	var dupErr *DuplicateStableKeyError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected a *DuplicateStableKeyError, got %T: %v", err, err)
	}
	if dupErr.Side != "built" {
		t.Errorf("Side = %q, want built", dupErr.Side)
	}
	if len(dupErr.Duplicates) != 1 || dupErr.Duplicates[0].StableKey != "dup-key" {
		t.Fatalf("unexpected Duplicates: %+v", dupErr.Duplicates)
	}
	if got, want := dupErr.Duplicates[0].IDs, []string{"a", "b"}; !stringSlicesEqual(got, want) {
		t.Errorf("colliding IDs = %v, want %v", got, want)
	}
	if err.Error() == "" {
		t.Error("expected a non-empty error message naming the key")
	}
}

// TestVerifyRefusesDuplicateStableKeyInPublished mirrors the built-side
// case on the published side, so the refusal is symmetric.
func TestVerifyRefusesDuplicateStableKeyInPublished(t *testing.T) {
	built, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	published := Catalog{Nodes: []Node{
		{Slug: "a", Title: "A", StableKey: "dup-key"},
		{Slug: "b", Title: "B", StableKey: "dup-key"},
	}}

	_, err = Verify(built, published)
	if err == nil {
		t.Fatal("expected Verify to refuse on a duplicate StableKey in published")
	}
	var dupErr *DuplicateStableKeyError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected a *DuplicateStableKeyError, got %T: %v", err, err)
	}
	if dupErr.Side != "published" {
		t.Errorf("Side = %q, want published", dupErr.Side)
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
