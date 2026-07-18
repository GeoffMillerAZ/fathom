// Package embeddingprovenance defines CUE contracts for embedding provenance records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package embeddingprovenance

// #EmbeddingProvenance captures the origin of a stored embedding vector.
#EmbeddingProvenance: {
	// VectorID is the unique identifier of the stored vector.
	VectorID: string & !=""

	// Model is the name/version of the model that produced the embedding
	// (e.g. "text-embedding-ada-002", "bge-base-en-v1.5").
	Model: string & !=""

	// SourceID is the identifier of the source document or chunk that was
	// embedded (e.g. a document UUID or chunk key).
	SourceID: string & !=""

	// CreatedAt is the Unix timestamp (seconds) when the embedding was created.
	// Must be a positive integer.
	CreatedAt: int & >0

	// Dims is the dimensionality of the embedding vector (e.g. 768, 1536).
	// Must be a positive integer.
	Dims: int & >0
}
