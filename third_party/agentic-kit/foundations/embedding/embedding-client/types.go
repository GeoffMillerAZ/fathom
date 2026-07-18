// Package embeddingclient provides the single embedding interface for the
// agentic-kit platform. Every consumer that needs vector embeddings goes
// through here so cache hits, retry behaviour, and dimension consistency are
// not reimplemented throughout the codebase.
//
// Composition: backends implement Embedder (and optionally
// DimensionedEmbedder); CachingEmbedder decorates any Embedder with a bounded
// LRU cache; MockEmbedder is the deterministic test double.
//
// Concurrency: all exported types in this package are safe for concurrent
// use — CachingEmbedder via an internal sync.Mutex plus atomic counters,
// MockEmbedder via a sync.Mutex plus an atomic call counter.
package embeddingclient

import "context"

// Vector is a slice of float64 values representing a dense embedding.
type Vector []float64

// Embedder is the interface implemented by any embedding backend.
type Embedder interface {
	// Embed returns the embedding vector for a single text string.
	Embed(ctx context.Context, text string) (Vector, error)

	// EmbedBatch returns embedding vectors for a slice of text strings.
	// The returned slice is parallel to texts: result[i] is the vector for
	// texts[i]. Implementations are free to call the underlying backend in
	// parallel, but callers must not rely on any particular ordering of
	// in-flight requests.
	EmbedBatch(ctx context.Context, texts []string) ([]Vector, error)
}

// DimensionedEmbedder is an optional capability interface for Embedders that
// know the dimensionality of the vectors they produce. It is deliberately not
// part of Embedder so existing implementations stay valid; callers discover
// the capability via DimensionsOf.
type DimensionedEmbedder interface {
	Embedder

	// Dimensions returns the number of elements in every Vector this
	// embedder produces, or 0 when the dimension is not (yet) known —
	// e.g. an HTTP backend that infers it from its first response.
	Dimensions() int
}

// DimensionsOf reports the vector dimensionality of e. It returns (d, true)
// when e implements DimensionedEmbedder and reports a positive dimension, and
// (0, false) otherwise — including when the capability exists but the
// dimension is not yet known.
func DimensionsOf(e Embedder) (int, bool) {
	de, ok := e.(DimensionedEmbedder)
	if !ok {
		return 0, false
	}
	d := de.Dimensions()
	if d <= 0 {
		return 0, false
	}
	return d, true
}
