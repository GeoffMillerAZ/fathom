package viz

import (
	"regexp"
	"strings"
	"testing"
	"time"
)

// fluidPairs renders every chart that takes a fixed pixel width (W, or Size for
// Donut) twice: with the default fixed width and with Fluid set. Heatmap,
// Rating and LayoutGraph size themselves from their content and have no W.
func fluidPairs() map[string][2]string {
	segs := []Segment{{Value: 3, Label: "a"}, {Value: 1, Label: "b", Role: "caution"}}
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []Event{{Label: "a", At: t0}, {Label: "b", At: t0.Add(3 * time.Hour), Lane: "x"}}
	pair := func(render func(fluid bool) string) [2]string { return [2]string{render(false), render(true)} }
	return map[string][2]string{
		"Sparkline": pair(func(f bool) string { return Sparkline([]float64{1, 4, 2, 8}, SparklineOptions{W: 240, Fluid: f}) }),
		"StackBar":  pair(func(f bool) string { return StackBar(segs, StackBarOptions{Fluid: f}) }),
		"Bars": pair(func(f bool) string {
			return Bars([]Bar{{Label: "a", Value: 3}, {Label: "b", Value: 7}}, BarsOptions{W: 560, ShowValue: true, Fluid: f})
		}),
		"BarsEmpty": pair(func(f bool) string { return Bars(nil, BarsOptions{Fluid: f}) }),
		"Donut":     pair(func(f bool) string { return Donut(segs, DonutOptions{Fluid: f}) }),
		"Funnel": pair(func(f bool) string {
			return Funnel([]Stage{{Label: "in", Value: 10}, {Label: "out", Value: 4}}, FunnelOptions{Fluid: f})
		}),
		"FunnelEmpty": pair(func(f bool) string { return Funnel(nil, FunnelOptions{Fluid: f}) }),
		"MeterRow":    pair(func(f bool) string { return MeterRow(3, 10, MeterOptions{Fluid: f}) }),
		"Gauge":       pair(func(f bool) string { return Gauge(7, 10, GaugeOptions{Fluid: f}) }),
		"Steps": pair(func(f bool) string {
			return Steps([]Step{{Label: "a", State: StepDone}, {Label: "b", State: StepCurrent}}, StepsOptions{Fluid: f})
		}),
		"RangeBars": pair(func(f bool) string {
			return RangeBars([]RangeItem{{Label: "a", Low: 1, High: 5, Point: 3, HasPoint: true}}, RangeBarsOptions{Fluid: f})
		}),
		"Waterfall": pair(func(f bool) string {
			return Waterfall([]WaterfallStep{{Label: "a", Delta: 5}, {Label: "b", Delta: -2}}, WaterfallOptions{Fluid: f})
		}),
		"CostReward": pair(func(f bool) string {
			return CostReward([]CostRewardItem{{Label: "a", Reward: 5, Cost: 2}}, CostRewardOptions{Fluid: f})
		}),
		"Tornado": pair(func(f bool) string {
			return Tornado([]TornadoItem{{Label: "a", Low: -3, High: 4}}, TornadoOptions{Fluid: f})
		}),
		"Timeline": pair(func(f bool) string {
			return Timeline(events[:1], TimelineOptions{W: 900, Fluid: f, Spans: []Span{{Label: "s", Start: t0, End: t0.Add(2 * time.Hour)}}})
		}),
		"TimelineSwimlanes": pair(func(f bool) string { return Timeline(events, TimelineOptions{Fluid: f}) }),
		"TimelineDensity": pair(func(f bool) string {
			return Timeline(events, TimelineOptions{View: ViewDensity, Fluid: f})
		}),
	}
}

var (
	svgOpenTag  = regexp.MustCompile(`^<svg [^>]*>`)
	fixedSize   = regexp.MustCompile(` width="(\d+)" height="(\d+)" `)
	viewBoxAttr = regexp.MustCompile(` viewBox="0 0 (\d+) (\d+)"`)
)

// FATHOM-F0-KIT-04: Fluid charts fill their container's width. The opening tag
// replaces the fixed width/height with width="100%" (no height attribute, so
// the browser derives it from the unchanged viewBox) and pins the drawing to
// the top-left when a consumer constrains the height; the drawing itself is
// byte-identical, so Fluid changes size, never content.
func TestFluid_SameDrawingWidthFollowsContainer(t *testing.T) {
	for name, p := range fluidPairs() {
		t.Run(name, func(t *testing.T) {
			fixed, fluid := p[0], p[1]
			fixedTag, fluidTag := svgOpenTag.FindString(fixed), svgOpenTag.FindString(fluid)
			if fixedTag == "" || fluidTag == "" {
				t.Fatalf("no <svg> opening tag:\nfixed %.80q\nfluid %.80q", fixed, fluid)
			}
			size := fixedSize.FindStringSubmatch(fixedTag)
			box := viewBoxAttr.FindStringSubmatch(fixedTag)
			if size == nil || box == nil || size[1] != box[1] || size[2] != box[2] {
				t.Fatalf("fixed chart must keep width/height equal to its viewBox: %s", fixedTag)
			}
			if strings.Contains(fixedTag, "preserveAspectRatio") || strings.Contains(fixedTag, "100%") {
				t.Errorf("default (fixed) output changed: %s", fixedTag)
			}
			if !strings.Contains(fluidTag, ` width="100%" `) {
				t.Errorf("fluid tag lacks width=\"100%%\": %s", fluidTag)
			}
			if strings.Contains(fluidTag, " height=") {
				t.Errorf("fluid tag must not fix a height (the viewBox ratio sets it): %s", fluidTag)
			}
			if got := viewBoxAttr.FindString(fluidTag); got != box[0] {
				t.Errorf("fluid viewBox %q, want the fixed chart's %q", got, box[0])
			}
			if !strings.HasSuffix(fluidTag, ` preserveAspectRatio="xMinYMin meet">`) {
				t.Errorf("fluid tag must end with preserveAspectRatio=\"xMinYMin meet\": %s", fluidTag)
			}
			for _, attr := range []string{`role="img"`, `aria-label="`, `xmlns="http://www.w3.org/2000/svg"`} {
				if !strings.Contains(fluidTag, attr) {
					t.Errorf("fluid tag lost %s: %s", attr, fluidTag)
				}
			}
			if fixed[len(fixedTag):] != fluid[len(fluidTag):] {
				t.Errorf("fluid drawing differs from the fixed drawing after the opening tag")
			}
			if strings.Contains(fluid, "class=") || strings.Contains(fluid, "style=") {
				t.Errorf("fluid output must stay L3a (no class, no inline style for strict CSPs)")
			}
		})
	}
}

// Fluid is opt-in: the zero value renders the pre-change bytes, which the
// existing goldens (frozen before Fluid existed) already pin for every chart.
// These goldens pin the fluid tag itself for a text-bearing chart and a
// text-free one.
func TestFluid_Goldens(t *testing.T) {
	assertGolden(t, Bars([]Bar{
		{Label: "find_symbol", Value: 1204, Role: "accent"},
		{Label: "references", Value: 311, Role: "accent"},
		{Label: "symbol_context", Value: 87, Role: "accent"},
	}, BarsOptions{W: 560, LabelW: 120, ShowValue: true, AriaLabel: "tool calls", Fluid: true}), "bars_fluid")
	assertGolden(t, Sparkline([]float64{3, 5, 4, 8, 6, 9}, SparklineOptions{W: 240, H: 32, FillRole: "accent", Fluid: true}), "sparkline_fluid")
}
