package reliabilitystats

import (
	"math"
	"math/big"
)

// McNemarResult is an exact two-sided McNemar test result.
// CUE mirror: #McNemarResult in mcnemar.cue.
type McNemarResult struct {
	// P is the two-sided p-value in (0, 1], clamped to the smallest
	// positive float64 if the exact probability underflows.
	P float64 `json:"p"`
	// B counts pairs where x is true and y is false.
	B int `json:"b"`
	// C counts pairs where x is false and y is true.
	C int `json:"c"`
}

// McNemarExact tests paired binary outcomes x and y, conditional on their
// discordant count n = B+C. Under the null, B is Binomial(n, 0.5).
// P = min(1, 2*Pr[Binomial(n, 0.5) <= min(B,C)]), with inclusive tails.
// Concordant pairs do not contribute; no discordance gives P = 1.
// Integer arithmetic avoids underflow at large n before the final float64
// conversion. Positive probabilities that underflow are clamped to
// math.SmallestNonzeroFloat64. Errors: ErrLengthMismatch; ErrEmptySample.
func McNemarExact(x, y []bool) (McNemarResult, error) {
	if len(x) != len(y) {
		return McNemarResult{}, ErrLengthMismatch
	}
	if len(x) == 0 {
		return McNemarResult{}, ErrEmptySample
	}
	r := McNemarResult{P: 1}
	for i := range x {
		if x[i] && !y[i] {
			r.B++
		}
		if !x[i] && y[i] {
			r.C++
		}
	}
	n, k := r.B+r.C, min(r.B, r.C)
	if r.B == r.C || n-2*k == 1 {
		return r, nil
	}
	term, sum := big.NewInt(1), big.NewInt(1)
	for i := 1; i <= k; i++ {
		term.Mul(term, big.NewInt(int64(n-i+1)))
		term.Quo(term, big.NewInt(int64(i)))
		sum.Add(sum, term)
	}
	sum.Lsh(sum, 1)
	den := new(big.Int).Lsh(big.NewInt(1), uint(n))
	r.P, _ = new(big.Rat).SetFrac(sum, den).Float64()
	if r.P == 0 {
		r.P = math.SmallestNonzeroFloat64
	}
	return r, nil
}
