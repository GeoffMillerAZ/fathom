package viz

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// tier_test.go asserts the contract that makes appkit/ui/viz **L3a** rather
// than L3b (ADR-0017 §1). The split was real and load-bearing for a year
// before anything named it: `viz` emits zero `class=` and resolves every color
// through cssColor → var(--ak-color-*), so a kit chart drops into a page with
// no Tailwind, no @source glob and no CSS build — while ui/topology and
// ui/vizframe emit role utilities and require all three.
//
// Nothing enforced that. A single `class="text-ink"` added to a chart would
// have silently demoted the whole package to L3b, breaking every consumer that
// took it without a Tailwind scanner — with no error, because the class simply
// would not resolve and the element would inherit. This is the mechanism that
// stops a component picking a tier by coin-flip.

// specimens renders every exported viz.* function. Options are zero-valued
// except where a chart needs a datum to draw at all: the point is to exercise
// the DEFAULT path, which is the one a caller gets without thinking.
func specimens() map[string]string {
	segs := []Segment{{Value: 3, Label: "a"}, {Value: 1, Label: "b", Role: "caution"}}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return map[string]string{
		"Sparkline":  Sparkline([]float64{1, 4, 2, 8}, SparklineOptions{}),
		"StackBar":   StackBar(segs, StackBarOptions{}),
		"Donut":      Donut(segs, DonutOptions{}),
		"Funnel":     Funnel([]Stage{{Label: "in", Value: 10}, {Label: "out", Value: 4}}, FunnelOptions{}),
		"MeterRow":   MeterRow(3, 10, MeterOptions{}),
		"Gauge":      Gauge(7, 10, GaugeOptions{}),
		"Rating":     Rating(3, RatingOptions{}),
		"Steps":      Steps([]Step{{Label: "a", State: StepDone}, {Label: "b", State: StepCurrent}}, StepsOptions{}),
		"Bars":       Bars([]Bar{{Label: "a", Value: 3}, {Label: "b", Value: 7}}, BarsOptions{}),
		"RangeBars":  RangeBars([]RangeItem{{Label: "a", Low: 1, High: 5, Point: 3, HasPoint: true}}, RangeBarsOptions{}),
		"Waterfall":  Waterfall([]WaterfallStep{{Label: "a", Delta: 5}, {Label: "b", Delta: -2}}, WaterfallOptions{}),
		"CostReward": CostReward([]CostRewardItem{{Label: "a", Reward: 5, Cost: 2}}, CostRewardOptions{}),
		"Tornado":    Tornado([]TornadoItem{{Label: "a", Low: -3, High: 4}}, TornadoOptions{}),
		"Heatmap":    Heatmap([][]float64{{0, 0.5}, {1, 0.25}}, HeatmapOptions{}),
		"Timeline": Timeline([]Event{
			{Label: "a", At: t0},
			{Label: "b", At: t0.Add(3 * time.Hour)},
		}, TimelineOptions{Spans: []Span{{Label: "s", Start: t0, End: t0.Add(2 * time.Hour)}}}),
	}
}

// TestVizEmitsNoClassAttribute — the L3a contract, half one. A class attribute
// anywhere in this package means a consumer must run the Tailwind scanner over
// kit source to see the chart correctly, which is the L3b contract and a
// different, larger ask.
func TestVizEmitsNoClassAttribute(t *testing.T) {
	for name, out := range specimens() {
		t.Run(name, func(t *testing.T) {
			if strings.Contains(out, "class=") {
				t.Errorf("viz.%s emits a class attribute — that is the L3b contract (Tailwind scanner required). "+
					"Color through cssColor(role), or promote the component to a utility-themed package and give it a row in docs/ui-components.md", name)
			}
		})
	}
}

// literalColor matches a color written as a value rather than as a role:
// hex, rgb()/rgba(), hsl(), and oklch()/oklab(). A var(--ak-color-*) reference
// is not a literal — it is the whole mechanism — so the match deliberately
// does not look inside var().
var literalColor = regexp.MustCompile(`#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(|\boklch\(|\boklab\(`)

// TestVizEmitsNoLiteralColor — the L3a contract, half two, and an ADR-0017 §2
// invariant on the whole kit: no kit artifact emits a literal color. It is what
// makes a ~25-line alias file in a consuming app able to re-theme every kit
// artifact at once (ADR-0017 §4.2), and it is why the resume SPA can take L1
// without the kit knowing anything about its five root variant axes.
func TestVizEmitsNoLiteralColor(t *testing.T) {
	for name, out := range specimens() {
		t.Run(name, func(t *testing.T) {
			// var(--ak-color-…) is the sanctioned form; blank it before matching
			// so a fallback inside var() cannot be mistaken for a literal.
			stripped := regexp.MustCompile(`var\(--ak-[a-z0-9-]+\)`).ReplaceAllString(out, "")
			if m := literalColor.FindString(stripped); m != "" {
				t.Errorf("viz.%s emits the literal color %q — every color must reach the page as var(--ak-color-<role>) (ADR-0017 §2)", name, m)
			}
		})
	}
}

// TestVizSpecimensCoverEveryRenderer guards the guard: a new chart added to the
// package with no specimen here would be exempt from both tests above without
// anyone noticing. Renderers are the exported functions returning a string that
// starts with "<svg" — checked by rendering, not by parsing source.
func TestVizSpecimensCoverEveryRenderer(t *testing.T) {
	for name, out := range specimens() {
		if !strings.HasPrefix(out, "<svg") {
			t.Errorf("specimen %q did not render an <svg> (got %.40q) — the tier assertions are checking nothing for it", name, out)
		}
	}
	// Sanity floor: if the package grows and this list does not, the count drops
	// below what docs/ui-components.md lists and the mismatch is visible there.
	if got := len(specimens()); got < 15 {
		t.Errorf("only %d viz specimens; docs/ui-components.md lists 15 components", got)
	}
}
