// Package correctionestimator estimates the effort and likelihood of correction
// required for a set of findings using deterministic scoring rules.
// No LLM is involved; the estimate is fully reproducible.
//
// Concurrency: values are immutable and safe for concurrent use.
package correctionestimator

// Band classifies the overall correction burden into a coarse category.
type Band string

const (
	// BandNegligible means no meaningful correction effort is expected.
	BandNegligible Band = "negligible"

	// BandLow means minor corrections may be needed.
	BandLow Band = "low"

	// BandMedium means moderate correction work is anticipated.
	BandMedium Band = "medium"

	// BandHigh means significant correction effort is expected.
	BandHigh Band = "high"

	// BandCritical means the output likely requires fundamental rework.
	BandCritical Band = "critical"
)

// FindingStats summarises the counts of findings by severity, as produced by a
// review or QA run.
type FindingStats struct {
	// Critical is the count of critical-severity findings.
	Critical int

	// Major is the count of major-severity findings.
	Major int

	// Minor is the count of minor-severity findings.
	Minor int

	// Informational is the count of informational findings (no correction
	// expected, but included for completeness tracking).
	Informational int
}

// Estimate is the output of the correction estimator.
type Estimate struct {
	// Score is a composite correction-burden score in [0, 1].
	// Higher = more correction effort expected.
	Score float64

	// Band is the coarse category derived from Score.
	Band Band
}
