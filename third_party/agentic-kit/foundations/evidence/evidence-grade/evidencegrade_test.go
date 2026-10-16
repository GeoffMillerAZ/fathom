package evidencegrade_test

import (
	"strings"
	"testing"

	eg "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func mustGrade(t *testing.T, spans []eg.GradedSpan, rule eg.PromotionRule) eg.ClaimGrade {
	t.Helper()
	g, err := eg.GradeClaim(spans, rule)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return g
}

func necessarySpan(id string, tier eg.Tier, conf float64) eg.GradedSpan {
	return eg.GradedSpan{SpanID: id, Tier: tier, Confidence: conf, Necessary: true, Independent: false}
}

func corrSpan(id string, tier eg.Tier, conf float64) eg.GradedSpan {
	return eg.GradedSpan{SpanID: id, Tier: tier, Confidence: conf, Necessary: false, Independent: true}
}

// noPromo is the default rule with no promotion allowed.
var noPromo = eg.PromotionRule{}

// ---------------------------------------------------------------------------
// Tier
// ---------------------------------------------------------------------------

func TestTierRank(t *testing.T) {
	cases := []struct {
		tier eg.Tier
		want int
	}{
		{eg.TierDirect, 5},
		{eg.TierDocumentary, 4},
		{eg.TierCircumstantial, 3},
		{eg.TierTestimonial, 2},
		{eg.TierInferred, 1},
		{eg.Tier("unknown"), 0},
	}
	for _, tc := range cases {
		t.Run(string(tc.tier), func(t *testing.T) {
			if got := tc.tier.Rank(); got != tc.want {
				t.Errorf("Rank() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTierValidate(t *testing.T) {
	valid := []eg.Tier{eg.TierDirect, eg.TierDocumentary, eg.TierCircumstantial, eg.TierTestimonial, eg.TierInferred}
	for _, tier := range valid {
		t.Run(string(tier), func(t *testing.T) {
			if err := tier.Validate(); err != nil {
				t.Errorf("expected no error for %q, got %v", tier, err)
			}
		})
	}
	if err := eg.Tier("bogus").Validate(); err == nil {
		t.Error("expected error for unknown tier")
	}
}

func TestTierStrongerThan(t *testing.T) {
	cases := []struct {
		a, b eg.Tier
		want bool
	}{
		{eg.TierDirect, eg.TierInferred, true},
		{eg.TierDocumentary, eg.TierTestimonial, true},
		{eg.TierInferred, eg.TierDirect, false},
		{eg.TierDirect, eg.TierDirect, false},
		{eg.Tier("x"), eg.TierDirect, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.a)+">"+string(tc.b), func(t *testing.T) {
			if got := tc.a.StrongerThan(tc.b); got != tc.want {
				t.Errorf("StrongerThan = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Grade.Validate
// ---------------------------------------------------------------------------

func TestGradeValidate(t *testing.T) {
	cases := []struct {
		name    string
		g       eg.Grade
		wantErr bool
	}{
		{"valid", eg.Grade{Tier: eg.TierDirect, Confidence: 0.9}, false},
		{"confidence-zero", eg.Grade{Tier: eg.TierInferred, Confidence: 0.0}, false},
		{"confidence-one", eg.Grade{Tier: eg.TierDocumentary, Confidence: 1.0}, false},
		{"bad-tier", eg.Grade{Tier: "x", Confidence: 0.5}, true},
		{"conf-negative", eg.Grade{Tier: eg.TierDirect, Confidence: -0.01}, true},
		{"conf-above-one", eg.Grade{Tier: eg.TierDirect, Confidence: 1.01}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.g.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GradedSpan.Validate
// ---------------------------------------------------------------------------

func TestGradedSpanValidate(t *testing.T) {
	cases := []struct {
		name    string
		s       eg.GradedSpan
		wantErr bool
	}{
		{"valid-necessary", eg.GradedSpan{SpanID: "s1", Tier: eg.TierDirect, Confidence: 0.8, Necessary: true, Independent: false}, false},
		{"valid-corr", eg.GradedSpan{SpanID: "s2", Tier: eg.TierDocumentary, Confidence: 0.7, Necessary: false, Independent: true}, false},
		{"empty-spanid", eg.GradedSpan{SpanID: "", Tier: eg.TierDirect, Confidence: 0.5, Necessary: true}, true},
		{"bad-tier", eg.GradedSpan{SpanID: "s3", Tier: "x", Confidence: 0.5, Necessary: true}, true},
		{"conf-negative", eg.GradedSpan{SpanID: "s4", Tier: eg.TierDirect, Confidence: -0.1, Necessary: true}, true},
		{"conf-above-one", eg.GradedSpan{SpanID: "s5", Tier: eg.TierDirect, Confidence: 1.1, Necessary: true}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.s.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GradeClaim: error paths
// ---------------------------------------------------------------------------

func TestGradeClaimErrors(t *testing.T) {
	cases := []struct {
		name    string
		spans   []eg.GradedSpan
		errFrag string
	}{
		{
			name:    "empty-spans",
			spans:   nil,
			errFrag: "at least one span",
		},
		{
			name:    "no-necessary-span",
			spans:   []eg.GradedSpan{corrSpan("c1", eg.TierDirect, 0.9)},
			errFrag: "at least one necessary span",
		},
		{
			name: "invalid-span-bad-tier",
			spans: []eg.GradedSpan{
				{SpanID: "s1", Tier: "bogus", Confidence: 0.5, Necessary: true},
			},
			errFrag: "unknown tier",
		},
		{
			name: "invalid-span-empty-id",
			spans: []eg.GradedSpan{
				{SpanID: "", Tier: eg.TierDirect, Confidence: 0.5, Necessary: true},
			},
			errFrag: "SpanID",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := eg.GradeClaim(tc.spans, noPromo)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.errFrag) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.errFrag)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GradeClaim: FloorTier is weakest necessary tier
// ---------------------------------------------------------------------------

func TestGradeClaimFloorTier(t *testing.T) {
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierDirect, 0.9),
		necessarySpan("s2", eg.TierTestimonial, 0.8), // weakest
		necessarySpan("s3", eg.TierDocumentary, 0.85),
	}
	g := mustGrade(t, spans, noPromo)
	if g.FloorTier != eg.TierTestimonial {
		t.Errorf("FloorTier = %q, want testimonial", g.FloorTier)
	}
	if g.Tier != eg.TierTestimonial {
		t.Errorf("effective Tier = %q, want testimonial (no promotion)", g.Tier)
	}
}

// ---------------------------------------------------------------------------
// GradeClaim: confidence aggregation
// ---------------------------------------------------------------------------

func TestGradeClaimConfidenceProduct(t *testing.T) {
	// Single necessary span: confidence passes through.
	g := mustGrade(t, []eg.GradedSpan{necessarySpan("s1", eg.TierDirect, 0.8)}, noPromo)
	if g.Confidence != 0.8 {
		t.Errorf("single-span confidence = %g, want 0.8", g.Confidence)
	}

	// Two necessary spans: product model.
	// 0.8 * 0.5 = 0.4
	g2 := mustGrade(t, []eg.GradedSpan{
		necessarySpan("s1", eg.TierDirect, 0.8),
		necessarySpan("s2", eg.TierDirect, 0.5),
	}, noPromo)
	const want = 0.4
	if g2.Confidence != want {
		t.Errorf("product confidence = %g, want %g", g2.Confidence, want)
	}
}

func TestGradeClaimCorroborationRaisesConfidence(t *testing.T) {
	// Necessary: 0.6. Corroboration 0.8.
	// base = 0.6
	// combined = 1 − (1−0.6)*(1−0.8) = 1 − 0.4*0.2 = 1 − 0.08 = 0.92
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierDocumentary, 0.6),
		corrSpan("c1", eg.TierDocumentary, 0.8),
	}
	g := mustGrade(t, spans, noPromo)
	if g.Confidence <= 0.6 {
		t.Errorf("corroboration should raise confidence above 0.6, got %g", g.Confidence)
	}
	const want = 0.92
	if g.Confidence != want {
		t.Errorf("confidence = %g, want %g", g.Confidence, want)
	}
}

func TestGradeClaimConfidenceAtBoundaries(t *testing.T) {
	// Zero-confidence necessary span → result near 0.
	g := mustGrade(t, []eg.GradedSpan{necessarySpan("s1", eg.TierDirect, 0.0)}, noPromo)
	if g.Confidence != 0.0 {
		t.Errorf("zero confidence span: got %g, want 0.0", g.Confidence)
	}

	// Full-confidence necessary span → 1.0.
	g2 := mustGrade(t, []eg.GradedSpan{necessarySpan("s1", eg.TierDirect, 1.0)}, noPromo)
	if g2.Confidence != 1.0 {
		t.Errorf("full confidence span: got %g, want 1.0", g2.Confidence)
	}
}

// ---------------------------------------------------------------------------
// GradeClaim: promotion
// ---------------------------------------------------------------------------

func TestGradeClaimNoPromotionByDefault(t *testing.T) {
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.7),
		corrSpan("c1", eg.TierDirect, 0.9),
		corrSpan("c2", eg.TierDirect, 0.9),
	}
	g := mustGrade(t, spans, noPromo)
	if g.Promoted {
		t.Error("expected no promotion when AllowTierPromotion=false")
	}
	if g.Tier != eg.TierTestimonial {
		t.Errorf("tier = %q, want testimonial", g.Tier)
	}
}

func TestGradeClaimPromotionRequiresSufficientSources(t *testing.T) {
	rule := eg.PromotionRule{AllowTierPromotion: true, MinIndependentSourcesForPromotion: 2}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.7),
		corrSpan("c1", eg.TierDirect, 0.9), // only 1 qualifying
	}
	g := mustGrade(t, spans, rule)
	if g.Promoted {
		t.Error("should not promote with only 1 corroborating span when minimum is 2")
	}
}

func TestGradeClaimPromotionSucceeds(t *testing.T) {
	rule := eg.PromotionRule{AllowTierPromotion: true, MinIndependentSourcesForPromotion: 2}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.7),
		corrSpan("c1", eg.TierDocumentary, 0.85),
		corrSpan("c2", eg.TierCircumstantial, 0.80),
	}
	g := mustGrade(t, spans, rule)
	if !g.Promoted {
		t.Error("expected promotion")
	}
	// Promoted to weakest of qualifying corroborators: circumstantial (rank 3) < documentary (rank 4)
	if g.Tier != eg.TierCircumstantial {
		t.Errorf("promoted tier = %q, want circumstantial", g.Tier)
	}
	if g.FloorTier != eg.TierTestimonial {
		t.Errorf("FloorTier = %q, want testimonial", g.FloorTier)
	}
	if g.PromotionReason == "" {
		t.Error("PromotionReason should not be empty after promotion")
	}
}

// ---------------------------------------------------------------------------
// GradeClaim: hard invariant — all-inferred cannot be promoted
// ---------------------------------------------------------------------------

func TestGradeClaimInferredNotPromoted(t *testing.T) {
	rule := eg.PromotionRule{AllowTierPromotion: true, MinIndependentSourcesForPromotion: 1}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierInferred, 0.9),
		corrSpan("c1", eg.TierDirect, 0.95),
	}
	g := mustGrade(t, spans, rule)
	if g.Promoted {
		t.Error("AI-inferred necessary spans must never be promoted")
	}
	if g.Tier != eg.TierInferred {
		t.Errorf("tier = %q, want inferred", g.Tier)
	}
}

// ---------------------------------------------------------------------------
// GradeClaim: mixed-tier necessary spans with corroboration
// ---------------------------------------------------------------------------

func TestGradeClaimMixedNecessaryAndCorroboration(t *testing.T) {
	rule := eg.PromotionRule{AllowTierPromotion: true, MinIndependentSourcesForPromotion: 1}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierCircumstantial, 0.7),
		necessarySpan("s2", eg.TierTestimonial, 0.6), // floor
		corrSpan("c1", eg.TierDirect, 0.95),
	}
	g := mustGrade(t, spans, rule)
	if g.FloorTier != eg.TierTestimonial {
		t.Errorf("FloorTier = %q, want testimonial", g.FloorTier)
	}
	if !g.Promoted {
		t.Error("expected promotion: 1 direct independent corroborator qualifies")
	}
	if g.Tier != eg.TierDirect {
		t.Errorf("promoted tier = %q, want direct", g.Tier)
	}
	// Confidence: base = 0.7*0.6 = 0.42, corr raises it.
	// combined = 1 − (1−0.42)*(1−0.95) = 1 − 0.58*0.05 = 1 − 0.029 = 0.971
	if g.Confidence <= 0.42 {
		t.Errorf("confidence %g should be > base 0.42 with corroboration", g.Confidence)
	}
}

// ---------------------------------------------------------------------------
// GradeClaim: single necessary span, no corroboration
// ---------------------------------------------------------------------------

func TestGradeClaimSingleSpan(t *testing.T) {
	tiers := []eg.Tier{eg.TierDirect, eg.TierDocumentary, eg.TierCircumstantial, eg.TierTestimonial, eg.TierInferred}
	for _, tier := range tiers {
		t.Run(string(tier), func(t *testing.T) {
			g := mustGrade(t, []eg.GradedSpan{necessarySpan("s1", tier, 0.75)}, noPromo)
			if g.Tier != tier {
				t.Errorf("tier = %q, want %q", g.Tier, tier)
			}
			if g.FloorTier != tier {
				t.Errorf("FloorTier = %q, want %q", g.FloorTier, tier)
			}
			if g.Confidence != 0.75 {
				t.Errorf("confidence = %g, want 0.75", g.Confidence)
			}
			if g.Promoted {
				t.Error("should not be promoted without corroboration")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GradeClaim: promotion rule MinIndependentSourcesForPromotion ≤ 0 treated as 1
// ---------------------------------------------------------------------------

func TestGradeClaimPromotionMinZeroTreatedAsOne(t *testing.T) {
	rule := eg.PromotionRule{AllowTierPromotion: true, MinIndependentSourcesForPromotion: 0}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.7),
		corrSpan("c1", eg.TierDirect, 0.9),
	}
	g := mustGrade(t, spans, rule)
	if !g.Promoted {
		t.Error("min=0 should be treated as 1; single corroborator should promote")
	}
}

// ---------------------------------------------------------------------------
// FIX 1 regression: duplicate SpanID must not inflate independent-source count
// ---------------------------------------------------------------------------

func TestGradeClaimPromotionDuplicateSpanIDNotCounted(t *testing.T) {
	// Same SpanID "dup1" submitted twice as corroborator — must count as ONE source.
	rule := eg.PromotionRule{AllowTierPromotion: true, MinIndependentSourcesForPromotion: 2}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.7),
		corrSpan("dup1", eg.TierDirect, 0.9),
		corrSpan("dup1", eg.TierDirect, 0.9), // identical SpanID — must be deduped
	}
	g := mustGrade(t, spans, rule)
	if g.Promoted {
		t.Error("promotion must not fire when required sources are the same SpanID duplicated")
	}
	if g.Tier != eg.TierTestimonial {
		t.Errorf("tier = %q, want testimonial (floor, no promotion)", g.Tier)
	}
}

func TestGradeClaimPromotionDistinctSpanIDsPromote(t *testing.T) {
	// Two DISTINCT SpanIDs — promotion must fire (sanity check).
	rule := eg.PromotionRule{AllowTierPromotion: true, MinIndependentSourcesForPromotion: 2}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.7),
		corrSpan("c1", eg.TierDirect, 0.9),
		corrSpan("c2", eg.TierDirect, 0.9),
	}
	g := mustGrade(t, spans, rule)
	if !g.Promoted {
		t.Error("promotion must fire with two distinct independent SpanIDs")
	}
	if g.Tier != eg.TierDirect {
		t.Errorf("promoted tier = %q, want direct", g.Tier)
	}
}

// ---------------------------------------------------------------------------
// FIX 3 regression: Necessary && Independent is an invalid combo
// ---------------------------------------------------------------------------

func TestGradedSpanValidateNecessaryAndIndependent(t *testing.T) {
	gs := eg.GradedSpan{
		SpanID:      "s1",
		Tier:        eg.TierDirect,
		Confidence:  0.9,
		Necessary:   true,
		Independent: true,
	}
	err := gs.Validate()
	if err == nil {
		t.Fatal("expected error for Necessary=true && Independent=true, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be both Necessary and Independent") {
		t.Errorf("error %q does not contain expected message", err.Error())
	}
}

func TestGradeClaimRejectsNecessaryAndIndependentSpan(t *testing.T) {
	// GradeClaim must also reject via its per-span Validate() call.
	spans := []eg.GradedSpan{
		{SpanID: "s1", Tier: eg.TierDirect, Confidence: 0.9, Necessary: true, Independent: true},
	}
	_, err := eg.GradeClaim(spans, noPromo)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "cannot be both Necessary and Independent") {
		t.Errorf("error %q does not contain expected message", err.Error())
	}
}

// ---------------------------------------------------------------------------
// FINDING 1: Zero-confidence corroborator must not trigger tier promotion
// ---------------------------------------------------------------------------

func TestGradeClaimZeroConfidenceCorroboratorCannotPromote(t *testing.T) {
	// necessary=testimonial/0.8, corroborator=direct/0.0, AllowPromotion=true
	// A corroborator with Confidence=0.0 must not qualify for tier promotion.
	rule := eg.PromotionRule{AllowTierPromotion: true, MinIndependentSourcesForPromotion: 1}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.8),
		corrSpan("c1", eg.TierDirect, 0.0), // zero confidence — must not promote
	}
	g := mustGrade(t, spans, rule)
	if g.Promoted {
		t.Error("zero-confidence corroborator must not trigger tier promotion")
	}
	if g.Tier != eg.TierTestimonial {
		t.Errorf("tier = %q, want testimonial (no promotion from zero-conf corroborator)", g.Tier)
	}
}

