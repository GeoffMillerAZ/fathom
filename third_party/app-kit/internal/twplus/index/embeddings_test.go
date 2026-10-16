package index_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/embed"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/index"
)

// fakeEmbedder implements embed.Embedder deterministically (no network), so the
// full offline embed orchestration is testable without Ollama.
type fakeEmbedder struct{}

func (fakeEmbedder) Embed(_ context.Context, texts []string) ([]embed.Vector, error) {
	out := make([]embed.Vector, len(texts))
	for i, t := range texts {
		out[i] = fakeEmbed(t)
	}
	return out, nil
}
func (fakeEmbedder) Model() string { return "fake-4d" }
func (fakeEmbedder) Dims() int     { return 4 }

func TestBuildEmbeddingsOrchestration(t *testing.T) {
	ix := buildIndex(t)
	path := filepath.Join(t.TempDir(), "e.sqlite")
	// batch=2 forces multiple round-trips over the 4 variants.
	n, err := ix.BuildEmbeddings(context.Background(), fakeEmbedder{}, path, 2)
	if err != nil {
		t.Fatalf("BuildEmbeddings: %v", err)
	}
	if n != 4 {
		t.Fatalf("embedded %d, want 4", n)
	}
	store, err := index.OpenEmbeddings(path)
	if err != nil {
		t.Fatalf("OpenEmbeddings: %v", err)
	}
	defer store.Close()
	if store.Model() != "fake-4d" || store.Dims() != 4 {
		t.Errorf("meta = %s/%d", store.Model(), store.Dims())
	}
	res, err := ix.SearchHybrid("", fakeEmbed("Pricing"), store, index.Filters{}, 1)
	if err != nil {
		t.Fatalf("SearchHybrid: %v", err)
	}
	if len(res) == 0 || !strings.Contains(res[0].FullName, "Pricing") {
		t.Errorf("top hybrid result = %+v, want Pricing", res)
	}
}

// fakeEmbed maps text to a deterministic 4-dim vector by topic, so the index's
// semantic machinery can be tested end-to-end without a running model: pricing
// components point one way, badges another.
func fakeEmbed(text string) embed.Vector {
	switch {
	case strings.Contains(text, "Pricing"):
		return embed.Normalize(embed.Vector{1, 0, 0, 0})
	case strings.Contains(text, "Badges"):
		return embed.Normalize(embed.Vector{0, 1, 0, 0})
	default:
		return embed.Normalize(embed.Vector{0, 0, 1, 0})
	}
}

// buildEmbeddings embeds the corpus with fakeEmbed and returns an open store.
func buildEmbeddings(t *testing.T, ix *index.Index) *index.EmbeddingStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "embeddings.sqlite")
	b, err := index.NewEmbedBuilder(path, "fake-4d", 4, "digest123")
	if err != nil {
		t.Fatalf("NewEmbedBuilder: %v", err)
	}
	targets, err := ix.EmbedTargets()
	if err != nil {
		t.Fatalf("EmbedTargets: %v", err)
	}
	if len(targets) != 4 {
		t.Fatalf("embed targets = %d, want 4", len(targets))
	}
	for _, tg := range targets {
		if err := b.Add(tg.VariantID, fakeEmbed(tg.EmbedText)); err != nil {
			t.Fatalf("Add %d: %v", tg.VariantID, err)
		}
	}
	if n, err := b.Finalize(); err != nil || n != 4 {
		t.Fatalf("Finalize: n=%d err=%v", n, err)
	}
	store, err := index.OpenEmbeddings(path)
	if err != nil {
		t.Fatalf("OpenEmbeddings: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if store.Model() != "fake-4d" || store.Dims() != 4 || store.Digest() != "digest123" {
		t.Errorf("meta = %s/%d/%s", store.Model(), store.Dims(), store.Digest())
	}
	return store
}

func TestKNNFindsTopic(t *testing.T) {
	ix := buildIndex(t)
	store := buildEmbeddings(t, ix)
	// A probe pointing at the pricing direction must return pricing variants first.
	hits, err := store.KNN(embed.Normalize(embed.Vector{0.9, 0.1, 0, 0}), 2)
	if err != nil {
		t.Fatalf("KNN: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no KNN hits")
	}
	// Map the nearest variant id back through the index and confirm it's pricing.
	res, err := ix.SearchHybrid("", embed.Normalize(embed.Vector{0.9, 0.1, 0, 0}), store, index.Filters{}, 1)
	if err != nil {
		t.Fatalf("SearchHybrid: %v", err)
	}
	if len(res) == 0 || !strings.Contains(res[0].FullName, "Pricing") {
		t.Fatalf("nearest-by-vector top result = %+v, want a Pricing component", res)
	}
}

func TestSearchHybridSemanticOverridesWeakText(t *testing.T) {
	ix := buildIndex(t)
	store := buildEmbeddings(t, ix)
	// Query text "badge" lexically favors the Badges component, but a query
	// vector pointing at pricing should pull the pricing component up via RRF.
	res, err := ix.SearchHybrid("badge", fakeEmbed("Pricing"), store, index.Filters{}, 4)
	if err != nil {
		t.Fatalf("SearchHybrid: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("no hybrid results")
	}
	var sawPricing bool
	for _, r := range res {
		if strings.Contains(r.FullName, "Pricing") {
			sawPricing = true
		}
	}
	if !sawPricing {
		t.Errorf("hybrid results missing the semantically-near Pricing component: %+v", res)
	}
}

func TestSearchHybridFallsBackToBM25(t *testing.T) {
	ix := buildIndex(t)
	// nil store → identical to pure BM25.
	hybrid, err := ix.SearchHybrid("pricing tiers", nil, nil, index.Filters{}, 10)
	if err != nil {
		t.Fatalf("SearchHybrid(nil store): %v", err)
	}
	bm25, err := ix.SearchComponents("pricing tiers", index.Filters{}, 10)
	if err != nil {
		t.Fatalf("SearchComponents: %v", err)
	}
	if len(hybrid) != len(bm25) {
		t.Fatalf("fallback len %d != bm25 len %d", len(hybrid), len(bm25))
	}
	for i := range hybrid {
		if hybrid[i].FullName != bm25[i].FullName {
			t.Errorf("fallback rank %d = %q, bm25 = %q", i, hybrid[i].FullName, bm25[i].FullName)
		}
	}
}

func TestSearchHybridDimMismatchFallsBack(t *testing.T) {
	ix := buildIndex(t)
	store := buildEmbeddings(t, ix)
	// A 3-dim query against a 4-dim store must not error — it falls back to BM25.
	res, err := ix.SearchHybrid("pricing", embed.Vector{1, 0, 0}, store, index.Filters{}, 5)
	if err != nil {
		t.Fatalf("SearchHybrid dim mismatch should fall back, got error: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected BM25 fallback results")
	}
}

func TestRRFFusionViaHybridFilters(t *testing.T) {
	ix := buildIndex(t)
	store := buildEmbeddings(t, ix)
	// Structural filter must still apply through the fused path.
	res, err := ix.SearchHybrid("pricing", fakeEmbed("Pricing"), store, index.Filters{Framework: "react"}, 10)
	if err != nil {
		t.Fatalf("SearchHybrid: %v", err)
	}
	for _, r := range res {
		if r.Framework != "react" {
			t.Errorf("filter leaked through fusion: %q is %s", r.FullName, r.Framework)
		}
	}
}
