package analytics

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/app-kit/appkit/ui/vizframe"
)

var testNow = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

func value(v float64) *float64 { return &v }

func fixture() Dataset {
	return Dataset{
		Contract: Contract{
			ID: "monthly-costs", Title: "Scheduled payroll by month", Unit: "USD", Decimals: 2,
			Grain: "calendar month", Aggregation: "sum of selected scheduled payroll",
			Window: Window{Kind: RangeWindow,
				Start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), TimeZone: "UTC"},
			Source:       vizframe.Derived("Synthetic scheduled ledger; sum the selected rows."),
			Freshness:    Freshness{ObservedAt: testNow.Add(-time.Hour), MaxAge: 2 * time.Hour},
			Completeness: Complete, Uncertainty: "Scheduled amounts; timing uncertainty is not quantified.",
		},
		Rows: []Row{
			{ID: "sept", Label: "September 2026", Value: value(1250.5)},
			{ID: "oct", Label: "October 2026", Value: value(0)},
		},
	}
}

func TestDatasetRejectsInvalidContracts(t *testing.T) {
	tests := []struct {
		name, want string
		change     func(*Dataset)
	}{
		{"missing id", "id is required", func(d *Dataset) { d.Contract.ID = "" }},
		{"unsafe id", "id must begin", func(d *Dataset) { d.Contract.ID = `x" onclick="evil` }},
		{"missing title", "title is required", func(d *Dataset) { d.Contract.Title = " " }},
		{"missing unit", "unit is required", func(d *Dataset) { d.Contract.Unit = "" }},
		{"missing grain", "grain is required", func(d *Dataset) { d.Contract.Grain = "" }},
		{"missing aggregation", "aggregation is required", func(d *Dataset) { d.Contract.Aggregation = "" }},
		{"missing uncertainty", "uncertainty is required", func(d *Dataset) { d.Contract.Uncertainty = "" }},
		{"negative precision", "decimals", func(d *Dataset) { d.Contract.Decimals = -1 }},
		{"unsupported precision", "decimals", func(d *Dataset) { d.Contract.Decimals = 3 }},
		{"mislabeled snapshot", "snapshot window", func(d *Dataset) { d.Contract.Window.Kind = SnapshotWindow }},
		{"unknown window", "window kind", func(d *Dataset) { d.Contract.Window.Kind = "lifetime-ish" }},
		{"empty range", "end must follow", func(d *Dataset) { d.Contract.Window.End = d.Contract.Window.Start }},
		{"reversed range", "end must follow", func(d *Dataset) { d.Contract.Window.End = d.Contract.Window.Start.Add(-time.Hour) }},
		{"absent start", "window start", func(d *Dataset) { d.Contract.Window.Start = time.Time{} }},
		{"absent end", "end must follow", func(d *Dataset) { d.Contract.Window.End = time.Time{} }},
		{"no timezone", "timezone", func(d *Dataset) { d.Contract.Window.TimeZone = "" }},
		{"invalid timezone", "timezone", func(d *Dataset) { d.Contract.Window.TimeZone = "Mars/Olympus" }},
		{"missing source", "source", func(d *Dataset) { d.Contract.Source = vizframe.Source{} }},
		{"unknown source kind", "source", func(d *Dataset) { d.Contract.Source.Kind = "verified" }},
		{"missing source text", "source", func(d *Dataset) { d.Contract.Source.Text = " " }},
		{"absent freshness", "unknown source freshness", func(d *Dataset) { d.Contract.Freshness = Freshness{} }},
		{"future observation", "nonfuture", func(d *Dataset) { d.Contract.Freshness.ObservedAt = testNow.Add(time.Nanosecond) }},
		{"no max age", "positive max age", func(d *Dataset) { d.Contract.Freshness.MaxAge = 0 }},
		{"conflicting freshness", "no unknown reason", func(d *Dataset) { d.Contract.Freshness.UnknownReason = "Unknown" }},
		{"unknown with age", "no max age", func(d *Dataset) { d.Contract.Freshness = Freshness{UnknownReason: "Not recorded", MaxAge: time.Hour} }},
		{"absent completeness", "completeness must", func(d *Dataset) { d.Contract.Completeness = "" }},
		{"partial without reason", "coverage note", func(d *Dataset) { d.Contract.Completeness = Partial }},
		{"unknown without reason", "coverage note", func(d *Dataset) { d.Contract.Completeness = Unknown }},
		{"long text", "exceeds", func(d *Dataset) { d.Contract.Title = strings.Repeat("x", maxTextBytes+1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := fixture()
			tt.change(&d)
			if err := d.Validate(testNow); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, err)
			}
		})
	}
	if err := fixture().Validate(time.Time{}); err == nil {
		t.Fatal("an implicit clock was accepted")
	}
}

