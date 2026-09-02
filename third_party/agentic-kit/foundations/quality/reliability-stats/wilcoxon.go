package reliabilitystats

import (
	"math"
	"sort"
)

// wilcoxonExactMaxN is the largest pair count for which Wilcoxon
// enumerates the exact signed-rank null distribution; above it the
// normal approximation with continuity correction is used.
const wilcoxonExactMaxN = 25

// Wilcoxon runs the two-sided Wilcoxon signed-rank test on the paired
// differences x[i] − y[i] (Wilcoxon 1945, "Individual Comparisons by
// Ranking Methods"). Zero differences are dropped and counted in Zeros;
// tied |d| take average ranks. WPlus and WMinus are the rank sums of
// the positive and negative differences, W is the smaller of the two
// (the conventional T statistic), N is the pair count after dropping
// zeros.
//
// For N <= 25 the p-value is exact: the null distribution of the rank
// sum is enumerated over all 2^N sign assignments with the tie-adjusted
// ranks actually observed, and P = min(1, 2·Pr[T <= W]). Above 25 it is
// the normal approximation with the tie-corrected variance
// N(N+1)(2N+1)/24 − Σ(t³−t)/48 and a 0.5 continuity correction; Exact
// reports which branch produced P.
//
// Errors: ErrLengthMismatch when len(x) != len(y); ErrInvalidSample when
// a difference is NaN or Inf; ErrNoPairs when no non-zero difference
// remains.
func Wilcoxon(x, y []float64) (WilcoxonResult, error) {
	if len(x) != len(y) {
		return WilcoxonResult{}, ErrLengthMismatch
	}
	type pair struct {
		abs float64
		pos bool
	}
	pairs := make([]pair, 0, len(x))
	zeros := 0
	for i := range x {
		d := x[i] - y[i]
		switch {
		case math.IsNaN(d) || math.IsInf(d, 0):
			return WilcoxonResult{}, ErrInvalidSample
		case d == 0:
			zeros++
		case d > 0:
			pairs = append(pairs, pair{d, true})
		default:
			pairs = append(pairs, pair{-d, false})
		}
	}
	n := len(pairs)
	if n == 0 {
		return WilcoxonResult{}, ErrNoPairs
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].abs < pairs[j].abs })

	ranks := make([]float64, n)
	var tieCorrection float64 // Σ(t³ − t) over tie groups
	for i := 0; i < n; {
		j := i
		for j < n && pairs[j].abs == pairs[i].abs {
			j++
		}
		avg := float64(i+1+j) / 2 // mean of ranks i+1 .. j
		for k := i; k < j; k++ {
			ranks[k] = avg
		}
		if t := j - i; t > 1 {
			tieCorrection += float64(t*t*t - t)
		}
		i = j
	}

	var wPlus, wMinus float64
	for i, p := range pairs {
		if p.pos {
			wPlus += ranks[i]
		} else {
			wMinus += ranks[i]
		}
	}
	w := math.Min(wPlus, wMinus)
	res := WilcoxonResult{W: w, WPlus: wPlus, WMinus: wMinus, N: n, Zeros: zeros}
	if n <= wilcoxonExactMaxN {
		res.P = wilcoxonExactP(ranks, w)
		res.Exact = true
	} else {
		res.P = wilcoxonNormalP(n, w, tieCorrection)
	}
	return res, nil
}

// wilcoxonExactP returns min(1, 2·Pr[T <= w]) where T is the rank sum of
// a uniformly random subset of ranks (each sign equally likely under the
// null). Ranks are doubled to integers (average ranks are multiples of
// 0.5) and the distribution is built by the subset-sum recurrence; the
// counts are exact in float64 up to 2^53, far above the 2^25 subsets of
// the largest exact case.
func wilcoxonExactP(ranks []float64, w float64) float64 {
	total := 0
	doubled := make([]int, len(ranks))
	for i, r := range ranks {
		doubled[i] = int(math.Round(2 * r))
		total += doubled[i]
	}
	counts := make([]float64, total+1)
	counts[0] = 1
	for _, r := range doubled {
		for s := total; s >= r; s-- {
			counts[s] += counts[s-r]
		}
	}
	w2 := int(math.Round(2 * w))
	if w2 > total {
		w2 = total
	}
	var cum float64
	for s := 0; s <= w2; s++ {
		cum += counts[s]
	}
	p := 2 * cum / math.Ldexp(1, len(ranks))
	if p > 1 {
		p = 1
	}
	return p
}

// wilcoxonNormalP returns the two-sided p-value from the normal
// approximation to the signed-rank distribution with tie-corrected
// variance and a 0.5 continuity correction toward the mean, evaluated at
// the smaller rank sum w (so z <= 0 and P = 2·Φ(z)).
func wilcoxonNormalP(n int, w, tieCorrection float64) float64 {
	fn := float64(n)
	mu := fn * (fn + 1) / 4
	variance := fn*(fn+1)*(2*fn+1)/24 - tieCorrection/48
	if variance <= 0 {
		return 1
	}
	z := (w + 0.5 - mu) / math.Sqrt(variance)
	if z > 0 { // w never exceeds the mean; the correction must not cross it
		z = 0
	}
	p := 2 * normalCDF(z)
	if p > 1 {
		p = 1
	}
	return p
}

// normalCDF is Φ(z), the standard normal distribution function.
func normalCDF(z float64) float64 {
	return 0.5 * math.Erfc(-z/math.Sqrt2)
}
