// Package honestygate — valid test data that must unify with schema.cue.
package honestygate

import "github.com/geoffmilleraz/agentic-kit/foundations/quality/honesty-gate:honestygate"

// validClaim demonstrates a well-formed claim with good evidence.
validClaim: honestygate.#Claim & {
	Text:          "The deployment pipeline runs on Kubernetes."
	EvidenceCount: 3
	Confidence:    0.9
}

// validClaimLowEvidence demonstrates a claim with minimal evidence.
validClaimLowEvidence: honestygate.#Claim & {
	Text:          "Caching is enabled for the read path."
	EvidenceCount: 1
	Confidence:    0.5
}

// validClaimZeroEvidence demonstrates a claim with no supporting evidence.
validClaimZeroEvidence: honestygate.#Claim & {
	Text:          "Some unverified statement."
	EvidenceCount: 0
	Confidence:    0.0
}

// validDecisionAllowed demonstrates an allowed verdict.
validDecisionAllowed: honestygate.#Decision & {
	Verdict: "allowed"
	Reason:  "claim is supported by 3 evidence items with confidence 0.90"
}

// validDecisionHedge demonstrates a hedge verdict.
validDecisionHedge: honestygate.#Decision & {
	Verdict: "hedge"
	Reason:  "claim has 1 evidence item (below required 2 for allowed); qualify with uncertainty language"
}

// validDecisionBlock demonstrates a block verdict.
validDecisionBlock: honestygate.#Decision & {
	Verdict: "block"
	Reason:  "claim has 0 supporting evidence items; minimum for publication is 1"
}

// validVerdicts verifies all verdict enum values are accepted.
_v1: honestygate.#Verdict & "allowed"
_v2: honestygate.#Verdict & "hedge"
_v3: honestygate.#Verdict & "block"