func TestDatasetRejectsInvalidRows(t *testing.T) {
	tests := []struct {
		name, want string
		change     func(*Dataset)
	}{
		{"missing row id", "id is required", func(d *Dataset) { d.Rows[0].ID = "" }},
		{"missing label", "label is required", func(d *Dataset) { d.Rows[0].Label = "" }},
		{"duplicate ids", "duplicate row id", func(d *Dataset) { d.Rows[1].ID = d.Rows[0].ID }},
		{"not a semantic role", "semantic role", func(d *Dataset) { d.Rows[0].Role = `red);color:red` }},
		{"missing and complete", "missing row", func(d *Dataset) { d.Rows[0].Value = nil; d.Rows[0].MissingReason = "Not observed" }},
		{"missing without reason", "missing row", func(d *Dataset) {
			d.Contract.Completeness = Partial
			d.Contract.CoverageNote = "Some inputs missing"
			d.Rows[0].Value = nil
		}},
		{"present with missing reason", "present row", func(d *Dataset) { d.Rows[0].MissingReason = "Not observed" }},
		{"NaN", "finite", func(d *Dataset) { d.Rows[0].Value = value(math.NaN()) }},
		{"positive infinity", "finite", func(d *Dataset) { d.Rows[0].Value = value(math.Inf(1)) }},
		{"negative infinity", "finite", func(d *Dataset) { d.Rows[0].Value = value(math.Inf(-1)) }},
		{"magnitude", "within", func(d *Dataset) { d.Rows[0].Value = value(MaxMagnitude + 1) }},
		{"reversed bounds", "bounds", func(d *Dataset) { d.Rows[0].Bounds = &Interval{Low: 1400, High: 1200} }},
		{"outside bounds", "bounds", func(d *Dataset) { d.Rows[0].Bounds = &Interval{Low: 1300, High: 1400} }},
		{"nonfinite bounds", "bounds", func(d *Dataset) { d.Rows[0].Bounds = &Interval{Low: 0, High: math.Inf(1)} }},
		{"multi-role no legend", "legend label", func(d *Dataset) { d.Rows[1].Role = "danger" }},
		{"incomplete legend", "legend label", func(d *Dataset) {
			d.Rows[1].Role = "danger"
			d.Legend = []LegendItem{{Role: "accent", Label: "Scheduled"}}
		}},
		{"duplicate legend role", "valid and unique", func(d *Dataset) {
			d.Legend = []LegendItem{{Role: "accent", Label: "One"}, {Role: "accent", Label: "Two"}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := fixture()
			tt.change(&d)
			if err := d.Validate(testNow); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want error containing %q, got %v", tt.want, err)
			}
		})
	}
	d := fixture()
	d.Rows = make([]Row, MaxRows+1)
	if err := d.Validate(testNow); err == nil || !strings.Contains(err.Error(), "row limit") {
		t.Fatalf("oversized selection was not rejected: %v", err)
	}
}

