// Package evidencegraph defines the CUE-first contracts for the in-memory
// evidence graph. Go types in types.go mirror these definitions; any drift
// between the two is a bug.
package evidencegraph

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Span is the atomic evidence unit stored as a node in the graph.
// It records a single claimed value for a (EntityID, Field) pair along with
// the provenance and confidence of that claim.
#Span: {
	// ID uniquely identifies this span within the graph.
	ID: _nonEmpty

	// EntityID is the identifier of the real-world entity this span describes.
	EntityID: _nonEmpty

	// Field names the attribute being claimed (e.g. "owner", "language").
	Field: _nonEmpty

	// Value is the claimed value for EntityID.Field.
	Value: _nonEmpty

	// Confidence is the certainty of this claim in the closed interval [0, 1].
	// 0 = no confidence, 1 = fully verified.
	Confidence: >=0 & <=1

	// Source identifies the origin of this span (source family + artifact ref).
	Source: _nonEmpty

	// ExpiresAt is an RFC 3339 timestamp after which the span is considered
	// stale and will be evicted by Sweep. Empty string means no expiry.
	ExpiresAt: string
}

// #Edge is a typed, directed relationship between two spans.
#Edge: {
	// From is the ID of the source span.
	From: _nonEmpty

	// To is the ID of the target span.
	To: _nonEmpty

	// Rel is the relationship type (e.g. "supports", "contradicts",
	// "supersedes", "corroborates").
	Rel: _nonEmpty
}

// #Contradiction records a pair of non-expired spans that make incompatible
// claims about the same (EntityID, Field) pair. The graph never silently
// resolves contradictions — they are surfaced for human review. Mirrors the
// Go Contradiction struct in types.go.
#Contradiction: {
	// EntityID is the entity both spans describe.
	EntityID: _nonEmpty

	// Field is the attribute on which the two spans disagree.
	Field: _nonEmpty

	// A is one of the conflicting spans.
	A: #Span

	// B is the other conflicting span.
	B: #Span
}
