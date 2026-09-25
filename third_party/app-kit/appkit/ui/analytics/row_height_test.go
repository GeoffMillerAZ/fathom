package analytics

import (
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

func rowHeightRenderers() map[string]func(Dataset, viz.BarsOptions, time.Time) (string, error) {
	return map[string]func(Dataset, viz.BarsOptions, time.Time) (string, error){
		"bars": RenderBars, "diverging": RenderDivergingBars,
	}
}

func rowHeightDataset(signed bool) Dataset {
	d := fixture()
	d.Contract.Completeness, d.Contract.CoverageNote = Partial, "One selected observation is missing."
	first := 2.0
	if signed {
		first = -2
	}
	d.Rows = []Row{
		{ID: "first", Label: "Åg first", Value: value(first), Bounds: &Interval{Low: first - 1, High: first + 1}},
		{ID: "second", Label: "Éy second", Value: value(1)},
		{ID: "zero", Label: "Zero", Value: value(0)},
		{ID: "missing", Label: "Missing", MissingReason: "Not observed"},
	}
	d.Legend = []LegendItem{{Role: "accent", Label: "Observed amount"}}
	return d
}

func TestBarRowHeightFallbackPreservesTruth(t *testing.T) {
	for name, render := range rowHeightRenderers() {
		for _, tc := range []struct {
			name      string
			rowH, gap int
			single    bool
		}{
			{"overlap", 1, 1, false},
			{"single edge despite large gap", 1, 40, true},
			{"below row budget", 15, 1, false},
			{"spacing alone cannot protect edges", 15, 100, false},
		} {
			t.Run(name+"/"+tc.name, func(t *testing.T) {
				d := rowHeightDataset(name == "diverging")
				if tc.single {
					d.Rows = d.Rows[:1]
					d.Contract.Completeness, d.Contract.CoverageNote = Complete, ""
				}
				_, csvBefore := dataCSV(t, d)
				h, err := render(d, viz.BarsOptions{RowH: tc.rowH, Gap: tc.gap}, testNow)
				if err != nil || h == "" {
					t.Fatalf("positive small dimensions must remain non-error output: %v", err)
				}
				if !strings.Contains(h, "Bar chart omitted: row height") || !strings.Contains(h, "16") || strings.Contains(h, " bar chart, horizontally scrollable") || strings.Contains(h, `aria-label="Chart legend"`) {
					t.Fatalf("small rows need an explained table-only fallback: %s", h)
				}
				if !strings.Contains(h, renderTable(d)) || !strings.Contains(h, "Observed amount") || !strings.Contains(h, "Uncertainty:") {
					t.Fatal("fallback lost table/meaning/truth context")
				}
				_, csvAfter := dataCSV(t, d)
				if csvAfter != csvBefore {
					t.Fatal("layout changed selected data or export")
				}
			})
		}
	}
}

func TestBarRowHeightReadableBoundaryAndDefaults(t *testing.T) {
	for name, render := range rowHeightRenderers() {
		t.Run(name, func(t *testing.T) {
			d := rowHeightDataset(name == "diverging")
			for _, options := range []viz.BarsOptions{{RowH: 16, Gap: 1}, {RowH: 28}, {RowH: 256, Gap: 256}, {}, {RowH: -1, Gap: -1}} {
				h, err := render(d, options, testNow)
				if err != nil || strings.Contains(h, "Bar chart omitted:") || !strings.Contains(h, " bar chart, horizontally scrollable") || !strings.Contains(h, `aria-label="Chart legend"`) {
					t.Fatalf("fitting/default options lost chart: %+v, %v", options, err)
				}
			}
			defaults, err := render(d, viz.BarsOptions{}, testNow)
			if err != nil {
				t.Fatal(err)
			}
			for _, options := range []viz.BarsOptions{{RowH: 28, Gap: 6}, {RowH: -1, Gap: -1}} {
				explicit, err := render(d, options, testNow)
				if err != nil || explicit != defaults {
					t.Fatalf("default output changed: %+v, %v", options, err)
				}
			}
		})
	}
}

func TestBarRowHeightWithoutMeasuredRowsKeepsEmptyState(t *testing.T) {
	for name, render := range rowHeightRenderers() {
		for _, missing := range []bool{false, true} {
			d := rowHeightDataset(name == "diverging")
			if missing {
				d.Rows = d.Rows[3:]
			} else {
				d.Rows = nil
			}
			h, err := render(d, viz.BarsOptions{RowH: 1, Gap: 1}, testNow)
			if err != nil || !strings.Contains(h, "No measured values") || strings.Contains(h, "Bar chart omitted:") || strings.Contains(h, `aria-label="Chart legend"`) || !strings.Contains(h, renderTable(d)) {
				t.Fatalf("%s missing=%t changed no-measurements output: %v", name, missing, err)
			}
		}
	}
}
