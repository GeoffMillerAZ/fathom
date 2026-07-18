// Package embeddingprovenance — invalid examples.
// ALL blocks below are commented out so that `cue vet ./...` passes.
// Uncomment individual blocks to observe the validation error described.
package embeddingprovenance

// import "github.com/geoffmilleraz/agentic-kit/foundations/data/embedding-provenance:embeddingprovenance"

// -- Example 1: zero CreatedAt ----------------------------------------------
// zeroCreatedAt would fail because CreatedAt must be >0.
//
// zeroCreatedAt: embeddingprovenance.#EmbeddingProvenance & {
// 	VectorID:  "vec-bad"
// 	Model:     "model-v1"
// 	SourceID:  "src-1"
// 	CreatedAt: 0    // ERROR: violates int & >0
// 	Dims:      768
// }

// -- Example 2: zero Dims ---------------------------------------------------
// zeroDims would fail because Dims must be >0.
//
// zeroDims: embeddingprovenance.#EmbeddingProvenance & {
// 	VectorID:  "vec-bad2"
// 	Model:     "model-v1"
// 	SourceID:  "src-2"
// 	CreatedAt: 1717200000
// 	Dims:      0    // ERROR: violates int & >0
// }

// -- Example 3: empty VectorID ----------------------------------------------
// emptyVectorID would fail because VectorID must be a non-empty string.
//
// emptyVectorID: embeddingprovenance.#EmbeddingProvenance & {
// 	VectorID:  ""   // ERROR: violates & !=""
// 	Model:     "model-v1"
// 	SourceID:  "src-3"
// 	CreatedAt: 1717200000
// 	Dims:      768
// }
