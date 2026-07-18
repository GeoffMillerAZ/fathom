// Package truthbridge defines the CUE-first contract for the span↔chunk link
// that joins the evidence spine to vector retrieval. Go types in types.go
// mirror these definitions; any drift is a bug. CUE is authoritative for
// shape; Go is authoritative for behaviour.
package truthbridge

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #GradeTier re-declares the canonical evidence-grade tier enumeration inline
// so this schema is self-contained for CUE tooling (cross-module CUE imports
// require explicit package-name disambiguation). Must remain in sync with
// evidencegrade.#Tier in evidence-grade/schema.cue.
#GradeTier: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred"

// #ChunkLink is the span↔chunk bridge record: one vector-store chunk bound to
// one evidence span, carrying the grade that was assigned when the chunk was
// indexed.
//
//   - chunk_id: the Entry.ID from the vector store.
//   - span_locator_kind: the Locator.Kind value from the bound span.
//   - span_locator_value: the Locator.Value value from the bound span.
//   - grade_tier: the epistemic tier at index time (must be a valid #GradeTier).
//   - source_id: the SourceID of the originating evidence span.
//   - content_hash: SHA-256 hex digest of the chunk's raw text (64 chars,
//     lowercase). Used for ref-stability scoring in the ranker.
//   - anchor_proof_ref: optional reference (span digest hex) to a SpanProof
//     produced by evidence-anchor. Empty string means unanchored.
#ChunkLink: {
	chunk_id:           _nonEmpty
	span_locator_kind:  string // may be "" for legacy / non-located chunks
	span_locator_value: string
	grade_tier:         #GradeTier
	source_id:          string // may be "" for unregistered sources
	content_hash:       =~"^[0-9a-f]{64}$"
	anchor_proof_ref:   string // "" = unanchored
}

// #GradeLabel labels a retrieval result with its evidence grade. The special
// value "unknown" is the honest default: a chunk that arrived without a link
// is NEVER silently promoted to any tier — it must be explicitly graded.
#GradeLabel: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred" | "unknown"

// #GradedResult pairs a vector-store Scored result with provenance, an explicit
// grade label, and an anchoring flag.
//
//   - chunk_id: the Entry.ID echoed from the vector store for correlation.
//   - score: the cosine-similarity score from the vector store.
//   - grade: the evidence grade label. "unknown" when no ChunkLink was found.
//   - graded: false iff grade == "unknown" (derived; never silently omitted).
//   - source_id: the SourceID of the originating span (empty when ungraded).
//   - anchor_proof_ref: the SpanProof digest reference (empty when unanchored).
#GradedResult: {
	chunk_id:         _nonEmpty
	score:            >=0 & <=1
	grade:            #GradeLabel
	graded:           bool
	source_id:        string
	anchor_proof_ref: string

	// Consistency invariant: graded == true iff grade != "unknown".
	_gradedConsistent: (grade == "unknown") == !graded
}
