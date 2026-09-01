package viz

import (
	"strings"
	"testing"
)

func barsFixture() []Bar {
	return []Bar{
		{Label: "favorable", Value: 55, Role: "positive", Suffix: "%"},
		{Label: "mixed", Value: 30, Role: "caution", Suffix: "%"},
		{Label: "unfavorable", Value: 15, Role: "danger", Suffix: "%"},
	}
}

func TestBars_Golden(t *testing.T) {
	got := Bars(barsFixture(), BarsOptions{Max: 100, ShowValue: true, AriaLabel: "expected outcome"})
	assertGolden(t, got, "bars_default")
}

func TestBars_Empty(t *testing.T) {
	got := Bars(nil, BarsOptions{})
	if !strings.Contains(got, "<svg") {
		t.Errorf("empty bars missing svg")
	}
	if strings.Contains(got, "<rect") {
		t.Errorf("empty bars should render no rows")
	}
}

func TestBars_NegativeClamps(t *testing.T) {
	got := Bars([]Bar{{Label: "x", Value: -10, Role: "accent"}}, BarsOptions{Max: 100})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") || strings.Contains(got, `width="-`) {
		t.Errorf("negative bar produced bad width: %s", got)
	}
}

func TestBars_AutoMax(t *testing.T) {
	// Largest value should reach (near) full bar width; just ensure no panic and
	// a fill rect exists.
	got := Bars(barsFixture(), BarsOptions{}) // Max auto
	if strings.Count(got, "<rect") < 3 {
		t.Errorf("expected fill rects for bars")
	}
}

func TestBars_Escaping(t *testing.T) {
	got := Bars([]Bar{{Label: `<script>`, Value: 1, Role: "accent", Suffix: `<b>`}}, BarsOptions{ShowValue: true})
	for _, bad := range []string{"<script>", "<b>"} {
		if strings.Contains(got, bad) {
			t.Errorf("bars did not escape %q", bad)
		}
	}
}

func TestBars_InvalidRoleFallsBack(t *testing.T) {
	got := Bars([]Bar{{Label: "x", Value: 1, Role: "BAD ROLE"}}, BarsOptions{})
	if strings.Contains(got, "BAD ROLE") {
		t.Errorf("bars injected invalid role")
	}
}

func TestBars_Determinism(t *testing.T) {
	a := Bars(barsFixture(), BarsOptions{Max: 100, ShowValue: true})
	b := Bars(barsFixture(), BarsOptions{Max: 100, ShowValue: true})
	if a != b {
		t.Errorf("bars non-deterministic")
	}
}

func TestBars_GroupedValueLabels(t *testing.T) {
	got := Bars([]Bar{
		{Label: "big", Value: 7608, Role: "accent"},
		{Label: "small", Value: 42, Role: "accent"},
	}, BarsOptions{ShowValue: true})
	if !strings.Contains(got, "7,608") {
		t.Errorf("expected grouped value 7,608, got: %s", got)
	}
	if !strings.Contains(got, "42") || strings.Contains(got, "4,2") {
		t.Errorf("expected ungrouped value 42, got: %s", got)
	}
}

func TestBars_Accessibility(t *testing.T) {
	got := Bars(barsFixture(), BarsOptions{AriaLabel: "outcome"})
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, "<title>") {
		t.Errorf("bars missing accessibility")
	}
}
