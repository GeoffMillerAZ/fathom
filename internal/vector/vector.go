// Package vector provides the vector store interface and embedding integration
// for fathom's optional semantic search layer. When an EmbeddingClient and a
// VectorStore are wired into the query engine, Search calls embed the query
// text and retrieve the nearest symbols by cosine similarity.
package vector

import (
	"context"
	"math"
	"sort"
	"sync"

	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
	"github.com/geoffmilleraz/fathom/internal/core"
)

// Entry is the unit stored in a VectorStore: a pre-computed embedding together
// with the symbol coordinates it represents.
type Entry struct {
	ID     string
	Vector embeddingclient.Vector
	Symbol string
	File   string
}

// Hit is a search result — an Entry paired with its cosine-similarity score.
type Hit struct {
	Entry
	Score float64
}

// VectorStore is the persistence interface for dense embedding vectors. Callers
// insert or update entries via Upsert and retrieve the top-N nearest entries
// via Search.
type VectorStore interface {
	// Upsert inserts or replaces entries identified by Entry.ID.
	Upsert(ctx context.Context, entries []Entry) error

	// Search returns the topN entries nearest to query (cosine similarity),
	// ordered by score descending; ties broken by ID ascending.
	Search(ctx context.Context, query embeddingclient.Vector, topN int) ([]Hit, error)
}

// MemoryVectorStore is an in-memory VectorStore. It is safe for concurrent use.
type MemoryVectorStore struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

// NewMemoryVectorStore returns an empty MemoryVectorStore.
func NewMemoryVectorStore() *MemoryVectorStore {
	return &MemoryVectorStore{entries: make(map[string]Entry)}
}

// Upsert inserts or replaces entries by ID.
func (s *MemoryVectorStore) Upsert(_ context.Context, entries []Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		s.entries[e.ID] = e
	}
	return nil
}

// Search returns the topN entries with the highest cosine similarity to query.
// Results are ordered by score descending; ties are broken by ID ascending.
func (s *MemoryVectorStore) Search(_ context.Context, query embeddingclient.Vector, topN int) ([]Hit, error) {
	s.mu.RLock()
	snapshot := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		snapshot = append(snapshot, e)
	}
	s.mu.RUnlock()

	hits := make([]Hit, 0, len(snapshot))
	for _, e := range snapshot {
		score := cosineSimilarity(query, e.Vector)
		hits = append(hits, Hit{Entry: e, Score: score})
	}

	// Sort: score descending, then ID ascending for determinism.
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].ID < hits[j].ID
	})

	if topN > 0 && len(hits) > topN {
		hits = hits[:topN]
	}
	return hits, nil
}

// cosineSimilarity returns the cosine similarity between two vectors.
// Returns 0 for zero-length vectors.
func cosineSimilarity(a, b embeddingclient.Vector) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, normA, normB float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// Indexer embeds symbol nodes and stores them in a VectorStore, and provides
// text-based semantic search over the stored vectors.
type Indexer struct {
	embedder embeddingclient.Embedder
	store    VectorStore
}

// NewIndexer returns an Indexer that uses embedder to produce vectors and store
// to persist them.
func NewIndexer(embedder embeddingclient.Embedder, store VectorStore) *Indexer {
	return &Indexer{embedder: embedder, store: store}
}

// Index embeds each SymbolNode (using its Name + Signature as the text) and
// upserts the resulting entries into the VectorStore. The entry ID is the
// symbol key: "<Package>.<Name>".
func (idx *Indexer) Index(ctx context.Context, nodes []core.SymbolNode) error {
	if len(nodes) == 0 {
		return nil
	}

	texts := make([]string, len(nodes))
	for i, n := range nodes {
		texts[i] = symbolText(n)
	}

	vecs, err := idx.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return err
	}

	entries := make([]Entry, len(nodes))
	for i, n := range nodes {
		entries[i] = Entry{
			ID:     symbolID(n),
			Vector: vecs[i],
			Symbol: n.Name,
			File:   n.FilePath,
		}
	}
	return idx.store.Upsert(ctx, entries)
}

// SearchText embeds query and returns the topN nearest entries from the
// VectorStore by cosine similarity.
func (idx *Indexer) SearchText(ctx context.Context, text string, topN int) ([]Hit, error) {
	vec, err := idx.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}
	return idx.store.Search(ctx, vec, topN)
}

// symbolText returns the embedding input text for a SymbolNode.
func symbolText(n core.SymbolNode) string {
	if n.Signature == "" {
		return n.Name
	}
	return n.Name + " " + n.Signature
}

// symbolID returns the stable key used as the vector store entry ID.
func symbolID(n core.SymbolNode) string {
	if n.Package == "" {
		return n.Name
	}
	return n.Package + "." + n.Name
}
