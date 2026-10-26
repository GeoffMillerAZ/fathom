package reliabilitystats_test

import (
	"errors"
	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
	"math"
	"testing"
)

func TestEquivalenceTOST_Verdicts(t *testing.T) {
	for _, tc := range []struct {
		name       string
		x          []float64
		margin     float64
		equivalent bool
		outside    bool
	}{
		{"equivalent", []float64{-.02, .01, -.01, .02}, .1, true, false},
		{"not equivalent", []float64{1.9, 2, 2.1}, .5, false, true},
		{"inconclusive width", []float64{-2, 2}, 1, false, false},
		{"constant inside", []float64{.1, .1, .1}, .5, true, false},
		{"constant at boundary", []float64{.5, .5}, .5, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := rs.EquivalenceTOST(tc.x, make([]float64, len(tc.x)), tc.margin, .05)
			if err != nil {
				t.Fatal(err)
			}
			if r.Equivalent != tc.equivalent || r.Low > r.High {
				t.Errorf("verdict: %+v", r)
			}
			if tc.outside && r.Low <= tc.margin {
				t.Errorf("expected interval beyond margin: %+v", r)
			}
			if tc.name == "inconclusive width" && !(r.Low < -tc.margin && r.High > tc.margin) {
				t.Errorf("expected inconclusive wide interval: %+v", r)
			}
			if (tc.name == "constant inside" || tc.name == "constant at boundary") && (r.Low != tc.x[0] || r.High != tc.x[0]) {
				t.Errorf("expected degenerate interval at %v: %+v", tc.x[0], r)
			}
		})
	}
}

func TestEquivalenceTOST_StudentCriticalValues(t *testing.T) {
	// The 90% interval uses t_(.95, n-1), not a 95% interval or a normal z.
	for _, tc := range []struct {
		x        []float64
		critical float64
	}{
		{[]float64{-1, 1}, 6.313751514675041},                                                // df=1: Cauchy, tan(.45*pi)
		{[]float64{-1, 0, 1}, 2.919985580353725},                                             // df=2
		{[]float64{-4.5, -3.5, -2.5, -1.5, -.5, .5, 1.5, 2.5, 3.5, 4.5}, 1.8331129326536335}, // df=9
	} {
		var sumsq float64
		for _, x := range tc.x {
			sumsq += x * x
		}
		half := tc.critical * math.Sqrt(sumsq/float64(len(tc.x)*(len(tc.x)-1)))
		// Nonzero baselines verify actual pairing rather than a one-sample shortcut.
		x, y := make([]float64, len(tc.x)), make([]float64, len(tc.x))
		for i, d := range tc.x {
			y[i] = float64(i)
			x[i] = y[i] + d
		}
		r, err := rs.EquivalenceTOST(x, y, half+1, .05)
		if err != nil {
			t.Fatal(err)
		}
		if !r.Equivalent || !near(r.Low, -half, 1e-10) || !near(r.High, half, 1e-10) {
			t.Errorf("n=%d: %+v, want +/- %.15g", len(x), r, half)
		}
	}
}

func TestEquivalenceTOST_Alpha(t *testing.T) {
	// df=1, alpha=.25 gives a 50% interval with t=.75 quantile = 1.
	r, err := rs.EquivalenceTOST([]float64{-1, 1}, []float64{0, 0}, 2, .25)
	if err != nil || !near(r.Low, -1, 1e-12) || !near(r.High, 1, 1e-12) || !r.Equivalent {
		t.Errorf("50%% interval: %+v (%v)", r, err)
	}
}

func TestEquivalenceTOST_LargeFiniteInterval(t *testing.T) {
	// The balanced sample overflows an unscaled norm. In the imbalanced
	// sample a single residual overflows if subtraction precedes scaling.
	for _, x := range [][]float64{
		{-1, 1, -1, 1},
		{-1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 1},
	} {
		y := make([]float64, len(x))
		want, err := rs.EquivalenceTOST(x, y, math.MaxFloat64, .05)
		if err != nil {
			t.Fatal(err)
		}
		for i := range x {
			x[i] *= 1e308
		}
		got, err := rs.EquivalenceTOST(x, y, math.MaxFloat64, .05)
		if err != nil || !got.Equivalent || !near(got.Low/1e308, want.Low, 1e-12) || !near(got.High/1e308, want.High, 1e-12) {
			t.Errorf("scaled interval: %+v (%v), unscaled %+v", got, err, want)
		}
	}
}

func TestEquivalenceTOST_Errors(t *testing.T) {
	ok := []float64{1, 2}
	for _, tc := range []struct {
		x, y          []float64
		margin, alpha float64
		want          error
	}{
		{nil, nil, 1, .05, rs.ErrEmptySample}, {ok, []float64{0}, 1, .05, rs.ErrLengthMismatch},
		{[]float64{0}, []float64{0}, 1, .05, rs.ErrInsufficientPairs},
		{ok, ok, 0, .05, rs.ErrInvalidMargin}, {ok, ok, -1, .05, rs.ErrInvalidMargin}, {ok, ok, math.Inf(1), .05, rs.ErrInvalidMargin}, {ok, ok, math.NaN(), .05, rs.ErrInvalidMargin},
		{ok, ok, 1, 0, rs.ErrInvalidAlpha}, {ok, ok, 1, .5, rs.ErrInvalidAlpha}, {ok, ok, 1, math.NaN(), rs.ErrInvalidAlpha},
		{[]float64{math.NaN(), 0}, ok, 1, .05, rs.ErrInvalidSample}, {[]float64{math.Inf(1), 0}, ok, 1, .05, rs.ErrInvalidSample},
		{[]float64{math.MaxFloat64, 0}, []float64{-math.MaxFloat64, 0}, 1, .05, rs.ErrInvalidSample},
	} {
		if _, err := rs.EquivalenceTOST(tc.x, tc.y, tc.margin, tc.alpha); !errors.Is(err, tc.want) {
			t.Errorf("%+v: %v want %v", tc, err, tc.want)
		}
	}
}

func TestEquivalenceTOST_SubnormalNotEquivalent(t *testing.T) {
	d := math.SmallestNonzeroFloat64
	r, err := rs.EquivalenceTOST([]float64{-d, d, -d, d}, make([]float64, 4), d, .05)
	if err != nil || r.Equivalent || !(r.Low < 0 && r.High > 0) {
		t.Errorf("subnormal interval: %+v (%v)", r, err)
	}
}

func TestEquivalenceTOST_CauchyTailRange(t *testing.T) {
	for _, alpha := range []float64{1e-12, .01, .1, .499999999999} {
		// df=1 has analytic quantile cot(pi*alpha). Near alpha=.5,
		// tan(pi*(.5-alpha)) avoids subtracting nearly equal angles.
		critical := 1 / math.Tan(math.Pi*alpha)
		if alpha > .25 {
			critical = math.Tan(math.Pi * (.5 - alpha))
		}
		r, err := rs.EquivalenceTOST([]float64{-1, 1}, []float64{0, 0}, math.MaxFloat64, alpha)
		if err != nil {
			t.Fatal(err)
		}
		if math.Abs(r.High-critical) > critical*1e-4 {
			t.Errorf("alpha=%v: high %.16g want %.16g", alpha, r.High, critical)
		}
	}
}
