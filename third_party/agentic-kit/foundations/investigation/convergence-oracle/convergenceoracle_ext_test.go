package convergenceoracle_test

// convergenceoracle_ext_test.go — additional tests covering:
//  1. Contradicted verdict: unresolved contradictions block convergence.
//  2. Input validation via AssessWithThresholdsE.
//  3. CUE-schema negative-contract enforcement (Go equivalents of invalid.cue).

import (
	"errors"
	"testing"

	co "github.com/geoffmilleraz/agentic-kit/foundations/investigation/convergence-oracle"
)

// ---------------------------------------------------------------------------
// Finding 1 — Contradicted verdict
// ---------------------------------------------------------------------------

// TestAssessContradictedBlocksConverged ensures that when the number of
// contradictions exceeds maxContradictions, the verdict is Contradicted even
// when findings and agreement would otherwise satisfy convergence.
func TestAssessContradictedBlocksConverged(t *testing.T) {
	// 10 findings, high agreement, but 3 contradictions > maxContradictions=2.
	got, err := co.AssessWithThresholdsE(10, 0.9, 3, co.DefaultMinFindings, co.DefaultMinAgreement, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != co.Contradicted {
		t.Errorf("expected Contradicted, got %q", got)
	}
}

// TestAssessContradictedAtBoundary ensures that contradictions == maxContradictions
// is NOT blocked (boundary: > not >=).
func TestAssessContradictedAtBoundary(t *testing.T) {
	// contradictions == maxContradictions: should still be able to converge.
	got, err := co.AssessWithThresholdsE(10, 0.9, 2, co.DefaultMinFindings, co.DefaultMinAgreement, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != co.Converged {
		t.Errorf("expected Converged at boundary, got %q", got)
	}
}

// TestAssessZeroContradictionsConverges confirms the baseline case: zero
// contradictions with good agreement still converges.
func TestAssessZeroContradictionsConverges(t *testing.T) {
	got, err := co.AssessWithThresholdsE(5, 0.8, 0, co.DefaultMinFindings, co.DefaultMinAgreement, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != co.Converged {
		t.Errorf("expected Converged, got %q", got)
	}
}

// TestAssessContradictedStillNeedsMoreSources confirms NeedsMoreSources takes
// priority over Contradicted (not enough data to draw any conclusion yet).
func TestAssessContradictedStillNeedsMoreSources(t *testing.T) {
	// Only 1 finding (< defaultMinFindings=3) but 5 contradictions.
	got, err := co.AssessWithThresholdsE(1, 0.9, 5, co.DefaultMinFindings, co.DefaultMinAgreement, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != co.NeedsMoreSources {
		t.Errorf("expected NeedsMoreSources when findings too low, got %q", got)
	}
}

// TestAssessConvenienceContradicted exercises the Assess convenience wrapper
// that accepts contradictions (uses package-default thresholds).
func TestAssessConvenienceContradicted(t *testing.T) {
	// Default maxContradictions is 0, so any contradiction blocks convergence.
	got := co.AssessWithContradictions(10, 0.9, 1)
	if got != co.Contradicted {
		t.Errorf("expected Contradicted, got %q", got)
	}

	got = co.AssessWithContradictions(10, 0.9, 0)
	if got != co.Converged {
		t.Errorf("expected Converged with zero contradictions, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Finding 2 — Input validation via AssessWithThresholdsE
// ---------------------------------------------------------------------------

// TestAssessWithThresholdsERejectsNegativeFindings mirrors the CUE constraint
// Findings: >=0 (invalid.cue _badNegativeFindings).
func TestAssessWithThresholdsERejectsNegativeFindings(t *testing.T) {
	_, err := co.AssessWithThresholdsE(-1, 0.5, 0, co.DefaultMinFindings, co.DefaultMinAgreement, 0)
	if err == nil {
		t.Error("expected error for negative findings, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestAssessWithThresholdsERejectsAgreementAboveOne mirrors the CUE constraint
// Agreement: >=0 & <=1 (invalid.cue _badAgreementHigh).
func TestAssessWithThresholdsERejectsAgreementAboveOne(t *testing.T) {
	_, err := co.AssessWithThresholdsE(5, 1.5, 0, co.DefaultMinFindings, co.DefaultMinAgreement, 0)
	if err == nil {
		t.Error("expected error for agreement > 1, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestAssessWithThresholdsERejectsNegativeAgreement mirrors the CUE constraint
// Agreement: >=0 & <=1 (invalid.cue _badAgreementLow).
func TestAssessWithThresholdsERejectsNegativeAgreement(t *testing.T) {
	_, err := co.AssessWithThresholdsE(5, -0.1, 0, co.DefaultMinFindings, co.DefaultMinAgreement, 0)
	if err == nil {
		t.Error("expected error for agreement < 0, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestAssessWithThresholdsERejectsNegativeContradictions ensures contradiction
// count cannot be negative.
func TestAssessWithThresholdsERejectsNegativeContradictions(t *testing.T) {
	_, err := co.AssessWithThresholdsE(5, 0.5, -1, co.DefaultMinFindings, co.DefaultMinAgreement, 0)
	if err == nil {
		t.Error("expected error for negative contradictions, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestAssessWithThresholdsERejectsZeroMinFindings ensures minFindings >= 1 to
// prevent trivially-met thresholds.
func TestAssessWithThresholdsERejectsZeroMinFindings(t *testing.T) {
	_, err := co.AssessWithThresholdsE(5, 0.8, 0, 0, co.DefaultMinAgreement, 0)
	if err == nil {
		t.Error("expected error for minFindings=0, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestAssessWithThresholdsERejectsNegativeMinFindings ensures minFindings >= 1.
func TestAssessWithThresholdsERejectsNegativeMinFindings(t *testing.T) {
	_, err := co.AssessWithThresholdsE(5, 0.8, 0, -1, co.DefaultMinAgreement, 0)
	if err == nil {
		t.Error("expected error for negative minFindings, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestAssessWithThresholdsERejectsMinAgreementOutOfRange ensures minAgreement
// must be in [0, 1].
func TestAssessWithThresholdsERejectsMinAgreementOutOfRange(t *testing.T) {
	_, err := co.AssessWithThresholdsE(5, 0.8, 0, co.DefaultMinFindings, 1.1, 0)
	if err == nil {
		t.Error("expected error for minAgreement > 1, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}

	_, err = co.AssessWithThresholdsE(5, 0.8, 0, co.DefaultMinFindings, -0.1, 0)
	if err == nil {
		t.Error("expected error for minAgreement < 0, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestAssessWithThresholdsERejectsNegativeMaxContradictions ensures
// maxContradictions >= 0.
func TestAssessWithThresholdsERejectsNegativeMaxContradictions(t *testing.T) {
	_, err := co.AssessWithThresholdsE(5, 0.8, 0, co.DefaultMinFindings, co.DefaultMinAgreement, -1)
	if err == nil {
		t.Error("expected error for negative maxContradictions, got nil")
	}
	if !errors.Is(err, co.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

// TestAssessWithThresholdsEValidInputsWork confirms valid inputs still produce
// correct verdicts through the error-returning variant.
func TestAssessWithThresholdsEValidInputsWork(t *testing.T) {
	cases := []struct {
		findings       int
		agreement      float64
		contradictions int
		minFindings    int
		minAgreement   float64
		maxContradicts int
		wantVerdict    co.Verdict
	}{
		{1, 1.0, 0, 3, 0.75, 0, co.NeedsMoreSources},
		{3, 0.5, 0, 3, 0.75, 0, co.Exploring},
		{3, 0.75, 0, 3, 0.75, 0, co.Converged},
		{10, 0.9, 3, 3, 0.75, 2, co.Contradicted},
	}
	for _, tc := range cases {
		got, err := co.AssessWithThresholdsE(
			tc.findings, tc.agreement, tc.contradictions,
			tc.minFindings, tc.minAgreement, tc.maxContradicts,
		)
		if err != nil {
			t.Errorf("unexpected error for %+v: %v", tc, err)
			continue
		}
		if got != tc.wantVerdict {
			t.Errorf("AssessWithThresholdsE(%+v) = %q, want %q", tc, got, tc.wantVerdict)
		}
	}
}

// ---------------------------------------------------------------------------
// Finding 4 — Agreement grade-floor: by-design contract documentation test
// ---------------------------------------------------------------------------

// TestAgreementGradeFloorIsCallerContract documents that the oracle accepts
// agreement as an opaque proportion and does NOT enforce evidence-grade
// bounding internally. This is a caller contract (see package doc). The test
// exists to make the limitation visible and detectable if behavior changes.
func TestAgreementGradeFloorIsCallerContract(t *testing.T) {
	// The oracle accepts any valid [0,1] agreement — it is the caller's
	// responsibility to ensure that agreement is computed only from findings
	// at the required minimum evidence grade (see package documentation).
	// This test asserts the current by-design behavior and will fail if the
	// oracle starts enforcing grade internally.
	got, err := co.AssessWithThresholdsE(5, 0.8, 0, co.DefaultMinFindings, co.DefaultMinAgreement, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != co.Converged {
		t.Errorf("by-design: grade-floor is caller contract; expected Converged, got %q", got)
	}
}
