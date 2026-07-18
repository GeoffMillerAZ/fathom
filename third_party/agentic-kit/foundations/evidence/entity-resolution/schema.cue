// Package entityresolution defines the CUE-first contract for entity-resolution
// candidates and human-review workflow. Go types in types.go and similarity.go
// mirror these definitions; any drift is a bug.
//
// Purpose: propose match candidates between entities (by name similarity) for
// human review. This foundation NEVER auto-merges; it only proposes. Accepted
// candidates are consumed downstream by entity-crosswalk, which stores the
// confirmed mappings.
package entityresolution

// _nonEmpty is a reusable constraint for non-empty strings.
_nonEmpty: string & !=""

// #Tier mirrors evidencegrade.Tier — inlined here for CUE schema self-containment.
#Tier: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred"

// #Grade mirrors evidencegrade.Grade — inlined here for CUE schema self-containment.
#Grade: {
	// Tier is the epistemic strength.
	Tier: #Tier
	// Confidence is the certainty in [0, 1].
	Confidence: >=0 & <=1
}

// #HumanAuthor mirrors aiprovenance.HumanAuthor.
#HumanAuthor: {
	ID: _nonEmpty
}

// #AIAuthor mirrors aiprovenance.AIAuthor.
#AIAuthor: {
	ModelID:               _nonEmpty
	ModelVersion:          _nonEmpty
	PromptHash:            _nonEmpty
	PromptTemplateVersion: string
	ParamsHash:            string
	InputDigest:           _nonEmpty
	At:                    _nonEmpty
}

// #Author mirrors aiprovenance.Author — inlined here for CUE schema self-containment.
// Exactly one of the two branches must match:
//   - Kind: "human" → Human required; AI forbidden.
//   - Kind: "ai"    → AI required; Human forbidden.
#Author: ({
	Kind:  "human"
	Human: #HumanAuthor
	AI?:   _|_
}) | ({
	Kind:   "ai"
	AI:     #AIAuthor
	Human?: _|_
})

// #Candidate is a proposed match between two entities with a similarity score,
// the method that produced it, full author provenance, and an epistemic grade.
//
// Canonical-ordering invariant: LeftID must be lexicographically less than
// RightID. This prevents the same entity pair from appearing as both (a,b) and
// (b,a), which would break de-duplication in downstream review audit trails.
// GenerateCandidates enforces this automatically; Candidate.Validate() rejects
// violations for hand-constructed values.
#Candidate: {
	// LeftID is the identifier of the first (lexicographically smaller) entity.
	// Must be strictly less than RightID.
	LeftID: _nonEmpty

	// RightID is the identifier of the second entity. Must be strictly greater
	// than LeftID.
	RightID: _nonEmpty & >LeftID

	// Score is the similarity score in the closed interval [0, 1].
	// 0 = no similarity, 1 = identical.
	Score: >=0 & <=1

	// Method names the algorithm that produced this candidate
	// (e.g. "name-similarity").
	Method: _nonEmpty

	// Author records who or what produced this candidate (human or AI model).
	// Mandatory — every candidate must have a traceable author.
	Author: #Author

	// Grade is the epistemic strength of this candidate.
	Grade: #Grade

	// SourceSpanIDs are the IDs of the evidence spans from which the entity
	// IDs were extracted. May be empty when span-level tracking is unavailable.
	SourceSpanIDs?: [...string]
}

// #ReviewState represents the human-review lifecycle of a match candidate.
#ReviewState: "proposed" | "accepted" | "rejected"

// #Match wraps a Candidate with its review state, reviewer metadata, and
// timestamps. Every Match must have a CreatedAt timestamp; ReviewedBy and
// ReviewedAt are required when State is "accepted" or "rejected".
//
// Timestamp values are RFC 3339 strings; the Go layer uses time.Time.
#Match: {
	// Candidate is the underlying proposed match.
	Candidate: #Candidate

	// State is the current review lifecycle state.
	State: #ReviewState

	// CreatedAt is the RFC 3339 timestamp at which this Match was proposed.
	// Required on all matches.
	CreatedAt: _nonEmpty

	// ReviewedBy is the identifier of the human reviewer. Required when State
	// is "accepted" or "rejected".
	ReviewedBy?: string

	// ReviewedAt is the RFC 3339 timestamp at which the reviewer decided.
	// Required when State is "accepted" or "rejected".
	ReviewedAt?: string

	// Note is an optional free-text rationale from the reviewer.
	Note?: string

	// Author records the author of the review decision.
	Author?: #Author

	// Cross-field constraints: when State is decided, reviewer fields are required.
	if State == "accepted" || State == "rejected" {
		ReviewedBy: _nonEmpty
		ReviewedAt: _nonEmpty
	}
}
