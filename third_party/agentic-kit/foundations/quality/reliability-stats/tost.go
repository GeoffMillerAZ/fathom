package reliabilitystats

import (
	"errors"
	"math"
)

// Typed refusals for paired equivalence, matched with errors.Is.
var (
	// ErrInsufficientPairs is returned when fewer than two pairs cannot estimate variance.
	ErrInsufficientPairs = errors.New("reliabilitystats: at least two pairs required")
	// ErrInvalidMargin is returned unless the equivalence margin is finite and positive.
	ErrInvalidMargin = errors.New("reliabilitystats: margin must be finite and positive")
	// ErrInvalidAlpha is returned unless the one-sided significance level is in (0, 0.5).
	ErrInvalidAlpha = errors.New("reliabilitystats: alpha must be in (0, 0.5)")
)

// Verdict is the predicted-null outcome of paired equivalence testing.
// CUE mirror: #Verdict in tost.cue.
type Verdict string

const (
	// VerdictEquivalent means the TOST interval is strictly inside the margin.
	VerdictEquivalent Verdict = "equivalent"
	// VerdictHarm means equivalence failed and the one-sided 95% upper bound is below zero.
	VerdictHarm Verdict = "harm"
	// VerdictInconclusive means neither equivalence nor harm was established.
	VerdictInconclusive Verdict = "inconclusive"
)

// TOSTResult reports a predicted-null verdict and its 1-2*alpha interval.
// CUE mirror: #TOSTResult in schema.cue and tost.cue.
type TOSTResult struct {
	// Verdict distinguishes equivalent, harm and inconclusive outcomes.
	Verdict Verdict `json:"verdict"`
	// Upper95 is the one-sided 95% Student-t upper confidence bound for
	// the mean difference, always calibrated at .05 regardless of alpha.
	Upper95 float64 `json:"upper_95"`
	// Equivalent is true only when Low > -margin and High < margin.
	// Equivalent is retained for compatibility; use Verdict to distinguish harm
	// from inconclusive outcomes.
	Equivalent bool `json:"equivalent"`
	// UnexpectedEffect reports a positive effect beyond the equivalence
	// margin: Low > margin. Its Verdict remains inconclusive.
	UnexpectedEffect bool `json:"unexpected_effect"`
	// Low is the lower bound for the mean paired difference.
	Low float64 `json:"low"`
	// High is the upper bound for the mean paired difference; never below Low.
	High float64 `json:"high"`
}

// EquivalenceTOST performs paired Student-t equivalence tests of the mean
// differences x[i]-y[i] against the symmetric margin (-margin, margin).
// Both one-sided tests must reject at alpha: equivalently the 1-2*alpha
// Student-t interval, using n-1 degrees of freedom, lies strictly inside
// the margin. At alpha=.05 this is a 90% interval. Independent pairs and
// normally distributed differences are assumed for exact t calibration.
// Constant differences give a degenerate interval at their common value.
// Equivalence takes precedence. Otherwise the verdict is harm when the fixed
// one-sided 95% upper bound is strictly below zero, and inconclusive otherwise
// (including unexpected positive effects, reported by UnexpectedEffect when
// Low > margin). At alpha=.05, High is Upper95.
// Inputs are never mutated; no multiplicity correction is implicit.
//
// Errors: ErrLengthMismatch; ErrEmptySample; ErrInsufficientPairs;
// ErrInvalidMargin; ErrInvalidAlpha; ErrInvalidSample (nonfinite
// difference or arithmetic overflow in the interval).
func EquivalenceTOST(x, y []float64, margin, alpha float64) (TOSTResult, error) {
	if len(x) != len(y) {
		return TOSTResult{}, ErrLengthMismatch
	}
	if len(x) == 0 {
		return TOSTResult{}, ErrEmptySample
	}
	if len(x) < 2 {
		return TOSTResult{}, ErrInsufficientPairs
	}
	if !(margin > 0) || math.IsInf(margin, 0) {
		return TOSTResult{}, ErrInvalidMargin
	}
	if !(alpha > 0 && alpha < .5) {
		return TOSTResult{}, ErrInvalidAlpha
	}
	n := float64(len(x))
	var scale float64
	for i := range x {
		d := x[i] - y[i]
		if math.IsNaN(d) || math.IsInf(d, 0) {
			return TOSTResult{}, ErrInvalidSample
		}
		scale = math.Max(scale, math.Abs(d))
	}
	if scale == 0 {
		return TOSTResult{Equivalent: true, Verdict: VerdictEquivalent}, nil
	}
	// Work in units of the largest difference. This avoids overflow in
	// residuals/norms and preserves subnormal differences until the final
	// bounds are rounded back to the caller's units.
	var point float64
	for i := range x {
		point += (x[i] - y[i]) / scale
	}
	point /= n
	var se float64
	for i := range x {
		se = math.Hypot(se, (x[i]-y[i])/scale-point)
	}
	se = se / math.Sqrt(n) / math.Sqrt(n-1)
	var half, half95 float64
	if se != 0 {
		half = (studentCritical(alpha, n-1) * se) * scale
		half95 = (studentCritical(.05, n-1) * se) * scale
	}
	point *= scale
	r := TOSTResult{Low: point - half, High: point + half, Upper95: point + half95, Verdict: VerdictInconclusive}
	if math.IsNaN(r.Low) || math.IsNaN(r.High) || math.IsInf(r.Low, 0) || math.IsInf(r.High, 0) || math.IsNaN(r.Upper95) || math.IsInf(r.Upper95, 0) {
		return TOSTResult{}, ErrInvalidSample
	}
	r.Equivalent = r.Low > -margin && r.High < margin
	r.UnexpectedEffect = r.Low > margin
	if r.Equivalent {
		r.Verdict = VerdictEquivalent
	} else if r.Upper95 < 0 {
		r.Verdict = VerdictHarm
	}
	return r, nil
}

