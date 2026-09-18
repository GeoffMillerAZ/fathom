package findings

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestProducerOf(t *testing.T) {
	cases := []struct{ subject, want string }{
		{"arch_conformance#api.A", nativeReviewProducer},
		{"", nativeReviewProducer},
		{"ext:reviewkit.security-review:finding-123", "reviewkit.security-review"},
		{"ext:reviewkit.system-review/f-42", "reviewkit.system-review"},
		{"ext:", "external"},
	}
	for _, tc := range cases {
		if got := producerOf(tc.subject); got != tc.want {
			t.Errorf("producerOf(%q) = %q, want %q", tc.subject, got, tc.want)
		}
	}
}

func TestCheckFamilyFor(t *testing.T) {
	cases := []struct {
		class core.FindingClass
		want  string
	}{
		{core.FindingClassSecurity, "secret_literal"},
		{core.FindingClassConformance, "import_direction"},
		{core.FindingClassTestGap, "usecase_maintained"},
		{core.FindingClassConfigDrift, "workspace_output"},
		{core.FindingClassCorrectness, "advisory"},
		{core.FindingClassResourceLeak, "advisory"},
		{core.FindingClassErrorHandling, "advisory"},
		{core.FindingClassConcurrency, "advisory"},
		{core.FindingClassPerformance, "advisory"},
		{core.FindingClassContractViolation, "advisory"},
	}
	for _, tc := range cases {
		if got := checkFamilyFor(tc.class); got != tc.want {
			t.Errorf("checkFamilyFor(%q) = %q, want %q", tc.class, got, tc.want)
		}
	}
}

func TestDraftSentence(t *testing.T) {
	if got := draftSentence("log secrets in plaintext"); got != "Never log secrets in plaintext." {
		t.Errorf("draftSentence = %q", got)
	}
	if got := draftSentence(""); got != "" {
		t.Errorf("draftSentence(\"\") = %q, want \"\"", got)
	}
}

func TestRuleCandidates_TwoSites_OneCandidate(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("c1", "ext:reviewkit.security-review:f1", "Do not log API keys in plaintext.", core.FindingClassSecurity, core.ClaimFresh, t0, "wm1"),
		mkClaim("c2", "arch_conformance#api.B", "Do not log API keys in plaintext.", core.FindingClassSecurity, core.ClaimFresh, t0.Add(time.Hour), "wm2"),
	}
	cands := RuleCandidates(claims, nil, 2)
	if len(cands) != 1 {
		t.Fatalf("len(cands) = %d, want 1", len(cands))
	}
	c := cands[0]
	if c.Sites != 2 {
		t.Errorf("Sites = %d, want 2", c.Sites)
	}
	if c.Class != core.FindingClassSecurity {
		t.Errorf("Class = %q", c.Class)
	}
	if c.CheckFamily != "secret_literal" {
		t.Errorf("CheckFamily = %q, want secret_literal", c.CheckFamily)
	}
	wantProducers := []string{"fathom.review", "reviewkit.security-review"}
	if len(c.Producers) != 2 || c.Producers[0] != wantProducers[0] || c.Producers[1] != wantProducers[1] {
		t.Errorf("Producers = %v, want %v", c.Producers, wantProducers)
	}
	if len(c.Members) != 2 {
		t.Errorf("len(Members) = %d, want 2", len(c.Members))
	}
	if c.Draft == "" || c.Draft[:6] != "Never " {
		t.Errorf("Draft = %q, want a 'Never …' template", c.Draft)
	}
}

func TestRuleCandidates_MinSitesFilters(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("c1", "a", "Do a thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm1"),
		mkClaim("c2", "b", "Do a thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm2"),
	}
	if got := RuleCandidates(claims, nil, 3); len(got) != 0 {
		t.Errorf("min-sites 3 over a 2-site pattern must yield no candidates, got %d", len(got))
	}
	if got := RuleCandidates(claims, nil, 0); len(got) != 1 {
		t.Errorf("min-sites <2 must clamp to the 2-site floor, got %d candidates", len(got))
	}
}

func TestRuleCandidates_SortedSitesDescThenClassThenShape(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		// 2-site pattern (class B).
		mkClaim("c1", "a", "Shape one thing.", core.FindingClassPerformance, core.ClaimFresh, t0, "wm"),
		mkClaim("c2", "b", "Shape one thing.", core.FindingClassPerformance, core.ClaimFresh, t0, "wm"),
		// 3-site pattern (class A) — must sort FIRST despite class order.
		mkClaim("c3", "c", "Shape two thing.", core.FindingClassConcurrency, core.ClaimFresh, t0, "wm"),
		mkClaim("c4", "d", "Shape two thing.", core.FindingClassConcurrency, core.ClaimFresh, t0, "wm"),
		mkClaim("c5", "e", "Shape two thing.", core.FindingClassConcurrency, core.ClaimFresh, t0, "wm"),
	}
	cands := RuleCandidates(claims, nil, 2)
	if len(cands) != 2 {
		t.Fatalf("len(cands) = %d, want 2", len(cands))
	}
	if cands[0].Sites != 3 || cands[1].Sites != 2 {
		t.Errorf("sites order = [%d %d], want [3 2] (sites desc)", cands[0].Sites, cands[1].Sites)
	}
}

func TestRuleCandidates_EmptyInput(t *testing.T) {
	if got := RuleCandidates(nil, nil, 2); len(got) != 0 {
		t.Errorf("empty input must yield no candidates, got %d", len(got))
	}
}