func TestGradeClaimMinCorroboratorConfidenceRespected(t *testing.T) {
	// MinCorroboratorConfidence=0.5: a corroborator at 0.4 must not qualify; one at 0.6 must.
	ruleStrict := eg.PromotionRule{
		AllowTierPromotion:                true,
		MinIndependentSourcesForPromotion: 1,
		MinCorroboratorConfidence:         0.5,
	}
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.8),
		corrSpan("c1", eg.TierDirect, 0.4), // below threshold
	}
	g := mustGrade(t, spans, ruleStrict)
	if g.Promoted {
		t.Error("corroborator below MinCorroboratorConfidence must not trigger promotion")
	}

	// Same rule but corroborator meets threshold — must promote.
	spansAbove := []eg.GradedSpan{
		necessarySpan("s2", eg.TierTestimonial, 0.8),
		corrSpan("c2", eg.TierDirect, 0.6), // at/above threshold
	}
	g2 := mustGrade(t, spansAbove, ruleStrict)
	if !g2.Promoted {
		t.Error("corroborator at or above MinCorroboratorConfidence must trigger promotion")
	}
}

// ---------------------------------------------------------------------------
// FINDING 2: Silent span drop — spans with Necessary=false && Independent=false
// must not be silently discarded; GradeClaim must surface them.
// ---------------------------------------------------------------------------

