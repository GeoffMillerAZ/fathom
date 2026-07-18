package sourceregistry_test

import (
	"testing"

	sr "github.com/geoffmilleraz/agentic-kit/foundations/evidence/source-registry"
)

func makeSource(id string, kind sr.Kind, uri string) sr.Source {
	return sr.Source{ID: id, Kind: kind, URI: uri}
}

// TestRegisterAndGet verifies a registered source is retrievable.
func TestRegisterAndGet(t *testing.T) {
	var reg sr.Registry
	src := makeSource("src-1", sr.KindMarkdown, "docs/README.md")
	if err := reg.Register(src); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := reg.Get("src-1")
	if !ok {
		t.Fatal("expected source to be found")
	}
	if got.ID != "src-1" || got.Kind != sr.KindMarkdown {
		t.Errorf("unexpected source: %+v", got)
	}
}

// TestGetMissing verifies Get returns false for unknown IDs.
func TestGetMissing(t *testing.T) {
	var reg sr.Registry
	_, ok := reg.Get("nonexistent")
	if ok {
		t.Fatal("expected found=false for unknown id")
	}
}

// TestRegisterEmptyIDError verifies an empty ID is rejected.
func TestRegisterEmptyIDError(t *testing.T) {
	var reg sr.Registry
	err := reg.Register(sr.Source{ID: "", Kind: sr.KindDoc, URI: "x"})
	if err == nil {
		t.Fatal("expected error for empty ID, got nil")
	}
}

// TestRegisterOverwrite verifies later registrations overwrite previous ones.
func TestRegisterOverwrite(t *testing.T) {
	var reg sr.Registry
	_ = reg.Register(makeSource("src-1", sr.KindMarkdown, "old.md"))
	_ = reg.Register(makeSource("src-1", sr.KindReadme, "new.md"))

	got, ok := reg.Get("src-1")
	if !ok {
		t.Fatal("expected source after overwrite")
	}
	if got.Kind != sr.KindReadme {
		t.Errorf("expected KindReadme after overwrite, got %q", got.Kind)
	}
}

// TestListDeterministicOrder verifies List returns sources sorted by ID.
func TestListDeterministicOrder(t *testing.T) {
	var reg sr.Registry
	_ = reg.Register(makeSource("src-c", sr.KindDoc, "c"))
	_ = reg.Register(makeSource("src-a", sr.KindMarkdown, "a"))
	_ = reg.Register(makeSource("src-b", sr.KindReadme, "b"))

	list := reg.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(list))
	}
	wantOrder := []string{"src-a", "src-b", "src-c"}
	for i, want := range wantOrder {
		if list[i].ID != want {
			t.Errorf("position %d: expected %q, got %q", i, want, list[i].ID)
		}
	}
}

// TestListEmpty verifies List returns nil for an empty registry.
func TestListEmpty(t *testing.T) {
	var reg sr.Registry
	if list := reg.List(); list != nil {
		t.Errorf("expected nil for empty registry, got %v", list)
	}
}

// TestKindConstants verifies all exported Kind constants have expected values.
func TestKindConstants(t *testing.T) {
	cases := []struct {
		kind sr.Kind
		want string
	}{
		{sr.KindMarkdown, "markdown"},
		{sr.KindReadme, "readme"},
		{sr.KindDoc, "doc"},
		{sr.KindOther, "other"},
	}
	for _, tc := range cases {
		if string(tc.kind) != tc.want {
			t.Errorf("Kind %q: expected %q", tc.kind, tc.want)
		}
	}
}
