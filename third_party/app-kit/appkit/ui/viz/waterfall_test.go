package viz

import (
	"strings"
	"testing"
)

func waterfallFixture() []WaterfallStep {
	return []WaterfallStep{
		{Label: "habitability", Delta: 5200},
		{Label: "deposit", Delta: 2400},
		{Label: "overlap", Delta: -1800},
		{Label: "net", Total: true},
		{Label: "fees", Delta: -240},
		{Label: "recovery", Total: true},
	}
}

func TestWaterfall_Golden(t *testing.T) {
	got := Waterfall(waterfallFixture(), WaterfallOptions{Prefix: "$", AriaLabel: "combined recovery waterfall"})
	assertGolden(t, got, "waterfall_default")
}

func TestWaterfall_Empty(t *testing.T) {
	got := Waterfall(nil, WaterfallOptions{})
	if !strings.Contains(got, "<svg") {
		t.Errorf("empty waterfall missing svg")
	}
	if strings.Contains(got, "<rect") {
		t.Errorf("empty waterfall should render no bars")
	}
}

func TestWaterfall_NegativeRunningSafe(t *testing.T) {
	got := Waterfall([]WaterfallStep{{Label: "a", Delta: 100}, {Label: "b", Delta: -300}, {Label: "net", Total: true}}, WaterfallOptions{})
	if strings.Contains(got, "NaN") || strings.Contains(got, "Inf") || strings.Contains(got, `height="-`) {
		t.Errorf("negative-running waterfall produced bad output: %s", got)
	}
}

func TestWaterfall_RoleBySign(t *testing.T) {
	got := Waterfall([]WaterfallStep{{Label: "up", Delta: 10}, {Label: "down", Delta: -5}, {Label: "t", Total: true}}, WaterfallOptions{})
	if !strings.Contains(got, "ak-color-positive") {
		t.Errorf("up delta should be positive role")
	}
	if !strings.Contains(got, "ak-color-danger") {
		t.Errorf("down delta should be danger role")
	}
	if !strings.Contains(got, "ak-color-accent") {
		t.Errorf("total should be accent role")
	}
}

func TestWaterfall_Escaping(t *testing.T) {
	got := Waterfall([]WaterfallStep{{Label: `<script>`, Delta: 1}}, WaterfallOptions{Suffix: `<b>`})
	for _, bad := range []string{"<script>", "<b>"} {
		if strings.Contains(got, bad) {
			t.Errorf("waterfall did not escape %q", bad)
		}
	}
}

func TestWaterfall_Determinism(t *testing.T) {
	a := Waterfall(waterfallFixture(), WaterfallOptions{Prefix: "$"})
	b := Waterfall(waterfallFixture(), WaterfallOptions{Prefix: "$"})
	if a != b {
		t.Errorf("waterfall non-deterministic")
	}
}

func TestWaterfall_Accessibility(t *testing.T) {
	got := Waterfall(waterfallFixture(), WaterfallOptions{AriaLabel: "wf"})
	if !strings.Contains(got, `role="img"`) || !strings.Contains(got, "<title>") {
		t.Errorf("waterfall missing accessibility")
	}
}
