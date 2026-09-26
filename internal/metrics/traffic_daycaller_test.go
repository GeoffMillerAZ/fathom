package metrics

import "testing"

// The console's Operate page (OC-44) reads traffic per UTC day and per
// caller class: a day key is the calendar date, a caller key is the class
// the reader stamped, and an entry the reader did not classify is named as
// unclassified rather than folded into a class.
func TestAggregateTraffic_GroupByDayAndCaller(t *testing.T) {
	since, until := trafficWindow()
	entries := []CallEntry{
		{TS: "2026-09-24T23:59:00Z", Tool: "find_symbol", Surface: "agent", Caller: "agent", LatencyMS: 10},
		{TS: "2026-09-25T00:01:00Z", Tool: "obligations", Caller: "edit hook", LatencyMS: 20},
		{TS: "2026-09-25T08:00:00Z", Tool: "verify_plan", Caller: "gate", LatencyMS: 30, Error: "boom"},
		{TS: "2026-09-25T09:00:00Z", Tool: "list_services", LatencyMS: 5},
	}
	day := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until, GroupBy: TrafficGroupByDay})
	if day.GroupBy != "day" || len(day.Rows) != 2 || day.Rows[0].Group != "2026-09-24" || day.Rows[1].Group != "2026-09-25" || day.Rows[1].Calls != 3 {
		t.Fatalf("by day = %+v", day.Rows)
	}
	caller := AggregateTraffic(entries, TrafficOptions{Since: since, Until: until, GroupBy: TrafficGroupByCaller})
	got := map[string]int{}
	for _, r := range caller.Rows {
		got[r.Group] = r.Calls
	}
	want := map[string]int{"agent": 1, "edit hook": 1, "gate": 1, "(unclassified)": 1}
	if len(got) != len(want) {
		t.Fatalf("by caller = %+v", caller.Rows)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("caller %q = %d, want %d", k, got[k], v)
		}
	}
}
