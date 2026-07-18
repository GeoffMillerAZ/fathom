// Package evidenceladder defines the CUE-first contracts for the three-tier
// evidence-acquisition ladder. The ladder drives deterministic progression from
// fast cache (Tier-1) through exhaustive search (Tier-2) to human escalation
// (Tier-3), yielding verdicts with explicitly scoped, honest provenance.
//
// CUE is authoritative for shape; Go types in types.go and evidenceladder.go
// mirror these definitions; any drift is a bug.
package evidenceladder

// #Verdict classifies the ladder outcome for a single concern.
//   - "found":             decisive findings exist.
//   - "absent_proven":     zero findings, manifest is complete (earned ABSENT_PROVEN).
//   - "absent_unverified": zero findings but manifest has unexplained skips.
//   - "unresolved":        ladder exhausted or Tier-3 timed out; no conclusion.
#Verdict: "found" | "absent_proven" | "absent_unverified" | "unresolved"

// #EscalationRecord is the dossier sent to a human via Tier-3 / hitl.
// CUE-contracted so consumers can validate before proposing.
#EscalationRecord: {
	concern_id: string & !=""
	// signature_id references the ConcernSignature that triggered escalation.
	signature_id: string & !=""
	// manifest_tree_hash ties the escalation to a specific tree snapshot.
	manifest_tree_hash: string & !=""
	// candidates are the (potentially weak/non-decisive) findings returned by Tier-2.
	candidates: [...string]
	// question is the human-readable query for the SME.
	question: string & !=""
}

// #HumanVerdict is the structured response from an SME via Tier-3.
// feedback is MANDATORY: an SME answer without a signature amendment creates
// a recurrence — the same escalation fires again on the next run.
#HumanVerdict: {
	who:       string & !=""
	when:      string & !=""   // time.RFC3339
	// grade_tier is the epistemic tier the SME assigns to their finding.
	// Uses the evidence-grade tier vocabulary.
	grade_tier: "direct" | "documentary" | "circumstantial" | "testimonial" | "inferred"
	rationale:  string & !=""
	// feedback MUST describe how the signature or retrieval should be amended.
	// An empty feedback is invalid — it means the escalation will recur.
	feedback:   string & !=""
}

// #LadderResult is the output of one complete ladder run for one concern.
#LadderResult: {
	concern_id:  string & !=""
	verdict:     #Verdict
	// refs contains file:// or source:// references when verdict=="found".
	refs: [...string]
	// scope_hash ties the result to the manifest tree_hash.
	scope_hash:  string
	// reason explains non-found verdicts (e.g. which skips triggered downgrade).
	reason:      string
}
