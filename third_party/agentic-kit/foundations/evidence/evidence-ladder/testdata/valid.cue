// Package evidenceladder — valid test fixtures.
// Running `cue vet ./...` and `cue vet ./testdata/` both validate these against schema.cue.
package evidenceladder

// foundResult: a verdict where findings exist.
foundResult: #LadderResult & {
	concern_id: "concern-001"
	verdict:    "found"
	refs:       ["file://foundations/main.go#L10-L12"]
	scope_hash: "abc123def456"
	reason:     ""
}

// absentProvenResult: a verdict where absence is proven.
absentProvenResult: #LadderResult & {
	concern_id: "concern-002"
	verdict:    "absent_proven"
	refs:       []
	scope_hash: "deadbeef0001"
	reason:     "absent within declared scope at snapshot \"deadbeef0001\""
}

// absentUnverifiedResult: absence is claimed but manifest is incomplete.
absentUnverifiedResult: #LadderResult & {
	concern_id: "concern-003"
	verdict:    "absent_unverified"
	refs:       []
	scope_hash: "deadbeef0002"
	reason:     "manifest has 1 unexplained skip(s): [image.png]"
}

// unresolvedResult: ladder exhausted with no conclusion.
unresolvedResult: #LadderResult & {
	concern_id: "concern-004"
	verdict:    "unresolved"
	refs:       []
	scope_hash: ""
	reason:     "Tier-3 timeout or error (fail-closed): context deadline exceeded"
}

// escalationRecord: a valid escalation dossier.
escalationRecord: #EscalationRecord & {
	concern_id:         "concern-005"
	signature_id:       "sig-005"
	manifest_tree_hash: "tree001abc"
	candidates:         ["file://main.go#L1-L5"]
	question:           "Is the pattern absent from production code? Please review and update the signature."
}

// humanVerdict: a valid SME response with mandatory feedback.
humanVerdict: #HumanVerdict & {
	who:        "alice@example.com"
	when:       "2026-07-10T09:00:00Z"
	grade_tier: "documentary"
	rationale:  "Reviewed the codebase; pattern is genuinely absent."
	feedback:   "Add pattern variant X to the signature's predicates to cover edge cases."
}
