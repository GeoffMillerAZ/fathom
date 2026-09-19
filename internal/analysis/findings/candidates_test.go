package findings

import (
	"strings"
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

// TestRuleCandidatesWithPolicy_ZeroPolicy_MatchesRuleCandidates pins the
// backward-compat claim in RuleCandidates' own doc comment: the zero
// CandidatePolicy must reproduce byte-identical Score (== Sites), ordering,
// and count.
func TestRuleCandidatesWithPolicy_ZeroPolicy_MatchesRuleCandidates(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("c1", "a", "Do a thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm1"),
		mkClaim("c2", "b", "Do a thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm2"),
		mkClaim("c3", "c", "Do a thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm3"),
	}
	legacy := RuleCandidates(claims, nil, 2)
	policyOut, deferred := RuleCandidatesWithPolicy(claims, nil, CandidatePolicy{MinSites: 2})
	if deferred != 0 {
		t.Fatalf("deferred = %d, want 0 (no cadence cap)", deferred)
	}
	if len(legacy) != 1 || len(policyOut) != 1 {
		t.Fatalf("len = %d/%d, want 1/1", len(legacy), len(policyOut))
	}
	if policyOut[0].Score != float64(policyOut[0].Sites) {
		t.Errorf("Score = %v, want == Sites (%d) at the zero policy", policyOut[0].Score, policyOut[0].Sites)
	}
	if legacy[0].Draft != policyOut[0].Draft || legacy[0].Sites != policyOut[0].Sites {
		t.Errorf("RuleCandidates diverged from RuleCandidatesWithPolicy at the zero policy: %+v vs %+v", legacy[0], policyOut[0])
	}
}

// TestRuleCandidatesWithPolicy_Decay proves DecayPerDispute lowers Score
// once per member carrying a "refuted" claim_event, and leaves an
// undisputed pattern's score untouched.
func TestRuleCandidatesWithPolicy_Decay(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("d1", "a", "Disputed thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm1"),
		mkClaim("d2", "b", "Disputed thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm2"),
		mkClaim("u1", "c", "Undisputed thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm3"),
		mkClaim("u2", "d", "Undisputed thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm4"),
	}
	events := []core.ClaimEvent{
		{ClaimID: "d1", Verdict: "refuted", At: t0},
	}
	out, deferred := RuleCandidatesWithPolicy(claims, events, CandidatePolicy{MinSites: 2, DecayPerDispute: 0.5})
	if deferred != 0 {
		t.Fatalf("deferred = %d, want 0", deferred)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	var disputed, undisputed RuleCandidate
	for _, c := range out {
		if c.Shape == NormalizeRuleShape("Disputed thing.") {
			disputed = c
		} else {
			undisputed = c
		}
	}
	if disputed.Disputes != 1 {
		t.Errorf("disputed.Disputes = %d, want 1", disputed.Disputes)
	}
	if want := float64(disputed.Sites) * 0.5; disputed.Score != want {
		t.Errorf("disputed.Score = %v, want %v (sites=%d * 0.5 decay)", disputed.Score, want, disputed.Sites)
	}
	if undisputed.Disputes != 0 {
		t.Errorf("undisputed.Disputes = %d, want 0", undisputed.Disputes)
	}
	if undisputed.Score != float64(undisputed.Sites) {
		t.Errorf("undisputed.Score = %v, want == Sites (%d), decay must not apply", undisputed.Score, undisputed.Sites)
	}
}

