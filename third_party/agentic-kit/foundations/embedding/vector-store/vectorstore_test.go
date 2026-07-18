package vectorstore_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	vectorstore "github.com/geoffmilleraz/agentic-kit/foundations/embedding/vector-store"
)

// newMock builds a MockEmbedder with default dimensions.
func newMock() *embeddingclient.MockEmbedder {
	return embeddingclient.NewMockEmbedder(0)
}

// embed1 embeds a single text, failing the test on error.
func embed1(t *testing.T, e *embeddingclient.MockEmbedder, text string) embeddingclient.Vector {
	t.Helper()
	v, err := e.Embed(context.Background(), text)
	if err != nil {
		t.Fatalf("embed1(%q): %v", text, err)
	}
	return v
}

// -------------------------------------------------------------------
// Cosine
// -------------------------------------------------------------------

func TestCosine(t *testing.T) {
	tests := []struct {
		name string
		a, b embeddingclient.Vector
		want float64
	}{
		{"identical unit vectors", embeddingclient.Vector{1, 0}, embeddingclient.Vector{1, 0}, 1},
		{"parallel scaled vectors", embeddingclient.Vector{1, 2, 3}, embeddingclient.Vector{2, 4, 6}, 1},
		{"orthogonal vectors", embeddingclient.Vector{1, 0}, embeddingclient.Vector{0, 1}, 0},
		{"opposite vectors", embeddingclient.Vector{1, 0}, embeddingclient.Vector{-1, 0}, -1},
		{"zero-magnitude a", embeddingclient.Vector{0, 0}, embeddingclient.Vector{1, 0}, 0},
		{"zero-magnitude b", embeddingclient.Vector{1, 0}, embeddingclient.Vector{0, 0}, 0},
		{"empty a", embeddingclient.Vector{}, embeddingclient.Vector{1, 0}, 0},
		{"empty b", embeddingclient.Vector{1, 0}, embeddingclient.Vector{}, 0},
		{"both nil", nil, nil, 0},
		{"length mismatch", embeddingclient.Vector{1, 0}, embeddingclient.Vector{1, 0, 0}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := vectorstore.Cosine(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("Cosine(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------------
// MemoryVectorStore — Upsert + Search
// -------------------------------------------------------------------

func TestMemoryVectorStore_UpsertAndSearchReturnsNearest(t *testing.T) {
	mock := newMock()
	store := vectorstore.NewMemoryVectorStore()

	vA := embed1(t, mock, "alpha")
	vB := embed1(t, mock, "beta")
	vC := embed1(t, mock, "gamma")

	err := store.Upsert(
		vectorstore.Entry{ID: "a", Vector: vA, Meta: map[string]string{"file": "alpha.go"}},
		vectorstore.Entry{ID: "b", Vector: vB, Meta: map[string]string{"file": "beta.go"}},
		vectorstore.Entry{ID: "c", Vector: vC, Meta: map[string]string{"file": "gamma.go"}},
	)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	// Query with the exact vector for "alpha" — should rank entry "a" first.
	hits, err := store.Search(vA, 3)
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
		t.Errorf("want score ~1.0 for exact match, got %.6f", hits[0].Score)
	}
	if got := hits[0].Meta["file"]; got != "alpha.go" {
		t.Errorf("want Meta[file]=alpha.go on top hit, got %q", got)
	}
}

func TestMemoryVectorStore_TopKTruncation(t *testing.T) {
	mock := newMock()
	store := vectorstore.NewMemoryVectorStore()

	vA := embed1(t, mock, "alpha")
	vB := embed1(t, mock, "beta")
	vC := embed1(t, mock, "gamma")

	_ = store.Upsert(
		vectorstore.Entry{ID: "a", Vector: vA},
		vectorstore.Entry{ID: "b", Vector: vB},
		vectorstore.Entry{ID: "c", Vector: vC},
	)

	hits, err := store.Search(vA, 2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("want 2 hits with topK=2, got %d", len(hits))
	}

	// topK <= 0 returns everything.
	all, err := store.Search(vA, 0)
	if err != nil {
		t.Fatalf("Search(topK=0): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("want 3 hits with topK=0, got %d", len(all))
	}
}

func TestMemoryVectorStore_Determinism(t *testing.T) {
	// Two separate stores loaded with the same data must return hits in the
	// same order, and repeated searches on the same store must also agree.
	mock := newMock()

	vA := embed1(t, mock, "alpha")
	vB := embed1(t, mock, "beta")
	vC := embed1(t, mock, "gamma")

	makeStore := func() *vectorstore.MemoryVectorStore {
		s := vectorstore.NewMemoryVectorStore()
		_ = s.Upsert(
			vectorstore.Entry{ID: "a", Vector: vA},
			vectorstore.Entry{ID: "b", Vector: vB},
			vectorstore.Entry{ID: "c", Vector: vC},
		)
		return s
	}

	// Use a query that is not any stored vector so scores are distinct-ish.
	queryVec := embed1(t, mock, "query-text-xyz")

	s1 := makeStore()
	s2 := makeStore()

	h1, _ := s1.Search(queryVec, 3)
	h2, _ := s2.Search(queryVec, 3)
	h3, _ := s1.Search(queryVec, 3) // second call same store

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
	store := vectorstore.NewMemoryVectorStore()

	v := embeddingclient.Vector{0.6, 0.8} // arbitrary unit-ish vector
	_ = store.Upsert(
		vectorstore.Entry{ID: "zzz", Vector: v},
		vectorstore.Entry{ID: "aaa", Vector: v},
	)

	hits, _ := store.Search(v, 2)
	if hits[0].ID != "aaa" {
		t.Errorf("want aaa first (ID tiebreak), got %q", hits[0].ID)
	}
}

func TestMemoryVectorStore_UpsertReplaces(t *testing.T) {
	// Upserting with the same ID must replace the previous entry, including
	// its metadata.
	mock := newMock()
	store := vectorstore.NewMemoryVectorStore()

	vOld := embed1(t, mock, "old-text")
	vNew := embed1(t, mock, "new-text")

	_ = store.Upsert(vectorstore.Entry{ID: "x", Vector: vOld, Meta: map[string]string{"rev": "old"}})
	_ = store.Upsert(vectorstore.Entry{ID: "x", Vector: vNew, Meta: map[string]string{"rev": "new"}})

	if store.Len() != 1 {
		t.Fatalf("want Len=1 after replacing upsert, got %d", store.Len())
	}

	hits, _ := store.Search(vNew, 1)
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if got := hits[0].Meta["rev"]; got != "new" {
		t.Errorf("want Meta[rev]=new after upsert, got %q", got)
	}
	if hits[0].Score < 0.999 {
		t.Errorf("want score ~1.0 for replaced vector, got %.6f", hits[0].Score)
	}
}

func TestMemoryVectorStore_EmptySearch(t *testing.T) {
	store := vectorstore.NewMemoryVectorStore()
	hits, err := store.Search(embeddingclient.Vector{1, 0}, 5)
	if err != nil {
		t.Fatalf("Search on empty store: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("want 0 hits on empty store, got %d", len(hits))
	}
}

// -------------------------------------------------------------------
// MemoryVectorStore — Delete + Len
// -------------------------------------------------------------------

func TestMemoryVectorStore_DeleteAndLen(t *testing.T) {
	store := vectorstore.NewMemoryVectorStore()
	if store.Len() != 0 {
		t.Fatalf("want Len=0 on new store, got %d", store.Len())
	}

	v := embeddingclient.Vector{1, 0}
	_ = store.Upsert(
		vectorstore.Entry{ID: "a", Vector: v},
		vectorstore.Entry{ID: "b", Vector: v},
		vectorstore.Entry{ID: "c", Vector: v},
	)
	if store.Len() != 3 {
		t.Fatalf("want Len=3 after upsert, got %d", store.Len())
	}

	if err := store.Delete("a", "c"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.Len() != 1 {
		t.Errorf("want Len=1 after deleting 2 of 3, got %d", store.Len())
	}

	hits, _ := store.Search(v, 0)
	if len(hits) != 1 || hits[0].ID != "b" {
		t.Errorf("want only entry b to survive, got %+v", hits)
	}

	// Deleting unknown IDs is a no-op, not an error.
	if err := store.Delete("a", "does-not-exist"); err != nil {
		t.Errorf("Delete of unknown IDs: %v", err)
	}
	if store.Len() != 1 {
		t.Errorf("want Len unchanged after deleting unknown IDs, got %d", store.Len())
	}
}

// -------------------------------------------------------------------
// MemoryVectorStore — concurrency (-race)
// -------------------------------------------------------------------

func TestMemoryVectorStore_ConcurrentUpsertSearchDelete(t *testing.T) {
	store := vectorstore.NewMemoryVectorStore()
	query := embeddingclient.Vector{0.6, 0.8}

	const goroutines = 8
	const opsPerGoroutine = 50

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < opsPerGoroutine; i++ {
				id := fmt.Sprintf("g%d-i%d", g, i)
				if err := store.Upsert(vectorstore.Entry{
					ID:     id,
					Vector: embeddingclient.Vector{float64(g + 1), float64(i + 1)},
					Meta:   map[string]string{"goroutine": fmt.Sprintf("%d", g)},
				}); err != nil {
					t.Errorf("Upsert(%s): %v", id, err)
				}
				if _, err := store.Search(query, 5); err != nil {
					t.Errorf("Search: %v", err)
				}
				if i%10 == 9 {
					if err := store.Delete(id); err != nil {
						t.Errorf("Delete(%s): %v", id, err)
					}
				}
				_ = store.Len()
			}
		}(g)
	}
	wg.Wait()

	// Each goroutine deleted opsPerGoroutine/10 of its own entries.
	want := goroutines * (opsPerGoroutine - opsPerGoroutine/10)
	if got := store.Len(); got != want {
		t.Errorf("want Len=%d after concurrent ops, got %d", want, got)
	}
}

// -------------------------------------------------------------------
// Indexer — IndexTexts + SearchText
// -------------------------------------------------------------------

func TestIndexer_SearchTextFindsMatchingDocument(t *testing.T) {
	mock := newMock()
	ctx := context.Background()
	store := vectorstore.NewMemoryVectorStore()
	idx := vectorstore.NewIndexer(mock, store)

	docs := []vectorstore.Document{
		{ID: "auth.ValidateToken", Text: "ValidateToken func(ctx context.Context, token string) error", Meta: map[string]string{"file": "auth/token.go"}},
		{ID: "billing.ChargeCard", Text: "ChargeCard func(ctx context.Context, amount int) error", Meta: map[string]string{"file": "billing/charge.go"}},
		{ID: "logging.LogEvent", Text: "LogEvent func(event string)", Meta: map[string]string{"file": "logging/log.go"}},
	}

	if err := idx.IndexTexts(ctx, docs...); err != nil {
		t.Fatalf("IndexTexts: %v", err)
	}
	if store.Len() != 3 {
		t.Fatalf("want 3 stored entries, got %d", store.Len())
	}

	// Search for text identical to one document — it must rank highest, and
	// its metadata must round-trip.
	hits, err := idx.SearchText(ctx, "ValidateToken func(ctx context.Context, token string) error", 3)
	if err != nil {
		t.Fatalf("SearchText: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("want at least 1 hit")
	}
	if hits[0].ID != "auth.ValidateToken" {
		t.Errorf("want auth.ValidateToken at top, got %q", hits[0].ID)
	}
	if got := hits[0].Meta["file"]; got != "auth/token.go" {
		t.Errorf("want Meta[file]=auth/token.go, got %q", got)
	}
}

func TestIndexer_IndexTextsEmptyIsNoop(t *testing.T) {
	mock := newMock()
	ctx := context.Background()
	store := vectorstore.NewMemoryVectorStore()
	idx := vectorstore.NewIndexer(mock, store)

	if err := idx.IndexTexts(ctx); err != nil {
		t.Fatalf("IndexTexts(): %v", err)
	}
	if mock.CallCount() != 0 {
		t.Errorf("want 0 embedder calls for empty input, got %d", mock.CallCount())
	}
	if store.Len() != 0 {
		t.Errorf("want empty store after no-op index, got Len=%d", store.Len())
	}
}

func TestIndexer_SearchTextTopKRespected(t *testing.T) {
	mock := newMock()
	ctx := context.Background()
	store := vectorstore.NewMemoryVectorStore()
	idx := vectorstore.NewIndexer(mock, store)

	_ = idx.IndexTexts(
		ctx,
		vectorstore.Document{ID: "a", Text: "A"},
		vectorstore.Document{ID: "b", Text: "B"},
		vectorstore.Document{ID: "c", Text: "C"},
		vectorstore.Document{ID: "d", Text: "D"},
	)

	hits, err := idx.SearchText(ctx, "A", 2)
	if err != nil {
		t.Fatalf("SearchText: %v", err)
	}
	if len(hits) != 2 {
		t.Errorf("want 2 hits (topK=2), got %d", len(hits))
	}
}

func TestIndexer_EmbedderCallCounts(t *testing.T) {
	// IndexTexts with N docs -> EmbedBatch (N calls); SearchText -> 1 Embed
	// call.
	mock := newMock()
	ctx := context.Background()
	store := vectorstore.NewMemoryVectorStore()
	idx := vectorstore.NewIndexer(mock, store)

	_ = idx.IndexTexts(
		ctx,
		vectorstore.Document{ID: "x", Text: "X"},
		vectorstore.Document{ID: "y", Text: "Y"},
	)
	_, _ = idx.SearchText(ctx, "some query", 1)

	// 2 from EmbedBatch + 1 from Embed.
	if got := mock.CallCount(); got != 3 {
		t.Errorf("want 3 total embedder calls, got %d", got)
	}
}

func TestIndexer_Determinism(t *testing.T) {
	// Indexing the same documents twice (separate indexers, same mock) and
	// searching with the same query must return the same ordered hits.
	mock := newMock()
	ctx := context.Background()

	docs := []vectorstore.Document{
		{ID: "a.Foo", Text: "Foo func()"},
		{ID: "b.Bar", Text: "Bar func() int"},
		{ID: "c.Baz", Text: "Baz func(s string) bool"},
	}

	makeIdx := func() *vectorstore.Indexer {
		s := vectorstore.NewMemoryVectorStore()
		i := vectorstore.NewIndexer(mock, s)
		if err := i.IndexTexts(ctx, docs...); err != nil {
			t.Fatalf("IndexTexts: %v", err)
		}
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

// -------------------------------------------------------------------
// Indexer — error paths
// -------------------------------------------------------------------

// failingEmbedder returns a fixed error from every call.
type failingEmbedder struct{ err error }

func (f *failingEmbedder) Embed(context.Context, string) (embeddingclient.Vector, error) {
	return nil, f.err
}

func (f *failingEmbedder) EmbedBatch(context.Context, []string) ([]embeddingclient.Vector, error) {
	return nil, f.err
}

// shortBatchEmbedder violates the EmbedBatch contract by returning fewer
// vectors than texts.
type shortBatchEmbedder struct{}

func (shortBatchEmbedder) Embed(context.Context, string) (embeddingclient.Vector, error) {
	return embeddingclient.Vector{1}, nil
}

func (shortBatchEmbedder) EmbedBatch(_ context.Context, texts []string) ([]embeddingclient.Vector, error) {
	return make([]embeddingclient.Vector, len(texts)-1), nil
}

func TestIndexer_EmbedErrorsPropagate(t *testing.T) {
	wantErr := errors.New("backend down")
	store := vectorstore.NewMemoryVectorStore()
	idx := vectorstore.NewIndexer(&failingEmbedder{err: wantErr}, store)
	ctx := context.Background()

	if err := idx.IndexTexts(ctx, vectorstore.Document{ID: "a", Text: "alpha"}); !errors.Is(err, wantErr) {
		t.Errorf("IndexTexts: want %v, got %v", wantErr, err)
	}
	if store.Len() != 0 {
		t.Errorf("want nothing stored after embed failure, got Len=%d", store.Len())
	}

	if _, err := idx.SearchText(ctx, "alpha", 1); !errors.Is(err, wantErr) {
		t.Errorf("SearchText: want %v, got %v", wantErr, err)
	}
}

func TestIndexer_ShortBatchRejected(t *testing.T) {
	store := vectorstore.NewMemoryVectorStore()
	idx := vectorstore.NewIndexer(shortBatchEmbedder{}, store)

	err := idx.IndexTexts(
		context.Background(),
		vectorstore.Document{ID: "a", Text: "alpha"},
		vectorstore.Document{ID: "b", Text: "beta"},
	)
	if err == nil {
		t.Fatal("want error when embedder returns fewer vectors than texts")
	}
	if store.Len() != 0 {
		t.Errorf("want nothing stored after short batch, got Len=%d", store.Len())
	}
}