func TestFreshnessBoundaryAndUnknownAreVisible(t *testing.T) {
	for _, tt := range []struct {
		name string
		f    Freshness
		want string
	}{
		{"boundary fresh", Freshness{ObservedAt: testNow.Add(-time.Hour), MaxAge: time.Hour}, "Fresh: source observed"},
		{"past boundary stale", Freshness{ObservedAt: testNow.Add(-time.Hour - time.Nanosecond), MaxAge: time.Hour}, "Stale: source observed"},
		{"unknown", Freshness{UnknownReason: "Input observation time was not recorded"}, "Source freshness unknown: Input observation time was not recorded"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := fixture()
			d.Contract.Freshness = tt.f
			h, err := RenderBars(d, viz.BarsOptions{}, testNow)
			if err != nil || !strings.Contains(h, tt.want) {
				t.Fatalf("expected visible %q, error=%v, html=%s", tt.want, err, h)
			}
			var out bytes.Buffer
			if err := WriteCSV(&out, d, testNow); err != nil || !strings.Contains(out.String(), tt.want) {
				t.Fatalf("CSV lost freshness: %v, %s", err, out.String())
			}
		})
	}
}

func TestWindowLabelComesFromActualEndpoints(t *testing.T) {
	d := fixture()
	d.Contract.Window = Window{Kind: RangeWindow, Start: time.Date(2026, 9, 1, 7, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 1, 7, 0, 0, 0, time.UTC), TimeZone: "America/Phoenix"}
	h, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil || !strings.Contains(h, "2026-09-01T00:00:00-07:00 to 2026-10-01T00:00:00-07:00 (end exclusive; America/Phoenix)") {
		t.Fatalf("period was mislabeled: %v, %s", err, h)
	}
	d.Contract.Window = Window{Kind: SnapshotWindow, Start: testNow, TimeZone: "UTC"}
	h, err = RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil || !strings.Contains(h, "Snapshot at 2026-09-13T12:00:00Z [UTC]") {
		t.Fatalf("snapshot was mislabeled: %v, %s", err, h)
	}
}

func dataCSV(t *testing.T, d Dataset) ([][]string, string) {
	t.Helper()
	var out bytes.Buffer
	if err := WriteCSV(&out, d, testNow); err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(strings.NewReader(out.String()))
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		if len(row) == 10 && row[0] == "id" && row[1] == "label" && row[8] == "role" && row[9] == "role_label" {
			return rows[i+1:], out.String()
		}
	}
	t.Fatal("missing CSV data header")
	return nil, ""
}

func TestChartTableAndCSVShareRoundedNumbers(t *testing.T) {
	for _, tt := range []struct {
		decimals int
		input    float64
		printed  string
		csv      string
	}{
		{0, 1234.6, "1,235", "1235"},
		{1, 1234.56, "1234.6", "1234.6"},
		{2, 1234.567, "1234.57", "1234.57"},
		{2, 0, "0", "0"},
		{2, MaxMagnitude, "1,000,000,000,000", "1000000000000"},
	} {
		t.Run(fmt.Sprintf("%d_%v", tt.decimals, tt.input), func(t *testing.T) {
			d := fixture()
			d.Contract.Decimals = tt.decimals
			d.Rows = []Row{{ID: "one", Label: "Selected month", Value: value(tt.input)}}
			h, err := RenderBars(d, viz.BarsOptions{ShowValue: false}, testNow)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(h, ">"+tt.printed+" USD</text>") || !strings.Contains(h, `tabular-nums">`+tt.printed+"</td>") {
				t.Fatalf("chart/table must both print %q: %s", tt.printed, h)
			}
			rows, csvOut := dataCSV(t, d)
			if len(rows) != 1 || rows[0][2] != tt.csv || rows[0][3] != "USD" {
				t.Fatalf("CSV disagrees with selected numbers: %v", rows)
			}
			if _, err := strconv.ParseFloat(rows[0][2], 64); err != nil {
				t.Fatalf("typed numeric cell is not numeric: %v", err)
			}
			for _, required := range []string{"calendar month", "sum of selected scheduled payroll", "Synthetic scheduled ledger", "end exclusive", "complete", "Uncertainty:"} {
				if !strings.Contains(h, required) {
					t.Errorf("HTML lost semantic context %q", required)
				}
			}
			for _, key := range []string{"unit,USD", "grain,calendar month", "window_kind,range", "source_kind,derived", "completeness,complete", "decimal_places,"} {
				if !strings.Contains(csvOut, key) {
					t.Errorf("CSV lost context %q", key)
				}
			}
		})
	}
}

