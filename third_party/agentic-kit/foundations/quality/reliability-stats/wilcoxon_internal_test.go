package reliabilitystats

import "testing"

// TestWilcoxonNormalApproximation_TracksExactAtBoundary checks the
// approximation the package switches to above n = 25 against the exact
// distribution at n = 25 itself: within 0.01 two-sided across the range
// a caller would act on.
func TestWilcoxonNormalApproximation_TracksExactAtBoundary(t *testing.T) {
	ranks := make([]float64, 25)
	for i := range ranks {
		ranks[i] = float64(i + 1)
	}
	for _, w := range []float64{68, 89, 100, 120, 140, 160} {
		exact, approx := wilcoxonExactP(ranks, w), wilcoxonNormalP(25, w, 0)
		if d := exact - approx; d > 0.01 || d < -0.01 {
			t.Errorf("w=%v: exact %.6f vs normal %.6f (|diff| > 0.01)", w, exact, approx)
		}
	}
	if p := wilcoxonNormalP(25, 162.5, 0); p != 1 {
		t.Errorf("at the mean the two-sided p must be 1, got %v", p)
	}
}
