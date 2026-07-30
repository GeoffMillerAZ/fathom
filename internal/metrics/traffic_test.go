package metrics

import (
	"testing"
	"time"
)

func trafficWindow() (time.Time, time.Time) {
	return time.Unix(0, 0).UTC(), time.Unix(1<<32, 0).UTC()
}

func TestAggregateTraffic_GroupBySurface(t *testing.T) {
	since, until := trafficWindow()
	entries := []CallEntry{
		{TS: "2026-07-30T10:00:00Z", Tool: "find_symbol", Surface: "agent", LatencyMS: 100},
		{TS: "2026-07-30T10:01:00Z", Tool: "find_symbol", Surface: "agent", LatencyMS: 200},
		{TS: "2026-07-30T10:02:00Z", Tool: "ingest", Surface: "", LatencyMS: 50},
	}
	report := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until, GroupBy: TrafficGroupBySurface})
	if report.TotalCalls != 3 {
		t.Fatalf("TotalCalls = %d, want 3", report.TotalCalls)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("rows = %+v, want 2 (agent, unscoped)", report.Rows)
	}
	// Sorted: "" < "agent" lexically, and "(unscoped)" is what we render
	// for "" — check both present.
	var sawAgent, sawUnscoped bool
	for _, row := range report.Rows {
		if row.Group == "agent" {
			sawAgent = true
			if row.Calls != 2 {
				t.Errorf("agent calls = %d, want 2", row.Calls)
			}
		}
		if row.Group == "(unscoped)" {
			sawUnscoped = true
			if row.Calls != 1 {
				t.Errorf("unscoped calls = %d, want 1", row.Calls)
			}
		}
	}
	if !sawAgent || !sawUnscoped {
		t.Fatalf("rows = %+v, want both agent and (unscoped)", report.Rows)
	}
}

func TestAggregateTraffic_GroupByTool(t *testing.T) {
	since, until := trafficWindow()
	entries := []CallEntry{
		{TS: "2026-07-30T10:00:00Z", Tool: "find_symbol", LatencyMS: 100},
		{TS: "2026-07-30T10:01:00Z", Tool: "find_symbol", LatencyMS: 300},
		{TS: "2026-07-30T10:02:00Z", Tool: "ingest", LatencyMS: 50},
	}
	report := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until, GroupBy: TrafficGroupByTool})
	if len(report.Rows) != 2 {
		t.Fatalf("rows = %+v, want 2", report.Rows)
	}
	for _, row := range report.Rows {
		if row.Group == "find_symbol" {
			if row.Calls != 2 || row.P50MS == nil {
				t.Errorf("find_symbol row = %+v", row)
			}
		}
	}
}

func TestAggregateTraffic_GroupByHour(t *testing.T) {
	since, until := trafficWindow()
	entries := []CallEntry{
		{TS: "2026-07-30T10:15:00Z", Tool: "find_symbol", LatencyMS: 100},
		{TS: "2026-07-30T10:45:00Z", Tool: "find_symbol", LatencyMS: 100},
		{TS: "2026-07-30T11:05:00Z", Tool: "find_symbol", LatencyMS: 100},
	}
	report := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until, GroupBy: TrafficGroupByHour})
	if len(report.Rows) != 2 {
		t.Fatalf("rows = %+v, want 2 hour buckets", report.Rows)
	}
	if report.Rows[0].Group != "2026-07-30T10:00" || report.Rows[0].Calls != 2 {
		t.Errorf("row0 = %+v", report.Rows[0])
	}
	if report.Rows[1].Group != "2026-07-30T11:00" || report.Rows[1].Calls != 1 {
		t.Errorf("row1 = %+v", report.Rows[1])
	}
}

func TestAggregateTraffic_ErrorRate_Computed(t *testing.T) {
	since, until := trafficWindow()
	entries := []CallEntry{
		{TS: "2026-07-30T10:00:00Z", Tool: "find_symbol", Surface: "agent", LatencyMS: 100},
		{TS: "2026-07-30T10:01:00Z", Tool: "find_symbol", Surface: "agent", LatencyMS: 100, Error: "boom"},
	}
	report := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until})
	if len(report.Rows) != 1 {
		t.Fatalf("rows = %+v", report.Rows)
	}
	row := report.Rows[0]
	if row.ErrorRate == nil || *row.ErrorRate != 50 {
		t.Fatalf("ErrorRate = %v, want 50", row.ErrorRate)
	}
	if row.ErrorRateNote != "" {
		t.Errorf("ErrorRateNote = %q, want empty on a real computation", row.ErrorRateNote)
	}
}

// TestAggregateTraffic_DiscoveryPlumbingOtherSplit_Exhaustive: every tool
// lands in exactly one bucket, no double count.
func TestAggregateTraffic_DiscoveryPlumbingOtherSplit_Exhaustive(t *testing.T) {
	since, until := trafficWindow()
	entries := []CallEntry{
		{TS: "2026-07-30T10:00:00Z", Tool: "find_symbol"},   // discovery
		{TS: "2026-07-30T10:00:01Z", Tool: "references"},    // discovery
		{TS: "2026-07-30T10:00:02Z", Tool: "list_services"}, // plumbing
		{TS: "2026-07-30T10:00:03Z", Tool: "ingest"},        // plumbing
		{TS: "2026-07-30T10:00:04Z", Tool: "compile_layer"}, // other
	}
	report := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until})
	if report.DiscoveryCalls != 2 || report.PlumbingCalls != 2 || report.OtherCalls != 1 {
		t.Fatalf("split = discovery=%d plumbing=%d other=%d, want 2/2/1", report.DiscoveryCalls, report.PlumbingCalls, report.OtherCalls)
	}
	if sum := report.DiscoveryCalls + report.PlumbingCalls + report.OtherCalls; sum != report.TotalCalls {
		t.Errorf("split sum = %d, want TotalCalls %d", sum, report.TotalCalls)
	}
}

func TestAggregateTraffic_EmptyInput_EmptyReport(t *testing.T) {
	since, until := trafficWindow()
	report := AggregateTraffic(nil, TrafficOptions{Since: since, Until: until})
	if len(report.Rows) != 0 || report.TotalCalls != 0 {
		t.Fatalf("report = %+v, want empty", report)
	}
	if report.Note == "" {
		t.Error("Note is empty, want an honest empty-window note")
	}
}

func TestAggregateTraffic_WindowFiltersOutOfRangeCalls(t *testing.T) {
	since := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 7, 30, 23, 59, 59, 0, time.UTC)
	entries := []CallEntry{
		{TS: "2026-07-29T10:00:00Z", Tool: "find_symbol"}, // before window
		{TS: "2026-07-30T10:00:00Z", Tool: "find_symbol"}, // in window
		{TS: "2026-07-31T10:00:00Z", Tool: "find_symbol"}, // after window
	}
	report := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until})
	if report.TotalCalls != 1 {
		t.Fatalf("TotalCalls = %d, want 1", report.TotalCalls)
	}
}

func TestAggregateTraffic_MalformedTimestamp_Skipped(t *testing.T) {
	since, until := trafficWindow()
	entries := []CallEntry{
		{TS: "not-a-timestamp", Tool: "find_symbol"},
		{TS: "2026-07-30T10:00:00Z", Tool: "find_symbol"},
	}
	report := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until})
	if report.TotalCalls != 1 {
		t.Fatalf("TotalCalls = %d, want 1 (malformed timestamp skipped)", report.TotalCalls)
	}
}
