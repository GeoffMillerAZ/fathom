package hypothesisranker_test

// Audit-driven tests — one test per confirmed finding.
// Each test was written RED before the fix was applied (TDD).

import (
	"math"
	"testing"

	hr "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/hypothesis-ranker"
)

// ---------------------------------------------------------------------------
// Finding 1 + 2: SpanRefs, Status, Author, Grade fields (traceability + grade-awareness)
// ---------------------------------------------------------------------------

// TestHypothesisSpanRefsField verifies that Hypothesis carries SpanRefs for
// traceability (Invariant 6: outputs traceable or drafts).
func TestHypothesisSpanRefsField(t *testing.T) {
	h := hr.Hypothesis{
		ID:        "h-trace",
		Statement: "Some claim",
		Support:   0.8,
		SpanRefs:  []string{"span-001", "span-002"},
	}
	if len(h.SpanRefs) != 2 {
		t.Fatalf("SpanRefs: want 2 entries, got %d", len(h.SpanRefs))
	}
	if h.SpanRefs[0] != "span-001" || h.SpanRefs[1] != "span-002" {
		t.Errorf("SpanRefs values unexpected: %v", h.SpanRefs)
	}
}

// TestHypothesisStatusField verifies Status field accepts known values.
func TestHypothesisStatusField(t *testing.T) {
	cases := []hr.HypothesisStatus{
		hr.StatusOpen,
		hr.StatusSupported,
		hr.StatusFalsified,
		hr.StatusClosed,
	}
	for _, s := range cases {
		h := hr.Hypothesis{ID: "h-s", Support: 0.5, Status: s}
		if h.Status != s {
			t.Errorf("Status round-trip failed: want %q, got %q", s, h.Status)
		}
	}
}

// TestHypothesisAuthorField verifies Author carries AI provenance metadata.
func TestHypothesisAuthorField(t *testing.T) {
	h := hr.Hypothesis{
		ID:      "h-ai",
		Support: 0.6,
		Author: &hr.HypothesisAuthor{
			Kind:       hr.AuthorKindAI,
			Model:      "claude-sonnet-4",
			PromptHash: "abc123",
		},
	}
	if h.Author == nil {
		t.Fatal("Author is nil")
	}
	if h.Author.Kind != hr.AuthorKindAI {
		t.Errorf("Author.Kind: want %q, got %q", hr.AuthorKindAI, h.Author.Kind)
	}
	if h.Author.Model != "claude-sonnet-4" {
		t.Errorf("Author.Model: want %q, got %q", "claude-sonnet-4", h.Author.Model)
	}
}

// TestHypothesisGradeTierField verifies GradeTier carries the epistemic tier
// derived from the supporting spans (Invariant 4: grade <= weakest tier).
func TestHypothesisGradeTierField(t *testing.T) {
	h := hr.Hypothesis{
		ID:        "h-grade",
		Support:   0.7,
		GradeTier: hr.GradeTierInferred,
	}
	if h.GradeTier != hr.GradeTierInferred {
		t.Errorf("GradeTier: want %q, got %q", hr.GradeTierInferred, h.GradeTier)
	}
}

// ---------------------------------------------------------------------------
// Finding 3: NaN comparator
// ---------------------------------------------------------------------------

