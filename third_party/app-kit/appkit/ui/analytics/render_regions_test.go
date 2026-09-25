package analytics

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

func TestRenderedOverflowRegionsHaveNamesAndKeyboardFocus(t *testing.T) {
	for _, missing := range []bool{false, true} {
		d := fixture()
		if missing {
			d.Rows = nil
		}
		h, err := RenderBars(d, viz.BarsOptions{}, testNow)
		if err != nil {
			t.Fatal(err)
		}
		want := 2
		if missing {
			want = 1
		}
		if strings.Count(h, `tabindex="0" role="region" aria-label="`) != want {
			t.Fatalf("need %d named focusable regions: %s", want, h)
		}
		if strings.Count(h, "focus-visible:outline-2 focus-visible:outline-focus") != want {
			t.Fatal("scroll regions require explicit focus appearance")
		}
		if !strings.Contains(h, "data table, horizontally scrollable if needed") || (!missing && !strings.Contains(h, "bar chart, horizontally scrollable if needed")) {
			t.Fatal("chart and table need distinct descriptive accessible names")
		}
	}
}

func TestBarsLayoutFallbackKeepsDataInsteadOfZeroWidthOrClippedMarks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Dataset)
	}{
		{"long label", func(d *Dataset) { d.Rows[0].Label = strings.Repeat("Long service name ", 10) }},
		{"wide multilingual label", func(d *Dataset) { d.Rows[0].Label = strings.Repeat("資料", 20) }},
		{"large value and long units", func(d *Dataset) {
			d.Rows[0].Value = value(MaxMagnitude)
			d.Contract.Unit = "very long declared accounting units"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := fixture()
			tc.change(&d)
			h, err := RenderBars(d, viz.BarsOptions{}, testNow)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(h, "<svg") || !strings.Contains(h, "Bar chart omitted:") || strings.Contains(h, "No measured values") {
				t.Fatal("unreadable plot must become an explicitly explained table, not zero-width bars or a missing-data claim")
			}
			if !strings.Contains(h, d.Rows[0].Label) || !strings.Contains(h, d.Contract.Unit) || !strings.Contains(h, d.Contract.Source.Text) || !strings.Contains(h, `tabular-nums">`+number(*d.Rows[0].Value, d.Contract.Decimals, true)+"</td>") {
				t.Fatal("table fallback discarded selected value, label, unit or source")
			}
			rows, _ := dataCSV(t, d)
			if rows[0][1] != d.Rows[0].Label || rows[0][2] != number(*d.Rows[0].Value, d.Contract.Decimals, false) || rows[1][2] != "0" {
				t.Fatal("layout fallback changed CSV values or measured zero")
			}
		})
	}
}

func TestBarsWiderOptionsRecoverAPlottableSelection(t *testing.T) {
	d := fixture()
	d.Rows[0].Label = "A moderately longer service name"
	small, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil || strings.Contains(small, "<svg") {
		t.Fatal("default should select explicit table fallback")
	}
	wide, err := RenderBars(d, viz.BarsOptions{W: 1024, LabelW: 480}, testNow)
	if err != nil || !strings.Contains(wide, "<svg") || strings.Contains(wide, "Bar chart omitted:") {
		t.Fatal("caller-selected wider layout should recover the bars")
	}
}

func TestBarsTableOnlyFallbackDoesNotDisplayAnUnmappedLegend(t *testing.T) {
	d := fixture()
	d.Rows[0].Label = strings.Repeat("Long service name ", 10)
	d.Rows[1].Role = "danger"
	d.Legend = []LegendItem{{Role: "accent", Label: "Scheduled"}, {Role: "danger", Label: "Recorded"}}
	h, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil || strings.Contains(h, "<svg") || strings.Contains(h, `aria-label="Chart legend"`) || !strings.Contains(h, "Bar chart omitted:") {
		t.Fatal("table-only output must not claim an absent color encoding")
	}
}
