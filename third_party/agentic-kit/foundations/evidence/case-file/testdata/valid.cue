// valid.cue — concrete values that satisfy the #CaseFile, #GradedAssertion,
// #AuthenticityDigest, #ShiftDigest, #SupportSummary, and #SupportTier
// constraints defined in schema.cue.
//
// Running `cue vet ./...` from the package root validates these against the schema.
package casefile

// A minimal case file: one affirming assertion, no conflicts, no shifts.
_caseFileMinimal: #CaseFile & {
	PropositionID: "prop-001"
	Supporting: [{
		AssertionID: "asrt-001"
		Declarant:   "entity:alice"
		Stance:      "affirms"
		Content:     "Alice was present at the scene"
		Tier:        "testimonial"
		TierRank:    2
	}]
	Contradicting:       []
	Retracting:          []
	Qualifying:          []
	Conflicts:           []
	AuthenticityDigests: []
	ShiftHistory:        []
	SupportSummary: {
		SupportScore:    0.4
		AgainstScore:    0.0
		Tier:            "moderate_support"
		AffirmingCount:  1
		DenyingCount:    0
		RetractingCount: 0
		QualifyingCount: 0
	}
}

// A contested case file: affirming and denying with equal weight.
_caseFileContested: #CaseFile & {
	PropositionID: "prop-002"
	Supporting: [{
		AssertionID: "asrt-010"
		Declarant:   "entity:alice"
		Stance:      "affirms"
		Content:     "The document was signed on 2024-01-15"
		Tier:        "testimonial"
		TierRank:    2
	}]
	Contradicting: [{
		AssertionID: "asrt-011"
		Declarant:   "entity:bob"
		Stance:      "denies"
		Content:     "The document was not signed on 2024-01-15"
		Tier:        "testimonial"
		TierRank:    2
	}]
	Retracting:          []
	Qualifying:          []
	Conflicts: [{
		Kind:    "claim_vs_claim"
		Subject: "prop-002"
		Field:   ""
		A: {Kind: "assertion", ID: "asrt-010"}
		B: {Kind: "assertion", ID: "asrt-011"}
		Severity: 0.4
		Reason: "declarants alice and bob make incompatible assertions on proposition prop-002"
	}]
	AuthenticityDigests: []
	ShiftHistory:        []
	SupportSummary: {
		SupportScore:    0.2
		AgainstScore:    0.2
		Tier:            "contested"
		AffirmingCount:  1
		DenyingCount:    1
		RetractingCount: 0
		QualifyingCount: 0
	}
}

// A strong-support case file: high-tier affirming assertion.
_caseFileStrongSupport: #CaseFile & {
	PropositionID: "prop-003"
	Supporting: [{
		AssertionID: "asrt-020"
		Declarant:   "entity:lab"
		Stance:      "affirms"
		Content:     "DNA match confirmed"
		Tier:        "direct"
		TierRank:    5
	}]
	Contradicting:       []
	Retracting:          []
	Qualifying:          []
	Conflicts:           []
	AuthenticityDigests: []
	ShiftHistory:        []
	SupportSummary: {
		SupportScore:    1.0
		AgainstScore:    0.0
		Tier:            "strong_support"
		AffirmingCount:  1
		DenyingCount:    0
		RetractingCount: 0
		QualifyingCount: 0
	}
}

// A case file with authenticity digest and shift history.
_caseFileWithDigestAndShift: #CaseFile & {
	PropositionID: "prop-004"
	Supporting:          []
	Contradicting:       []
	Retracting: [{
		AssertionID: "asrt-030"
		Declarant:   "entity:carol"
		Stance:      "recants"
		Content:     "I did not see what I thought I saw"
		Tier:        "testimonial"
		TierRank:    2
	}]
	Qualifying: []
	Conflicts:  []
	AuthenticityDigests: [{
		RecordID:     "rec-abc"
		Overall:      "likely_authentic"
		Confidence:   0.72
		Conservative: false
	}]
	ShiftHistory: [{
		Declarant:     "entity:carol"
		PropositionID: "prop-004"
		FromID:        "asrt-029"
		ToID:          "asrt-030"
		Kind:          "recantation"
		GapDays:       7
	}]
	SupportSummary: {
		SupportScore:    0.0
		AgainstScore:    0.0
		Tier:            "unresolved"
		AffirmingCount:  0
		DenyingCount:    0
		RetractingCount: 1
		QualifyingCount: 0
	}
}

// Boundary: unresolved when no affirming or denying assertions exist.
_caseFileUnresolved: #CaseFile & {
	PropositionID:       "prop-empty"
	Supporting:          []
	Contradicting:       []
	Retracting:          []
	Qualifying:          []
	Conflicts:           []
	AuthenticityDigests: []
	ShiftHistory:        []
	SupportSummary: {
		SupportScore:    0.0
		AgainstScore:    0.0
		Tier:            "unresolved"
		AffirmingCount:  0
		DenyingCount:    0
		RetractingCount: 0
		QualifyingCount: 0
	}
}
