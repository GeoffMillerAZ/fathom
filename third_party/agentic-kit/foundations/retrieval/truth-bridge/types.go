// Package truthbridge is the span↔chunk link layer: it joins the evidence
// spine (evidence-span, evidence-grade, evidence-anchor) to vector retrieval
// (vectorstore.Entry, vectorstore.Scored). A LinkedChunk binds one vector-store
// chunk to one evidence span with an explicit grade and provenance reference;
// Graded decorates raw vectorstore.Scored results with that evidence context.
//
// The honest-default rule: a chunk that has no ChunkLink is NEVER silently
// promoted to any evidence tier. Graded marks such chunks with
// GradeLabel("unknown") and sets GradedResult.Graded = false. Callers that
// collapse or aggregate results MUST preserve this signal.
//
// Concurrency: ChunkLink and GradedResult are plain value types with no mutable
// state; values are safe to read concurrently. Bind and Graded are pure
// functions and may be called concurrently without additional synchronisation.
//
// CUE contracts live in schema.cue; this file mirrors those definitions.
package truthbridge

import (
	vectorstore "github.com/geoffmilleraz/agentic-kit/foundations/embedding/vector-store"
	evidencegrade "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade"
	evidencespan "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-span"
)

// GradeLabel is the evidence-grade label carried on a retrieval result.
// The special value GradeUnknown is the honest default for unlinked chunks.
//
// CUE mirror: #GradeLabel in schema.cue.
type GradeLabel string

const (
	// GradeUnknown is the explicit, honest label for chunks that arrived at the
	// retriever with no ChunkLink. It must NEVER be silently promoted or omitted.
	// Downstream consumers MUST surface or log this label rather than treating
	// the chunk as implicitly trusted.
	GradeUnknown GradeLabel = "unknown"
	// GradeDirect corresponds to evidencegrade.TierDirect.
	GradeDirect GradeLabel = "direct"
	// GradeDocumentary corresponds to evidencegrade.TierDocumentary.
	GradeDocumentary GradeLabel = "documentary"
	// GradeCircumstantial corresponds to evidencegrade.TierCircumstantial.
	GradeCircumstantial GradeLabel = "circumstantial"
	// GradeTestimonial corresponds to evidencegrade.TierTestimonial.
	GradeTestimonial GradeLabel = "testimonial"
	// GradeInferred corresponds to evidencegrade.TierInferred.
	GradeInferred GradeLabel = "inferred"
)

// tierToLabel maps the five canonical Tier values to their GradeLabel
// counterparts. Evidence-grade and truth-bridge must stay aligned.
var tierToLabel = map[evidencegrade.Tier]GradeLabel{
	evidencegrade.TierDirect:         GradeDirect,
	evidencegrade.TierDocumentary:    GradeDocumentary,
	evidencegrade.TierCircumstantial: GradeCircumstantial,
	evidencegrade.TierTestimonial:    GradeTestimonial,
	evidencegrade.TierInferred:       GradeInferred,
}

// ChunkLink is the bridge record joining one vector-store chunk to one
// evidence span. It carries the grade and provenance that were current at
// index time so retrieval can attach evidence context to results without
// re-fetching the span.
//
// CUE mirror: #ChunkLink in schema.cue.
type ChunkLink struct {
	// ChunkID is the vectorstore.Entry.ID of the stored chunk.
	// Must be non-empty.
	ChunkID string

	// SpanLocatorKind is the Locator.Kind from the bound span (may be empty for
	// legacy or non-located chunks).
	SpanLocatorKind string

	// SpanLocatorValue is the Locator.Value from the bound span.
	SpanLocatorValue string

	// GradeTier is the epistemic tier at index time. Must be one of the five
	// recognised evidencegrade.Tier values.
	GradeTier evidencegrade.Tier

	// SourceID is the SourceID of the originating evidence span. May be empty
	// for unregistered sources, but callers should prefer registered sources
	// so the full provenance chain is traceable.
	SourceID string

	// ContentHash is the SHA-256 hex digest (lowercase, 64 chars) of the
	// chunk's raw text content. Used by evidence-ranker for ref-stability
	// scoring: a hash-backed ref ranks above an unanchored one.
	// Must match the pattern [0-9a-f]{64}.
	ContentHash string

	// AnchorProofRef is the optional SpanProof digest hex reference produced
	// by evidence-anchor when the span was anchored. Empty means unanchored.
	// Anchored chunks rank higher under ref-stability scoring.
	AnchorProofRef string
}

