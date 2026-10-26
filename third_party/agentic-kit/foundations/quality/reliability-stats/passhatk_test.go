package reliabilitystats_test

import (
	"math"
	"testing"

	rs "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/reliability-stats"
)

// TestPassHatK_KnownAnswers pins hand-computed C(c,k)/C(n,k) values.
func TestPassHatK_KnownAnswers(t *testing.T) {
	cases := []struct {
		n, c, k int
		want    float64
	}{
		{4, 3, 2, 0.5},               // 3/6
		{10, 8, 3, 56.0 / 120.0},     // C(8,3)/C(10,3)
		{10, 9, 2, 0.8},              // 36/45
		{6, 4, 1, 4.0 / 6.0},         // k=1 is the pass rate
		{5, 5, 3, 1},                 // c = n
		{5, 2, 3, 0},                 // c < k
		{5, 0, 1, 0},                 // no passes
		{7, 7, 7, 1},                 // k = n = c
		{7, 6, 7, 0},                 // k = n, one failure
		{20, 15, 4, 1365.0 / 4845.0}, // C(15,4)/C(20,4)
	}
	for _, c := range cases {
		if got := rs.PassHatK(c.n, c.c, c.k); !near(got, c.want, 1e-12) {
			t.Errorf("PassHatK(%d, %d, %d) = %v, want %v", c.n, c.c, c.k, got, c.want)
		}
	}
}

// TestPassHatK_UndefinedInputsAreNaN mirrors PassAtK: n < 1, k < 1, k > n,
// c < 0, c > n.
func TestPassHatK_UndefinedInputsAreNaN(t *testing.T) {
	for _, c := range [][3]int{{0, 0, 1}, {-1, 0, 1}, {5, 2, 0}, {5, 2, -1}, {5, 2, 6}, {5, -1, 2}, {5, 6, 2}} {
		if got := rs.PassHatK(c[0], c[1], c[2]); !math.IsNaN(got) {
			t.Errorf("PassHatK(%d, %d, %d) = %v, want NaN", c[0], c[1], c[2], got)
		}
	}
}

// TestPassHatK_ConvergesToPassPowK shows the unbiased estimator approaches
// p^k as n grows with c/n fixed, and stays stable at n = 1000.
func TestPassHatK_ConvergesToPassPowK(t *testing.T) {
	want := rs.PassPowK(0.9, 8)
	prev := math.Inf(1)
	for _, n := range []int{10, 50, 100, 500, 1000} {
		got := rs.PassHatK(n, n*9/10, 8)
		if math.IsNaN(got) || got < 0 || got > want+1e-12 {
			t.Fatalf("n=%d: PassHatK = %v, want in [0, %v]", n, got, want)
		}
		d := math.Abs(got - want)
		if d >= prev {
			t.Errorf("n=%d: error %v did not shrink from %v", n, d, prev)
		}
		prev = d
	}
	if prev > 2e-3 {
		t.Errorf("n=1000 error %v, want < 2e-3", prev)
	}
}
