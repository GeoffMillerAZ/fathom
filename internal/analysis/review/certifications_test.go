package review_test

// certifications_test.go — TC-10 (vision 33 / ADR-043 stage 1): the
// certifications dimension's own contract, design doc §4: per changed
// file, its certificates (test, strength, state); one certification_gap
// finding per subject touching the diff with no live certificate; and the
// two "not checked" degradations (no backend, no declarations at all).

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/review"
	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestReview_CertificationsDimension(t *testing.T) {
	t.Run("unsupported backend", func(t *testing.T) {
		v := review.Run(review.Input{Service: "svc", ChangedFiles: []string{"pkg/a.go"}})
		d := dimension(t, v, review.DimCertifications)
		if d.Checked {
			t.Fatal("certifications should be unchecked when the backend does not serve semantic layers")
		}
		if d.Reason == "" {
			t.Error("expected a non-empty reason")
		}
	})

	t.Run("supported but nothing declared", func(t *testing.T) {
		v := review.Run(review.Input{
			Service:                 "svc",
			ChangedFiles:            []string{"pkg/a.go"},
			CertificationsSupported: true,
		})
		d := dimension(t, v, review.DimCertifications)
		if d.Checked {
			t.Fatal("certifications should be unchecked when the layer is empty")
		}
		if d.Reason != "no certifications declared" {
			t.Errorf("Reason = %q, want %q", d.Reason, "no certifications declared")
		}
	})

	t.Run("scoped to changed files, live and gap", func(t *testing.T) {
		v := review.Run(review.Input{
			Service:                 "svc",
			ChangedFiles:            []string{"pkg/a.go", "pkg/b.go"},
			CertificationsSupported: true,
			CertificationsDeclared:  true,
			CertificationsByFile: map[string][]review.CertificationRecord{
				"pkg/a.go": {{
					Subject: "invariant:I35", Test: "TestRefund_Idempotent",
					Strength: "asserts", State: "live", Tier: string(core.TierVerified),
					Rule:  "TestRefund_Idempotent certifies invariant I35 (asserts) — live",
					Scope: []string{"pkg/a.go"},
				}},
				"pkg/b.go": {{
					Subject: "invariant:I40", Test: "TestQuota_Enforced",
					Strength: "asserts", State: "broken", Tier: string(core.TierPreScan),
					Rule:   "TestQuota_Enforced certifies invariant I40 (asserts) — broken",
					Status: "stale",
					Scope:  []string{"pkg/b.go"},
				}},
			},
		})
		d := dimension(t, v, review.DimCertifications)
		if !d.Checked {
			t.Fatalf("certifications should be checked, reason=%q", d.Reason)
		}

		var certs, gaps []review.Finding
		for _, f := range d.Findings {
			switch f.Kind {
			case "certification":
				certs = append(certs, f)
			case "certification_gap":
				gaps = append(gaps, f)
			}
		}
		if len(certs) != 2 {
			t.Fatalf("expected 2 certificate findings, got %d: %+v", len(certs), d.Findings)
		}
		for _, c := range certs {
			if c.Test == "" || c.Strength == "" || c.State == "" {
				t.Errorf("certificate finding missing test/strength/state: %+v", c)
			}
		}
		if len(gaps) != 1 {
			t.Fatalf("expected 1 certification_gap finding (I40 has no live certificate), got %d: %+v", len(gaps), d.Findings)
		}
		if gaps[0].Message == "" {
			t.Error("gap finding should carry a non-empty message naming the subject")
		}
		// the live subject (I35) must never appear as a gap.
		for _, g := range gaps {
			if g.Message == "invariant:I35 has no live certificate" {
				t.Errorf("I35 is live and must not be reported as a gap: %+v", g)
			}
		}
		// the stale status label is carried through, never dropped.
		var brokenFinding review.Finding
		found := false
		for _, c := range certs {
			if c.Test == "TestQuota_Enforced" {
				brokenFinding, found = c, true
			}
		}
		if !found {
			t.Fatal("expected a certificate finding for TestQuota_Enforced")
		}
		if brokenFinding.Status != "stale" {
			t.Errorf("Status = %q, want stale (served-with-label discipline)", brokenFinding.Status)
		}
	})
}
