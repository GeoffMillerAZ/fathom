package analytics

import (
	"bytes"
	"reflect"
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

func TestBarsLegendOnlyDescribesMeasuredRoles(t *testing.T) {
	d := fixture()
	d.Contract.Completeness, d.Contract.CoverageNote = Partial, "One source row unavailable"
	d.Rows = []Row{
		{ID: "zero", Label: "Measured zero", Value: value(0)},
		{ID: "missing", Label: "Missing result", Role: "danger", MissingReason: "Source unavailable"},
	}
	d.Legend = []LegendItem{{Role: "danger", Label: "Recorded"}}
	rowsBefore := append([]Row(nil), d.Rows...)
	legendBefore := append([]LegendItem(nil), d.Legend...)

	h, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h, "<svg") || strings.Contains(h, `aria-label="Chart legend"`) || strings.Contains(h, `data-analytics-legend-swatch`) {
		t.Fatal("a missing-only role must not acquire a chart swatch while measured zero remains in the chart")
	}
	if !strings.Contains(h, ">Recorded</td>") || !strings.Contains(h, "Source unavailable</td>") || !strings.Contains(h, `tabular-nums">0</td>`) {
		t.Fatal("filtering a chart legend lost the missing row's table meaning or the measured zero")
	}
	context, csvRows := readRoleCSV(t, d)
	if len(context) != 17 || len(csvRows) != 2 || csvRows[0][2] != "0" || csvRows[0][8] != "accent" || csvRows[1][2] != "" || csvRows[1][4] != "missing" || csvRows[1][5] != "Source unavailable" || csvRows[1][8] != "danger" || csvRows[1][9] != "Recorded" {
		t.Fatalf("CSV context or established value/missing/role fields changed: context=%v rows=%v", context, csvRows)
	}
	var out bytes.Buffer
	if err := WriteCSV(&out, d, testNow); err != nil || !strings.Contains(out.String(), "id,label,value,unit,state,missing_reason,lower,upper,role,role_label") {
		t.Fatalf("CSV header changed: err=%v output=%q", err, out.String())
	}
	if !reflect.DeepEqual(d.Rows, rowsBefore) || !reflect.DeepEqual(d.Legend, legendBefore) {
		t.Fatal("render or export mutated app-selected rows or legend")
	}

	// An unrelated declaration remains absent, but the default accent role is
	// still a plotted role even when its measured value is exactly zero.
	d.Legend = []LegendItem{{Role: "danger", Label: "Recorded"}, {Role: "accent", Label: "Scheduled"}, {Role: "unrelated", Label: "Unplotted"}}
	h, err = RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil || strings.Count(h, `data-analytics-legend-swatch`) != 1 || !strings.Contains(h, `id="`+d.Contract.ID+`-legend-accent"`) || strings.Contains(h, `id="`+d.Contract.ID+`-legend-danger"`) || strings.Contains(h, `id="`+d.Contract.ID+`-legend-unrelated"`) {
		t.Fatalf("only measured accent should appear in the chart legend: err=%v", err)
	}

	// Once the second role is measured (including zero), both meanings appear
	// in app-declared legend order, without changing the selected dataset.
	d.Rows[1].Value, d.Rows[1].MissingReason = value(0), ""
	d.Contract.Completeness, d.Contract.CoverageNote = Complete, ""
	h, err = RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil || strings.Count(h, `data-analytics-legend-swatch`) != 2 {
		t.Fatalf("two measured roles must have two chart swatches: err=%v", err)
	}
	dangerAt := strings.Index(h, `id="`+d.Contract.ID+`-legend-danger"`)
	accentAt := strings.Index(h, `id="`+d.Contract.ID+`-legend-accent"`)
	if dangerAt < 0 || accentAt < 0 || dangerAt > accentAt {
		t.Fatal("chart legend must retain app-declared order for plotted roles")
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
