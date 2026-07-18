// Package confidencescoring provides a ConfidenceScore type (1..5) with a
// deterministic derivation from evidence count and agreement.
//
// Concurrency: all exported functions are pure and safe for concurrent use.
package confidencescoring

import "fmt"

// ConfidenceScore is a bounded integer in [1, 5] representing how certain
// we are about a claim or finding.
// 1 = very low confidence, 5 = very high confidence.
type ConfidenceScore int

// Validate returns an error if the score is outside the valid range [1, 5].
func (cs ConfidenceScore) Validate() error {
	if cs < 1 || cs > 5 {
		return fmt.Errorf("confidencescoring: ConfidenceScore must be 1–5, got %d", int(cs))
	}
	return nil
}

// EvidenceInput holds the evidence signals used to derive a ConfidenceScore.
type EvidenceInput struct {
	// Count is the number of distinct evidence items.
	Count int

	// AgreementRatio is the fraction of evidence items that agree with
	// the claim, in the range [0.0, 1.0].
	AgreementRatio float64
}
