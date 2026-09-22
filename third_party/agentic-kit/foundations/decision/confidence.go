package decision

// Confidence turns a probability distribution into a single confidence
// number in [0, 1], per the vendor-documented formula: with k the number of
// entries in probabilities and max the largest value among them,
//
//	k <= 1: 1
//	k >  1: (k*max - 1) / (k-1), clamped to [0, 1]
//
// A degenerate distribution (zero or one entries) is certain by
// construction, hence 1. For k=3 this is exactly TypeSafe's documented
// three-option formula (3*max-1)/2. Confidence does not require the values
// to sum to 1 — it only uses k and max — so it tolerates an unnormalised or
// even empty map.
func Confidence(probabilities map[string]float64) float64 {
	k := len(probabilities)
	if k <= 1 {
		return 1
	}
	max := 0.0
	first := true
	for _, v := range probabilities {
		if first || v > max {
			max = v
			first = false
		}
	}
	c := (float64(k)*max - 1) / float64(k-1)
	return clamp01(c)
}

// Band classifies a Confidence value into one of three named ranges a
// caller can switch on. See BandOf.
type Band string

// The three confidence bands. See BandOf for the exact boundaries.
const (
	// BandUnsure is a confidence below UnsureBelow: too low to act on
	// without more evidence.
	BandUnsure Band = "unsure"
	// BandConfirm is a confidence between UnsureBelow and
	// ProceedAtOrAbove: act, but confirm first.
	BandConfirm Band = "confirm"
	// BandProceed is a confidence at or above ProceedAtOrAbove: act
	// directly.
	BandProceed Band = "proceed"
)

// UnsureBelow is the confidence threshold below which BandOf returns
// BandUnsure.
const UnsureBelow = 0.5

// ProceedAtOrAbove is the confidence threshold at or above which BandOf
// returns BandProceed.
const ProceedAtOrAbove = 0.9

// BandOf classifies confidence into a Band: below UnsureBelow is
// BandUnsure, at or above ProceedAtOrAbove is BandProceed, otherwise
// BandConfirm. Boundaries are exact: confidence == UnsureBelow is
// BandConfirm, and confidence == ProceedAtOrAbove is BandProceed.
func BandOf(confidence float64) Band {
	switch {
	case confidence < UnsureBelow:
		return BandUnsure
	case confidence >= ProceedAtOrAbove:
		return BandProceed
	default:
		return BandConfirm
	}
}

// clamp01 clamps v to [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
