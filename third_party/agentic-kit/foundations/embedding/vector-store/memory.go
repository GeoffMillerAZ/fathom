package vectorstore

import (
	"sort"
	"sync"

	embeddingclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/embedding-client"
)

// MemoryVectorStore is an in-memory VectorStore backed by a map. It is safe
// for concurrent use via sync.RWMutex, and Search ordering is deterministic:
// score descending, ties broken by ID ascending.
type MemoryVectorStore struct {
	mu      sync.RWMutex
	entries map[string]Entry
}

var _ VectorStore = (*MemoryVectorStore)(nil)

// NewMemoryVectorStore returns an empty MemoryVectorStore.
func NewMemoryVectorStore() *MemoryVectorStore {
	return &MemoryVectorStore{entries: make(map[string]Entry)}
}

// Upsert inserts or replaces entries by ID. It never returns an error; the
// error result exists to satisfy VectorStore so persistent implementations
// can fail.
func (s *MemoryVectorStore) Upsert(entries ...Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entries {
		s.entries[e.ID] = e
	}
	return nil
}

// Search returns the topK entries with the highest cosine similarity to
// query. Results are ordered by score descending; ties are broken by ID
// ascending for determinism. A topK <= 0 returns all entries.
func (s *MemoryVectorStore) Search(query embeddingclient.Vector, topK int) ([]Scored, error) {
	s.mu.RLock()
	snapshot := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		snapshot = append(snapshot, e)
	}
	s.mu.RUnlock()

	scored := make([]Scored, 0, len(snapshot))
	for _, e := range snapshot {
		scored = append(scored, Scored{Entry: e, Score: Cosine(query, e.Vector)})
	}

	// Sort: score descending, then ID ascending for determinism.
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].ID < scored[j].ID
	})

	if topK > 0 && len(scored) > topK {
		scored = scored[:topK]
	}
	return scored, nil
}

// Delete removes the entries with the given IDs. Unknown IDs are ignored. It
// never returns an error; the error result exists to satisfy VectorStore.
func (s *MemoryVectorStore) Delete(ids ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		delete(s.entries, id)
	}
	return nil
}

// Len reports the number of entries currently stored.
func (s *MemoryVectorStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}
