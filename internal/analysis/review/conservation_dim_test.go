package review_test

// conservation_dim_test.go — CV-24 (ADR-044 §1.6/§2): the conservation
// dimension's own contract — unchecked without a base ref (mirrors
// DimIntegrity), unchecked when the caller could not build a conserve
// input at all, and — given a caller-precomputed set of
// ConservationPlaneSummary rows — a finding per plane Finding (severity
// warn unless Declared=="prior") plus an informational finding per plane
// Missing entry, never silently dropped.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
)

func TestReview_ConservationDimension(t *testing.T) {
	t.Run("no base ref", func(t *testing.T) {
		v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}})
		d := dimension(t, v, review.DimConservation)
		if d.Checked {
			t.Fatal("conservation should be unchecked without a base ref")
		}
		if d.Reason != "needs a base ref" {
			t.Errorf("Reason = %q, want %q", d.Reason, "needs a base ref")
		}
	})

	t.Run("base given but input could not be built", func(t *testing.T) {
		v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef"})
		d := dimension(t, v, review.DimConservation)
		if d.Checked {
			t.Fatal("conservation should be unchecked when ConservationSupported is false")
		}
		if d.Reason == "" {
			t.Error("expected a non-empty reason")
		}
	})

	t.Run("plane findings and missing inputs both surface", func(t *testing.T) {
		v := review.Run(review.Input{
			Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef",
			ConservationSupported: true,
			ConservationPlanes: []review.ConservationPlaneSummary{
				{
					Plane: "surface", Verdict: "losses",
					Findings: []review.ConservationFinding{
						{Plane: "surface", Kind: "removed", Key: "api:foo.Old", Detail: "func()", Declared: "none"},
						{Plane: "surface", Kind: "removed", Key: "api:foo.Excused", Detail: "func()", Declared: "prior"},
					},
				},
				{Plane: "certificates", Verdict: "unproven", Missing: []string{"no store"}},
			},
		})
		d := dimension(t, v, review.DimConservation)
		if !d.Checked {
			t.Fatalf("expected conservation to be checked; reason=%q", d.Reason)
		}
		if d.Count != 3 {
			t.Fatalf("Count = %d, want 3 (two findings + one missing-input note); findings=%+v", d.Count, d.Findings)
		}

		var sawUndeclared, sawExcused, sawMissing bool
		for _, f := range d.Findings {
			switch {
			case f.Message == "api:foo.Old: func()":
				sawUndeclared = true
				if f.Severity != "warn" {
					t.Errorf("undeclared loss severity = %q, want warn", f.Severity)
				}
			case f.Message == "api:foo.Excused: func()":
				sawExcused = true
				if f.Severity != "info" {
					t.Errorf("prior-declared loss severity = %q, want info", f.Severity)
				}
			case f.Kind == "conservation_unproven":
				sawMissing = true
			}
		}
		if !sawUndeclared || !sawExcused || !sawMissing {
			t.Errorf("missing an expected finding: undeclared=%v excused=%v missing=%v (findings=%+v)", sawUndeclared, sawExcused, sawMissing, d.Findings)
		}
	})

	t.Run("supported with no planes is a clean checked pass", func(t *testing.T) {
		v := review.Run(review.Input{
			Service: "svc", ChangedFiles: []string{"pkg/a.go"}, Base: "deadbeef",
			ConservationSupported: true,
		})
		d := dimension(t, v, review.DimConservation)
		if !d.Checked {
			t.Fatal("expected conservation to be checked")
		}
		if d.Count != 0 {
			t.Errorf("Count = %d, want 0", d.Count)
		}
	})
}
