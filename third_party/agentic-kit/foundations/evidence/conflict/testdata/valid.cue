// valid.cue — concrete values that satisfy the #Conflict constraint defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package conflict

// A claim_vs_claim conflict: two declarants make incompatible assertions.
_conflictClaimVsClaim: #Conflict & {
	Kind:     "claim_vs_claim"
	Subject:  "prop-alice-location"
	Field:    ""
	A: {Kind: "assertion", ID: "asrt-001"}
	B: {Kind: "assertion", ID: "asrt-002"}
	Severity: 0.4
	Reason:   "declarants alice and bob make incompatible assertions on proposition prop-alice-location"
}

// A claim_vs_evidence conflict: an assertion contradicts a documentary span.
_conflictClaimVsEvidence: #Conflict & {
	Kind:     "claim_vs_evidence"
	Subject:  "entity-contract-42"
	Field:    "signatory"
	A: {Kind: "assertion", ID: "asrt-003"}
	B: {Kind: "span", ID: "span-001"}
	Severity: 0.82
	Reason:   "assertion asrt-003 conflicts with span span-001 on entity entity-contract-42, field signatory"
}

// A self_inconsistency conflict: same declarant, incompatible non-retraction content.
_conflictSelfInconsistency: #Conflict & {
	Kind:     "self_inconsistency"
	Subject:  "prop-contract-date"
	Field:    ""
	A: {Kind: "assertion", ID: "asrt-010"}
	B: {Kind: "assertion", ID: "asrt-011"}
	Severity: 0.4
	Reason:   "declarant alice made incompatible assertions on proposition prop-contract-date"
}

// A recantation conflict: declarant retracts a prior assertion.
_conflictRecantation: #Conflict & {
	Kind:     "recantation"
	Subject:  "prop-meeting-attendance"
	Field:    ""
	A: {Kind: "assertion", ID: "asrt-020"}
	B: {Kind: "assertion", ID: "asrt-021"}
	Severity: 0.4
	Reason:   "declarant bob retracted a prior assertion on proposition prop-meeting-attendance"
}

// A qualification conflict: a qualifies-stance assertion co-present with affirm/deny.
_conflictQualification: #Conflict & {
	Kind:     "qualification"
	Subject:  "prop-payment-received"
	Field:    ""
	A: {Kind: "assertion", ID: "asrt-030"}
	B: {Kind: "assertion", ID: "asrt-031"}
	Severity: 0.4
	Reason:   "declarant carol qualifies a proposition that also has an affirming or denying assertion"
}

// Minimum severity boundary (both sides rank 0, e.g. ungraded).
_conflictMinSeverity: #Conflict & {
	Kind:     "claim_vs_claim"
	Subject:  "prop-x"
	Field:    ""
	A: {Kind: "assertion", ID: "asrt-a"}
	B: {Kind: "assertion", ID: "asrt-b"}
	Severity: 0.0
	Reason:   "test: minimum severity"
}

// Maximum severity boundary (both sides direct, rank 5).
_conflictMaxSeverity: #Conflict & {
	Kind:     "claim_vs_evidence"
	Subject:  "entity-y"
	Field:    "status"
	A: {Kind: "assertion", ID: "asrt-c"}
	B: {Kind: "span", ID: "span-c"}
	Severity: 1.0
	Reason:   "test: maximum severity"
}

// A healthy coverage where all proposition subjects matched spans.
_coverageHealthy: #Coverage & {
	AssertionCount:               3
	PropositionCount:             2
	SpanCount:                    4
	PropositionSubjectsMatched:   2
	PropositionSubjectsUnmatched: 0
	SpanEntityIDsMatched:         2
	SpanEntityIDsUnmatched:       2
}

// A degenerate coverage where IDs are disjoint (the cracked-keystone scenario).
// PropositionSubjectsMatched == 0 and SpanEntityIDsMatched == 0 despite
// non-zero PropositionCount and SpanCount.
_coverageDegenerate: #Coverage & {
	AssertionCount:               5
	PropositionCount:             3
	SpanCount:                    4
	PropositionSubjectsMatched:   0
	PropositionSubjectsUnmatched: 3
	SpanEntityIDsMatched:         0
	SpanEntityIDsUnmatched:       4
}

// A zero-input coverage (nothing passed in — not degenerate by definition).
_coverageEmpty: #Coverage & {
	AssertionCount:               0
	PropositionCount:             0
	SpanCount:                    0
	PropositionSubjectsMatched:   0
	PropositionSubjectsUnmatched: 0
	SpanEntityIDsMatched:         0
	SpanEntityIDsUnmatched:       0
}

// A DetectResult from a healthy run.
_detectResultHealthy: #DetectResult & {
	Coverage: _coverageHealthy
}

// A DetectResult from a degenerate run (disjoint ID namespaces).
_detectResultDegenerate: #DetectResult & {
	Coverage: _coverageDegenerate
}
