// Package elementcoverage defines the CUE-first contracts for the element
// coverage board. Go types in types.go and assess.go mirror these definitions;
// any drift is a bug.
//
// Relationship to sibling schemas:
//   - evidencegrade.#Tier and #GradedSpan are the upstream primitives used by
//     the Assess function; they are not redefined here.
//   - investigation/coverage.#SourceCoverage tracks ingestion completeness;
//     #ElementCoverage tracks epistemic completeness per theory element.
package elementcoverage

import eg "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade:evidencegrade"

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Status is the coverage outcome for a single theory element.
// Exactly three values are valid.
#Status: "met" | "under_evidenced" | "unmet"

// #Element is a named component of a theory or claim that must be supported by
// evidence (e.g. "duty", "breach", "causation", "damages" in negligence law).
//
//   - ID must be non-empty and unique within the theory.
//   - Name is the human-readable label.
//   - Required flags whether this element is necessary to the theory. Optional
//     elements are still assessed; they do not change Status semantics.
#Element: {
	ID:       _nonEmpty
	Name:     _nonEmpty
	Required: bool
}

// #ElementCoverage is the assessment result for one element after calling
// Assess. It pairs the element identity with the graded outcome and an
// actionable gap description.
//
//   - WeakestTier is empty when Status == "unmet" (no evidence was submitted).
//   - Gap is empty when Status == "met".
//   - EvidenceSpanIDs lists every SpanID in the submitted evidence (provenance
//     chain; invariant §7: no silent truncation).
//   - Confidence is in [0, 1]; 0 when no evidence was submitted.
#ElementCoverage: {
	ElementID:       _nonEmpty
	Status:          #Status
	WeakestTier:     eg.#Tier | ""
	Confidence:      >=0 & <=1
	EvidenceSpanIDs: [...string]
	Gap:             string

	// _metImpliesNoGap: when met, Gap must be empty.
	_metImpliesNoGap: Status != "met" || Gap == ""
	_metImpliesNoGap: true

	// _unmetImpliesNoTier: when unmet, WeakestTier must be empty.
	_unmetImpliesNoTier: Status != "unmet" || WeakestTier == ""
	_unmetImpliesNoTier: true
}
