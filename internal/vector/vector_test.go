package vector_test

import (
	"context"
	"testing"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/vector"
)

// helper builds a MockEmbedder with default dimensions.
func newMock() *embeddingclient.MockEmbedder {
	return embeddingclient.NewMockEmbedder(0)
}

// embed1 is a convenience that panics on error (tests only).
func embed1(t *testing.T, e *embeddingclient.MockEmbedder, text string) embeddingclient.Vector {
	t.Helper()
	v, err := e.Embed(context.Background(), text)
	if err != nil {
		t.Fatalf("embed1(%q): %v", text, err)
	}
	return v
}

// -------------------------------------------------------------------
// MemoryVectorStore — Upsert + Search
// -------------------------------------------------------------------

func TestMemoryVectorStore_UpsertAndSearchReturnsNearest(t *testing.T) {
	mock := newMock()
	ctx := context.Background()

	store := vector.NewMemoryVectorStore()

	// Build three deterministic vectors.
	vA := embed1(t, mock, "alpha")
	vB := embed1(t, mock, "beta")
	vC := embed1(t, mock, "gamma")

	entries := []vector.Entry{
		{ID: "a", Vector: vA, Symbol: "Alpha", File: "alpha.go"},
		{ID: "b", Vector: vB, Symbol: "Beta", File: "beta.go"},
		{ID: "c", Vector: vC, Symbol: "Gamma", File: "gamma.go"},
	}
	if err := store.Upsert(ctx, entries); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Query with the exact vector for "alpha" — should rank entry "a" first.
	hits, err := store.Search(ctx, vA, 3)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 3 {
		t.Fatalf("want 3 hits, got %d", len(hits))
	}
	if hits[0].ID != "a" {
		t.Errorf("want top hit ID=a, got %q (score=%.4f)", hits[0].ID, hits[0].Score)
	}
	// Score for exact match (unit vectors) should be ~1.0.
	if hits[0].Score < 0.999 {
		t.Errorf("want score ≈1.0 for exact match, got %.6f", hits[0].Score)
	}
}

func TestMemoryVectorStore_TopNTruncation(t *testing.T) {
	mock := newMock()
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()

	vA := embed1(t, mock, "alpha")
	vB := embed1(t, mock, "beta")
	vC := embed1(t, mock, "gamma")

	_ = store.Upsert(ctx, []vector.Entry{
		{ID: "a", Vector: vA, Symbol: "A", File: "a.go"},
		{ID: "b", Vector: vB, Symbol: "B", File: "b.go"},
		{ID: "c", Vector: vC, Symbol: "C", File: "c.go"},
	})

	hits, err := store.Search(ctx, vA, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("want 2 hits with topN=2, got %d", len(hits))
	}
}

func TestMemoryVectorStore_Determinism(t *testing.T) {
	// Two separate stores loaded with the same data must return hits in the
	// same order, and repeated searches on the same store must also agree.
	mock := newMock()
	ctx := context.Background()

	vA := embed1(t, mock, "alpha")
	vB := embed1(t, mock, "beta")
	vC := embed1(t, mock, "gamma")

	makeStore := func() *vector.MemoryVectorStore {
		s := vector.NewMemoryVectorStore()
		_ = s.Upsert(ctx, []vector.Entry{
			{ID: "a", Vector: vA, Symbol: "A", File: "a.go"},
			{ID: "b", Vector: vB, Symbol: "B", File: "b.go"},
			{ID: "c", Vector: vC, Symbol: "C", File: "c.go"},
		})
		return s
	}

	// Use a query that is not any stored vector so scores are distinct-ish.
	queryVec := embed1(t, mock, "query-text-xyz")

	s1 := makeStore()
	s2 := makeStore()

	h1, _ := s1.Search(ctx, queryVec, 3)
	h2, _ := s2.Search(ctx, queryVec, 3)
	h3, _ := s1.Search(ctx, queryVec, 3) // second call same store

	for i := range h1 {
		if h1[i].ID != h2[i].ID || h1[i].ID != h3[i].ID {
			t.Errorf("non-deterministic: position %d: %q vs %q vs %q",
				i, h1[i].ID, h2[i].ID, h3[i].ID)
		}
	}
}

func TestMemoryVectorStore_IDTiebreak(t *testing.T) {
	// Inject two identical vectors to force a tie in score; the one with the
	// lexicographically smaller ID must rank first.
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()

	v := embeddingclient.Vector{0.6, 0.8} // arbitrary unit-ish vector
	_ = store.Upsert(ctx, []vector.Entry{
		{ID: "zzz", Vector: v, Symbol: "Z", File: "z.go"},
		{ID: "aaa", Vector: v, Symbol: "A", File: "a.go"},
	})

	hits, _ := store.Search(ctx, v, 2)
	if hits[0].ID != "aaa" {
		t.Errorf("want aaa first (ID tiebreak), got %q", hits[0].ID)
	}
}

func TestMemoryVectorStore_UpsertReplaces(t *testing.T) {
	// Upserting with the same ID must replace the previous entry.
	mock := newMock()
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()

	vOld := embed1(t, mock, "old-text")
	vNew := embed1(t, mock, "new-text")

	_ = store.Upsert(ctx, []vector.Entry{{ID: "x", Vector: vOld, Symbol: "Old", File: "old.go"}})
	_ = store.Upsert(ctx, []vector.Entry{{ID: "x", Vector: vNew, Symbol: "New", File: "new.go"}})

	hits, _ := store.Search(ctx, vNew, 1)
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if hits[0].Symbol != "New" {
		t.Errorf("want Symbol=New after upsert, got %q", hits[0].Symbol)
	}
	if hits[0].Score < 0.999 {
		t.Errorf("want score ≈1.0 for replaced vector, got %.6f", hits[0].Score)
	}
}

