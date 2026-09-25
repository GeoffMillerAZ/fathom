package analytics

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

func resolutionFixture(values ...float64) Dataset {
	d := fixture()
	d.Contract.Unit, d.Contract.Decimals = "n", 0
	d.Contract.Completeness, d.Contract.CoverageNote = Partial, "One observation is missing."
	d.Rows = nil
	for i, v := range values {
		role := "accent"
		if i > 0 {
			role = "caution"
		}
		d.Rows = append(d.Rows, Row{ID: role, Label: strings.ToUpper(role), Value: value(v), Role: role})
	}
	d.Rows = append(d.Rows,
		Row{ID: "zero", Label: "Zero", Value: value(0)},
		Row{ID: "missing", Label: "Unobserved", MissingReason: "Not reported"},
	)
	d.Legend = []LegendItem{{Role: "accent", Label: "Rare outcome"}, {Role: "caution", Label: "Common outcome"}}
	return d
}

func TestBarResolutionPrimitiveCanRoundPositiveWidthToZero(t *testing.T) {
	h := viz.Bars([]viz.Bar{
		{Label: "rare", Value: 1, Suffix: " n"},
		{Label: "common", Value: MaxMagnitude, Suffix: " n"},
	}, viz.BarsOptions{W: 560, LabelW: 100, ShowValue: true})
	if !strings.Contains(h, `<rect x="106" y="3.3" width="0"`) {
		t.Fatalf("low-level primitive no longer demonstrates the rounded zero-width risk: %s", h)
	}
}

func TestBarResolutionFallsBackWithoutLosingTruth(t *testing.T) {
	for _, tc := range []struct {
		name string
		max  float64
		rows []float64
	}{
		{name: "auto maximum", rows: []float64{1, MaxMagnitude}},
		{name: "explicit maximum", max: 1000, rows: []float64{1, 10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := resolutionFixture(tc.rows...)
			h, err := RenderBars(d, viz.BarsOptions{W: 560, LabelW: 100, Max: tc.max}, testNow)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{"Bar chart omitted:", "positive values would render smaller than one logical pixel", "Rare outcome", "Common outcome", "Not reported", "Coverage: partial"} {
				if !strings.Contains(h, want) {
					t.Errorf("fallback lost %q: %s", want, h)
				}
			}
			if strings.Contains(h, `aria-label="Scheduled payroll by month (n)"`) || strings.Contains(h, `aria-label="Chart legend"`) {
				t.Fatal("fallback retained the unreadable chart or its legend")
			}
			rows, _ := dataCSV(t, d)
			if len(rows) != 4 || rows[0][2] != "1" || rows[2][2] != "0" || rows[3][2] != "" || rows[3][5] != "Not reported" || rows[0][8] != "accent" || rows[0][9] != "Rare outcome" {
				t.Fatalf("fallback changed CSV truth: %v", rows)
			}
		})
	}
}

func TestBarResolutionControlsRemainCharts(t *testing.T) {
	for _, tc := range []struct {
		name string
		max  float64
		rows []float64
	}{
		{name: "all zero", rows: []float64{0, 0}},
		{name: "comfortably readable", rows: []float64{2, 4}},
		{name: "exactly one logical pixel", rows: []float64{1, 391}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := RenderBars(resolutionFixture(tc.rows...), viz.BarsOptions{W: 560, LabelW: 100, Max: tc.max}, testNow)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(h, `aria-label="Scheduled payroll by month (n)"`) || strings.Contains(h, "smaller than one logical pixel") {
				t.Fatalf("readable control fell back: %s", h)
			}
		})
	}
}
