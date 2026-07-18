// Package embeddingprovenance — valid test data that must unify with schema.cue.
package embeddingprovenance

import "github.com/geoffmilleraz/agentic-kit/foundations/data/embedding-provenance:embeddingprovenance"

// adaEmbedding is a well-formed provenance record for an OpenAI Ada embedding.
adaEmbedding: embeddingprovenance.#EmbeddingProvenance & {
	VectorID:  "vec-001"
	Model:     "text-embedding-ada-002"
	SourceID:  "doc-abc123"
	CreatedAt: 1717200000
	Dims:      1536
}

// bgeEmbedding is a well-formed provenance record for a BGE local embedding.
bgeEmbedding: embeddingprovenance.#EmbeddingProvenance & {
	VectorID:  "vec-002"
	Model:     "bge-base-en-v1.5"
	SourceID:  "chunk-xyz789"
	CreatedAt: 1717201000
	Dims:      768
}

// minimalEmbedding uses the smallest valid integer values for CreatedAt and Dims.
minimalEmbedding: embeddingprovenance.#EmbeddingProvenance & {
	VectorID:  "vec-min"
	Model:     "model-v1"
	SourceID:  "src-1"
	CreatedAt: 1
	Dims:      1
}
