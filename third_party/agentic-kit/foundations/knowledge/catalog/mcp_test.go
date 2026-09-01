package catalog

import "testing"

func TestToolSpecs(t *testing.T) {
	specs := ToolSpecs()
	if len(specs) != 3 {
		t.Fatalf("expected 3 tool specs, got %d", len(specs))
	}
	names := map[string]bool{}
	for _, s := range specs {
		if s.Description == "" {
			t.Errorf("%s: expected a non-empty Description", s.Name)
		}
		names[s.Name] = true
	}
	for _, want := range []string{"catalog_search", "catalog_get", "catalog_tree"} {
		if !names[want] {
			t.Errorf("expected tool spec %q", want)
		}
	}
}

func TestSearchHandler(t *testing.T) {
	cat, _ := New([]Node{{Slug: "hero", Title: "Hero section"}})
	handler := SearchHandler(cat, NewBM25Ranker())
	out := handler("hero")
	if out.NoMatch() {
		t.Fatal("expected a match")
	}
}

func TestGetHandler(t *testing.T) {
	cat, _ := New([]Node{{Slug: "a", Title: "A"}, {ParentID: "a", Slug: "b"}})
	handler := GetHandler(cat)

	found := handler("a")
	if !found.Found || len(found.Children) != 1 {
		t.Errorf("unexpected result for a: %+v", found)
	}

	notFound := handler("nope")
	if notFound.Found {
		t.Error("expected Found=false for an unknown ID")
	}
}

func TestTreeHandler(t *testing.T) {
	cat, _ := New([]Node{{Slug: "a"}, {ParentID: "a", Slug: "b"}})
	handler := TreeHandler(cat)
	full := handler("", 0)
	if full == "" {
		t.Fatal("expected non-empty tree for the whole catalog")
	}
	sub := handler("a", 0)
	if sub == "" {
		t.Fatal("expected non-empty tree for a subtree")
	}
}