func TestGradeClaimDroppedSpansReturned(t *testing.T) {
	// span-003 has Necessary=false, Independent=false — it is neither necessary
	// nor a corroborator. GradeClaim must surface it in DroppedSpanIDs.
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierDocumentary, 0.8),
		{SpanID: "span-drop", Tier: eg.TierInferred, Confidence: 0.4, Necessary: false, Independent: false},
	}
	g, err := eg.GradeClaim(spans, noPromo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(g.DroppedSpanIDs) == 0 {
		t.Fatal("DroppedSpanIDs must list spans that are neither Necessary nor Independent")
	}
	found := false
	for _, id := range g.DroppedSpanIDs {
		if id == "span-drop" {
			found = true
		}
	}
	if !found {
		t.Errorf("DroppedSpanIDs = %v, want to contain %q", g.DroppedSpanIDs, "span-drop")
	}
}

func TestGradeClaimNoDroppedSpansWhenAllPartitioned(t *testing.T) {
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierDirect, 0.9),
		corrSpan("c1", eg.TierDocumentary, 0.7),
	}
	g := mustGrade(t, spans, noPromo)
	if len(g.DroppedSpanIDs) != 0 {
		t.Errorf("DroppedSpanIDs should be empty when all spans are partitioned, got %v", g.DroppedSpanIDs)
	}
}