func TestMissingIsNotZeroAndBoundsSurvive(t *testing.T) {
	d := fixture()
	d.Contract.Completeness, d.Contract.CoverageNote = Partial, "The third month's source is unavailable"
	d.Contract.Window.End = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	d.Rows[1].Bounds = &Interval{Low: -1, High: 1}
	d.Rows = append(d.Rows, Row{ID: "missing", Label: "Unobserved month", MissingReason: "The import failed"})
	h, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	svg := h[strings.Index(h, "<svg"):strings.Index(h, "</svg>")]
	if strings.Contains(svg, "Unobserved month") || !strings.Contains(svg, ">0 USD</text>") {
		t.Fatalf("missing and measured zero were conflated: %s", svg)
	}
	for _, required := range []string{"Unobserved month", ">Missing</td>", "The import failed", "Coverage: partial", "-1 to 1", "<caption", `scope="row"`} {
		if !strings.Contains(h, required) {
			t.Errorf("missing accessible data/state %q", required)
		}
	}
	rows, _ := dataCSV(t, d)
	if rows[1][2] != "0" || rows[1][6] != "-1" || rows[1][7] != "1" || rows[2][2] != "" || rows[2][4] != "missing" || rows[2][5] != "The import failed" {
		t.Fatalf("CSV confused missing, zero or interval: %v", rows)
	}
}

func TestEmptyDatasetStatesAreExplicit(t *testing.T) {
	d := fixture()
	d.Rows = nil
	h, err := RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil || strings.Contains(h, "<svg") || !strings.Contains(h, "No measured values") || !strings.Contains(h, "No rows in this selection") {
		t.Fatalf("empty selection must be explicit, not an empty shape: %v, %s", err, h)
	}
	d.Contract.Completeness, d.Contract.CoverageNote = Unknown, "Import unavailable"
	d.Rows = []Row{{ID: "unavailable", Label: "Selected month", MissingReason: "No source observation"}}
	h, err = RenderBars(d, viz.BarsOptions{}, testNow)
	if err != nil || strings.Contains(h, "<svg") || !strings.Contains(h, "No source observation") {
		t.Fatalf("all-missing selection must be explicit: %v, %s", err, h)
	}
}

func TestBarsRefuseNegativeAndClippedData(t *testing.T) {
	d := fixture()
	d.Rows[0].Value = value(-12.5)
	if err := d.Validate(testNow); err != nil {
		t.Fatalf("signed semantic data is valid independently of this renderer: %v", err)
	}
	if h, err := RenderBars(d, viz.BarsOptions{}, testNow); err == nil || h != "" || !strings.Contains(err.Error(), "negative") {
		t.Fatalf("negative data was silently clamped: %v", err)
	}
	rows, _ := dataCSV(t, d)
	if rows[0][2] != "-12.5" {
		t.Errorf("typed signed number was treated as a text formula: %v", rows)
	}
	for _, o := range []viz.BarsOptions{{Max: 10}, {Max: math.NaN()}, {Max: math.Inf(1)}, {W: 200, LabelW: 180}} {
		if h, err := RenderBars(fixture(), o, testNow); err == nil || h != "" {
			t.Errorf("unsafe chart options were accepted: %+v", o)
		}
	}
}