// TestRuleCandidatesWithPolicy_OwnershipWeight proves the score is scaled by
// the MEAN OwnershipWeight over every member's scoped file(s), and that a
// nil weight func is a no-op (1.0).
func TestRuleCandidatesWithPolicy_OwnershipWeight(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("w1", "owned/a.go", "Weighted thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm1"),
		mkClaim("w2", "unowned/b.go", "Weighted thing.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm2"),
	}
	weight := func(path string) float64 {
		if strings.HasPrefix(path, "owned/") {
			return 1.0
		}
		return 0.0
	}
	out, _ := RuleCandidatesWithPolicy(claims, nil, CandidatePolicy{MinSites: 2, OwnershipWeight: weight})
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	// mean(1.0, 0.0) = 0.5; sites = 2 -> score = 1.0.
	if out[0].Score != 1.0 {
		t.Errorf("Score = %v, want 1.0 (sites=2 * mean-weight=0.5)", out[0].Score)
	}

	nilWeighted, _ := RuleCandidatesWithPolicy(claims, nil, CandidatePolicy{MinSites: 2})
	if nilWeighted[0].Score != float64(nilWeighted[0].Sites) {
		t.Errorf("nil OwnershipWeight Score = %v, want == Sites (%d) — no CODEOWNERS must never penalize", nilWeighted[0].Score, nilWeighted[0].Sites)
	}
}

// TestRuleCandidatesWithPolicy_CadenceCap proves at most CadenceCap
// candidates are emitted, the rest counted as deferred, and the surviving
// ones are the highest-scoring.
func TestRuleCandidatesWithPolicy_CadenceCap(t *testing.T) {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	claims := []core.SemanticClaim{
		mkClaim("a1", "a", "Shape one.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm"),
		mkClaim("a2", "b", "Shape one.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm"),
		mkClaim("a3", "c", "Shape one.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm"),
		mkClaim("b1", "d", "Shape two.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm"),
		mkClaim("b2", "e", "Shape two.", core.FindingClassCorrectness, core.ClaimFresh, t0, "wm"),
	}
	out, deferred := RuleCandidatesWithPolicy(claims, nil, CandidatePolicy{MinSites: 2, CadenceCap: 1})
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1 (CadenceCap)", len(out))
	}
	if deferred != 1 {
		t.Errorf("deferred = %d, want 1", deferred)
	}
	if out[0].Sites != 3 {
		t.Errorf("surviving candidate Sites = %d, want 3 (the higher-scoring one)", out[0].Sites)
	}

	uncapped, deferred2 := RuleCandidatesWithPolicy(claims, nil, CandidatePolicy{MinSites: 2})
	if len(uncapped) != 2 || deferred2 != 0 {
		t.Errorf("CadenceCap<=0 must be unbounded: len=%d deferred=%d, want 2/0", len(uncapped), deferred2)
	}
}

// TestPatternID_DeterministicAndStable proves patternID depends only on
// (Class, RuleShape) — the Pattern grouping key — so re-derivation from an
// unchanged claim set always reproduces the same promoted_from tag.
func TestPatternID_DeterministicAndStable(t *testing.T) {
	p1 := Pattern{Class: core.FindingClassSecurity, RuleShape: "do a thing"}
	p2 := Pattern{Class: core.FindingClassSecurity, RuleShape: "do a thing"}
	p3 := Pattern{Class: core.FindingClassSecurity, RuleShape: "do another thing"}
	if patternID(p1) != patternID(p2) {
		t.Errorf("patternID not stable across identical (class, shape): %q vs %q", patternID(p1), patternID(p2))
	}
	if patternID(p1) == patternID(p3) {
		t.Errorf("patternID collided across different shapes")
	}
	if len(patternID(p1)) != 12 {
		t.Errorf("patternID length = %d, want 12", len(patternID(p1)))
	}
}

// TestDraftWithProvenance_CarriesTag pins the exact provenance fields the
// plan requires in Draft.
func TestDraftWithProvenance_CarriesTag(t *testing.T) {
	got := draftWithProvenance("log secrets", "abc123def456", 2, []string{"fathom.review", "human.review"})
	want := "Never log secrets. [promoted_from: abc123def456, members: 2, producers: [fathom.review, human.review]]"
	if got != want {
		t.Errorf("draftWithProvenance = %q, want %q", got, want)
	}
	if got := draftWithProvenance("", "x", 1, nil); got != "" {
		t.Errorf("draftWithProvenance(empty shape) = %q, want \"\"", got)
	}
}
