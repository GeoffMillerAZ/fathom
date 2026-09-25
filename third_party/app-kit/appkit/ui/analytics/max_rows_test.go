package analytics

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/app-kit/appkit/ui/vizframe"
)

const (
	maxRowsHTMLBudget  = 256 << 10
	maxRowsAllocBudget = 4 << 20
	maxRowsTimeBudget  = 25 * time.Millisecond
)

var maxRowsNow = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func maxRowsDataset(long bool) Dataset {
	rows := make([]Row, MaxRows)
	for i := range rows {
		v := float64(i)
		row := Row{ID: fmt.Sprintf("service-%03d", i+1), Label: fmt.Sprintf("Service %03d", i+1), Role: "accent", Value: &v}
		if i%2 == 1 {
			row.Role = "info"
		}
		if i > 0 && i%17 == 0 {
			row.Value = nil
			row.MissingReason = "Synthetic source unavailable for this selected row."
		} else if (i > 0 && i%13 == 0) || i == MaxRows-1 {
			row.Bounds = &Interval{Low: v - .25, High: v + .25}
		}
		rows[i] = row
	}
	if long {
		rows[0].Label = strings.Repeat("Long service label ", 18)
	}
	return Dataset{
		Contract: Contract{
			ID: "maximum-service-events", Title: "Selected service events", Unit: "events", Decimals: 2,
			Grain: "selected service", Aggregation: "synthetic event count per selected service",
			Window:       Window{Kind: RangeWindow, Start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), TimeZone: "UTC"},
			Source:       vizframe.Derived("Synthetic scale fixture; no operational data or claim."),
			Freshness:    Freshness{ObservedAt: time.Date(2026, 10, 5, 11, 0, 0, 0, time.UTC), MaxAge: 2 * time.Hour},
			Completeness: Partial, CoverageNote: "11 unavailable of 200 selected rows.",
			Uncertainty: "Synthetic scale fixture; uncertainty is not quantified.",
		},
		Rows: rows,
		Legend: []LegendItem{
			{Role: "accent", Label: "API services"},
			{Role: "info", Label: "Worker services"},
		},
	}
}

func maxRowsCSV(t testing.TB, d Dataset) ([][]string, []byte) {
	t.Helper()
	var out bytes.Buffer
	if err := WriteCSV(&out, d, maxRowsNow); err != nil {
		t.Fatal(err)
	}
	r := csv.NewReader(bytes.NewReader(out.Bytes()))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range records {
		if len(row) == 10 && row[0] == "id" {
			return records[i+1:], out.Bytes()
		}
	}
	t.Fatal("CSV data header missing")
	return nil, nil
}

func TestMaximumRowsRetainOrderedMeaningAcrossHTMLAndCSV(t *testing.T) {
	d := maxRowsDataset(false)
	want := maxRowsDataset(false)
	h, err := RenderBars(d, viz.BarsOptions{}, maxRowsNow)
	if err != nil {
		t.Fatal(err)
	}
	rows, csvBody := maxRowsCSV(t, d)
	if !reflect.DeepEqual(d, want) {
		t.Fatal("render or export mutated the caller's dataset")
	}
	if len(h) > maxRowsHTMLBudget || len(rows) != MaxRows || len(csvBody) == 0 {
		t.Fatalf("maximum output changed: html=%d csv=%d rows=%d", len(h), len(csvBody), len(rows))
	}
	table := h[strings.Index(h, "<tbody>"):]
	if strings.Count(h, `fill-opacity="0.5"`) != 189 || strings.Count(table, `<tr class="border-b border-line">`) != MaxRows {
		t.Fatalf("HTML lost plotted or table rows: tracks=%d table=%d", strings.Count(h, `fill-opacity="0.5"`), strings.Count(table, `<tr class="border-b border-line">`))
	}
	missing, bounds := 0, 0
	for i, got := range rows {
		wantID, wantLabel := fmt.Sprintf("service-%03d", i+1), fmt.Sprintf("Service %03d", i+1)
		wantRole, wantMeaning := "accent", "API services"
		if i%2 == 1 {
			wantRole, wantMeaning = "info", "Worker services"
		}
		if got[0] != wantID || got[1] != wantLabel || got[8] != wantRole || got[9] != wantMeaning {
			t.Fatalf("row %d identity/meaning changed: %v", i, got)
		}
		if i > 0 && i%17 == 0 {
			missing++
			if got[2] != "" || got[4] != "missing" || got[5] == "" || got[6] != "" || got[7] != "" {
				t.Fatalf("missing row %d changed: %v", i, got)
			}
			continue
		}
		if got[2] != strconv.Itoa(i) || got[4] != "present" || got[5] != "" {
			t.Fatalf("present row %d changed: %v", i, got)
		}
		if (i > 0 && i%13 == 0) || i == MaxRows-1 {
			bounds++
			if got[6] != fmt.Sprintf("%.2f", float64(i)-.25) || got[7] != fmt.Sprintf("%.2f", float64(i)+.25) {
				t.Fatalf("bounds row %d changed: %v", i, got)
			}
		} else if got[6] != "" || got[7] != "" {
			t.Fatalf("row %d invented bounds: %v", i, got)
		}
	}
	if missing != 11 || bounds != 16 || rows[0][2] != "0" {
		t.Fatalf("truth-state counts changed: missing=%d bounds=%d zero=%q", missing, bounds, rows[0][2])
	}
}

