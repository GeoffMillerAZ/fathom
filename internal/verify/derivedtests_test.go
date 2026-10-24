package verify

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestDerivedTestsRequireReachabilityAndCertificate(t *testing.T) {
	for _, tc := range []struct {
		name, state, label  string
		reaches             bool
		wantReach, wantCert CriterionStatus
	}{
		{"both pass", "live", "verified", true, Pass, Pass},
		{"reachability fails", "live", "present-but-untested", false, Fail, Pass},
		{"certificate absent", "", "certificate-absent", true, Pass, Fail},
		{"certificate unproven", "unproven", "certificate-unproven", true, Pass, Fail},
		{"both fail", "", "present-but-untested", false, Fail, Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := ParseMarkdownPlan("svc", []byte(`- [x] Check {symbol: Check, derived_tests: "TestMissing,TestAlpha"}`))
			ev := proofTestEvidence{symbols: map[string][]core.SymbolNode{"Check": {proofTestNode("Check", "check.go", 1, core.TierVerified)}}}
			if tc.reaches {
				ev.tests = map[string][]string{"Check": {"TestAlpha"}}
			}
			if tc.state != "" {
				ev.byTest = map[string][]Certification{"TestAlpha": {{Test: "TestAlpha", Strength: "asserts", State: tc.state, Scope: []string{"check.go"}}}}
			}
			v := VerifyPlan(plan, ev).Tasks[0]
			wantStatus := Fail
			if tc.wantReach == Pass && tc.wantCert == Pass {
				wantStatus = Pass
			}
			if v.Status != wantStatus || v.Label != tc.label {
				t.Errorf("status/label = %s/%s, want %s/%s", v.Status, v.Label, wantStatus, tc.label)
			}
			for _, expected := range []struct {
				name string
				want CriterionStatus
			}{{"test_reaches", tc.wantReach}, {"derived_tests", tc.wantCert}} {
				name, want := expected.name, expected.want
				found := false
				for _, c := range v.Criteria {
					if c.Name == name {
						found = true
						if c.Status != want || c.Detail == "" || (name == "derived_tests" && !strings.Contains(c.Detail, "derived test certificates")) {
							t.Errorf("%s = %+v, want %s with detail naming the requirement", name, c, want)
						}
					}
				}
				if !found {
					t.Errorf("missing %s criterion: %+v", name, v.Criteria)
				}
			}
		})
	}
}