func TestRenderedTextIsEscapedAndLegendScoped(t *testing.T) {
	d := fixture()
	d.Contract.Title = `<script>alert("title")</script>`
	d.Contract.Source.Text = `<img src=x onerror="evil">`
	d.Contract.Source.Href = `javascript:alert(1)`
	d.Rows[0].Label = `<a onclick="evil">bad & label</a>`
	d.Rows[1].Role = "danger"
	d.Legend = []LegendItem{{Role: "accent", Label: `<b>scheduled</b>`}, {Role: "danger", Label: "Recorded"}}
	// Keep the escaped label in a fitting plot so the SVG and legend paths
	// remain exercised independently of the table-only layout fallback.
	h, err := RenderBars(d, viz.BarsOptions{W: 1024, LabelW: 512}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"<script>", "<img", `<a onclick=`, `href="javascript:`, "<b>scheduled</b>"} {
		if strings.Contains(h, forbidden) {
			t.Errorf("untrusted markup reached output: %s", forbidden)
		}
	}
	for _, required := range []string{`&lt;script&gt;`, `bad &amp; label`, `&lt;b&gt;scheduled&lt;/b&gt;`, `aria-label="Chart legend"`, `var(--ak-color-danger)`} {
		if !strings.Contains(h, required) {
			t.Errorf("escaped label or legend missing: %s", required)
		}
	}
}

func TestCSVNeutralizesUntrustedTextAndMetadata(t *testing.T) {
	for _, text := range []string{
		"=SUM(A1:A2)", "+1", "-1", "@SUM(A1:A2)", "   =cmd", "\t=cmd", "\r=cmd", "\n=cmd",
		"\ufeff=cmd", "\u2003+1", "\u200b=cmd", "ordinary\nmultiline", `=HYPERLINK("https://example.com","open")`,
	} {
		t.Run(fmt.Sprintf("%q", text), func(t *testing.T) {
			d := fixture()
			d.Rows[0].Label = text
			d.Contract.Title, d.Contract.Source.Text = text, text
			rows, csvOut := dataCSV(t, d)
			if rows[0][1] != "'"+text {
				t.Fatalf("untrusted formula/control text not neutralized: %q", rows[0][1])
			}
			r := csv.NewReader(strings.NewReader(csvOut))
			r.FieldsPerRecord = -1
			all, err := r.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			for _, record := range all {
				if len(record) == 2 && (record[0] == "title" || record[0] == "source") && record[1] != "'"+text {
					t.Errorf("metadata was not protected: %v", record)
				}
			}
		})
	}
	// NUL is now refused rather than transformed: HTML parsing would replace
	// it while CSV retained it. Keep the old formula-prefixed case as an
	// explicit atomic-refusal canary, not an omitted security regression.
	dNUL := fixture()
	dNUL.Rows[0].Label = "\x00=cmd"
	dNUL.Contract.Title, dNUL.Contract.Source.Text = "\x00=cmd", "\x00=cmd"
	var nulOutput bytes.Buffer
	if err := WriteCSV(&nulOutput, dNUL, testNow); err == nil || nulOutput.Len() != 0 {
		t.Fatal("NUL-prefixed formula must be refused before any export")
	}
	d := fixture()
	d.Rows[0].Label = `A normal "quoted", label`
	rows, _ := dataCSV(t, d)
	if rows[0][1] != d.Rows[0].Label {
		t.Error("ordinary text failed CSV quoting round trip")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("storage unavailable") }

func TestInvalidExportWritesNothingAndIOErrorsPropagate(t *testing.T) {
	d := fixture()
	d.Contract.Unit = ""
	var out bytes.Buffer
	if err := WriteCSV(&out, d, testNow); err == nil || out.Len() != 0 {
		t.Fatalf("invalid contract produced a partial export: %v, %s", err, out.String())
	}
	if err := WriteCSV(failingWriter{}, fixture(), testNow); err == nil || !strings.Contains(err.Error(), "storage unavailable") {
		t.Fatalf("export swallowed write failure: %v", err)
	}
}
