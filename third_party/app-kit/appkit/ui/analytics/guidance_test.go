package analytics

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

func TestNegativeBarsRecommendExistingSignedRenderer(t *testing.T) {
	d := signedFixture()
	h, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if h != "" || err == nil || !strings.Contains(err.Error(), "analytics.RenderDivergingBars") {
		t.Fatalf("negative refusal must direct adopters to the existing signed renderer: %q, %v", h, err)
	}
	if _, err := RenderDivergingBars(d, viz.BarsOptions{}, testNow); err != nil {
		t.Fatalf("recommended renderer rejects this otherwise valid selection: %v", err)
	}
}

func TestDivergingSubpixelGuidancePreservesData(t *testing.T) {
	for _, tiny := range []float64{-.01, .01} {
		d := fixture()
		d.Rows = []Row{{ID: "tiny", Label: "Tiny", Value: value(tiny)}, {ID: "large", Label: "Large", Value: value(MaxMagnitude)}}
		d.Legend = []LegendItem{{Role: "accent", Label: "Observed amounts"}}
		h, err := RenderDivergingBars(d, viz.BarsOptions{W: 2000}, testNow)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(h, "nonzero values would render smaller than one logical pixel") || !strings.Contains(h, "Choose a narrower selection") || strings.Contains(h, "48 pixels") {
			t.Errorf("wrong reason or remedy for subpixel signed value %v: %s", tiny, h)
		}
		if strings.Contains(h, "<svg") || strings.Contains(h, `aria-label="Chart legend"`) || !strings.Contains(h, renderTable(d)) {
			t.Fatal("fallback added marks/legend or changed the complete table")
		}
		rows, _ := dataCSV(t, d)
		want := "0.01"
		if tiny < 0 {
			want = "-0.01"
		}
		if len(rows) != 2 || rows[0][2] != want || rows[1][2] != "1000000000000" || rows[0][9] != "Observed amounts" {
			t.Fatalf("signed fallback lost CSV meaning: %v", rows)
		}
	}
	// An actual label-space failure keeps its own remedy, even if a mark would
	// also be tiny. Ordinary signed geometry remains a chart.
	d := signedFixture()
	d.Rows[0].Label = strings.Repeat("Long", 100)
	h, err := RenderDivergingBars(d, viz.BarsOptions{}, testNow)
	if err != nil || !strings.Contains(h, "48 pixels") || strings.Contains(h, "one logical pixel") {
		t.Fatal("label-space guidance was replaced", err)
	}
	h, err = RenderDivergingBars(signedFixture(), viz.BarsOptions{}, testNow)
	if err != nil || !strings.Contains(h, `data-analytics-diverging="true"`) || strings.Contains(h, "Bar chart omitted:") {
		t.Fatal("ordinary signed geometry changed", err)
	}
}
