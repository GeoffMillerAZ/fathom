// Package confidencescoring defines the CUE schema for confidence scoring.
// These constraints mirror the Go types and are evaluated with `cue vet ./...`.
package confidencescoring

// #ConfidenceScore is a bounded integer in [1, 5].
// 1 = very low confidence, 5 = very high confidence.
#ConfidenceScore: >=1 & <=5 & int

// #EvidenceInput holds the evidence signals used to derive a ConfidenceScore.
#EvidenceInput: {
	// Count is the number of distinct evidence items (must be non-negative).
	Count: >=0 & int

	// AgreementRatio is the fraction of agreeing evidence, in [0.0, 1.0].
	AgreementRatio: >=0.0 & <=1.0 & number
}
