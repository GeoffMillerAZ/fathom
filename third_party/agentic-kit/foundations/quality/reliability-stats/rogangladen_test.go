package reliabilitystats_test

import (
	"errors"
	"math"
	"testing"

	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
)

// TestRoganGladen_HandComputedPoints pins (observed + tnr − 1)/(tpr + tnr − 1).
func TestRoganGladen_HandComputedPoints(t *testing.T) {
	cases := []struct {
		obs, tpr, tnr, want float64
	}{
		{0.6, 0.9, 0.8, 0.4 / 0.7},
		{0.5, 0.9, 0.9, 0.5}, // symmetric judge, no correction at 0.5
		{0.3, 1, 1, 0.3},     // perfect judge is the identity
		{0.37, 0.85, 0.95, 0.32 / 0.8},
		{0.2, 0.75, 0.75, 0}, // raw (0.2−0.25)/0.5 < 0, clamped
	}
	for _, c := range cases {
		got, err := rs.RoganGladen(c.obs, c.tpr, c.tnr, 400, 1, 200, 0.95)
		if err != nil {
			t.Fatalf("RoganGladen(%v,%v,%v): %v", c.obs, c.tpr, c.tnr, err)
		}
		if !near(got.Point, c.want, 1e-12) {
			t.Errorf("RoganGladen(%v,%v,%v).Point = %v, want %v", c.obs, c.tpr, c.tnr, got.Point, c.want)
		}
		if got.Low > got.High || got.Low < 0 || got.High > 1 {
			t.Errorf("interval [%v, %v] not an ordered subset of [0,1]", got.Low, got.High)
		}
	}
}

// TestRoganGladen_ClampsBothEnds checks the raw estimate below 0 and above 1.
func TestRoganGladen_ClampsBothEnds(t *testing.T) {
	lo, err := rs.RoganGladen(0.05, 0.9, 0.8, 100, 1, 100, 0.9) // (0.05−0.2)/0.7 < 0
	if err != nil || lo.Point != 0 {
		t.Errorf("low clamp: point %v err %v, want 0", lo.Point, err)
	}
	hi, err := rs.RoganGladen(0.95, 0.8, 0.9, 100, 1, 100, 0.9) // (0.95−0.1)/0.7 > 1
	if err != nil || hi.Point != 1 {
		t.Errorf("high clamp: point %v err %v, want 1", hi.Point, err)
	}
}

// TestRoganGladen_NonIdentifiableIsNaN: tpr + tnr <= 1 means the judge is no
// better than chance and the correction is undefined.
func TestRoganGladen_NonIdentifiableIsNaN(t *testing.T) {
	for _, c := range [][2]float64{{0.5, 0.5}, {0.4, 0.5}, {0, 0}, {0.3, 0.7}} {
		got, err := rs.RoganGladen(0.5, c[0], c[1], 100, 1, 100, 0.95)
		if err != nil {
			t.Fatalf("tpr=%v tnr=%v: unexpected error %v", c[0], c[1], err)
		}
		if !math.IsNaN(got.Point) || !math.IsNaN(got.Low) || !math.IsNaN(got.High) {
			t.Errorf("tpr=%v tnr=%v: %+v, want all NaN", c[0], c[1], got)
		}
	}
}

func TestRoganGladen_SeedDeterministicAndSeedSensitive(t *testing.T) {
	a, _ := rs.RoganGladen(0.55, 0.9, 0.85, 80, 7, 500, 0.95)
	b, _ := rs.RoganGladen(0.55, 0.9, 0.85, 80, 7, 500, 0.95)
	if a != b {
		t.Errorf("same seed differs: %+v vs %+v", a, b)
	}
	c, _ := rs.RoganGladen(0.55, 0.9, 0.85, 80, 8, 500, 0.95)
	if a.Low == c.Low && a.High == c.High {
		t.Errorf("different seeds gave identical bounds %+v", a)
	}
}

// TestRoganGladen_IntervalWidensWithSmallerN: fewer judged items, less
// evidence, wider interval.
func TestRoganGladen_IntervalWidensWithSmallerN(t *testing.T) {
	prev := -1.0
	for _, n := range []int{5000, 500, 50, 20} {
		got, err := rs.RoganGladen(0.55, 0.9, 0.85, n, 3, 2000, 0.95)
		if err != nil {
			t.Fatal(err)
		}
		w := got.High - got.Low
		if w <= prev {
			t.Errorf("n=%d width %v not wider than %v", n, w, prev)
		}
		prev = w
		if got.Point < got.Low || got.Point > got.High {
			t.Errorf("n=%d point %v outside [%v, %v]", n, got.Point, got.Low, got.High)
		}
	}
}

func TestRoganGladen_InvalidInputs(t *testing.T) {
	cases := []struct {
		name          string
		obs, tpr, tnr float64
		n, res        int
		level         float64
		want          error
	}{
		{"n", 0.5, 0.9, 0.9, 0, 100, 0.95, rs.ErrEmptySample},
		{"resamples", 0.5, 0.9, 0.9, 10, 0, 0.95, rs.ErrInvalidResamples},
		{"level", 0.5, 0.9, 0.9, 10, 100, 1, rs.ErrInvalidLevel},
		{"observed", 1.5, 0.9, 0.9, 10, 100, 0.95, rs.ErrInvalidSample},
		{"tpr", 0.5, -0.1, 0.9, 10, 100, 0.95, rs.ErrInvalidSample},
		{"tnr nan", 0.5, 0.9, math.NaN(), 10, 100, 0.95, rs.ErrInvalidSample},
	}
	for _, c := range cases {
		if _, err := rs.RoganGladen(c.obs, c.tpr, c.tnr, c.n, 1, c.res, c.level); !errors.Is(err, c.want) {
			t.Errorf("%s: err %v, want %v", c.name, err, c.want)
		}
	}
}