// ---------------------------------------------------------------------------
// FINDING 3: AI-inferred corroborators inflate confidence — AIContributedConfidence
// must track the confidence contributed solely from TierInferred corroborators.
// ---------------------------------------------------------------------------

func TestGradeClaimAIContributedConfidenceTracked(t *testing.T) {
	// necessary=testimonial/0.5 (base=0.5), corr=inferred/0.99
	// combined = 1 - (1-0.5)*(1-0.99) = 1 - 0.5*0.01 = 0.995
	// AIContributedConfidence must be > 0 since an inferred corroborator participated.
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.5),
		corrSpan("c1", eg.TierInferred, 0.99),
	}
	g := mustGrade(t, spans, noPromo)
	if g.AIContributedConfidence <= 0 {
		t.Errorf("AIContributedConfidence = %g, want > 0 when TierInferred corroborator is present", g.AIContributedConfidence)
	}
	// The aggregate should be higher than base.
	if g.Confidence <= 0.5 {
		t.Errorf("aggregate confidence %g should exceed base 0.5", g.Confidence)
	}
}

func TestGradeClaimNoAIContributedConfidenceWithoutInferred(t *testing.T) {
	// No TierInferred corroborators → AIContributedConfidence must be 0.
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierDocumentary, 0.7),
		corrSpan("c1", eg.TierDocumentary, 0.8),
	}
	g := mustGrade(t, spans, noPromo)
	if g.AIContributedConfidence != 0 {
		t.Errorf("AIContributedConfidence = %g, want 0 when no TierInferred corroborators", g.AIContributedConfidence)
	}
}

