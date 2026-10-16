package vectorstore

import (
	"math"

	embeddingclient "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/embedding/embedding-client"
)

// Cosine returns the cosine similarity between a and b, in [-1, 1].
//
// Degenerate inputs are handled explicitly rather than panicking or guessing:
// if the vectors differ in length, or either vector is empty, Cosine returns
// 0 — vectors of different dimensionality are simply not comparable. If either
// vector has zero magnitude (all components zero) the angle is undefined and
// Cosine likewise returns 0.
func Cosine(a, b embeddingclient.Vector) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
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