// studentCritical inverts the upper tail directly, avoiding cancellation
// in 1-alpha. Bisection is deterministic and bounded (1024 bracket doublings,
// 128 refinements). An unrepresentable critical value returns +Inf, which
// EquivalenceTOST refuses as a nonfinite interval.
func studentCritical(alpha, df float64) float64 {
	lo, hi := 0.0, 1.0
	for i := 0; i < 1024 && studentTail(hi, df) > alpha; i++ {
		lo, hi = hi, hi*2
		if math.IsInf(hi, 0) {
			return hi
		}
	}
	for i := 0; i < 128; i++ {
		mid := lo + (hi-lo)/2
		if studentTail(mid, df) > alpha {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo + (hi-lo)/2
}

// studentTail(t, df) = I_x(df/2, 1/2)/2, x=df/(df+t*t), for t>=0.
// The log form keeps very small tails usable without squaring a large t.
func studentTail(t, df float64) float64 {
	if t == 0 {
		return .5
	}
	a, b := df/2, .5
	ratio := t / math.Sqrt(df)
	var logx, logy float64
	if ratio > 1 {
		logy = -math.Log1p(1 / ratio / ratio)
		logx = -2*math.Log(ratio) + logy
	} else {
		logx = -math.Log1p(ratio * ratio)
		logy = 2*math.Log(ratio) + logx
	}
	x := math.Exp(logx)
	la, _ := math.Lgamma(a)
	lb, _ := math.Lgamma(b)
	lab, _ := math.Lgamma(a + b)
	front := math.Exp(lab - la - lb + a*logx + b*logy)
	if x < (a+1)/(a+b+2) {
		return .5 * front * betaFraction(a, b, x) / a
	}
	return .5 * (1 - front*betaFraction(b, a, math.Exp(logy))/b)
}

// betaFraction evaluates the incomplete-beta continued fraction with
// modified Lentz iteration. Positive a,b and x in [0,1] are supplied by
// studentTail; 512 iterations bound work, 3e-14 is the relative tolerance.
func betaFraction(a, b, x float64) float64 {
	const tiny = 1e-300
	clamp := func(v float64) float64 {
		if math.Abs(v) < tiny {
			return math.Copysign(tiny, v)
		}
		return v
	}
	c := 1.0
	d := 1 / clamp(1-(a+b)*x/(a+1))
	h := d
	for m := 1; m <= 512; m++ {
		fm := float64(m)
		aa := fm * (b - fm) * x / ((a + 2*fm - 1) * (a + 2*fm))
		d = 1 / clamp(1+aa*d)
		c = clamp(1 + aa/c)
		h *= d * c
		aa = -(a + fm) * (a + b + fm) * x / ((a + 2*fm) * (a + 2*fm + 1))
		d = 1 / clamp(1+aa*d)
		c = clamp(1 + aa/c)
		delta := d * c
		h *= delta
		if math.Abs(delta-1) < 3e-14 {
			break
		}
	}
	return h
}
