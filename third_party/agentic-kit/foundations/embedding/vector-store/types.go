// Package vectorstore gives the platform somewhere to put dense embedding
// vectors and retrieve the nearest ones by cosine similarity.
//
// Composition: Entry is the stored unit — an ID, a pre-computed
// embeddingclient.Vector, and an opaque Meta map the store never interprets.
// The VectorStore interface defines Upsert/Search/Delete/Len; MemoryVectorStore
// is the in-memory reference implementation (and the natural test double).
// Indexer composes an embeddingclient.Embedder with a VectorStore so callers
// can index and search raw text without handling vectors directly. Cosine is
// the shared similarity function.
//
// Concurrency: MemoryVectorStore is safe for concurrent use via sync.RWMutex;
// callers must not mutate an Entry's Vector or Meta after handing it to the
// store or after receiving it in a search result. Indexer is safe for
// concurrent use whenever its Embedder and VectorStore are.
package vectorstore

import (
	embeddingclient "github.com/geoffmilleraz/agentic-kit/foundations/embedding/embedding-client"
)

// Entry is the unit stored in a VectorStore: a pre-computed embedding together
// with an opaque metadata map describing what the vector represents.
type Entry struct {
	// ID uniquely identifies the entry; Upsert replaces entries by ID.
	ID string

	// Vector is the dense embedding for the entry.
	Vector embeddingclient.Vector

	// Meta carries caller-defined metadata (for example "file", "symbol",
	// "title"). The store treats it as opaque and never interprets it.
	Meta map[string]string
}

// Scored is a search result — an Entry paired with its cosine-similarity
// score against the query vector.
type Scored struct {
	Entry
	Score float64
}

// VectorStore is the persistence interface for dense embedding vectors.
// Callers insert or replace entries via Upsert and retrieve the top-K nearest
// entries via Search.
type VectorStore interface {
	// Upsert inserts or replaces entries identified by Entry.ID.
	Upsert(entries ...Entry) error

	// Search returns the topK entries nearest to query (cosine similarity),
	// ordered by score descending; ties broken by ID ascending. A topK <= 0
	// returns all entries.
	Search(query embeddingclient.Vector, topK int) ([]Scored, error)

	// Delete removes the entries with the given IDs. Unknown IDs are ignored.
	Delete(ids ...string) error

	// Len reports the number of entries currently stored.
	Len() int
}
