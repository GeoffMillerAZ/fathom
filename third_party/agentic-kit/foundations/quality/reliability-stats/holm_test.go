package reliabilitystats_test

import (
	"errors"
	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
	"math"
	"reflect"
	"testing"
)

func TestHolmAdjust_Textbook(t *testing.T) {
	// Sorted p: .005, .01, .03, .04; multipliers 4,3,2,1;
	// cumulative maxima .02, .03, .06, .06, restored to input order.
	p := []float64{.01, .04, .03, .005}
	original := append([]float64(nil), p...)
	got, err := rs.HolmAdjust(p)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{.03, .06, .06, .02} {
		if !near(got[i], want, 1e-15) {
			t.Errorf("index %d: %v want %v", i, got[i], want)
		}
	}
	if !reflect.DeepEqual(p, original) {
		t.Fatal("mutated input")
	}
}

func TestHolmAdjust_Edges(t *testing.T) {
	for _, tc := range []struct{ p, want []float64 }{
		{nil, nil}, {[]float64{.2}, []float64{.2}},
		{[]float64{0, .6, .6, 1}, []float64{0, 1, 1, 1}},
		{[]float64{.02, .02, .03}, []float64{.06, .06, .06}},
	} {
		got, err := rs.HolmAdjust(tc.p)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("length %d want %d", len(got), len(tc.want))
		}
		for i, want := range tc.want {
			if !near(got[i], want, 1e-15) {
				t.Errorf("%v: %v want %v", tc.p, got, tc.want)
			}
		}
	}
}

func TestHolmAdjust_Invalid(t *testing.T) {
	for _, p := range []float64{-1, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got, err := rs.HolmAdjust([]float64{.1, p}); !errors.Is(err, rs.ErrInvalidPValue) || got != nil {
			t.Errorf("p=%v: %v, %v", p, got, err)
		}
	}
}