// TestRankNaNSupportIsLowest verifies that hypotheses with NaN support are
// treated as lower than any real value and sorted deterministically.
func TestRankNaNSupportIsLowest(t *testing.T) {
	nan := math.NaN()
	input := []hr.Hypothesis{
		{ID: "h-nan", Support: nan},
		{ID: "h-mid", Support: 0.5},
		{ID: "h-low", Support: 0.1},
	}
	// NaN must be treated as the lowest value, i.e. sorted last.
	got := hr.Rank(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	// h-nan must be last.
	if got[2].ID != "h-nan" {
		t.Errorf("NaN hypothesis should be last; got order [%s, %s, %s]",
			got[0].ID, got[1].ID, got[2].ID)
	}
	// h-mid must be first (highest real Support).
	if got[0].ID != "h-mid" {
		t.Errorf("h-mid should be first; got %q", got[0].ID)
	}
}

// TestRankTwoNaNSupportDeterministic verifies that two NaN hypotheses are
// ordered deterministically (by ID tie-break, not by undefined float comparison).
func TestRankTwoNaNSupportDeterministic(t *testing.T) {
	nan := math.NaN()
	input := []hr.Hypothesis{
		{ID: "h-nan-zzz", Support: nan},
		{ID: "h-nan-aaa", Support: nan},
	}
	got := hr.Rank(input)
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	// Both NaN: tie-break by ID ascending.
	if got[0].ID != "h-nan-aaa" || got[1].ID != "h-nan-zzz" {
		t.Errorf("NaN tie-break should be ID ascending; got [%s, %s]", got[0].ID, got[1].ID)
	}
}

// ---------------------------------------------------------------------------
// Finding 4: Validate() — Support bounds and empty ID
// ---------------------------------------------------------------------------

// TestValidateHappyPath verifies Validate() passes for a well-formed hypothesis.
func TestValidateHappyPath(t *testing.T) {
	h := hr.Hypothesis{ID: "h-ok", Statement: "valid", Support: 0.5}
	if err := h.Validate(); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

// TestValidateEmptyID verifies Validate() rejects an empty ID.
func TestValidateEmptyID(t *testing.T) {
	h := hr.Hypothesis{ID: "", Support: 0.5}
	if err := h.Validate(); err == nil {
		t.Error("expected error for empty ID, got nil")
	}
}

// TestValidateSupportAboveOne verifies Validate() rejects Support > 1.
func TestValidateSupportAboveOne(t *testing.T) {
	h := hr.Hypothesis{ID: "h-hi", Support: 1.5}
	if err := h.Validate(); err == nil {
		t.Error("expected error for Support > 1, got nil")
	}
}

// TestValidateSupportBelowZero verifies Validate() rejects Support < 0.
func TestValidateSupportBelowZero(t *testing.T) {
	h := hr.Hypothesis{ID: "h-lo", Support: -0.1}
	if err := h.Validate(); err == nil {
		t.Error("expected error for Support < 0, got nil")
	}
}

// TestValidateSupportNaN verifies Validate() rejects NaN Support.
func TestValidateSupportNaN(t *testing.T) {
	h := hr.Hypothesis{ID: "h-nan", Support: math.NaN()}
	if err := h.Validate(); err == nil {
		t.Error("expected error for NaN Support, got nil")
	}
}

// TestValidateSupportBoundaries verifies Validate() accepts 0 and 1 exactly.
func TestValidateSupportBoundaries(t *testing.T) {
	for _, sup := range []float64{0.0, 1.0} {
		h := hr.Hypothesis{ID: "h-bound", Support: sup}
		if err := h.Validate(); err != nil {
			t.Errorf("Support=%v: expected no error, got: %v", sup, err)
		}
	}
}

// TestRankERejectsInvalidHypothesis verifies that RankE returns an error when
// any hypothesis fails Validate().
func TestRankERejectsInvalidHypothesis(t *testing.T) {
	input := []hr.Hypothesis{
		{ID: "h-ok", Support: 0.5},
		{ID: "", Support: 0.3}, // empty ID — invalid
	}
	_, err := hr.RankE(input)
	if err == nil {
		t.Error("RankE: expected error for empty ID hypothesis, got nil")
	}
}

// TestRankEHappyPath verifies RankE returns correct order for valid inputs.
func TestRankEHappyPath(t *testing.T) {
	input := []hr.Hypothesis{
		{ID: "h1", Support: 0.2},
		{ID: "h2", Support: 0.9},
	}
	got, err := hr.RankE(input)
	if err != nil {
		t.Fatalf("RankE: unexpected error: %v", err)
	}
	if len(got) != 2 || got[0].ID != "h2" {
		t.Errorf("RankE: unexpected order: %v", got)
	}
}

// TestSpanRefsCopiedNotShared verifies that SpanRefs in returned Hypothesis
// values are defensive copies (mutating the returned slice does not affect
// the original).
func TestSpanRefsCopiedNotShared(t *testing.T) {
	input := []hr.Hypothesis{
		{ID: "h1", Support: 0.9, SpanRefs: []string{"s1", "s2"}},
		{ID: "h2", Support: 0.5, SpanRefs: []string{"s3"}},
	}
	got := hr.Rank(input)
	if len(got) == 0 {
		t.Fatal("expected non-empty result")
	}
	// Mutate returned SpanRefs — original must be unaffected.
	if len(got[0].SpanRefs) > 0 {
		got[0].SpanRefs[0] = "MUTATED"
	}
	if len(input[0].SpanRefs) > 0 && input[0].SpanRefs[0] == "MUTATED" {
		t.Error("Rank returned SpanRefs slice shares backing array with input")
	}
}