func TestMemoryVectorStore_EmptySearch(t *testing.T) {
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()
	hits, err := store.Search(ctx, embeddingclient.Vector{1, 0}, 5)
	if err != nil {
		t.Fatalf("Search on empty store: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("want 0 hits on empty store, got %d", len(hits))
	}
}

// -------------------------------------------------------------------
// Indexer — Index + SearchText
// -------------------------------------------------------------------

func TestIndexer_SearchTextFindsMatchingSymbol(t *testing.T) {
	mock := newMock()
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()
	idx := vector.NewIndexer(mock, store)

	nodes := []core.SymbolNode{
		{Package: "auth", Name: "ValidateToken", Signature: "func(ctx context.Context, token string) error", FilePath: "auth/token.go"},
		{Package: "billing", Name: "ChargeCard", Signature: "func(ctx context.Context, amount int) error", FilePath: "billing/charge.go"},
		{Package: "logging", Name: "LogEvent", Signature: "func(event string)", FilePath: "logging/log.go"},
	}

	if err := idx.Index(ctx, nodes); err != nil {
		t.Fatalf("Index: %v", err)
	}

	// Search for text that matches "ValidateToken func(ctx context.Context, token string) error"
	// exactly — should rank it highest.
	hits, err := idx.SearchText(ctx, "ValidateToken func(ctx context.Context, token string) error", 3)
	if err != nil {
		t.Fatalf("SearchText: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("want at least 1 hit")
	}
	if hits[0].Symbol != "ValidateToken" {
		t.Errorf("want ValidateToken at top, got %q", hits[0].Symbol)
	}
}

func TestIndexer_IndexEmptyNodesIsNoop(t *testing.T) {
	mock := newMock()
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()
	idx := vector.NewIndexer(mock, store)

	if err := idx.Index(ctx, nil); err != nil {
		t.Fatalf("Index(nil): %v", err)
	}
	if mock.CallCount() != 0 {
		t.Errorf("want 0 embedder calls for empty slice, got %d", mock.CallCount())
	}
}

func TestIndexer_SearchTextTopNRespected(t *testing.T) {
	mock := newMock()
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()
	idx := vector.NewIndexer(mock, store)

	nodes := []core.SymbolNode{
		{Package: "p", Name: "A", FilePath: "a.go"},
		{Package: "p", Name: "B", FilePath: "b.go"},
		{Package: "p", Name: "C", FilePath: "c.go"},
		{Package: "p", Name: "D", FilePath: "d.go"},
	}
	_ = idx.Index(ctx, nodes)

	hits, err := idx.SearchText(ctx, "A", 2)
	if err != nil {
		t.Fatalf("SearchText: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("want 2 hits (topN=2), got %d", len(hits))
	}
}

func TestIndexer_EmbedderCallCounts(t *testing.T) {
	// Index N nodes → EmbedBatch (N calls); SearchText → 1 Embed call.
	mock := newMock()
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()
	idx := vector.NewIndexer(mock, store)

	nodes := []core.SymbolNode{
		{Package: "p", Name: "X", FilePath: "x.go"},
		{Package: "p", Name: "Y", FilePath: "y.go"},
	}
	_ = idx.Index(ctx, nodes)
	_, _ = idx.SearchText(ctx, "some query", 1)

	// 2 from EmbedBatch + 1 from Embed.
	if got := mock.CallCount(); got != 3 {
		t.Errorf("want 3 total embedder calls, got %d", got)
	}
}

func TestIndexer_SymbolWithoutSignature(t *testing.T) {
	// Nodes without a Signature should embed just their Name without error.
	mock := newMock()
	ctx := context.Background()
	store := vector.NewMemoryVectorStore()
	idx := vector.NewIndexer(mock, store)

	nodes := []core.SymbolNode{
		{Package: "util", Name: "Helper", FilePath: "util.go"},
	}
	if err := idx.Index(ctx, nodes); err != nil {
		t.Fatalf("Index: %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("want 1 embedder call, got %d", len(calls))
	}
	if calls[0] != "Helper" {
		t.Errorf("want text=Helper for symbol without signature, got %q", calls[0])
	}
}

func TestIndexer_Determinism(t *testing.T) {
	// Indexing the same nodes twice (separate indexers, same mock) and
	// searching with the same query must return the same ordered hits.
	mock := newMock()
	ctx := context.Background()

	nodes := []core.SymbolNode{
		{Package: "a", Name: "Foo", Signature: "func()", FilePath: "foo.go"},
		{Package: "b", Name: "Bar", Signature: "func() int", FilePath: "bar.go"},
		{Package: "c", Name: "Baz", Signature: "func(s string) bool", FilePath: "baz.go"},
	}

	makeIdx := func() *vector.Indexer {
		s := vector.NewMemoryVectorStore()
		i := vector.NewIndexer(mock, s)
		_ = i.Index(ctx, nodes)
		return i
	}

	idx1 := makeIdx()
	idx2 := makeIdx()

	h1, _ := idx1.SearchText(ctx, "Foo func()", 3)
	h2, _ := idx2.SearchText(ctx, "Foo func()", 3)

	if len(h1) != len(h2) {
		t.Fatalf("lengths differ: %d vs %d", len(h1), len(h2))
	}
	for i := range h1 {
		if h1[i].ID != h2[i].ID {
			t.Errorf("position %d: %q vs %q", i, h1[i].ID, h2[i].ID)
		}
	}
}