func TestGradeClaimAIContributedConfidenceMeasuresInflation(t *testing.T) {
	// Verify that AIContributedConfidence equals aggConf - (conf without inferred corroborators).
	// necessary=testimonial/0.5, corr1=documentary/0.8, corr2=inferred/0.99
	// base = 0.5
	// without inferred: 1 - (1-0.5)*(1-0.8) = 1 - 0.1 = 0.9
	// with inferred:    1 - (1-0.5)*(1-0.8)*(1-0.99) = 1 - 0.001 = 0.999
	// AIContributedConfidence ≈ 0.999 - 0.9 = 0.099 (rounded)
	spans := []eg.GradedSpan{
		necessarySpan("s1", eg.TierTestimonial, 0.5),
		corrSpan("c1", eg.TierDocumentary, 0.8),
		corrSpan("c2", eg.TierInferred, 0.99),
	}
	g := mustGrade(t, spans, noPromo)
	if g.AIContributedConfidence <= 0 {
		t.Errorf("AIContributedConfidence = %g, want > 0", g.AIContributedConfidence)
	}
	// It must be less than the total aggregate (it's just the incremental AI contribution).
	if g.AIContributedConfidence >= g.Confidence {
		t.Errorf("AIContributedConfidence %g must be less than total Confidence %g",
			g.AIContributedConfidence, g.Confidence)
	}
}
