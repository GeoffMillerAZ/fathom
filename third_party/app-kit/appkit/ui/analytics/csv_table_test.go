package analytics

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
	"time"
)

var tableCSVHeader = []string{
	"context_chart_id", "context_title", "context_unit", "context_grain", "context_aggregation",
	"context_window_kind", "context_window", "context_source_kind", "context_source", "context_source_href",
	"context_source_draft", "context_evaluated_at", "context_freshness", "context_completeness",
	"context_coverage_note", "context_uncertainty", "context_decimal_places",
	"id", "label", "value", "unit", "state", "missing_reason", "lower", "upper", "role", "role_label",
}

func readTableCSV(t *testing.T, d Dataset, now time.Time) ([][]string, []byte) {
	t.Helper()
	var out bytes.Buffer
	if err := WriteTableCSV(&out, d, now); err != nil {
		t.Fatal(err)
	}
	records, err := csv.NewReader(bytes.NewReader(out.Bytes())).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return records, out.Bytes()
}

func readSectionedCSV(t *testing.T, d Dataset, now time.Time) ([][]string, [][]string) {
	t.Helper()
	var out bytes.Buffer
	if err := WriteCSV(&out, d, now); err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(bytes.NewReader(out.Bytes()))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for i, record := range records {
		if reflect.DeepEqual(record, tableCSVHeader[17:]) {
			return records[:i], records[i+1:]
		}
	}
	t.Fatal("sectioned CSV data header missing")
	return nil, nil
}

func TestWriteCSVFixtureBytesRemainStable(t *testing.T) {
	want := "chart_id,monthly-costs\n" +
		"title,Scheduled payroll by month\n" +
		"unit,USD\n" +
		"grain,calendar month\n" +
		"aggregation,sum of selected scheduled payroll\n" +
		"window_kind,range\n" +
		"window,2026-09-01T00:00:00Z to 2026-11-01T00:00:00Z (end exclusive; UTC)\n" +
		"source_kind,derived\n" +
		"source,Synthetic scheduled ledger; sum the selected rows.\n" +
		"source_href,\nsource_draft,false\n" +
		"evaluated_at,2026-09-13T12:00:00Z\n" +
		"freshness,Fresh: source observed 2026-09-13T11:00:00Z; maximum age 2h0m0s\n" +
		"completeness,complete\ncoverage_note,\n" +
		"uncertainty,Scheduled amounts; timing uncertainty is not quantified.\n" +
		"decimal_places,2\n\n" +
		"id,label,value,unit,state,missing_reason,lower,upper,role,role_label\n" +
		"sept,September 2026,1250.5,USD,present,,,,accent,\n" +
		"oct,October 2026,0,USD,present,,,,accent,\n"
	var out bytes.Buffer
	if err := WriteCSV(&out, fixture(), testNow); err != nil {
		t.Fatal(err)
	}
	if out.String() != want {
		t.Fatalf("sectioned CSV bytes changed:\n%s", out.String())
	}
}

