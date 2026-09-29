// Package claimlifecycle defines the CUE schema for claim lifecycle records.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
//
// Tier and grade shapes are imported directly from the evidencegrade package
// (the canonical source of truth) so that any upstream change is caught at
// `cue vet` time rather than silently diverging.
package claimlifecycle

import eg "github.com/geoffmilleraz/agentic-kit/foundations/evidence/evidence-grade:evidencegrade"

// #ClaimState enumerates the valid lifecycle states of a Claim.
#ClaimState: "proposed" | "supported" | "refuted" | "uncertain" | "superseded" | "recanted"

// #ClaimEventKind enumerates the valid FSM event discriminators.
#ClaimEventKind: "proposed" | "supported" | "refuted" | "marked_uncertain" | "reopened" | "superseded" | "recanted" | "assertion_linked" | "evidence_linked"

// #ClaimEvent enumerates the simple transition event names (legacy / external
// surface). New code should use #ClaimEventKind.
#ClaimEvent: "support" | "refute" | "mark_uncertain" | "reopen"

// #Claim represents a statement under review moving through the lifecycle FSM.
#Claim: {
	// ID is a non-empty stable identifier for the claim.
	ID: string & !=""

	// Text is the human-readable statement being evaluated.
	Text: string & !=""

	// State is the current lifecycle state.
	State: #ClaimState

	// CreatedAt is the RFC 3339 timestamp when the claim was first proposed.
	CreatedAt: string

	// UpdatedAt is the RFC 3339 timestamp of the last state change.
	UpdatedAt: string
}

// #ClaimView is the projected state of a claim as of a given instant, derived
// by folding its event stream.
#ClaimView: {
	// ClaimID is the stable identifier of the claim.
	ClaimID: string & !=""

	// Text is the most recently recorded claim text.
	Text: string

	// State is the current lifecycle state derived from the event fold.
	State: #ClaimState

	// Grade is the computed epistemic grade derived from linked evidence spans.
	// Null when no evidence has been linked. Stores the full #ClaimGrade so
	// FloorTier, Promoted, and PromotionReason are preserved for legal audit
	// (docs/practices.md §2).
	Grade?: eg.#ClaimGrade | null

	// LinkedAssertions contains the IDs of all assertions linked to the claim.
	LinkedAssertions: [...string]

	// LinkedEvidence contains the SpanIDs of all evidence spans linked to the claim.
	LinkedEvidence: [...string]

	// History records the sequence of event kinds in append order.
	History: [...#ClaimEventKind]
}
