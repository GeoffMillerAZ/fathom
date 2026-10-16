package correctionestimator_test

import (
	"testing"

	ce "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/correction-estimator"
)

func TestEstimate_NoFindings(t *testing.T) {
	e := ce.Compute(ce.FindingStats{})
	if e.Score != 0.0 {
		t.Errorf("expected score=0.0, got %v", e.Score)
	}
	if e.Band != ce.BandNegligible {
		t.Errorf("expected negligible, got %q", e.Band)
	}
}

func TestEstimate_SingleCritical(t *testing.T) {
	// 1 critical = 1.0 / 5.0 = 0.2 → low
	e := ce.Compute(ce.FindingStats{Critical: 1})
	want := 0.2
	if e.Score < want-1e-9 || e.Score > want+1e-9 {
		t.Errorf("expected score≈%.2f, got %.4f", want, e.Score)
	}
	if e.Band != ce.BandLow {
		t.Errorf("expected low, got %q", e.Band)
	}
}

func TestEstimate_TwoCriticals(t *testing.T) {
	// 2 critical = 2.0 / 5.0 = 0.4 → medium
	e := ce.Compute(ce.FindingStats{Critical: 2})
	want := 0.4
	if e.Score < want-1e-9 || e.Score > want+1e-9 {
		t.Errorf("expected score≈%.2f, got %.4f", want, e.Score)
	}
	if e.Band != ce.BandMedium {
		t.Errorf("expected medium, got %q", e.Band)
	}
}

func TestEstimate_FiveCriticals_Saturates(t *testing.T) {
	// 5 critical = 5.0 / 5.0 = 1.0 → critical
	e := ce.Compute(ce.FindingStats{Critical: 5})
	if e.Score != 1.0 {
		t.Errorf("expected score=1.0, got %v", e.Score)
	}
	if e.Band != ce.BandCritical {
		t.Errorf("expected critical, got %q", e.Band)
	}
}

func TestEstimate_OverflowClampedToOne(t *testing.T) {
	// 10 critical = 10.0 / 5.0 = 2.0 → clamped to 1.0
	e := ce.Compute(ce.FindingStats{Critical: 10})
	if e.Score != 1.0 {
		t.Errorf("expected score clamped to 1.0, got %v", e.Score)
	}
	if e.Band != ce.BandCritical {
		t.Errorf("expected critical, got %q", e.Band)
	}
}

func TestEstimate_MixedFindings(t *testing.T) {
	// 1 major (0.4) + 5 minor (0.5) = 0.9 / 5.0 = 0.18 → low
	e := ce.Compute(ce.FindingStats{Major: 1, Minor: 5})
	want := 0.18
	if e.Score < want-1e-9 || e.Score > want+1e-9 {
		t.Errorf("expected score≈%.2f, got %.4f", want, e.Score)
	}
	if e.Band != ce.BandLow {
		t.Errorf("expected low, got %q", e.Band)
	}
}

func TestEstimate_InformationalNotCounted(t *testing.T) {
	// 100 informational findings contribute zero weight
	e := ce.Compute(ce.FindingStats{Informational: 100})
	if e.Score != 0.0 {
		t.Errorf("expected score=0.0 (informational only), got %v", e.Score)
	}
	if e.Band != ce.BandNegligible {
		t.Errorf("expected negligible, got %q", e.Band)
	}
}

func TestEstimate_BandThresholds(t *testing.T) {
	cases := []struct {
		stats ce.FindingStats
		band  ce.Band
	}{
		{ce.FindingStats{}, ce.BandNegligible},
		{ce.FindingStats{Minor: 1}, ce.BandNegligible},  // 0.1/5=0.02
		{ce.FindingStats{Major: 1}, ce.BandLow},         // 0.4/5=0.08
		{ce.FindingStats{Critical: 2}, ce.BandMedium},   // 2.0/5=0.40
		{ce.FindingStats{Critical: 3}, ce.BandHigh},     // 3.0/5=0.60
		{ce.FindingStats{Critical: 5}, ce.BandCritical}, // 5.0/5=1.00
	}
	for _, tc := range cases {
		e := ce.Compute(tc.stats)
		if e.Band != tc.band {
			t.Errorf("stats=%+v: expected band=%q, got %q (score=%.4f)",
				tc.stats, tc.band, e.Band, e.Score)
		}
	}
}

func TestEstimate_Deterministic(t *testing.T) {
	stats := ce.FindingStats{Critical: 1, Major: 2, Minor: 3, Informational: 4}
	e1 := ce.Compute(stats)
	e2 := ce.Compute(stats)
	if e1.Score != e2.Score || e1.Band != e2.Band {
		t.Errorf("non-deterministic: %+v vs %+v", e1, e2)
	}
}
