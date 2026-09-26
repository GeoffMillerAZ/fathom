package mcp

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

// TestCallLogger_SurfaceTraffic_CountsAndLastTS is A1-1's instrument-repair
// unit (docs/plans/activation.md): server_stats needs "is anything flowing
// on /rpc/agent?" answerable from the daemon without tailing the JSONL call
// log, so CallLogger itself accumulates process-lifetime per-surface call
// counts and last-call timestamps as it logs — the same site that already
// sees every request's resolved surface.
func TestCallLogger_SurfaceTraffic_CountsAndLastTS(t *testing.T) {
	var buf bytes.Buffer
	l := NewCallLogger(&buf)

	// Zero calls on a WIRED logger is a non-nil empty map — "instrument
	// live, zero traffic" — never nil, which is reserved for "no logger at
	// all" (bounce finding 3 / K3's existence check).
	if got := l.SurfaceTraffic(); got == nil || len(got) != 0 {
		t.Fatalf("SurfaceTraffic() before any calls = %v, want non-nil empty map", got)
	}

	l.log("find_symbol", "fathom", "shared", "agent", "", "", false, 0, nil, callOutcome{}, time.Millisecond, nil)
	l.log("outline", "fathom", "shared", "agent", "", "", false, 0, nil, callOutcome{}, time.Millisecond, nil)
	l.log("secret_scan", "fathom", "shared", "", "", "", false, 0, nil, callOutcome{}, time.Millisecond, nil)

	traffic := l.SurfaceTraffic()
	if len(traffic) != 2 {
		t.Fatalf("SurfaceTraffic() = %v, want 2 distinct surfaces", traffic)
	}
	agent, ok := traffic["agent"]
	if !ok {
		t.Fatalf("SurfaceTraffic() missing %q: %v", "agent", traffic)
	}
	if agent.Calls != 2 {
		t.Errorf("agent surface calls = %d, want 2", agent.Calls)
	}
	if agent.LastTS == "" {
		t.Error("agent surface last_ts is empty, want a timestamp")
	}
	bare, ok := traffic[""]
	if !ok {
		t.Fatalf("SurfaceTraffic() missing the bare/unscoped surface: %v", traffic)
	}
	if bare.Calls != 1 {
		t.Errorf("bare surface calls = %d, want 1", bare.Calls)
	}

	// A later call on the same surface bumps LastTS forward (never backward)
	// and increments Calls again.
	before := traffic["agent"].LastTS
	time.Sleep(2 * time.Millisecond)
	l.log("repo_map", "fathom", "shared", "agent", "", "", false, 0, nil, callOutcome{}, time.Millisecond, nil)
	after := l.SurfaceTraffic()["agent"]
	if after.Calls != 3 {
		t.Errorf("agent surface calls after 3rd call = %d, want 3", after.Calls)
	}
	if after.LastTS == before {
		t.Errorf("agent surface last_ts did not advance: still %q", after.LastTS)
	}
}

// TestCallLogger_SurfaceTraffic_NilLoggerIsEmpty: a nil *CallLogger (call
// logging disabled) must not panic and reports no traffic — mirrors log's
// existing nil-receiver no-op contract.
func TestCallLogger_SurfaceTraffic_NilLoggerIsEmpty(t *testing.T) {
	var l *CallLogger
	if got := l.SurfaceTraffic(); got != nil {
		t.Errorf("nil CallLogger SurfaceTraffic() = %v, want nil", got)
	}
}

// TestCallLogVersionStamp pins IO-11 (vision 14 §3): every call-log row
// carries the schema version and the producing binary's version, so a
// trend fold can filter by the semantics that wrote each row instead of
// re-living the 251,834-catch class of cross-semantics aggregation error.
//
// certifies: invariant I32 (asserts)
func TestCallLogVersionStamp(t *testing.T) {
	var buf bytes.Buffer
	l := NewCallLogger(&buf)
	l.log("find_symbol", "svc", "nav", "agent", "", "", false, 0, nil, callOutcome{}, time.Millisecond, nil)
	var row map[string]any
	if err := json.Unmarshal(buf.Bytes(), &row); err != nil {
		t.Fatalf("unmarshal: %v (%q)", err, buf.String())
	}
	if row["schema_version"] != float64(2) {
		t.Errorf("schema_version = %v, want 2", row["schema_version"])
	}
	if v, _ := row["fathom_version"].(string); v == "" {
		t.Error("fathom_version must never be empty — 'unknown' is the floor")
	}
}

