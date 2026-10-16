package elementcoverage_test

import (
	"strings"
	"testing"

	evidencegrade "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/evidence/evidence-grade"
	elementcoverage "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/element-coverage"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// noPromote is a PromotionRule that disables tier promotion.
var noPromote = evidencegrade.PromotionRule{AllowTierPromotion: false}

// span builds a minimal necessary GradedSpan.
func span(id string, tier evidencegrade.Tier, confidence float64) evidencegrade.GradedSpan {
	return evidencegrade.GradedSpan{
		SpanID:     id,
		Tier:       tier,
		Confidence: confidence,
		Necessary:  true,
	}
}

// findCoverage returns the ElementCoverage for the given element ID or fails.
func findCoverage(t *testing.T, results []elementcoverage.ElementCoverage, elemID string) elementcoverage.ElementCoverage {
	t.Helper()
	for _, ec := range results {
		if ec.ElementID == elemID {
			return ec
		}
	}
	t.Fatalf("no ElementCoverage found for element %q", elemID)
	return elementcoverage.ElementCoverage{}
}

// ---------------------------------------------------------------------------
// Assess — all-met theory
// ---------------------------------------------------------------------------

func TestAssessAllMet(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "breach", Name: "Breach of duty", Required: true},
		{ID: "damages", Name: "Damages", Required: true},
	}
	evidenceByElement := map[string][]evidencegrade.GradedSpan{
		"breach":  {span("s1", evidencegrade.TierDocumentary, 0.9)},
		"damages": {span("s2", evidencegrade.TierDocumentary, 0.85)},
	}
	// requiredTier = Documentary, minConfidence = 0.8
	results, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierDocumentary, 0.8, noPromote)
	if err != nil {
		t.Fatalf("Assess: unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Assess: got %d results, want 2", len(results))
	}

	breach := findCoverage(t, results, "breach")
	if breach.Status != elementcoverage.StatusMet {
		t.Errorf("breach.Status = %q, want %q", breach.Status, elementcoverage.StatusMet)
	}
	if breach.Gap != "" {
		t.Errorf("breach.Gap should be empty when met, got %q", breach.Gap)
	}

	damages := findCoverage(t, results, "damages")
	if damages.Status != elementcoverage.StatusMet {
		t.Errorf("damages.Status = %q, want %q", damages.Status, elementcoverage.StatusMet)
	}
}

// ---------------------------------------------------------------------------
// Assess — under_evidenced (inferred only, below required tier)
// ---------------------------------------------------------------------------

func TestAssessUnderEvidenced(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "intent", Name: "Intent to defraud", Required: true},
	}
	evidenceByElement := map[string][]evidencegrade.GradedSpan{
		"intent": {span("s3", evidencegrade.TierInferred, 0.7)},
	}
	// requiredTier = Circumstantial — inferred is below that
	results, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierCircumstantial, 0.5, noPromote)
	if err != nil {
		t.Fatalf("Assess: unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Assess: got %d results, want 1", len(results))
	}

	ec := results[0]
	if ec.Status != elementcoverage.StatusUnderEvidenced {
		t.Errorf("Status = %q, want %q", ec.Status, elementcoverage.StatusUnderEvidenced)
	}
	if ec.Gap == "" {
		t.Error("Gap should be non-empty for under_evidenced element")
	}
	// Gap must mention the element name or the weak tier.
	if !strings.Contains(ec.Gap, "Intent to defraud") && !strings.Contains(ec.Gap, "inferred") {
		t.Errorf("Gap %q does not mention element name or weak tier", ec.Gap)
	}
}

// ---------------------------------------------------------------------------
// Assess — unmet (no evidence at all)
// ---------------------------------------------------------------------------

