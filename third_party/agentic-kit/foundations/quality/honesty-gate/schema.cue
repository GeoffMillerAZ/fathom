// Package honestygate defines the CUE schema for the honesty-gate foundation.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package honestygate

// #Verdict enumerates the three possible gate outcomes.
#Verdict: "allowed" | "hedge" | "block"

// #Claim is a statement asserted by a model output under evaluation.
#Claim: {
	// Text is the human-readable statement being evaluated.
	Text: string & !=""

	// EvidenceCount is the number of distinct supporting evidence items; >= 0.
	EvidenceCount: int & >=0

	// Confidence is the model's self-reported confidence in [0, 1].
	Confidence: number & >=0 & <=1
}

// #Decision is the result of a honesty-gate evaluation.
#Decision: {
	// Verdict is the gate's ruling.
	Verdict: #Verdict

	// Reason is a non-empty human-readable explanation.
	Reason: string & !=""
}
