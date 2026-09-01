package viz

import (
	"strings"
	"testing"
)

func rangeFixture() []RangeItem {
	return []RangeItem{
		{Label: "habitability", Low: 2400, High: 9600, Point: 5200, HasPoint: true, Role: "positive", Prefix: "$"},
		{Label: "deposit", Low: 1200, High: 3600, Point: 2400, HasPoint: true, Role: "accent", Prefix: "$"},
		{Label: "fees", Low: 0, High: 1500, Role: "caution", Prefix: "$"},
	}
}

func TestRangeBars_Golden(t *testing.T) {
	got := RangeBars(rangeFixture(), RangeBarsOptions{Max: 10000, AriaLabel: "recovery ranges"})
	assertGolden(t, got, "rangebars_default")
}

func TestRangeBars_Empty(t *testing.T) {
	got := RangeBars(nil, RangeBarsOptions{})
	if !strings.Contains(got, "<svg") {
		t.Errorf("empty rangebars missing svg")
	}
	if strings.Contains(got, "<rect") {
		t.Errorf("empty rangebars should render no rows")
	}
}

func TestRangeBars_InvertedAndNegativeSafe(t *testing.T) {
	got := RangeBars([]RangeItem{{Label: "x", Low: 80, High: 20, Role: "accent"}, {Label: "y", Low: -10, High: 50, Role: "info"}}, RangeBarsOptions{Max: 100})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") || strings.Contains(got, `width="-`) {
		t.Errorf("inverted/negative range produced bad output: %s", got)
	}
}

func TestRangeBars_PointMarker(t *testing.T) {
	with := RangeBars([]RangeItem{{Label: "a", Low: 0, High: 10, Point: 5, HasPoint: true, Role: "accent"}}, RangeBarsOptions{Max: 10})
	if !strings.Contains(with, "most likely") || !strings.Contains(with, "<line") {
		t.Errorf("point marker not rendered: %s", with)
	}
	without := RangeBars([]RangeItem{{Label: "a", Low: 0, High: 10, Role: "accent"}}, RangeBarsOptions{Max: 10})
	if strings.Contains(without, "<line") {
		t.Errorf("no point marker expected when HasPoint=false")
	}
}

func TestRangeBars_HideRange(t *testing.T) {
	shown := RangeBars(rangeFixture(), RangeBarsOptions{Max: 10000})
	hidden := RangeBars(rangeFixture(), RangeBarsOptions{Max: 10000, HideRange: true})
	// Shown has more text nodes (range labels) than hidden.
	if strings.Count(shown, "<text") <= strings.Count(hidden, "<text") {
		t.Errorf("HideRange should drop the value labels")
	}
}

func TestRangeBars_Escaping(t *testing.T) {
	got := RangeBars([]RangeItem{{Label: `<script>`, Low: 1, High: 2, Role: "accent", Suffix: `<b>`}}, RangeBarsOptions{Max: 2})
	for _, bad := range []string{"<script>", "<b>"} {
		if strings.Contains(got, bad) {
			t.Errorf("rangebars did not escape %q", bad)
		}
	}
}

func TestRangeBars_InvalidRoleFallsBack(t *testing.T) {
	got := RangeBars([]RangeItem{{Label: "x", Low: 1, High: 2, Role: "BAD ROLE"}}, RangeBarsOptions{Max: 2})
	if strings.Contains(got, "BAD ROLE") {
		t.Errorf("rangebars injected invalid role")
	}
}

func TestRangeBars_Determinism(t *testing.T) {
	a := RangeBars(rangeFixture(), RangeBarsOptions{Max: 10000})
	b := RangeBars(rangeFixture(), RangeBarsOptions{Max: 10000})
	if a != b {
		t.Errorf("rangebars non-deterministic")
	}
}

func TestRangeBars_Accessibility(t *testing.T) {
	got := RangeBars(rangeFixture(), RangeBarsOptions{AriaLabel: "ranges"})
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, "<title>") {
		t.Errorf("rangebars missing accessibility")
	}
}
