package convergenceoracle_test

import (
	"testing"

	co "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/convergence-oracle"
)

// TestAssessNeedsMoreSources ensures too few findings returns NeedsMoreSources.
func TestAssessNeedsMoreSources(t *testing.T) {
	cases := []struct {
		findings  int
		agreement float64
	}{
		{0, 1.0},
		{1, 1.0},
		{2, 1.0},
	}
	for _, tc := range cases {
		got := co.Assess(tc.findings, tc.agreement)
		if got != co.NeedsMoreSources {
			t.Errorf("Assess(%d, %g) = %q, want %q", tc.findings, tc.agreement, got, co.NeedsMoreSources)
		}
	}
}

// TestAssessExploring ensures sufficient findings but low agreement returns Exploring.
func TestAssessExploring(t *testing.T) {
	cases := []struct {
		findings  int
		agreement float64
	}{
		{3, 0.0},
		{3, 0.5},
		{3, 0.74},
		{10, 0.0},
	}
	for _, tc := range cases {
		got := co.Assess(tc.findings, tc.agreement)
		if got != co.Exploring {
			t.Errorf("Assess(%d, %g) = %q, want %q", tc.findings, tc.agreement, got, co.Exploring)
		}
	}
}

// TestAssessConverged ensures sufficient findings and high agreement returns Converged.
func TestAssessConverged(t *testing.T) {
	cases := []struct {
		findings  int
		agreement float64
	}{
		{3, 0.75},
		{3, 1.0},
		{10, 0.8},
		{100, 0.99},
	}
	for _, tc := range cases {
		got := co.Assess(tc.findings, tc.agreement)
		if got != co.Converged {
			t.Errorf("Assess(%d, %g) = %q, want %q", tc.findings, tc.agreement, got, co.Converged)
		}
	}
}

// TestAssessDeterministic ensures the same inputs always produce the same output.
func TestAssessDeterministic(t *testing.T) {
	for i := 0; i < 100; i++ {
		a := co.Assess(5, 0.8)
		b := co.Assess(5, 0.8)
		if a != b {
			t.Fatalf("non-deterministic: got %q then %q", a, b)
		}
	}
}

// TestAssessWithThresholdsCustom ensures custom thresholds are honoured.
func TestAssessWithThresholdsCustom(t *testing.T) {
	// With minFindings=1 and minAgreement=0.5, 1 finding at 0.6 should converge.
	got := co.AssessWithThresholds(1, 0.6, 1, 0.5)
	if got != co.Converged {
		t.Errorf("expected Converged, got %q", got)
	}

	// With minFindings=5 and minAgreement=0.9, 3 findings at 1.0 should need sources.
	got = co.AssessWithThresholds(3, 1.0, 5, 0.9)
	if got != co.NeedsMoreSources {
		t.Errorf("expected NeedsMoreSources, got %q", got)
	}

	// With minFindings=2 and minAgreement=0.9, 3 findings at 0.5 should be exploring.
	got = co.AssessWithThresholds(3, 0.5, 2, 0.9)
	if got != co.Exploring {
		t.Errorf("expected Exploring, got %q", got)
	}
}
