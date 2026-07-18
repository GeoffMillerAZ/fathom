package correctionestimator

// Severity weights used for score calculation.
const (
	weightCritical = 1.0
	weightMajor    = 0.4
	weightMinor    = 0.1
	// Informational findings carry no weight toward the correction score.

	// capScore is the finding-count that saturates the score at 1.0.
	// Above this weighted sum the score is clamped to 1.0.
	capScore = 5.0
)

// Compute calculates a correction burden score and band from FindingStats.
//
// Algorithm:
//
//  1. weighted = critical×1.0 + major×0.4 + minor×0.1
//  2. score = min(weighted / capScore, 1.0)
//  3. band is determined by the score:
//     [0,   0.05) → negligible
//     [0.05,0.25) → low
//     [0.25,0.55) → medium
//     [0.55,0.85) → high
//     [0.85,1.0]  → critical
func Compute(stats FindingStats) Estimate {
	weighted := float64(stats.Critical)*weightCritical +
		float64(stats.Major)*weightMajor +
		float64(stats.Minor)*weightMinor

	score := weighted / capScore
	if score > 1.0 {
		score = 1.0
	}

	var band Band
	switch {
	case score < 0.05:
		band = BandNegligible
	case score < 0.25:
		band = BandLow
	case score < 0.55:
		band = BandMedium
	case score < 0.85:
		band = BandHigh
	default:
		band = BandCritical
	}

	return Estimate{
		Score: score,
		Band:  band,
	}
}
