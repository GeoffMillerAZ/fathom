package catalog

import (
	"reflect"
	"testing"
)

func testSearchCatalog(t *testing.T) Catalog {
	t.Helper()
	cat, err := New([]Node{
		{Slug: "hero-section", Title: "Hero section", Synopsis: "A large marketing hero with a call to action."},
		{Slug: "pricing-table", Title: "Pricing table", Synopsis: "A table comparing pricing tiers."},
		{Slug: "hero-banner", Title: "Hero banner", Synopsis: "A compact hero banner variant."},
	})
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestSearchRanksByRelevance(t *testing.T) {
	cat := testSearchCatalog(t)
	out := Search(cat, "hero", NewBM25Ranker())
	if out.NoMatch() {
		t.Fatal("expected matches for \"hero\"")
	}
	if out.Results[0].NodeID != "hero-banner" && out.Results[0].NodeID != "hero-section" {
		t.Errorf("expected a hero-* result first, got %s", out.Results[0].NodeID)
	}
	for _, r := range out.Results {
		if r.NodeID == "pricing-table" {
			t.Errorf("pricing-table should not match query %q", out.Query)
		}
	}
	for _, r := range out.Results {
		if r.Score <= 0 {
			t.Errorf("expected every result to have a positive Score, got %+v", r)
		}
	}
}

func TestSearchDeterministicTieBreak(t *testing.T) {
	cat := testSearchCatalog(t)
	ranker := NewBM25Ranker()
	first := Search(cat, "hero", ranker)
	second := Search(cat, "hero", ranker)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("Search must be deterministic: %+v != %+v", first, second)
	}
}

func TestSearchNoMatchIsExplicit(t *testing.T) {
	cat := testSearchCatalog(t)
	out := Search(cat, "nonexistent-term-xyz", NewBM25Ranker())
	if !out.NoMatch() {
		t.Fatalf("expected NoMatch, got %+v", out.Results)
	}
	if out.Query != "nonexistent-term-xyz" {
		t.Errorf("Outcome must still record what was searched, got %q", out.Query)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	cat := testSearchCatalog(t)
	out := Search(cat, "", NewBM25Ranker())
	if !out.NoMatch() {
		t.Errorf("expected NoMatch for an empty query, got %+v", out.Results)
	}
}

func TestBuildDocsIndexesFacetValuesNotAxisNames(t *testing.T) {
	cat, err := New([]Node{
		{Slug: "hero", Title: "Hero", Facets: map[string][]string{"framework": {"react"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	docs := BuildDocs(cat)
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	found := false
	for _, term := range docs[0].Terms {
		if term == "framework" {
			t.Error("axis name \"framework\" must not be indexed as a term")
		}
		if term == "react" {
			found = true
		}
	}
	if !found {
		t.Error("expected facet value \"react\" to be indexed")
	}
}

// injectedRanker lets a test prove the Ranker seam works: a consumer's
// own ranker (e.g. semantic) can be injected without this package
// depending on it.
type constantRanker struct{ order []string }

func (r constantRanker) Rank(query string, docs []Doc) []Result {
	var out []Result
	for _, id := range r.order {
		out = append(out, Result{NodeID: id, Score: 1})
	}
	return out
}

func TestSearchAcceptsInjectedRanker(t *testing.T) {
	cat := testSearchCatalog(t)
	custom := constantRanker{order: []string{"pricing-table", "hero-section"}}
	out := Search(cat, "anything", custom)
	if len(out.Results) != 2 || out.Results[0].NodeID != "pricing-table" {
		t.Errorf("expected the injected Ranker's own order, got %+v", out.Results)
	}
}
