package main

// console_origin_test.go — the F0 proof's second defect: the console's own
// page renders wrote list_services rows into the call log it reports as
// usage, so Fleet's "tool calls", its tool-load chart, Impact's per-tool
// table and Now's logged calls all counted the console reading itself. The
// console's reads now carry the console origin (mcp.OriginConsole) — logged,
// auditable — and every usage reader leaves them out.

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func TestConsoleReads_TaggedAndNeverCountedAsUsage(t *testing.T) {
	f := newConsoleFixtureOpts(t, consoleFixtureOpts{LogCalls: true})
	before, _ := os.ReadFile(f.callLog)

	// Every model-backed page and the fleet page's verb section read the
	// registry; drop the model cache between them so each render reads.
	for _, path := range []string{"/", "/fleet", "/impact", "/brief", "/sessions"} {
		resetObsCache()
		f.get(t, path)
	}
	resetObsCache()
	f.pageJSON(t, "/fleet")

	after, _ := os.ReadFile(f.callLog)
	added := strings.Split(strings.TrimSpace(strings.TrimPrefix(string(after), string(before))), "\n")
	if len(added) < 6 {
		t.Fatalf("the console's reads must stay in the log (auditable): %d rows added", len(added))
	}
	for _, line := range added {
		var row struct{ Tool, Origin string }
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			t.Fatal(err)
		}
		if row.Origin != mcp.OriginConsole {
			t.Errorf("a console read logged without the console origin: %s", line)
		}
	}

	// The shared call total, the fleet tile, the tool-load table and the
	// verbs all count the fixture's five rows — never the console's reads —
	// and the total names what it left out.
	resetObsCache()
	env := f.pageJSON(t, "/fleet")
	var calls console.Value[int]
	if err := json.Unmarshal(env.Metrics[string(console.MetricCalls)], &calls); err != nil {
		t.Fatal(err)
	}
	if calls.V != f.callRows || !strings.Contains(calls.Denominator, "console page reads excluded (not usage)") {
		t.Errorf("calls.total = %+v, want the fixture's %d rows with the console's reads named as excluded", calls, f.callRows)
	}
	_, _, fleet := f.get(t, "/fleet")
	if got := tileValue(t, fleet, "tool calls (log)"); got != fmtThousands(f.callRows) {
		t.Errorf("fleet tile shows %q calls, want %d", got, f.callRows)
	}
	var model observatoryModel
	if err := json.Unmarshal(env.Model, &model); err != nil {
		t.Fatal(err)
	}
	for _, miss := range model.Misses {
		if miss.Tool == "list_services" && miss.Calls != 1 {
			t.Errorf("the tool-load table counts %d list_services calls, want the fixture's 1 (the console's own excluded)", miss.Calls)
		}
	}
	var out, errb bytes.Buffer
	if code := run([]string{"metrics", "traffic", "--json", "--call-log", f.callLog, "--from", "2026-01-01T00:00:00Z", "--to", "2031-01-01T00:00:00Z"}, &out, &errb); code != 0 {
		t.Fatalf("metrics traffic: %d %s", code, errb.String())
	}
	var traffic struct {
		TotalCalls int `json:"total_calls"`
	}
	if err := json.Unmarshal(out.Bytes(), &traffic); err != nil {
		t.Fatal(err)
	}
	if traffic.TotalCalls != f.callRows {
		t.Errorf("metrics traffic counts %d calls, want %d (console reads are not traffic)", traffic.TotalCalls, f.callRows)
	}
	final, _ := os.ReadFile(f.callLog)
	logged := strings.Count(string(final), `"origin":"console"`)
	entries, _, consoleReads, err := callLogReadCounting(f.callLog)
	if err != nil || len(entries) != f.callRows || consoleReads != logged || logged < len(added) {
		t.Errorf("callLogReadCounting = %d entries, %d console reads, %v; want %d and the %d logged", len(entries), consoleReads, err, f.callRows, logged)
	}
}
