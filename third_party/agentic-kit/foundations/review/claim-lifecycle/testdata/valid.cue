// Package claimlifecycle — valid test data that must unify with schema.cue.
package claimlifecycle

import "github.com/geoffmilleraz/agentic-kit/foundations/review/claim-lifecycle:claimlifecycle"

// validClaimProposed demonstrates a well-formed claim in proposed state.
validClaimProposed: claimlifecycle.#Claim & {
	ID:        "claim-001"
	Text:      "The revenue figure for Q4 is $4.8M."
	State:     "proposed"
	CreatedAt: "2024-11-01T09:00:00Z"
	UpdatedAt: "2024-11-01T09:00:00Z"
}

// validClaimSupported demonstrates a supported claim.
validClaimSupported: claimlifecycle.#Claim & {
	ID:        "claim-002"
	Text:      "Code review process requires two approvals."
	State:     "supported"
	CreatedAt: "2024-11-01T09:00:00Z"
	UpdatedAt: "2024-11-01T11:00:00Z"
}

// validClaimRefuted demonstrates a refuted claim.
validClaimRefuted: claimlifecycle.#Claim & {
	ID:        "claim-003"
	Text:      "System uses SHA-1 for password hashing."
	State:     "refuted"
	CreatedAt: "2024-11-02T08:00:00Z"
	UpdatedAt: "2024-11-02T10:00:00Z"
}

// validClaimUncertain demonstrates an uncertain claim.
validClaimUncertain: claimlifecycle.#Claim & {
	ID:        "claim-004"
	Text:      "Backup runs nightly."
	State:     "uncertain"
	CreatedAt: "2024-11-03T07:00:00Z"
	UpdatedAt: "2024-11-03T09:00:00Z"
}

// validClaimSuperseded demonstrates a superseded (terminal) claim.
validClaimSuperseded: claimlifecycle.#Claim & {
	ID:        "claim-005"
	Text:      "Superseded by newer evidence."
	State:     "superseded"
	CreatedAt: "2024-11-04T09:00:00Z"
	UpdatedAt: "2024-11-04T12:00:00Z"
}

// validClaimRecanted demonstrates a recanted (terminal) claim.
validClaimRecanted: claimlifecycle.#Claim & {
	ID:        "claim-006"
	Text:      "Author withdrew this claim."
	State:     "recanted"
	CreatedAt: "2024-11-05T09:00:00Z"
	UpdatedAt: "2024-11-05T14:00:00Z"
}

// validStates verifies all state enum values are accepted.
_s1: claimlifecycle.#ClaimState & "proposed"
_s2: claimlifecycle.#ClaimState & "supported"
_s3: claimlifecycle.#ClaimState & "refuted"
_s4: claimlifecycle.#ClaimState & "uncertain"
_s5: claimlifecycle.#ClaimState & "superseded"
_s6: claimlifecycle.#ClaimState & "recanted"

// validEvents verifies all legacy event enum values are accepted.
_e1: claimlifecycle.#ClaimEvent & "support"
_e2: claimlifecycle.#ClaimEvent & "refute"
_e3: claimlifecycle.#ClaimEvent & "mark_uncertain"
_e4: claimlifecycle.#ClaimEvent & "reopen"

// validEventKinds verifies all #ClaimEventKind discriminators are accepted.
_k1: claimlifecycle.#ClaimEventKind & "proposed"
_k2: claimlifecycle.#ClaimEventKind & "supported"
_k3: claimlifecycle.#ClaimEventKind & "refuted"
_k4: claimlifecycle.#ClaimEventKind & "marked_uncertain"
_k5: claimlifecycle.#ClaimEventKind & "reopened"
_k6: claimlifecycle.#ClaimEventKind & "superseded"
_k7: claimlifecycle.#ClaimEventKind & "recanted"
_k8: claimlifecycle.#ClaimEventKind & "assertion_linked"
_k9: claimlifecycle.#ClaimEventKind & "evidence_linked"

// validClaimViewNoGrade demonstrates a ClaimView with no evidence linked yet.
validClaimViewNoGrade: claimlifecycle.#ClaimView & {
	ClaimID:          "claim-007"
	Text:             "Proposed claim awaiting evidence."
	State:            "proposed"
	Grade:            null
	LinkedAssertions: []
	LinkedEvidence:   []
	History:          ["proposed"]
}

// validClaimViewWithGrade demonstrates a ClaimView with a full ClaimGrade,
// preserving FloorTier and Promoted (CLAUDE.md invariant §4, §6).
validClaimViewWithGrade: claimlifecycle.#ClaimView & {
	ClaimID: "claim-008"
	Text:    "Supported claim with evidence grade."
	State:   "supported"
	Grade: {
		Grade: {
			Tier:       "documentary"
			Confidence: 0.8
		}
		Promoted:                false
		PromotionReason:         ""
		FloorTier:               "documentary"
		DroppedSpanIDs:          []
		AIContributedConfidence: 0.0
	}
	LinkedAssertions: []
	LinkedEvidence:   ["span-001"]
	History:          ["proposed", "evidence_linked", "supported"]
}
