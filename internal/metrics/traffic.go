package metrics

// traffic.go — Q-10 (docs/plans/expert-suite-q.md): `fathom metrics
// traffic`'s pure aggregation core over the MCP call log — volume, p50/p95
// latency, error rate, and a discovery/plumbing/other split, grouped by
// surface, tool, or hour. Pure: no I/O — cmd/fathom's cmdMetricsTraffic
// reads the call log (readCallLog/resolveCallLogPath, already in
// cmd/fathom) and converts each adoptionCallEntry into a CallEntry here.

import (
	"math"
	"sort"
	"time"
)

// TrafficGroupBySurface/Tool/Hour are AggregateTraffic's valid --by values.
const (
	TrafficGroupBySurface = "surface"
	TrafficGroupByTool    = "tool"
	TrafficGroupByHour    = "hour"
)

// CallEntry is the minimal call-log shape AggregateTraffic needs — decoupled
// from cmd/fathom's adoptionCallEntry (which cannot be imported here; this
// package cannot import package main) the same way adoptionCallEntry itself
// is decoupled from internal/mcp's own unexported wire type.
type CallEntry struct {
	TS        string
	Tool      string
	Service   string
	Surface   string
	LatencyMS float64
	Error     string
}

// TrafficOptions configures AggregateTraffic.
type TrafficOptions struct {
	Since, Until time.Time
	// GroupBy is one of TrafficGroupBySurface (default), TrafficGroupByTool,
	// TrafficGroupByHour.
	GroupBy string
}

// TrafficRow is one group's aggregate traffic.
type TrafficRow struct {
	Group         string   `json:"group"`
	Calls         int      `json:"calls"`
	P50MS         *float64 `json:"p50_ms"`
	P95MS         *float64 `json:"p95_ms"`
	ErrorRate     *float64 `json:"error_rate_percent"`
	ErrorRateNote string   `json:"error_rate_note,omitempty"`
}

// TrafficReport is AggregateTraffic's output.
type TrafficReport struct {
	Since   string       `json:"since"`
	Until   string       `json:"until"`
	GroupBy string       `json:"group_by"`
	Rows    []TrafficRow `json:"rows"`

	TotalCalls int `json:"total_calls"`

	// DiscoveryCalls/PlumbingCalls/OtherCalls partition TotalCalls
	// exhaustively — every call lands in exactly one bucket, so the three
	// always sum to TotalCalls (design §4's own plumbing-vs-discovery
	// split, computed over CALL-LOG traffic here — a different
	// denominator from the discovery-observer beats phase P's
	// buildDiscoverySection reads; see channels.go's own note on that
	// distinction).
	DiscoveryCalls int    `json:"discovery_calls"`
	PlumbingCalls  int    `json:"plumbing_calls"`
	OtherCalls     int    `json:"other_calls"`
	Note           string `json:"note"`
}

// AggregateTraffic groups call-log entries (already read and scoped to
// service by the caller) by opts.GroupBy and computes each row's volume,
// latency percentiles, and error rate.
func AggregateTraffic(entries []CallEntry, opts TrafficOptions) TrafficReport {
	groupBy := opts.GroupBy
	if groupBy == "" {
		groupBy = TrafficGroupBySurface
	}
	r := TrafficReport{
		Since:   opts.Since.UTC().Format(time.RFC3339),
		Until:   opts.Until.UTC().Format(time.RFC3339),
		GroupBy: groupBy,
	}
	if len(entries) == 0 {
		r.Note = "no calls in window"
		return r
	}

	type acc struct {
		latencies []float64
		errors    int
	}
	byGroup := map[string]*acc{}
	for _, e := range entries {
		ts, ok := parseTrafficTS(e.TS)
		if !ok || ts.Before(opts.Since) || ts.After(opts.Until) {
			continue
		}
		key := trafficGroupKey(e, groupBy, ts)
		a, ok := byGroup[key]
		if !ok {
			a = &acc{}
			byGroup[key] = a
		}
		a.latencies = append(a.latencies, e.LatencyMS)
		if e.Error != "" {
			a.errors++
		}
		r.TotalCalls++
		switch {
		case IsDiscoveryIndexTool(e.Tool):
			r.DiscoveryCalls++
		case IsPlumbingTool(e.Tool):
			r.PlumbingCalls++
		default:
			r.OtherCalls++
		}
	}
	if r.TotalCalls == 0 {
		r.Note = "no calls in window"
		return r
	}
	r.Note = "discovery/plumbing/other partition call-log traffic exhaustively (docs/design/production-tokenomics.md §4's exclusion list)"

	keys := make([]string, 0, len(byGroup))
	for k := range byGroup {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := byGroup[k]
		row := TrafficRow{Group: k, Calls: len(a.latencies)}
		sorted := append([]float64(nil), a.latencies...)
		sort.Float64s(sorted)
		p50 := percentile(sorted, 0.50)
		p95 := percentile(sorted, 0.95)
		row.P50MS, row.P95MS = &p50, &p95
		if pct, note := Ratio(a.errors, len(a.latencies)); note != "" {
			row.ErrorRateNote = note
		} else {
			row.ErrorRate = pct
		}
		r.Rows = append(r.Rows, row)
	}
	return r
}

// trafficGroupKey resolves one entry's group key for the given GroupBy
// value.
func trafficGroupKey(e CallEntry, groupBy string, ts time.Time) string {
	switch groupBy {
	case TrafficGroupByTool:
		return e.Tool
	case TrafficGroupByHour:
		return ts.UTC().Format("2006-01-02T15:00")
	default: // TrafficGroupBySurface
		if e.Surface == "" {
			return "(unscoped)"
		}
		return e.Surface
	}
}

// parseTrafficTS parses a call-log entry's RFC3339Nano timestamp, falling
// back to plain RFC3339 for older/hand-written fixtures — mirrors
// research_report.go's parseCallLogTS exactly (a tiny, self-contained
// helper; not worth a cross-package extraction for two call sites).
func parseTrafficTS(ts string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// percentile returns the nearest-rank percentile p (0..1) of a SORTED,
// non-empty slice — mirrors research_report.go's own percentile exactly
// (ceil(p*n), 1-indexed, clamped to [0, n-1]).
func percentile(sorted []float64, p float64) float64 {
	n := len(sorted)
	idx := int(math.Ceil(p*float64(n))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}