func TestAssessUnmet(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "causation", Name: "Causation", Required: true},
	}
	// No evidence for "causation".
	evidenceByElement := map[string][]evidencegrade.GradedSpan{}

	results, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierDocumentary, 0.5, noPromote)
	if err != nil {
		t.Fatalf("Assess: unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Assess: got %d results, want 1", len(results))
	}

	ec := results[0]
	if ec.Status != elementcoverage.StatusUnmet {
		t.Errorf("Status = %q, want %q", ec.Status, elementcoverage.StatusUnmet)
	}
	if ec.Gap == "" {
		t.Error("Gap should be non-empty for unmet element")
	}
	if !strings.Contains(ec.Gap, "Causation") {
		t.Errorf("Gap %q does not mention element name", ec.Gap)
	}
	if ec.WeakestTier != "" {
		t.Errorf("WeakestTier should be empty for unmet element with no evidence, got %q", ec.WeakestTier)
	}
}

// ---------------------------------------------------------------------------
// Assess — optional element with no evidence is still unmet but not required
// ---------------------------------------------------------------------------

func TestAssessOptionalElementNoEvidence(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "aggravating", Name: "Aggravating factor", Required: false},
	}
	evidenceByElement := map[string][]evidencegrade.GradedSpan{}

	results, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierDocumentary, 0.5, noPromote)
	if err != nil {
		t.Fatalf("Assess: unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Assess: got %d results, want 1", len(results))
	}

	ec := results[0]
	// Optional with no evidence is still unmet — it just isn't Required.
	if ec.Status != elementcoverage.StatusUnmet {
		t.Errorf("Status = %q, want %q", ec.Status, elementcoverage.StatusUnmet)
	}
}

// ---------------------------------------------------------------------------
// Assess — confidence below minimum triggers under_evidenced
// ---------------------------------------------------------------------------

func TestAssessConfidenceBelowMinimum(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "nexus", Name: "Causal nexus", Required: true},
	}
	evidenceByElement := map[string][]evidencegrade.GradedSpan{
		// Tier is strong enough but confidence is low.
		"nexus": {span("s5", evidencegrade.TierDocumentary, 0.3)},
	}
	results, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierDocumentary, 0.8, noPromote)
	if err != nil {
		t.Fatalf("Assess: unexpected error: %v", err)
	}

	ec := findCoverage(t, results, "nexus")
	if ec.Status != elementcoverage.StatusUnderEvidenced {
		t.Errorf("Status = %q, want under_evidenced (confidence too low)", ec.Status)
	}
}

// ---------------------------------------------------------------------------
// Assess — EvidenceSpanIDs are populated
// ---------------------------------------------------------------------------

func TestAssessEvidenceSpanIDsPopulated(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "duty", Name: "Duty of care", Required: true},
	}
	evidenceByElement := map[string][]evidencegrade.GradedSpan{
		"duty": {
			span("span-a", evidencegrade.TierDocumentary, 0.9),
			{SpanID: "span-b", Tier: evidencegrade.TierDocumentary, Confidence: 0.8, Independent: true},
		},
	}
	results, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierDocumentary, 0.5, noPromote)
	if err != nil {
		t.Fatalf("Assess: unexpected error: %v", err)
	}

	ec := findCoverage(t, results, "duty")
	if len(ec.EvidenceSpanIDs) != 2 {
		t.Errorf("EvidenceSpanIDs: got %d, want 2", len(ec.EvidenceSpanIDs))
	}
}

// ---------------------------------------------------------------------------
// Assess — deterministic output order matches element input order
// ---------------------------------------------------------------------------

func TestAssessDeterministicOrder(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "z-elem", Name: "Z element", Required: true},
		{ID: "a-elem", Name: "A element", Required: true},
		{ID: "m-elem", Name: "M element", Required: true},
	}
	evidenceByElement := map[string][]evidencegrade.GradedSpan{
		"z-elem": {span("sz", evidencegrade.TierDocumentary, 0.9)},
		"a-elem": {span("sa", evidencegrade.TierDocumentary, 0.9)},
		"m-elem": {span("sm", evidencegrade.TierDocumentary, 0.9)},
	}

	// Run twice and verify same order.
	r1, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierDocumentary, 0.5, noPromote)
	if err != nil {
		t.Fatalf("Assess (run 1): %v", err)
	}
	r2, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierDocumentary, 0.5, noPromote)
	if err != nil {
		t.Fatalf("Assess (run 2): %v", err)
	}

	wantOrder := []string{"z-elem", "a-elem", "m-elem"} // preserves input order
	for i, want := range wantOrder {
		if r1[i].ElementID != want || r2[i].ElementID != want {
			t.Errorf("position %d: r1=%q r2=%q, want %q", i, r1[i].ElementID, r2[i].ElementID, want)
		}
	}
}

