package hypothesisranker_test

import (
	"testing"

	hr "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/investigation/hypothesis-ranker"
)

// TestRankBySupport ensures hypotheses are sorted by Support descending.
func TestRankBySupport(t *testing.T) {
	input := []hr.Hypothesis{
		{ID: "h1", Statement: "low support", Support: 0.2},
		{ID: "h2", Statement: "high support", Support: 0.9},
		{ID: "h3", Statement: "mid support", Support: 0.5},
	}
	got := hr.Rank(input)
	wantOrder := []string{"h2", "h3", "h1"}
	if len(got) != len(wantOrder) {
		t.Fatalf("expected %d hypotheses, got %d", len(wantOrder), len(got))
	}
	for i, id := range wantOrder {
		if got[i].ID != id {
			t.Errorf("rank[%d]: want ID %q, got %q", i, id, got[i].ID)
		}
	}
}

// TestRankTieBreakByID ensures ties in Support are broken by ID ascending.
func TestRankTieBreakByID(t *testing.T) {
	input := []hr.Hypothesis{
		{ID: "h-zzz", Support: 0.7},
		{ID: "h-aaa", Support: 0.7},
		{ID: "h-mmm", Support: 0.7},
	}
	got := hr.Rank(input)
	wantOrder := []string{"h-aaa", "h-mmm", "h-zzz"}
	for i, id := range wantOrder {
		if got[i].ID != id {
			t.Errorf("rank[%d]: want ID %q, got %q", i, id, got[i].ID)
		}
	}
}

// TestRankDeterministic ensures the same input produces identical output on
// repeated calls.
func TestRankDeterministic(t *testing.T) {
	input := []hr.Hypothesis{
		{ID: "h3", Support: 0.3},
		{ID: "h1", Support: 0.9},
		{ID: "h2", Support: 0.9},
	}
	first := hr.Rank(input)
	second := hr.Rank(input)
	if len(first) != len(second) {
		t.Fatalf("length mismatch: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Errorf("rank[%d] differs between calls: %q vs %q", i, first[i].ID, second[i].ID)
		}
	}
}

// TestRankDoesNotMutateInput ensures the original slice is not modified.
func TestRankDoesNotMutateInput(t *testing.T) {
	input := []hr.Hypothesis{
		{ID: "h1", Support: 0.1},
		{ID: "h2", Support: 0.9},
	}
	originalFirst := input[0].ID
	hr.Rank(input)
	if input[0].ID != originalFirst {
		t.Error("Rank mutated the input slice")
	}
}

// TestRankEmpty ensures nil is returned for an empty input.
func TestRankEmpty(t *testing.T) {
	got := hr.Rank(nil)
	if got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
	got = hr.Rank([]hr.Hypothesis{})
	if got != nil {
		t.Errorf("expected nil for empty slice, got %v", got)
	}
}

// TestRankSingle ensures a single hypothesis is returned unchanged.
func TestRankSingle(t *testing.T) {
	h := hr.Hypothesis{ID: "only", Support: 0.5}
	got := hr.Rank([]hr.Hypothesis{h})
	if len(got) != 1 || got[0].ID != "only" {
		t.Errorf("unexpected result for single hypothesis: %v", got)
	}
}