func TestWriteTableCSVRectangularValuesAgreeWithSectionedCSV(t *testing.T) {
	d := fixture()
	d.Contract.Title = "=Résumé, \"Q3\"\nsummary"
	d.Contract.Source.Text = "\u200b=source"
	d.Contract.Source.Href, d.Contract.Source.Draft = "/source?x=1", true
	d.Contract.Freshness = Freshness{UnknownReason: "Feed \"offline\",\nretry"}
	d.Contract.Completeness, d.Contract.CoverageNote = Partial, "Only 2 of 3,\nrows"
	d.Rows = []Row{
		{ID: "zero", Label: "+measured zero", Value: value(0), Role: "actual"},
		{ID: "negative", Label: "\u200b-formula", Value: value(-12.345), Bounds: &Interval{Low: -13, High: -12}, Role: "forecast"},
		{ID: "missing", Label: `plain, "quoted"`, MissingReason: "Unavailable\nat source", Role: "forecast"},
	}
	d.Legend = []LegendItem{{Role: "actual", Label: `@Actual "series"`}, {Role: "forecast", Label: "Forecast\nseries"}}
	wantDataset := d
	wantDataset.Rows = append([]Row(nil), d.Rows...)
	wantDataset.Legend = append([]LegendItem(nil), d.Legend...)
	wantBounds := *d.Rows[1].Bounds
	wantDataset.Rows[1].Bounds = &wantBounds

	records, first := readTableCSV(t, d, testNow)
	_, second := readTableCSV(t, d, testNow)
	if !bytes.Equal(first, second) {
		t.Fatal("identical input and explicit clock produced different bytes")
	}
	if !reflect.DeepEqual(d, wantDataset) {
		t.Fatal("table export mutated the caller's dataset")
	}
	if len(records) != 4 || !reflect.DeepEqual(records[0], tableCSVHeader) {
		t.Fatalf("rectangular header or row count changed: %v", records)
	}

	wantContext := []string{
		"monthly-costs", "'=Résumé, \"Q3\"\nsummary", "USD", "calendar month", "sum of selected scheduled payroll",
		"range", "2026-09-01T00:00:00Z to 2026-11-01T00:00:00Z (end exclusive; UTC)",
		"derived", "'\u200b=source", "/source?x=1", "true", "2026-09-13T12:00:00Z",
		"'Source freshness unknown: Feed \"offline\",\nretry", "partial", "'Only 2 of 3,\nrows",
		"Scheduled amounts; timing uncertainty is not quantified.", "2",
	}
	wantRows := [][]string{
		{"zero", "'+measured zero", "0", "USD", "present", "", "", "", "actual", "'@Actual \"series\""},
		{"negative", "'\u200b-formula", "-12.35", "USD", "present", "", "-13", "-12", "forecast", "'Forecast\nseries"},
		{"missing", `plain, "quoted"`, "", "USD", "missing", "'Unavailable\nat source", "", "", "forecast", "'Forecast\nseries"},
	}
	oldContext, oldRows := readSectionedCSV(t, d, testNow)
	if len(oldContext) != 17 || !reflect.DeepEqual(oldRows, wantRows) {
		t.Fatalf("independent sectioned values changed: context=%v rows=%v", oldContext, oldRows)
	}
	for i, got := range records[1:] {
		if len(got) != len(tableCSVHeader) || !reflect.DeepEqual(got[:17], wantContext) || !reflect.DeepEqual(got[17:], wantRows[i]) {
			t.Fatalf("row %d lost repeated context or presentation values: %v", i, got)
		}
		for j, pair := range oldContext {
			if len(pair) != 2 || pair[0] != strings.TrimPrefix(tableCSVHeader[j], "context_") || pair[1] != got[j] {
				t.Fatalf("row %d context column %d disagrees with sectioned CSV: %v", i, j, pair)
			}
		}
		if !reflect.DeepEqual(got[17:], oldRows[i]) {
			t.Fatalf("row %d data cells disagree with sectioned CSV", i)
		}
	}
}

func TestWriteTableCSVEmptySelectionIsHeaderOnly(t *testing.T) {
	d := fixture()
	d.Rows = nil
	records, body := readTableCSV(t, d, testNow)
	if len(records) != 1 || !reflect.DeepEqual(records[0], tableCSVHeader) {
		t.Fatalf("empty selection fabricated data: %v", records)
	}
	if strings.Count(string(body), "\n") != 1 {
		t.Fatalf("header-only export contains extra records: %q", body)
	}
}

func TestWriteTableCSVValidationIsAtomicAndIOErrorsPropagate(t *testing.T) {
	for _, tt := range []struct {
		name string
		now  time.Time
		edit func(*Dataset)
	}{
		{name: "invalid dataset", now: testNow, edit: func(d *Dataset) { d.Contract.Unit = "" }},
		{name: "implicit clock", now: time.Time{}, edit: func(*Dataset) {}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := fixture()
			tt.edit(&d)
			var out bytes.Buffer
			if err := WriteTableCSV(&out, d, tt.now); err == nil || out.Len() != 0 {
				t.Fatalf("invalid input wrote bytes: err=%v body=%q", err, out.Bytes())
			}
		})
	}
	if err := WriteTableCSV(failingWriter{}, fixture(), testNow); err == nil || !strings.Contains(err.Error(), "storage unavailable") {
		t.Fatalf("writer failure was swallowed: %v", err)
	}
}

func TestWriteTableCSVHonorsMaximumRows(t *testing.T) {
	d := maxRowsDataset(false)
	records, _ := readTableCSV(t, d, maxRowsNow)
	if len(records) != MaxRows+1 || records[1][17] != "service-001" || records[MaxRows][17] != "service-200" {
		t.Fatalf("maximum selection lost rows or order: records=%d", len(records))
	}
	v := float64(MaxRows)
	d.Rows = append(d.Rows, Row{ID: "service-201", Label: "Service 201", Role: "accent", Value: &v})
	var out bytes.Buffer
	if err := WriteTableCSV(&out, d, maxRowsNow); err == nil || out.Len() != 0 {
		t.Fatalf("201 rows were accepted or partially written: err=%v bytes=%d", err, out.Len())
	}
}
