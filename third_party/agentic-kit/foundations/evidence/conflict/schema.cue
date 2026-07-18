// Package conflict defines the CUE-first contract for the legal-grade conflict
// and contradiction taxonomy. Go types in conflict.go mirror these definitions;
// any drift is a build failure (cue vet runs in pre-commit).
package conflict

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #ConflictKind enumerates the recognised conflict classifications.
//
// claim_vs_claim      — two different declarants, same proposition, incompatible content.
// claim_vs_evidence   — an assertion conflicts with a documentary/physical span.
// self_inconsistency  — same declarant, same proposition, incompatible non-retraction content.
// recantation         — same declarant, same proposition, a later retraction.
// qualification       — a qualifies-stance assertion co-present with an affirm/deny assertion.
// partial             — reserved for partial overlaps; not emitted by Detect in v1.
#ConflictKind: "claim_vs_claim" | "claim_vs_evidence" | "self_inconsistency" | "recantation" | "qualification" | "partial"

// #RefKind discriminates which type of evidence item a #Ref points at.
#RefKind: "assertion" | "span"

// #Ref is a discriminated pointer at one side of a conflict.
#Ref: {
	// Kind discriminates whether this ref points at an assertion or a span.
	Kind: #RefKind

	// ID is the ID of the referenced item. Must be non-empty.
	ID: _nonEmpty
}

// #Conflict is a typed, graded conflict between two evidence items.
//
// A and B are ordered so that A.ID <= B.ID within each (Kind, Subject, Field)
// group, ensuring each unordered pair is reported exactly once.
//
// Severity formula (see Go package doc for full rationale):
//   maxRank  = max(rank(A), rank(B))   // Tier.Rank(), 1..5
//   minRank  = min(rank(A), rank(B))
//   severity = (maxRank/5.0) * 0.7 + (minRank/5.0) * 0.3
#Conflict: {
	// Kind classifies the nature of the conflict.
	Kind: #ConflictKind

	// Subject is the shared anchor: PropositionID for assertion-vs-assertion
	// conflicts, or Span.EntityID for claim-vs-evidence conflicts.
	Subject: _nonEmpty

	// Field is the Span.Field when one side is a span, otherwise empty string.
	Field: string

	// A is the first (lexicographically smaller or equal ID) side.
	A: #Ref

	// B is the second side.
	B: #Ref

	// Severity is a deterministic score in [0, 1].
	Severity: >=0 & <=1

	// Reason is a human-readable explanation of the conflict.
	Reason: string
}

// #Coverage reports the join statistics produced by DetectWithPropositions.
// All counts are non-negative integers.
//
// A result is degenerate when PropositionCount > 0 AND SpanCount > 0 AND
// PropositionSubjectsMatched == 0 AND SpanEntityIDsMatched == 0 — meaning
// non-empty inputs on both sides but zero joins formed. This is a strong signal
// that the ID namespaces on each side are disjoint (the cracked-keystone bug).
#Coverage: {
	// AssertionCount is the number of assertions passed in.
	AssertionCount: int & >=0

	// PropositionCount is the number of propositions passed in.
	PropositionCount: int & >=0

	// SpanCount is the number of spans passed in.
	SpanCount: int & >=0

	// PropositionSubjectsMatched is the number of distinct Proposition.Subject
	// values that matched at least one Span.EntityID.
	PropositionSubjectsMatched: int & >=0

	// PropositionSubjectsUnmatched is the number of distinct Proposition.Subject
	// values that matched NO Span.EntityID.
	PropositionSubjectsUnmatched: int & >=0

	// SpanEntityIDsMatched is the number of distinct Span.EntityID values that
	// matched at least one Proposition.Subject.
	SpanEntityIDsMatched: int & >=0

	// SpanEntityIDsUnmatched is the number of distinct Span.EntityID values that
	// matched NO Proposition.Subject.
	SpanEntityIDsUnmatched: int & >=0
}

// #DetectResult is the return value of DetectWithPropositions.
#DetectResult: {
	// Coverage reports join statistics for the claim-vs-evidence pass.
	Coverage: #Coverage
}
