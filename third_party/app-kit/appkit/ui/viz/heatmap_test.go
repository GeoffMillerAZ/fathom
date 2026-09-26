package viz

import (
	"math"
	"strings"
	"testing"
)

func TestHeatmap_Basics(t *testing.T) {
	got := Heatmap([][]float64{{0, 5, 10}, {2, 4, 6}}, HeatmapOptions{AriaLabel: "envelope"})
	for _, want := range []string{
		`<svg`, `role="img"`, `aria-label="envelope"`, `<title>envelope</title>`,
		`var(--ak-color-accent)`, // default role, themed (no literal color)
		`fill-opacity=`, `</svg>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("heatmap missing %q", want)
		}
	}
	if strings.Contains(got, "#") {
		t.Errorf("heatmap must not contain literal hex colors: %s", got)
	}
}

func TestHeatmap_OpacityRamp(t *testing.T) {
	// auto range [0,10]: min→0.12, max→0.97
	got := Heatmap([][]float64{{0, 10}}, HeatmapOptions{})
	if !strings.Contains(got, `fill-opacity="0.12"`) {
		t.Errorf("min value should map to lowest opacity: %s", got)
	}
	if !strings.Contains(got, `fill-opacity="0.97"`) {
		t.Errorf("max value should map to highest opacity: %s", got)
	}
}

func TestHeatmap_CustomRole(t *testing.T) {
	got := Heatmap([][]float64{{1}}, HeatmapOptions{Role: "positive"})
	if !strings.Contains(got, `var(--ak-color-positive)`) {
		t.Errorf("custom role not applied: %s", got)
	}
}

func TestHeatmap_InvalidRoleFallsBack(t *testing.T) {
	got := Heatmap([][]float64{{1}}, HeatmapOptions{Role: "evil; }</style>"})
	if strings.Contains(got, "evil") || strings.Contains(got, "</style>") {
		t.Errorf("invalid role leaked: %s", got)
	}
	if !strings.Contains(got, "var(--ak-color-ink-muted)") {
		t.Errorf("invalid role should fall back to ink-muted")
	}
}

func TestHeatmap_NaNHole(t *testing.T) {
	got := Heatmap([][]float64{{1, math.NaN()}}, HeatmapOptions{})
	if !strings.Contains(got, `fill="none"`) || !strings.Contains(got, `stroke-dasharray="2 2"`) {
		t.Errorf("NaN cell should render as a dashed hole: %s", got)
	}
}

func TestHeatmap_Labels(t *testing.T) {
	got := Heatmap([][]float64{{1, 2}}, HeatmapOptions{
		XLabels: []string{"a", "b"}, YLabels: []string{"row0"},
	})
	for _, w := range []string{">a</text>", ">b</text>", ">row0</text>"} {
		if !strings.Contains(got, w) {
			t.Errorf("label %q missing: %s", w, got)
		}
	}
}

func TestHeatmap_ShowValuesAndEscaping(t *testing.T) {
	got := Heatmap([][]float64{{1.5}}, HeatmapOptions{ShowValues: true})
	if !strings.Contains(got, ">1.5</text>") {
		t.Errorf("cell value not rendered: %s", got)
	}
	// a hostile label must be escaped
	g2 := Heatmap([][]float64{{1}}, HeatmapOptions{YLabels: []string{"<b>x</b>"}})
	if strings.Contains(g2, "<b>x</b>") {
		t.Errorf("label not escaped: %s", g2)
	}
}

func TestHeatmap_Empty(t *testing.T) {
	got := Heatmap(nil, HeatmapOptions{})
	if !strings.Contains(got, "<svg") || !strings.Contains(got, "</svg>") {
		t.Errorf("empty heatmap should still be a valid svg: %s", got)
	}
}

func TestHeatmap_Frontier(t *testing.T) {
	got := Heatmap([][]float64{{1, 2, 3}, {4, 5, 6}}, HeatmapOptions{
		Frontier: []float64{0.5, 1.0, 1.5}, FrontierRole: "danger",
	})
	if !strings.Contains(got, "<path ") || !strings.Contains(got, `fill="none" stroke="var(--ak-color-danger)"`) {
		t.Errorf("frontier path/role not rendered: %s", got)
	}
	// a NaN breaks the line into a new subpath (a second M command)
	g2 := Heatmap([][]float64{{1, 2, 3}}, HeatmapOptions{Frontier: []float64{0.5, math.NaN(), 0.5}})
	if strings.Count(g2, "M") < 2 {
		t.Errorf("NaN should break the frontier into separate subpaths: %s", g2)
	}
}

func TestHeatmap_Deterministic(t *testing.T) {
	rows := [][]float64{{0, 5, 10}, {3, math.NaN(), 7}}
	o := HeatmapOptions{Role: "info", XLabels: []string{"x", "y", "z"}, ShowValues: true}
	if Heatmap(rows, o) != Heatmap(rows, o) {
		t.Errorf("heatmap is non-deterministic")
	}
}

// legacyHeatmapFixture is the pre-truth-state call shape: NaN holes, a ragged
// row, labels, values and a frontier. Its golden was generated from the renderer
// BEFORE truth states existed, so it proves existing callers get identical bytes.
func legacyHeatmapFixture() string {
	return Heatmap([][]float64{
		{0, 2.5, 5, math.NaN()},
		{1, math.NaN(), 7.25, 10},
		{3, 4},
	}, HeatmapOptions{
		Role:       "info",
		XLabels:    []string{"q1", "q2", "q3", "q4"},
		YLabels:    []string{"north", "south", "<east>"},
		ShowValues: true,
		Frontier:   []float64{0.5, 1, math.NaN(), 2},
		AriaLabel:  "legacy coverage",
	})
}

func TestHeatmap_LegacyGolden(t *testing.T) {
	assertGolden(t, legacyHeatmapFixture(), "heatmap_legacy")
	assertGolden(t, Heatmap([][]float64{{0, 0.5}, {1, 0.25}}, HeatmapOptions{}), "heatmap_default")
}
