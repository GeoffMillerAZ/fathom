package reliabilitystats

import "math"

// A12 returns the Vargha–Delaney A statistic (Vargha & Delaney 2000;
// the effect size Arcuri & Briand 2011 recommend for comparing
// randomized algorithms): the probability that a value drawn from a is
// greater than one drawn from b, ties counted half —
// (#[a > b] + ½·#[a == b]) / (len(a)·len(b)). 0.5 is no effect; 1 means
// every a beats every b; the conventional magnitude reading is 0.56
// small, 0.64 medium, 0.71 large (and symmetrically below 0.5). It is
// computed pairwise — the definition itself — in O(len(a)·len(b)); the
// rank-sum form A = (R₁/m − (m+1)/2)/n is the same number.
//
// An empty a or b returns NaN.
func A12(a, b []float64) float64 {
	if len(a) == 0 || len(b) == 0 {
		return math.NaN()
	}
	var score float64
	for _, x := range a {
		for _, y := range b {
			switch {
			case x > y:
				score++
			case x == y:
				score += 0.5
			}
		}
	}
	return score / float64(len(a)*len(b))
}
