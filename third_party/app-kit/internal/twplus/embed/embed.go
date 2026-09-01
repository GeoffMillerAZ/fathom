// Package embed turns component text into vectors for semantic search. It is a
// strictly optional enhancement (ADR-0010): the corpus and BM25 search never
// depend on it. The Embedder interface is the seam — the real implementation
// talks to a local Ollama daemon, but everything downstream (the vec1 store,
// the hybrid query) is exercised with a deterministic fake in tests, so the
// machinery is verifiable without a running model.
package embed

import (
	"context"
	"math"
)

// Vector is an L2-normalized embedding. Cosine similarity reduces to a dot
// product on normalized vectors, which is what vec1's distance ranks by.
type Vector []float32

// Embedder maps text to fixed-dimension, L2-normalized vectors. Implementations
// must return exactly one vector per input, in order.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([]Vector, error)
	// Model identifies the embedder for pinning (e.g. "qwen3-embedding-0.6b").
	// A query embedded with a different model than the index must not be fused.
	Model() string
	// Dims is the vector dimensionality every returned Vector has.
	Dims() int
}

// Normalize scales v to unit L2 length in place and returns it. A zero vector is
// returned unchanged (no NaNs). Implementations call this so callers can rely on
// every Vector being normalized regardless of what the backend returned.
func Normalize(v Vector) Vector {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= inv
	}
	return v
}
