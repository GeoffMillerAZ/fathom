package reliabilitystats_test

import (
	"errors"
	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
	"testing"
)

func TestMcNemarExact_KnownAnswers(t *testing.T) {
	for _, tc := range []struct {
		b, c int
		p    float64
	}{
		{1, 9, 22.0 / 1024}, {9, 1, 22.0 / 1024}, {5, 5, 1}, {0, 0, 1}, {0, 5, 2.0 / 32}, {500, 500, 1}, {600, 601, 1},
	} {
		x, y := make([]bool, tc.b+tc.c+2), make([]bool, tc.b+tc.c+2)
		for i := 0; i < tc.b; i++ {
			x[i] = true
		}
		for i := tc.b; i < tc.b+tc.c; i++ {
			y[i] = true
		}
		x[len(x)-1], y[len(y)-1] = true, true // concordant outcomes do not contribute
		got, err := rs.McNemarExact(x, y)
		if err != nil {
			t.Fatal(err)
		}
		if got.B != tc.b || got.C != tc.c || !near(got.P, tc.p, 1e-12) {
			t.Errorf("b=%d c=%d: %+v, want p=%v", tc.b, tc.c, got, tc.p)
		}
	}
}

func TestMcNemarExact_Errors(t *testing.T) {
	if _, err := rs.McNemarExact(nil, []bool{true}); !errors.Is(err, rs.ErrLengthMismatch) {
		t.Errorf("mismatch: %v", err)
	}
	if _, err := rs.McNemarExact(nil, nil); !errors.Is(err, rs.ErrEmptySample) {
		t.Errorf("empty: %v", err)
	}
}
