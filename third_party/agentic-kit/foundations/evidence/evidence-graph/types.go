// Package evidencegraph provides an in-memory evidence graph with TTL-based
// eviction and contradiction detection. It is the Go counterpart of the CUE
// contracts defined in schema.cue — the two must be kept in sync.
//
// Concurrency contract: Graph is not safe for concurrent use; callers that
// share a Graph across goroutines must guard access with their own
// synchronization.
package evidencegraph

import "time"

// Span is the atomic evidence unit. It records a single claimed value for an
// (EntityID, Field) pair together with provenance metadata.
//
// CUE mirror: #Span in schema.cue.
type Span struct {
	// ID uniquely identifies this span within the graph.
	ID string

	// EntityID is the identifier of the real-world entity this span describes.
	EntityID string

	// Field names the attribute being claimed (e.g. "owner", "language").
	Field string

	// Value is the claimed value for EntityID.Field.
	Value string

	// Confidence is the certainty of this claim in the closed interval [0, 1].
	Confidence float64

	// Source identifies the origin of this span (source family + artifact ref).
	Source string

	// ExpiresAt is the wall-clock instant after which the span is stale and
	// will be evicted by Sweep. The zero value means the span never expires.
	ExpiresAt time.Time
}

// Edge is a typed, directed relationship between two spans.
//
// CUE mirror: #Edge in schema.cue.
type Edge struct {
	// From is the ID of the source span.
	From string

	// To is the ID of the target span.
	To string

	// Rel is the relationship type (e.g. "supports", "contradicts",
	// "supersedes", "corroborates").
	Rel string
}

// Contradiction records a pair of non-expired spans that make incompatible
// claims about the same (EntityID, Field) pair. The graph never silently
// resolves contradictions — they are surfaced for human review.
type Contradiction struct {
	// EntityID is the entity both spans describe.
	EntityID string

	// Field is the attribute on which the two spans disagree.
	Field string

	// A is one of the conflicting spans.
	A Span

	// B is the other conflicting span.
	B Span
}