// ---------------------------------------------------------------------------
// Assess — empty element list returns nil, no error
// ---------------------------------------------------------------------------

func TestAssessEmptyElements(t *testing.T) {
	results, err := elementcoverage.Assess(nil, nil, evidencegrade.TierDocumentary, 0.5, noPromote)
	if err != nil {
		t.Fatalf("Assess(nil): unexpected error: %v", err)
	}
	if results != nil {
		t.Errorf("Assess(nil): got non-nil results %v, want nil", results)
	}
}

// ---------------------------------------------------------------------------
// Assess — element with no evidence entry in map is unmet (no silent drop)
// ---------------------------------------------------------------------------

func TestAssessNoSilentDrop(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "e1", Name: "Element one", Required: true},
		{ID: "e2", Name: "Element two", Required: true},
	}
	// Only e1 has evidence.
	evidenceByElement := map[string][]evidencegrade.GradedSpan{
		"e1": {span("s1", evidencegrade.TierDocumentary, 0.9)},
	}
	results, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierDocumentary, 0.5, noPromote)
	if err != nil {
		t.Fatalf("Assess: unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Assess: got %d results, want 2 (no silent drop)", len(results))
	}
	e2 := findCoverage(t, results, "e2")
	if e2.Status != elementcoverage.StatusUnmet {
		t.Errorf("e2 Status = %q, want unmet", e2.Status)
	}
}

// ---------------------------------------------------------------------------
// Status — validate round-trip
// ---------------------------------------------------------------------------

func TestStatusValidate(t *testing.T) {
	for _, s := range []elementcoverage.Status{
		elementcoverage.StatusMet,
		elementcoverage.StatusUnderEvidenced,
		elementcoverage.StatusUnmet,
	} {
		if err := s.Validate(); err != nil {
			t.Errorf("Status %q: unexpected validation error: %v", s, err)
		}
	}
	if err := elementcoverage.Status("bogus").Validate(); err == nil {
		t.Error("Status \"bogus\": expected validation error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Element — validate
// ---------------------------------------------------------------------------

func TestElementValidate(t *testing.T) {
	good := elementcoverage.Element{ID: "e1", Name: "Foo", Required: true}
	if err := good.Validate(); err != nil {
		t.Errorf("valid Element: unexpected error: %v", err)
	}

	noID := elementcoverage.Element{Name: "Foo"}
	if err := noID.Validate(); err == nil {
		t.Error("Element with empty ID: expected error, got nil")
	}

	noName := elementcoverage.Element{ID: "e1"}
	if err := noName.Validate(); err == nil {
		t.Error("Element with empty Name: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Assess — WeakestTier reflects actual graded tier
// ---------------------------------------------------------------------------

func TestAssessWeakestTierPopulated(t *testing.T) {
	elements := []elementcoverage.Element{
		{ID: "knowledge", Name: "Knowledge of falsity", Required: true},
	}
	evidenceByElement := map[string][]evidencegrade.GradedSpan{
		// Two spans: one testimonial (weaker) + one documentary. WeakestTier = testimonial.
		"knowledge": {
			span("t1", evidencegrade.TierTestimonial, 0.8),
			span("t2", evidencegrade.TierDocumentary, 0.9),
		},
	}
	results, err := elementcoverage.Assess(elements, evidenceByElement, evidencegrade.TierInferred, 0.3, noPromote)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	ec := findCoverage(t, results, "knowledge")
	if ec.WeakestTier != evidencegrade.TierTestimonial {
		t.Errorf("WeakestTier = %q, want %q", ec.WeakestTier, evidencegrade.TierTestimonial)
	}
}
