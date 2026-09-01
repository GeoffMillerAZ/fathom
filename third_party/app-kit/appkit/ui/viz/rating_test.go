package viz

import (
	"strings"
	"testing"
)

func TestRating_Golden(t *testing.T) {
	got := Rating(4, RatingOptions{Max: 5, Graded: true, AriaLabel: "evidence strength 4 of 5"})
	assertGolden(t, got, "rating_default")
}

func TestRating_ClampAndCount(t *testing.T) {
	// Over/under clamp; correct number of pips.
	got := Rating(9, RatingOptions{Max: 5})
	if c := strings.Count(got, "<circle"); c != 5 {
		t.Errorf("expected 5 pips, got %d", c)
	}
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") {
		t.Errorf("rating has NaN/Inf")
	}
	neg := Rating(-3, RatingOptions{Max: 4})
	if strings.Count(neg, "<circle") != 4 {
		t.Errorf("negative rating should still render all track pips")
	}
}

func TestRating_GradedColors(t *testing.T) {
	strong := Rating(5, RatingOptions{Max: 5, Graded: true})
	if !strings.Contains(strong, "ak-color-positive") {
		t.Errorf("full rating should be positive")
	}
	weak := Rating(1, RatingOptions{Max: 5, Graded: true})
	if !strings.Contains(weak, "ak-color-danger") {
		t.Errorf("low rating should be danger")
	}
}

func TestRating_DefaultAriaAndA11y(t *testing.T) {
	got := Rating(3, RatingOptions{Max: 5})
	if !strings.Contains(got, "3 of 5") {
		t.Errorf("default aria should be 'N of M': %s", got)
	}
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, "<title>") {
		t.Errorf("rating missing accessibility")
	}
}

func TestRating_Determinism(t *testing.T) {
	a := Rating(3, RatingOptions{Max: 5, Graded: true})
	b := Rating(3, RatingOptions{Max: 5, Graded: true})
	if a != b {
		t.Errorf("rating non-deterministic")
	}
}
