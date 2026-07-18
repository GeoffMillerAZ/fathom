// Package correctionledger — valid test data that must unify with schema.cue.
package correctionledger

import "github.com/geoffmilleraz/agentic-kit/foundations/review/correction-ledger:correctionledger"

// validFinding demonstrates a well-formed finding in the "reviewing" state.
validFinding: correctionledger.#Finding & {
	ID:          "finding-001"
	State:       "reviewing"
	Description: "Revenue figure for Q4 does not match audited financials."
	CreatedAt:   "2024-11-01T09:00:00Z"
	UpdatedAt:   "2024-11-01T10:30:00Z"
}

// validCorrection demonstrates a well-formed correction with confidence 4.
validCorrection: correctionledger.#Correction & {
	FindingID: "finding-001"
	Target: {
		StableTarget: "report/section/revenue.q4.v3"
	}
	Original:    "$4.2M"
	Corrected:   "$4.8M"
	CorrectedBy: "alice@example.com"
	CorrectedAt: "2024-11-01T10:30:00Z"
	Reason:      "Original figure used pre-audit draft; audited figure is $4.8M."
	Confidence:  4
}

// validDismissal demonstrates a well-formed dismissal that carries NO corrected fact.
validDismissal: correctionledger.#Dismissal & {
	FindingID:   "finding-002"
	DismissedBy: "bob@example.com"
	Reason:      "Known limitation documented in the methodology appendix; acceptable risk."
	DismissedAt: "2024-11-02T14:15:00Z"
}

// validDismissalWithAuthor demonstrates a dismissal with human author provenance.
validDismissalWithAuthor: correctionledger.#Dismissal & {
	FindingID:   "finding-003"
	DismissedBy: "carol@example.com"
	Reason:      "False positive; the methodology appendix explicitly permits this pattern."
	DismissedAt: "2024-11-03T09:00:00Z"
	Author: {
		Kind:       "human"
		IdentityID: "carol@example.com"
	}
}

// validCorrectionWithAuthor demonstrates a correction with full AI author provenance.
validCorrectionWithAuthor: correctionledger.#Correction & {
	FindingID: "finding-004"
	Target: {
		StableTarget: "report/section/revenue.q1.v2"
	}
	Original:    "$3.1M"
	Corrected:   "$3.5M"
	CorrectedBy: "claude-opus-4-8"
	CorrectedAt: "2024-11-03T10:00:00Z"
	Reason:      "Original figure used pre-audit draft; audited figure is $3.5M."
	Confidence:  4
	Author: {
		Kind:          "ai"
		IdentityID:    "claude-opus-4-8"
		AIModelID:     "claude-opus-4-8"
		AIPromptHash:  "sha256:abc123"
		AIInputDigest: "sha256:def456"
	}
	SourceHash: "sha256:fedcba987654"
	Locator:    "doc://report/section/revenue#q1"
}

// validCorrectionTarget demonstrates a versioned stable target identifier.
validCorrectionTarget: correctionledger.#CorrectionTarget & {
	StableTarget: "table/revenue/col/q4.v3"
}

// validStates verifies all state enum values are accepted.
_s1: correctionledger.#State & "open"
_s2: correctionledger.#State & "reviewing"
_s3: correctionledger.#State & "corrected"
_s4: correctionledger.#State & "dismissed"
_s5: correctionledger.#State & "deferred"

// validEvents verifies all event enum values are accepted.
_e1: correctionledger.#Event & "claim"
_e2: correctionledger.#Event & "correct"
_e3: correctionledger.#Event & "dismiss"
_e4: correctionledger.#Event & "defer"
_e5: correctionledger.#Event & "reopen"
