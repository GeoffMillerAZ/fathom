package main

// callfold_response_cost_test.go — BA-6 (docs/plans/ambient-fathom.md): a
// call's response bytes are recorded on its call-log row and totalled per
// tool across a window — on the path a product page and verb read, not on a
// library seam. The row's product anchor is callLogFold.add: the console's
// tail and every verb fold a call-log read through it, and the cost panel
// (BA-8, cost_panel.go) renders the totals it keeps.

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// responseCostFixture is a call log across two days. The console's own read,
// a torn line and a blank line are noise a reader must survive; the find_symbol
// row without response_bytes is a call logged before BA-6, "not accounted",
// never a zero-byte response.
const responseCostFixture = `{"ts":"2026-09-28T10:00:00Z","tool":"query_symbols","service":"svc","surface":"agent","response_bytes":1000,"elided_count":3}
{"ts":"2026-09-28T10:00:05Z","tool":"query_symbols","service":"svc","surface":"agent","response_bytes":500,"elided_count":0}

{"ts":"2026-09-28T10:00:09Z","tool":"query_symbols","service":"svc","surface":"agent","origin":"console","response_bytes":9999}
{"ts":"2026-09-29T09:00:00Z","tool":"find_symbol","service":"svc","surface":"agent","response_bytes":200}
{torn
{"ts":"2026-09-29T09:00:07Z","tool":"find_symbol","service":"svc","surface":"agent"}
{"ts":"2026-09-29T09:00:11Z","tool":"search","service":"svc","surface":"agent","response_bytes":300,"elided_count":1}
`

// writeCallLog writes body as a call log and returns its path.
func writeCallLog(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "calls.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// fixedFleet is a fleet of n services with no symbols: list_services answers
// with an envelope whose size grows with n.
type fixedFleet struct{ n int }

func (f fixedFleet) Services() []string {
	out := make([]string, f.n)
	for i := range out {
		out[i] = "service-" + strings.Repeat("x", i+1)
	}
	return out
}
func (fixedFleet) SymbolsFor(string) []core.SymbolNode           { return nil }
func (fixedFleet) ObservationsFor(string) []core.Observation     { return nil }
func (fixedFleet) UnsupportedLanguagesFor(string) map[string]int { return nil }

// TestCallLogFoldTotalsResponseBytesPerTool is BA-6: over a fixture call log
// each call's response bytes are on its row, and callLogFold totals them per
// tool — the totals the cost panel renders. A second leg records real calls
// through the MCP server and proves the bytes on each row are the size of the
// envelope that call returned.
func TestCallLogFoldTotalsResponseBytesPerTool(t *testing.T) {
	entries, skipped, read := readCallLog(writeCallLog(t, responseCostFixture))
	if !read {
		t.Fatal("the fixture call log was not read")
	}
	if skipped != 1 {
		t.Errorf("skipped malformed = %d, want 1 (the torn line)", skipped)
	}
	// The console's own read is not traffic; the blank line is not a row.
	wantBytes := []int{1000, 500, 200, 0, 300}
	if len(entries) != len(wantBytes) {
		t.Fatalf("%d rows read, want %d (console read and torn line left out): %+v", len(entries), len(wantBytes), entries)
	}
	for i, want := range wantBytes {
		if entries[i].ResponseBytes != want {
			t.Errorf("row %d (%s) response_bytes = %d, want %d", i, entries[i].Tool, entries[i].ResponseBytes, want)
		}
	}

	fold := foldCallLog(entries)
	for tool, want := range map[string]toolFold{
		"query_symbols": {calls: 2, totalBytes: 1500, truncating: 2, elided: 1},
		"find_symbol":   {calls: 2, totalBytes: 200},
		"search":        {calls: 1, totalBytes: 300, truncating: 1, elided: 1},
	} {
		got := fold.tools[tool]
		if got.calls != want.calls || got.totalBytes != want.totalBytes || got.truncating != want.truncating || got.elided != want.elided {
			t.Errorf("fold.tools[%s] = %+v, want calls=%d totalBytes=%d truncating=%d elided=%d",
				tool, got, want.calls, want.totalBytes, want.truncating, want.elided)
		}
	}
	if len(fold.tools) != 3 {
		t.Errorf("fold names %d tools, want 3: %+v", len(fold.tools), fold.tools)
	}

	// The console's tail adds each row once as it is read; it must total the
	// same bytes as the verbs' whole-read fold.
	tail := newCallLogFold()
	for i := range entries {
		tail.add(&entries[i])
	}
	for tool, want := range fold.tools {
		if got := tail.tools[tool]; got.totalBytes != want.totalBytes || got.calls != want.calls {
			t.Errorf("tail fold of %s = %+v, whole-read fold = %+v", tool, got, want)
		}
	}

	// The cost panel renders the fold's totals: bytes descending, a token
	// estimate per tool, and an unaccounted call adds a call but no bytes.
	cp := fold.costPanel(8, 8000, nil)
	if cp.TotalBytes != 2000 || cp.TotalEstTokens != 500 {
		t.Errorf("panel total = %d bytes / %d est tokens, want 2000 / 500", cp.TotalBytes, cp.TotalEstTokens)
	}
	var order []string
	for _, r := range cp.Tools {
		order = append(order, r.Tool)
		if r.TotalBytes != fold.tools[r.Tool].totalBytes || r.EstTokens != r.TotalBytes/costEstBytesPerToken {
			t.Errorf("panel row %+v does not carry the fold's %d bytes", r, fold.tools[r.Tool].totalBytes)
		}
	}
	if got, want := strings.Join(order, ","), "query_symbols,search,find_symbol"; got != want {
		t.Errorf("panel tool order = %s, want %s (most bytes first)", got, want)
	}

	// Real calls: the server records each call's envelope size on its row.
	var logged bytes.Buffer
	logger := mcp.NewCallLogger(&logged)
	small := mcp.NewServerWithFleetAndLogger(nil, fixedFleet{n: 0}, logger)
	large := mcp.NewServerWithFleetAndLogger(nil, fixedFleet{n: 6}, logger)
	var envelopes []int
	call := func(srv *mcp.Server) {
		t.Helper()
		resp, _ := srv.HandleRequest(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_services","arguments":{}}}`))
		var rpc struct {
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal(resp, &rpc); err != nil || len(rpc.Result) == 0 || len(rpc.Error) != 0 {
			t.Fatalf("list_services did not answer a result: %s (%v)", resp, err)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, rpc.Result); err != nil {
			t.Fatal(err)
		}
		envelopes = append(envelopes, compact.Len())
	}
	call(small)
	call(large)
	call(small)
	if envelopes[0] == envelopes[1] {
		t.Fatalf("the fixture fleets answer equal envelopes (%d B): the leg cannot tell one call's bytes from another's", envelopes[0])
	}

	liveEntries, _, ok := readCallLog(writeCallLog(t, logged.String()))
	if !ok || len(liveEntries) != len(envelopes) {
		t.Fatalf("read %d live rows (ok=%v), want %d", len(liveEntries), ok, len(envelopes))
	}
	want := 0
	for i, e := range liveEntries {
		if e.Tool != "list_services" || e.ResponseBytes != envelopes[i] {
			t.Errorf("live row %d = %s / %d B, want list_services / %d B (the envelope it returned)", i, e.Tool, e.ResponseBytes, envelopes[i])
		}
		want += envelopes[i]
	}
	if got := foldCallLog(liveEntries).tools["list_services"]; got.calls != 3 || got.totalBytes != want {
		t.Errorf("live fold list_services = %d calls / %d B, want 3 / %d B", got.calls, got.totalBytes, want)
	}
}
