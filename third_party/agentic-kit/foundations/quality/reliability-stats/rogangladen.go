package reliabilitystats

import (
	"math"
	"math/rand"
	"sort"
)

// RoganGladen corrects an observed positive rate for an imperfect binary
// judge (Rogan & Gladen 1978, "Estimating prevalence from the results of
// a screening test"). With the judge's true-positive rate tpr
// (sensitivity) and true-negative rate tnr (specificity), the corrected
// rate is (observed + tnr − 1)/(tpr + tnr − 1), clamped to [0, 1].
//
// The interval is a seeded parametric percentile bootstrap. Each of
// resamples replicates draws the positive count of n judged items from
// Binomial(n, observed), applies the same correction with tpr and tnr
// held fixed, clamps, and the bounds are type-7 quantiles of the sorted
// replicates (the same rule as the other intervals here). It therefore
// carries sampling noise in the n judged items only; uncertainty in tpr
// and tnr themselves (the size of the calibration set) is NOT included,
// so the interval is too narrow when the judge was calibrated on few
// items. The RNG is local and seeded: same arguments, same bytes. The
// result's Tasks and Clusters are zero.
//
// A judge no better than chance (tpr + tnr <= 1) makes the correction
// undefined or sign-flipped: the point and both bounds are NaN with a nil
// error.
//
// Errors: ErrEmptySample (n < 1); ErrInvalidResamples; ErrInvalidLevel;
// ErrInvalidSample (observed, tpr or tnr NaN or outside [0, 1]).
func RoganGladen(observed, tpr, tnr float64, n int, seed int64, resamples int, level float64) (Interval, error) {
	if n < 1 {
		return Interval{}, ErrEmptySample
	}
	if resamples < 1 {
		return Interval{}, ErrInvalidResamples
	}
	if !(level > 0 && level < 1) {
		return Interval{}, ErrInvalidLevel
	}
	for _, v := range [...]float64{observed, tpr, tnr} {
		if math.IsNaN(v) || v < 0 || v > 1 {
			return Interval{}, ErrInvalidSample
		}
	}
	nan := math.NaN()
	if tpr+tnr <= 1 {
		return Interval{Low: nan, High: nan, Point: nan}, nil
	}
	denom := tpr + tnr - 1
	correct := func(obs float64) float64 {
		return math.Min(1, math.Max(0, (obs+tnr-1)/denom))
	}
	rng := rand.New(rand.NewSource(seed))
	reps := make([]float64, resamples)
	for b := range reps {
		pos := 0
		for i := 0; i < n; i++ {
			if rng.Float64() < observed {
				pos++
			}
		}
		reps[b] = correct(float64(pos) / float64(n))
	}
	sort.Float64s(reps)
	alpha := 1 - level
	return Interval{
		Low:   quantile(reps, alpha/2),
		High:  quantile(reps, 1-alpha/2),
		Point: correct(observed),
	}, nil
}
