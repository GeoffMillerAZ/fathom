package reliabilitystats_test

import (
	"math"
	"testing"

	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
)

func TestMcNemarExact_ClampsPositiveUnderflow(t *testing.T) {
	for _, n := range []int{1076, 1100, 1200} {
		for _, reverse := range []bool{false, true} {
			x, y := make([]bool, n), make([]bool, n)
			for i := range y {
				y[i] = true
			}
			if reverse {
				x, y = y, x
			}
			r, err := rs.McNemarExact(x, y)
			if err != nil {
				t.Fatal(err)
			}
			b, c := 0, n
			if reverse {
				b, c = c, b
			}
			if r.P != math.SmallestNonzeroFloat64 || r.B != b || r.C != c {
				t.Errorf("n=%d reverse=%v: %+v, want smallest positive p with counts %d,%d", n, reverse, r, b, c)
			}
			adjusted, err := rs.HolmAdjust([]float64{r.P, .5})
			if err != nil || adjusted[0] != 2*math.SmallestNonzeroFloat64 {
				t.Errorf("Holm n=%d: %v (%v)", n, adjusted, err)
			}
		}
	}
}
