// Package hypothesisranker defines the CUE-first contracts for hypothesis
// ranking. Go types in types.go mirror these definitions; any drift is a bug.
package hypothesisranker

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #HypothesisStatus is the lifecycle state of a hypothesis.
// CUE mirror of HypothesisStatus in types.go.
#HypothesisStatus: "open" | "supported" | "falsified" | "closed"

// #GradeTierLabel is the epistemic tier label derived from supporting spans.
// Mirrors evidencegrade.Tier without a cross-package import (Invariant 8: purity).
// CUE mirror of GradeTierLabel in types.go.
#GradeTierLabel: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred" | ""

// #AuthorKind identifies whether a hypothesis was produced by a human or AI.
// CUE mirror of AuthorKind in types.go.
#AuthorKind: "human" | "ai"

// #HypothesisAuthor carries provenance metadata for AI-generated hypotheses
// (Invariants 1 + 3: provenance mandatory; AI contributions labelled).
// CUE mirror of HypothesisAuthor in types.go.
#HypothesisAuthor: {
	// Kind is "human" or "ai".
	Kind: #AuthorKind

	// Model is the AI model identifier. Empty for human authors.
	Model: string

	// PromptHash is the content-addressed hash of the prompt that produced this
	// hypothesis. Empty for human authors.
	PromptHash: string

	// InputDigest is the content hash of the evidence inputs fed to the model.
	// Empty for human authors.
	InputDigest: string
}

// #Hypothesis is a candidate explanation under investigation, together with
// its current evidence-backed support score.
//
// Enrichment added 2026-06-08 (audit findings):
//   - SpanRefs: evidence span IDs backing the Support score (Invariant 6: traceability).
//   - Status: lifecycle state (open|supported|falsified|closed).
//   - GradeTier: epistemic tier of the supporting spans (Invariant 4: grade <= weakest tier).
//   - Author: who or what produced this hypothesis (Invariant 3: AI labelled).
#Hypothesis: {
	// ID uniquely identifies this hypothesis. Must be non-empty.
	ID: _nonEmpty

	// Statement is a human-readable description of the hypothesis.
	Statement: string

	// Support is the evidence-weighted probability score in [0, 1].
	// 0 = no support, 1 = fully supported. NaN is not representable in CUE
	// and is rejected by Go Validate().
	Support: >=0 & <=1

	// SpanRefs is the ordered list of evidence span IDs whose grades produced
	// the Support score (Invariant 6: outputs traceable or drafts). May be empty
	// during hypothesis construction but should be populated before the
	// hypothesis is surfaced as a citable conclusion.
	SpanRefs: [...string]

	// Status is the lifecycle state. The zero value ("") is allowed in Go (open
	// by convention) but validators should prefer an explicit value.
	Status?: #HypothesisStatus

	// GradeTier is the epistemic tier derived from supporting spans.
	// "" (unset) is the zero value and means no grade has been assigned.
	GradeTier?: #GradeTierLabel

	// Author carries provenance for AI-generated hypotheses.
	// Absent (null/omit) means the author is unknown or unrecorded.
	Author?: #HypothesisAuthor

	// CreatedAt is the wall-clock creation timestamp for audit trail purposes.
	// Represented as a CUE string (RFC 3339) because CUE has no native time
	// type; the Go layer uses time.Time. The zero value (omitted) is valid.
	// Added 2026-06-08 as an additive field; existing instances may omit it.
	CreatedAt?: string
}
