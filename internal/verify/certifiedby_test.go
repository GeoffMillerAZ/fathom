package verify

import "testing"

// TestEvaluateCertifiedBy_EveryVerdict (TC-9, design doc §4): certified
// proves the row; certificate-unproven, certificate-broken,
// certificate-unreachable, and certificate-absent never do.
func TestEvaluateCertifiedBy_EveryVerdict(t *testing.T) {
	const file = "billing/refund.go"

	cases := []struct {
		name        string
		certifiedBy []string
		strength    string
		byTest      map[string][]Certification
		wantStatus  CriterionStatus
		wantLabel   string
	}{
		{
			name:        "certified: a live certificate scoped to the file",
			certifiedBy: []string{"TestRefund"},
			byTest: map[string][]Certification{
				"TestRefund": {{Test: "TestRefund", Strength: "asserts", State: "live", Scope: []string{"pkg/refund_test.go", file}}},
			},
			wantStatus: Pass,
		},
		{
			name:        "certificate-unproven: exists, scoped, never run",
			certifiedBy: []string{"TestRefund"},
			byTest: map[string][]Certification{
				"TestRefund": {{Test: "TestRefund", Strength: "asserts", State: "unproven", Scope: []string{file}}},
			},
			wantStatus: Fail, wantLabel: "certificate-unproven",
		},
		{
			name:        "certificate-broken: exists, scoped, last run failed",
			certifiedBy: []string{"TestRefund"},
			byTest: map[string][]Certification{
				"TestRefund": {{Test: "TestRefund", Strength: "asserts", State: "broken", Scope: []string{file}}},
			},
			wantStatus: Fail, wantLabel: "certificate-broken",
		},
		{
			name:        "certificate-unreachable: exists, scoped, but the test never reaches the target",
			certifiedBy: []string{"TestRefund"},
			byTest: map[string][]Certification{
				"TestRefund": {{Test: "TestRefund", Strength: "asserts", State: "unreachable", Scope: []string{file}}},
			},
			wantStatus: Fail, wantLabel: "certificate-unreachable",
		},
		{
			name:        "certificate-absent: no certificate names this test at all",
			certifiedBy: []string{"TestRefund"},
			byTest:      map[string][]Certification{},
			wantStatus:  Fail, wantLabel: "certificate-absent",
		},
		{
			name:        "certificate-absent: the named test certifies something, but not this file",
			certifiedBy: []string{"TestRefund"},
			byTest: map[string][]Certification{
				"TestRefund": {{Test: "TestRefund", Strength: "asserts", State: "live", Scope: []string{"other/file.go"}}},
			},
			wantStatus: Fail, wantLabel: "certificate-absent",
		},
		{
			name:        "strength floor: a live certificate below the required strength does not certify",
			certifiedBy: []string{"TestRefund"},
			strength:    "conformance",
			byTest: map[string][]Certification{
				"TestRefund": {{Test: "TestRefund", Strength: "asserts", State: "live", Scope: []string{file}}},
			},
			wantStatus: Fail, wantLabel: "certificate-absent",
		},
		{
			name:        "the most informative reason wins across several named tests",
			certifiedBy: []string{"TestA", "TestB"},
			byTest: map[string][]Certification{
				"TestA": {{Test: "TestA", Strength: "asserts", State: "unproven", Scope: []string{file}}},
				"TestB": {{Test: "TestB", Strength: "asserts", State: "broken", Scope: []string{file}}},
			},
			wantStatus: Fail, wantLabel: "certificate-broken",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			crit, label := EvaluateCertifiedBy(file, tc.certifiedBy, tc.strength, tc.byTest)
			if crit.Status != tc.wantStatus {
				t.Errorf("status = %s, want %s (detail: %s)", crit.Status, tc.wantStatus, crit.Detail)
			}
			if label != tc.wantLabel {
				t.Errorf("label = %q, want %q", label, tc.wantLabel)
			}
			if crit.Name != "certified_by" {
				t.Errorf("Criterion.Name = %q, want certified_by", crit.Name)
			}
		})
	}
}