// GradedResult is a vectorstore.Scored result decorated with evidence grade
// and provenance from the matched ChunkLink (or explicitly marked unknown).
//
// Invariant: Graded == true iff Grade != GradeUnknown. This invariant is
// enforced by Graded() and must be preserved by any code that constructs a
// GradedResult directly.
//
// CUE mirror: #GradedResult in schema.cue.
type GradedResult struct {
	// ChunkID echoes vectorstore.Entry.ID for correlation with the raw result.
	ChunkID string

	// Score is the cosine-similarity score from the vector store, in [0, 1].
	Score float64

	// Grade is the evidence grade label. GradeUnknown when no ChunkLink matched.
	// Must never be empty.
	Grade GradeLabel

	// Graded is false when Grade == GradeUnknown (the chunk has no registered
	// evidence link). Callers must never discard ungraded chunks silently.
	Graded bool

	// SourceID is the SourceID from the matched ChunkLink. Empty when Graded == false.
	SourceID string

	// AnchorProofRef is the SpanProof digest reference from the matched ChunkLink.
	// Empty when unanchored or when Graded == false.
	AnchorProofRef string
}

// LinkedChunk pairs a ChunkLink with the raw chunk text, forming the indexed
// unit that Bind produces. It is what the indexing layer stores alongside the
// vector-store entry so that retrieval can later call Graded.
type LinkedChunk struct {
	// Link is the evidence binding for this chunk.
	Link ChunkLink

	// Text is the raw chunk text that was embedded and stored.
	Text string
}

// Bind constructs a LinkedChunk from a vectorstore.Document (an Entry plus its
// raw text), the evidence span it was extracted from, the grade assigned at
// index time, and an optional anchor proof reference.
//
// The contentHash is computed by the caller (typically SHA-256 of the text).
// Bind does not call any hash function — it is pure and allocation-only.
//
// Parameters:
//   - doc: the vectorstore.Entry to link. doc.ID becomes ChunkLink.ChunkID.
//   - span: the evidencespan.Span this chunk was extracted from. The span's
//     Locator and SourceID are copied to the link. The span need not have
//     passed ValidateAsEvidence; partial provenance is supported at index time.
//   - grade: the evidencegrade.Grade assigned to this chunk at index time.
//   - contentHash: the SHA-256 hex digest of the chunk text (caller-supplied).
//   - anchorProofRef: the SpanProof digest reference (empty if unanchored).
//   - text: the raw chunk text to store in LinkedChunk.Text.
//
// Bind never returns an error; all validation is the caller's responsibility.
func Bind(doc vectorstore.Entry, span evidencespan.Span, grade evidencegrade.Grade, contentHash, anchorProofRef, text string) LinkedChunk {
	return LinkedChunk{
		Link: ChunkLink{
			ChunkID:          doc.ID,
			SpanLocatorKind:  span.Locator.Kind,
			SpanLocatorValue: span.Locator.Value,
			GradeTier:        grade.Tier,
			SourceID:         span.SourceID,
			ContentHash:      contentHash,
			AnchorProofRef:   anchorProofRef,
		},
		Text: text,
	}
}

// Graded decorates a slice of vectorstore.Scored results with their evidence
// grades and provenance by looking up each result's ID in links.
//
// Every result in results is included in the output (no silent drops). Results
// with no matching ChunkLink are assigned Grade == GradeUnknown and
// Graded == false. The output is in the same order as results.
//
// links may be nil or empty; in that case every result is marked unknown.
// Graded is a pure function: it does not modify results or links.
func Graded(results []vectorstore.Scored, links []ChunkLink) []GradedResult {
	// Build a lookup map from chunk ID to ChunkLink.
	linkMap := make(map[string]ChunkLink, len(links))
	for _, l := range links {
		linkMap[l.ChunkID] = l
	}

	out := make([]GradedResult, len(results))
	for i, r := range results {
		gr := GradedResult{
			ChunkID: r.ID,
			Score:   r.Score,
		}
		if link, ok := linkMap[r.ID]; ok {
			label, known := tierToLabel[link.GradeTier]
			if !known {
				// Unknown tier string in the link — honest unknown.
				label = GradeUnknown
			}
			gr.Grade = label
			gr.Graded = label != GradeUnknown
			gr.SourceID = link.SourceID
			gr.AnchorProofRef = link.AnchorProofRef
		} else {
			// No link registered for this chunk: explicit GradeUnknown.
			// This is the honest default: never silently ungraded.
			gr.Grade = GradeUnknown
			gr.Graded = false
		}
		out[i] = gr
	}
	return out
}
