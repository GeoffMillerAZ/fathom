package gallery

import (
	"html"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

// The chart's per-cell titles and the table alternative come from one fixture;
// this pins their agreement cell by cell, so a keyboard or screen-reader user
// reading the table gets exactly the state and reason a pointer user hovers.
func TestHeatmapSpecimenChartAndTableAgree(t *testing.T) {
	content, err := buildHeatmap()
	if err != nil {
		t.Fatal(err)
	}
	body := string(content)
	projects, layers, cells := heatmapFixture()
	for r, row := range cells {
		for c, cell := range row {
			text := heatmapCellText(cell)
			title := "<title>" + html.EscapeString(projects[r]+", "+layers[c]+": "+text) + "</title>"
			if !strings.Contains(body, title) {
				t.Errorf("chart lacks cell title %s", title)
			}
			if !strings.Contains(body, `<td class="px-4 py-3 text-ink">`+html.EscapeString(text)+`</td>`) {
				t.Errorf("table lacks cell %q", text)
			}
		}
	}
	for _, want := range []string{
		`<caption class="sr-only">`, `<desc>16 cells: value 7 (range 0–14); zero 2; missing 3; unknown 2; error 2.</desc>`,
		">missing: not compiled</text>", ">unknown</text>", ">error</text>", ">zero</text>",
		`aria-label="Layer coverage table, horizontally scrollable if needed"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("heatmap specimen missing %q", want)
		}
	}
	if got := strings.Count(body, `<th scope="col"`); got != 5 {
		t.Errorf("column headers = %d, want 5", got)
	}
	if got := strings.Count(body, `<th scope="row"`); got != 4 {
		t.Errorf("row headers = %d, want 4", got)
	}
}

func TestHeatmapSpecimenAdversarialAndLegacyFigures(t *testing.T) {
	content, err := buildHeatmap()
	if err != nil {
		t.Fatal(err)
	}
	svgs := regexp.MustCompile(`(?s)<svg .*?</svg>`).FindAllString(string(content), -1)
	if len(svgs) != 3 {
		t.Fatalf("specimen svgs = %d, want truth, all-missing and legacy", len(svgs))
	}
	missing, legacy := svgs[1], svgs[2]
	if strings.Contains(missing, "fill-opacity") || strings.Contains(missing, ">value ") || !strings.Contains(missing, "<desc>9 cells: missing 9.</desc>") {
		t.Errorf("all-missing figure drew a value or lost its summary: %s", missing)
	}
	if strings.Contains(legacy, "<desc>") || strings.Contains(legacy, "<title>atlas") || strings.Count(legacy, `stroke-dasharray="2 2"`) != 7 {
		t.Errorf("legacy comparison must be the unchanged hole rendering: %s", legacy)
	}
}

func TestHeatmapSpecimenRouteKeepsComponentsSection(t *testing.T) {
	h := New()
	res, body := get(t, h, heatmapPath+"?theme=dark")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d", heatmapPath, res.StatusCode)
	}
	for _, want := range []string{`data-theme="dark"`, `<title>Heatmap truth states · app-kit Gallery</title>`, `href="/components?theme=dark" aria-current="page">Components</a>`} {
		if !strings.Contains(body, want) {
			t.Errorf("heatmap route missing %q", want)
		}
	}
	if res, _ := get(t, h, heatmapPath+"/extra"); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown heatmap sub-path = %d, want 404", res.StatusCode)
	}
	_, components := get(t, h, "/components")
	if !strings.Contains(components, `href="/components/heatmap"`) || !strings.Contains(components, "Heatmap — truth states") {
		t.Error("components page must show the truth-state card and link the full specimen")
	}
}

// The specimen fixture is integers only, so it cannot show the table and the
// chart disagreeing on a fraction. This drives the same conversion and table
// wording with coverage ratios, including one below the chart's two-decimal
// rounding: the chart must not say 0 where the table says 0.004.
func TestHeatmapTableTextMatchesChartForFractions(t *testing.T) {
	cells := [][]heatmapFixtureCell{{{Value: 0}, {Value: 0.004}, {Value: -0.004}, {Value: 0.5}, {Value: 1}}}
	layers := []string{"dialect", "provider", "platform", "concern", "total"}
	rows, states := heatmapInputs(cells)
	svg := viz.Heatmap(rows, viz.HeatmapOptions{States: states, ShowValues: true, XLabels: layers, YLabels: []string{"atlas"}})
	for c, cell := range cells[0] {
		text := heatmapCellText(cell)
		if title := "<title>" + html.EscapeString("atlas, "+layers[c]+": "+text) + "</title>"; !strings.Contains(svg, title) {
			t.Errorf("chart title disagrees with table text %q: %s", text, svg)
		}
	}
	if got := strings.Count(svg, "(zero)"); got != 1 {
		t.Errorf("zero titles = %d, want only the measured zero", got)
	}
}
