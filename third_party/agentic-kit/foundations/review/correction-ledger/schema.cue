// Package correctionledger defines the schema for correction-ledger records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package correctionledger

// #AuthorKind discriminates between a human reviewer and an AI model.
#AuthorKind: "human" | "ai"

// #Author carries the provenance of the entity that produced a Correction or
// Dismissal. Throughline invariant 1: every evidence span must be traceable to
// its exact producer.
//
// For human authors, IdentityID is the reviewer's identity (e.g. email or UUID).
// For AI authors, IdentityID is the model identifier and the optional AI* fields
// record the full reproduction key.
#Author: {
	Kind:       #AuthorKind
	IdentityID: string & !=""
	// AI-specific fields — required only when Kind == "ai".
	AIModelID?:     string
	AIPromptHash?:  string
	AIParamsHash?:  string
	AIInputDigest?: string
}

// #State enumerates the valid lifecycle states of a Finding.
#State: "open" | "reviewing" | "corrected" | "dismissed" | "deferred"

// #Event enumerates the valid FSM events that trigger state transitions.
#Event: "claim" | "correct" | "dismiss" | "defer" | "reopen"

// #CorrectionTarget is the versioned identifier for the entity being corrected.
#CorrectionTarget: {
	// StableTarget is a non-empty versioned identifier (e.g. "table/field.v2").
	StableTarget: string & !=""
}

// #Finding represents a surfaced claim or concern requiring human review.
#Finding: {
	ID:          string & !=""
	State:       #State
	Description: string
	CreatedAt:   string // RFC 3339 timestamp
	UpdatedAt:   string // RFC 3339 timestamp
}

// #Correction records that an underlying fact was wrong and has been corrected.
// A Correction DOES carry both the original and corrected fact values.
#Correction: {
	FindingID:   string & !=""
	Target:      #CorrectionTarget
	Original:    string & !=""
	Corrected:   string & !=""
	CorrectedBy: string & !=""
	CorrectedAt: string // RFC 3339 timestamp
	Reason:      string & !=""
	// Confidence is the corrector's certainty on a scale of 1–5.
	Confidence: >=1 & <=5 & int
	// Author optionally carries the full provenance of who or what produced this
	// correction (Throughline invariant 1). Omit if not recorded.
	Author?:     #Author
	// SourceHash is the content hash of the source document supporting this
	// correction. Optional; used for click-to-source traceability.
	SourceHash?: string
	// Locator is a URI/offset reference into the source document. Optional.
	Locator?:    string
}

// #Dismissal records that a finding was a false positive or acceptable risk.
// A Dismissal NEVER carries a corrected fact value — it does NOT mutate the
// underlying fact. This separation is a critical audit invariant.
#Dismissal: {
	FindingID:   string & !=""
	DismissedBy: string & !=""
	Reason:      string & !=""
	DismissedAt: string // RFC 3339 timestamp
	// Author optionally carries the full provenance of who or what produced this
	// dismissal (Throughline invariant 1). Omit if not recorded.
	Author?: #Author
}
