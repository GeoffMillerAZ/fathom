// Package honestygate flags overclaiming in model outputs using deterministic
// rules. No LLM is involved; decisions are fully reproducible given the same
// inputs.
//
// The gate decides between three verdicts:
//
//   - Allowed  — the claim is adequately supported.
//   - Hedge    — the claim is marginal; the output should be qualified.
//   - Block    — the claim is unsupported and must not be published.
//
// Concurrency: values are immutable and safe for concurrent use.
package honestygate

// Verdict is the outcome of a honesty-gate evaluation.
type Verdict string

const (
	// VerdictAllowed means the claim is adequately supported by evidence.
	VerdictAllowed Verdict = "allowed"

	// VerdictHedge means the claim is marginal; output should be qualified with
	// uncertainty language before being published.
	VerdictHedge Verdict = "hedge"

	// VerdictBlock means the claim lacks sufficient evidence and must not be
	// published as-is.
	VerdictBlock Verdict = "block"
)

// Claim is a statement asserted by a model output.
type Claim struct {
	// Text is the human-readable statement being evaluated.
	Text string

	// EvidenceCount is the number of distinct supporting evidence items
	// available for this claim (e.g. distinct documents, citations, or facts).
	// Must be >= 0.
	EvidenceCount int

	// Confidence is the model's self-reported confidence in [0, 1].
	// Zero means no confidence signal was provided.
	Confidence float64
}

// Decision is the result of Gate.Evaluate.
type Decision struct {
	// Verdict is the gate's ruling.
	Verdict Verdict

	// Reason explains why the verdict was reached.
	Reason string
}

// Gate holds the thresholds for a honesty-gate evaluation. All thresholds are
// inclusive lower bounds unless noted otherwise.
type Gate struct {
	// MinEvidenceAllowed is the minimum evidence count required for Allowed.
	// Default (zero-value) is 2.
	MinEvidenceAllowed int

	// MinEvidenceHedge is the minimum evidence count required for Hedge.
	// Below this count the verdict is Block. Default is 1.
	MinEvidenceHedge int

	// MinConfidenceAllowed is the minimum Confidence required for Allowed when
	// evidence count alone is at the boundary. Default is 0.7.
	MinConfidenceAllowed float64
}