// TestCallLogOutcomes pins IQ-1 (docs/plans/impact-questions.md): logged
// calls carry result_count and matched_via so hits, did_you_mean
// consolations and empty misses are distinguishable fleet-wide.
func TestCallLogOutcomes(t *testing.T) {
	mkResult := func(text string) any {
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}
	}
	cases := []struct {
		name      string
		result    any
		wantCount int
		hasCount  bool
		wantVia   string
	}{
		{"hit with symbols", mkResult(`{"symbols":[{"name":"a"},{"name":"b"}]}`), 2, true, "hit"},
		{"result's own matched_via wins", mkResult(`{"symbols":[{"name":"a"}],"matched_via":"exact"}`), 1, true, "exact"},
		{"did_you_mean consolation", mkResult(`{"symbols":[],"did_you_mean":[{"name":"x"}],"matched_via":"none"}`), 0, true, "did_you_mean"},
		{"empty miss", mkResult(`{"symbols":[]}`), 0, true, "miss"},
		{"nodes list counts", mkResult(`{"nodes":[{"n":1},{"n":2},{"n":3}]}`), 3, true, "hit"},
		{"no recognizable list", mkResult(`{"ok":true}`), 0, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			count, via := outcomeFrom(c.result)
			if c.hasCount {
				if count == nil || *count != c.wantCount {
					t.Fatalf("result_count = %v, want %d", count, c.wantCount)
				}
			} else if count != nil {
				t.Fatalf("result_count = %v, want nil (not extractable)", *count)
			}
			if via != c.wantVia {
				t.Errorf("matched_via = %q, want %q", via, c.wantVia)
			}
		})
	}

	var buf bytes.Buffer
	l := NewCallLogger(&buf)
	n := 2
	l.log("find_symbol", "svc", "nav", "agent", "", "", false, 0, nil, callOutcome{ResultCount: &n, MatchedVia: "hit"}, 0, nil)
	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["result_count"] != float64(2) || entry["matched_via"] != "hit" {
		t.Errorf("logged entry missing outcomes: %v", entry)
	}
}

// TestResponseCostAccounting is BA-6: response bytes are recorded per call
// (calllog.go's own JSONL row, via callOutcome.ResponseBytes) and
// recordResponseCost totals them per tool across a fixture window — the
// aggregation seam, not a new log.
func TestResponseCostAccounting(t *testing.T) {
	var buf bytes.Buffer
	l := NewCallLogger(&buf)

	oneElided := 3
	zeroElided := 0
	l.log("query_symbols", "svc", "coding", "agent", "", "", false, 0, nil,
		callOutcome{ResponseBytes: 1000, ElidedCount: &oneElided}, time.Millisecond, nil)
	l.log("query_symbols", "svc", "coding", "agent", "", "", false, 0, nil,
		callOutcome{ResponseBytes: 500, ElidedCount: &zeroElided}, time.Millisecond, nil)
	l.log("find_symbol", "svc", "coding", "agent", "", "", false, 0, nil,
		callOutcome{ResponseBytes: 200}, time.Millisecond, nil) // no elided field at all: not a truncating call
	// A blank line and a malformed line must not derail the fold.
	buf.WriteString("\n{not json\n")

	totals := recordResponseCost(bytes.NewReader(buf.Bytes()))
	if len(totals) != 2 {
		t.Fatalf("recordResponseCost = %+v, want 2 distinct tools", totals)
	}

	byTool := map[string]ToolCost{}
	for _, tc := range totals {
		byTool[tc.Tool] = tc
	}

	qs, ok := byTool["query_symbols"]
	if !ok {
		t.Fatalf("query_symbols missing from totals: %+v", totals)
	}
	if qs.Calls != 2 {
		t.Errorf("query_symbols.Calls = %d, want 2", qs.Calls)
	}
	if qs.TotalBytes != 1500 {
		t.Errorf("query_symbols.TotalBytes = %d, want 1500 (1000+500)", qs.TotalBytes)
	}
	if qs.TruncatingCalls != 2 {
		t.Errorf("query_symbols.TruncatingCalls = %d, want 2 (both calls carried an elided field)", qs.TruncatingCalls)
	}
	if qs.ElidedCalls != 1 {
		t.Errorf("query_symbols.ElidedCalls = %d, want 1 (only the count=3 call actually elided)", qs.ElidedCalls)
	}

	fs, ok := byTool["find_symbol"]
	if !ok {
		t.Fatalf("find_symbol missing from totals: %+v", totals)
	}
	if fs.Calls != 1 || fs.TotalBytes != 200 {
		t.Errorf("find_symbol = %+v, want Calls=1 TotalBytes=200", fs)
	}
	if fs.TruncatingCalls != 0 || fs.ElidedCalls != 0 {
		t.Errorf("find_symbol = %+v, want zero truncating/elided (no elided field logged)", fs)
	}
}

// TestCallLogger_ConsoleReadsLoggedNotCounted: a call the console makes to
// render a page (origin OriginConsole) is written to the log with its
// origin — auditable — but never bumps the surface traffic server_stats
// reports: a page render must not make an idle surface look used.
func TestCallLogger_ConsoleReadsLoggedNotCounted(t *testing.T) {
	var buf bytes.Buffer
	l := NewCallLogger(&buf)
	l.log("list_services", "", "platform", "", "", OriginConsole, false, 0, nil, callOutcome{}, time.Millisecond, nil)
	if got := l.SurfaceTraffic(); len(got) != 0 {
		t.Errorf("a console read counted as surface traffic: %v", got)
	}
	var row struct {
		Tool   string `json:"tool"`
		Origin string `json:"origin"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &row); err != nil || row.Tool != "list_services" || row.Origin != OriginConsole {
		t.Errorf("console read row = %+v (%v), want list_services with origin %q", row, err, OriginConsole)
	}
	l.log("list_services", "", "platform", "", "", "", false, 0, nil, callOutcome{}, time.Millisecond, nil)
	if got := l.SurfaceTraffic()[""]; got.Calls != 1 {
		t.Errorf("an agent's list_services must still count: %+v", got)
	}
	if !IsConsoleRead(OriginConsole) || IsConsoleRead("") || IsConsoleRead("edit-hook") {
		t.Error("IsConsoleRead must match the console origin only")
	}
}