func TestMaximumRowsLongFallbackRetainsAllRowsWithoutOrphanLegend(t *testing.T) {
	d := maxRowsDataset(true)
	want := maxRowsDataset(true)
	h, err := RenderBars(d, viz.BarsOptions{}, maxRowsNow)
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := maxRowsCSV(t, d)
	if strings.Contains(h, "<svg") || strings.Contains(h, `aria-label="Chart legend"`) || !strings.Contains(h, "Bar chart omitted") {
		t.Fatal("long selection did not use the explicit table-only fallback")
	}
	table := h[strings.Index(h, "<tbody>"):]
	if len(rows) != MaxRows || strings.Count(table, `<tr class="border-b border-line">`) != MaxRows || !strings.Contains(h, d.Rows[0].Label) {
		t.Fatal("table-only fallback lost selected rows")
	}
	if !reflect.DeepEqual(d, want) {
		t.Fatal("fallback render or export mutated the caller's dataset")
	}
}

func TestMaximumRowsPlusOneRefusesBeforeOutput(t *testing.T) {
	d := maxRowsDataset(false)
	v := float64(MaxRows)
	d.Rows = append(d.Rows, Row{ID: "service-201", Label: "Service 201", Role: "accent", Value: &v})
	if err := d.Validate(maxRowsNow); err == nil || !strings.Contains(err.Error(), "row limit") {
		t.Fatalf("Validate accepted 201 rows: %v", err)
	}
	if h, err := RenderBars(d, viz.BarsOptions{}, maxRowsNow); err == nil || h != "" {
		t.Fatalf("RenderBars accepted or emitted 201 rows: bytes=%d err=%v", len(h), err)
	}
	var out bytes.Buffer
	if err := WriteCSV(&out, d, maxRowsNow); err == nil || out.Len() != 0 {
		t.Fatalf("WriteCSV accepted or emitted 201 rows: bytes=%d err=%v", out.Len(), err)
	}
}

func benchmarkMaximumRowsRender(b *testing.B) {
	d := maxRowsDataset(false)
	b.ReportAllocs()
	b.ReportMetric(float64(maxRowsTimeBudget.Nanoseconds()), "local-time-ceiling-ns/op")
	b.ReportMetric(float64(maxRowsAllocBudget), "local-allocation-ceiling-B/op")
	for i := 0; i < b.N; i++ {
		h, err := RenderBars(d, viz.BarsOptions{}, maxRowsNow)
		if err != nil || len(h) == 0 || len(h) > maxRowsHTMLBudget {
			b.Fatalf("render failed budget: bytes=%d err=%v", len(h), err)
		}
	}
}

func benchmarkMaximumRowsCSV(b *testing.B) {
	d := maxRowsDataset(false)
	b.ReportAllocs()
	b.ReportMetric(float64(maxRowsTimeBudget.Nanoseconds()), "local-time-ceiling-ns/op")
	b.ReportMetric(float64(maxRowsAllocBudget), "local-allocation-ceiling-B/op")
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		if err := WriteCSV(&out, d, maxRowsNow); err != nil || out.Len() == 0 {
			b.Fatalf("CSV failed: bytes=%d err=%v", out.Len(), err)
		}
	}
}

func BenchmarkMaximumRowsRender(b *testing.B) { benchmarkMaximumRowsRender(b) }
func BenchmarkMaximumRowsCSV(b *testing.B)    { benchmarkMaximumRowsCSV(b) }
