package analytics

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

func TestBarsTableRetainsRoleMeaningWithoutColorOrPlot(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		d := fixture()
		d.Rows[0].Label, d.Rows[1].Label = "Service A", "Service B"
		d.Rows[1].Role = "danger"
		d.Legend = []LegendItem{{Role: "danger", Label: "Recorded"}, {Role: "accent", Label: "Scheduled"}}
		if fallback {
			d.Rows[0].Label = strings.Repeat("Long service name ", 10)
		}
		h, err := RenderBars(d, viz.BarsOptions{}, testNow)
		if err != nil {
			t.Fatal(err)
		}
		table := h[strings.Index(h, "<table"):strings.Index(h, "</table>")]
		if !strings.Contains(table, "[&_th]:align-top [&_td]:align-top") {
			t.Fatal("wrapped row labels and their values must share a top baseline")
		}
		if !strings.Contains(table, ">Meaning</th>") || !strings.Contains(table, ">Scheduled</td>") || !strings.Contains(table, ">Recorded</td>") {
			t.Fatal("visible data table lost role-to-meaning association")
		}
		rows := strings.Split(table[strings.Index(table, "<tbody>"):], `<tr class="border-b border-line">`)
		if len(rows) != 3 || !strings.Contains(rows[1], ">Scheduled</td>") || !strings.Contains(rows[2], ">Recorded</td>") {
			t.Fatal("role meaning followed legend order instead of the row's declared role")
		}
		if fallback && (strings.Contains(h, "<svg") || strings.Contains(h, `aria-label="Chart legend"`)) {
			t.Fatal("table fallback must not claim a plotted color encoding")
		}
		if strings.Contains(h, " style=") {
			t.Fatal("role legend must work under style-src self without inline CSS")
		}
		if !fallback && strings.Count(h, `data-analytics-legend-swatch aria-hidden="true" focusable="false"`) != 2 {
			t.Fatal("each declared legend swatch must be explicitly decorative")
		}
	}
}

func TestBarsMeaningPreservesMissingnessAndEscapesLabels(t *testing.T) {
	d := fixture()
	d.Contract.Completeness, d.Contract.CoverageNote = Partial, "Unavailable source"
	d.Rows = []Row{
		{ID: "zero", Label: "Zero", Value: value(0)},
		{ID: "missing", Label: "Missing row", Role: "danger", MissingReason: "Import failed"},
		{ID: "unmapped", Label: "Unmapped row", Role: "other", MissingReason: "No observation"},
	}
	d.Legend = []LegendItem{{Role: "accent", Label: "Scheduled"}, {Role: "danger", Label: `<b>Recorded & pending</b>`}}
	h, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{">Meaning</th>", ">Scheduled</td>", "&lt;b&gt;Recorded &amp; pending&lt;/b&gt;", ">Not specified</td>", ">Missing</td>", ">Import failed</td>", ">No observation</td>", `tabular-nums">0</td>`} {
		if !strings.Contains(h, want) {
			t.Errorf("lost declared meaning or truth state: %s", want)
		}
	}
	if strings.Contains(h, "<b>Recorded") {
		t.Fatal("legend label was treated as markup")
	}
}

func TestBarsWithoutMappedMeaningKeepsCompactTable(t *testing.T) {
	for _, empty := range []bool{false, true} {
		d := fixture()
		d.Legend = []LegendItem{{Role: "unrelated", Label: "Not assigned"}}
		if empty {
			d.Rows = nil
		}
		h, err := RenderBars(d, viz.BarsOptions{}, testNow)
		if err != nil || strings.Contains(h, ">Meaning</th>") {
			t.Fatal("unmapped or absent legend must not invent a row meaning column")
		}
	}
}
