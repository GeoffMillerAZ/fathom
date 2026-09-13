package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ─── resultPointers (RC-25) ──────────────────────────────────────────────

func TestResultPointers_FindSymbol(t *testing.T) {
	result := map[string]any{
		"content": []any{map[string]any{"type": "text", "text": `{"symbols":[{"name":"registerReferences","file_path":"internal/mcp/references_tool.go","line":41}],"matched_via":"exact"}`}},
	}
	got := resultPointers("find_symbol", json.RawMessage(`{"service":"fathom","name":"registerReferences"}`), result)
	want := []string{"internal/mcp.registerReferences"}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("resultPointers(find_symbol) = %v, want %v", got, want)
	}
}

func TestResultPointers_References_QueryPlusFiles(t *testing.T) {
	result := map[string]any{
		"content": []any{map[string]any{"type": "text", "text": `{"query":"PutObservations","definitions":[{"file":"internal/store/store.go","line":10}],"occurrences":[{"loc":"internal/mcp/memory_tools.go:20"}]}`}},
	}
	got := resultPointers("references", json.RawMessage(`{"query":"PutObservations"}`), result)
	want := []string{"PutObservations", "internal/store/store.go", "internal/mcp/memory_tools.go"}
	if len(got) != len(want) {
		t.Fatalf("resultPointers(references) = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("resultPointers(references)[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResultPointers_Obligations_PathAndRuleIDs(t *testing.T) {
	result := map[string]any{
		"content": []any{map[string]any{"type": "text", "text": `{"service":"fathom","file":"internal/store/store.go","rules":[{"rule":"store stays pure Go","src":"CLAUDE.md#L46-L46","tier":"doc-claimed"}]}`}},
	}
	got := resultPointers("obligations", json.RawMessage(`{"service":"fathom","file":"internal/store/store.go"}`), result)
	want := []string{"internal/store/store.go", "CLAUDE.md#L46-L46"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("resultPointers(obligations) = %v, want %v", got, want)
	}
}

func TestResultPointers_OutlineUsesArgsFile(t *testing.T) {
	// outline's own result never echoes the file (nav_tools.go), so the
	// pointer must come from the request arguments.
	result := map[string]any{
		"content": []any{map[string]any{"type": "text", "text": `{"symbols":[]}`}},
	}
	got := resultPointers("outline", json.RawMessage(`{"service":"fathom","file":"internal/core/model.go"}`), result)
	if len(got) != 1 || got[0] != "internal/core/model.go" {
		t.Fatalf("resultPointers(outline) = %v, want [internal/core/model.go]", got)
	}
}

// TestResultPointers_References_ZeroResultMissOmitsQuery is the MINOR
// finding's fix: a references/find_callers call that found NOTHING must
// not add the raw query string as a discovery — there is nothing to hand
// off, and re-issuing the same miss is not a pointer.
func TestResultPointers_References_ZeroResultMissOmitsQuery(t *testing.T) {
	result := map[string]any{
		"content": []any{map[string]any{"type": "text", "text": `{"query":"NoSuchSymbol","definitions":[],"occurrences":[]}`}},
	}
	got := resultPointers("references", json.RawMessage(`{"query":"NoSuchSymbol"}`), result)
	if len(got) != 0 {
		t.Fatalf("resultPointers(references, zero-result) = %v, want empty (nothing to hand off)", got)
	}
}

// TestResultPointers_References_RegexQueryOmitsQuery proves the OTHER half
// of the same fix: regex:true means the query text is a REGEXP, never a
// resolvable symbol/file pointer — it must never be added even when the
// call found hits (the hit files/locations themselves still are pointers).
func TestResultPointers_References_RegexQueryOmitsQuery(t *testing.T) {
	result := map[string]any{
		"content": []any{map[string]any{"type": "text", "text": `{"query":"Put.*","definitions":[{"file":"internal/store/store.go"}]}`}},
	}
	got := resultPointers("references", json.RawMessage(`{"query":"Put.*","regex":true}`), result)
	want := []string{"internal/store/store.go"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("resultPointers(references, regex:true) = %v, want %v (the regex query text itself must be omitted)", got, want)
	}
}

func TestResultPointers_UnknownToolIsNil(t *testing.T) {
	result := map[string]any{"content": []any{map[string]any{"type": "text", "text": `{}`}}}
	if got := resultPointers("secret_scan", json.RawMessage(`{}`), result); got != nil {
		t.Fatalf("resultPointers(secret_scan) = %v, want nil", got)
	}
}

func TestResultPointers_CapsAtEight(t *testing.T) {
	nodes := ""
	for i := 0; i < 20; i++ {
		if i > 0 {
			nodes += ","
		}
		nodes += `{"name":"Sym` + string(rune('A'+i)) + `","file_path":"pkg/a.go"}`
	}
	result := map[string]any{
		"content": []any{map[string]any{"type": "text", "text": `{"symbols":[` + nodes + `]}`}},
	}
	got := resultPointers("find_symbol", json.RawMessage(`{"service":"fathom","name":"x"}`), result)
	if len(got) != maxResultPointers {
		t.Fatalf("resultPointers capped at %d, got %d: %v", maxResultPointers, len(got), got)
	}
}

// ─── the discoveries tool (RC-27) ────────────────────────────────────────

func TestDiscoveriesTool_ResolvesFreshAndRanks(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mcp-calls.jsonl")
	now := time.Now().UTC()
	rows := []string{
		// older, established once — should still resolve.
		`{"ts":"` + now.Add(-1*time.Hour).Format(time.RFC3339Nano) + `","service":"svc","pointers":["pkg.Live"]}`,
		// most recent, established three times — should rank first.
		`{"ts":"` + now.Add(-2*time.Minute).Format(time.RFC3339Nano) + `","service":"svc","pointers":["pkg.Moved"]}`,
		`{"ts":"` + now.Add(-1*time.Minute).Format(time.RFC3339Nano) + `","service":"svc","pointers":["pkg.Moved"]}`,
		// a different service's row must never leak into svc's discoveries.
		`{"ts":"` + now.Format(time.RFC3339Nano) + `","service":"other","pointers":["pkg.NotMine"]}`,
	}
	if err := os.WriteFile(logPath, []byte(joinLines(rows)), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := &elisionFakeBackend{
		nodes: []core.SymbolNode{
			{Name: "Live", FilePath: "pkg/live.go", Line: 5},
			// "Moved" is no longer where the pointer says — stale.
		},
		sources: nil,
	}
	l := NewCallLogger(mustOpenAppend(t, logPath))
	srv := NewServerWithLogger(backend, l)

	req := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "discoveries", "arguments": map[string]any{"service": "svc"}},
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := srv.HandleRequest(context.Background(), raw)
	if !ok {
		t.Fatalf("discoveries: no response")
	}
	var env map[string]any
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, resp)
	}
	if env["error"] != nil {
		t.Fatalf("discoveries: rpc error: %v", env["error"])
	}
	result := env["result"].(map[string]any)
	content := result["content"].([]any)
	first := content[0].(map[string]any)
	var payload discoveriesResult
	if err := json.Unmarshal([]byte(first["text"].(string)), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload.Pointers) != 2 {
		t.Fatalf("discoveries pointers = %v, want 2 (svc-scoped only)", payload.Pointers)
	}
	if payload.Pointers[0].Pointer != "pkg.Moved" || payload.Pointers[0].Count != 2 {
		t.Errorf("rank[0] = %+v, want pkg.Moved count=2 (most recent, most frequent)", payload.Pointers[0])
	}
	if payload.Pointers[0].Resolved {
		t.Errorf("pkg.Moved should be stale (no such symbol in the fixture), got resolved:true")
	}
	if payload.Pointers[1].Pointer != "pkg.Live" || !payload.Pointers[1].Resolved {
		t.Errorf("rank[1] = %+v, want pkg.Live resolved:true", payload.Pointers[1])
	}
}

func TestDiscoveriesTool_NoCallLogConfigured(t *testing.T) {
	backend := &elisionFakeBackend{}
	srv := NewServer(backend) // no logger at all
	req := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "discoveries", "arguments": map[string]any{"service": "svc"}},
	}
	raw, _ := json.Marshal(req)
	resp, ok := srv.HandleRequest(context.Background(), raw)
	if !ok {
		t.Fatalf("discoveries: no response")
	}
	var env map[string]any
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env["error"] != nil {
		t.Fatalf("discoveries with no logger should still exit cleanly, got error: %v", env["error"])
	}
	result := env["result"].(map[string]any)
	content := result["content"].([]any)
	first := content[0].(map[string]any)
	var payload discoveriesResult
	if err := json.Unmarshal([]byte(first["text"].(string)), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if payload.Note == "" {
		t.Errorf("expected a note explaining the empty answer, got none: %+v", payload)
	}
}

// TestDiscoveriesTool_PointersBudgetedSameAsRendered is the MAJOR finding's
// fix: `pointers` used to return the whole aggregate unbudgeted while only
// `rendered` was cut. With enough distinct pointers to force a cut at a
// small budget, `pointers` must ALSO be cut, and the two must describe the
// SAME kept prefix — every pointers[i].Line equal to rendered's i-th line.
func TestDiscoveriesTool_PointersBudgetedSameAsRendered(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mcp-calls.jsonl")
	now := time.Now().UTC()
	const total = 30
	var rows []string
	for i := 0; i < total; i++ {
		ts := now.Add(-time.Duration(i) * time.Second).Format(time.RFC3339Nano)
		rows = append(rows, fmt.Sprintf(`{"ts":%q,"service":"svc","pointers":["pkg.Sym%02d"]}`, ts, i))
	}
	if err := os.WriteFile(logPath, []byte(joinLines(rows)), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := &elisionFakeBackend{} // every pointer resolves to "stale (unresolved)"
	l := NewCallLogger(mustOpenAppend(t, logPath))
	srv := NewServerWithLogger(backend, l)

	req := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "discoveries", "arguments": map[string]any{"service": "svc", "budget": 300}},
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := srv.HandleRequest(context.Background(), raw)
	if !ok {
		t.Fatalf("discoveries: no response")
	}
	var env map[string]any
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, resp)
	}
	if env["error"] != nil {
		t.Fatalf("discoveries: rpc error: %v", env["error"])
	}
	result := env["result"].(map[string]any)
	content := result["content"].([]any)
	first := content[0].(map[string]any)
	var payload discoveriesResult
	if err := json.Unmarshal([]byte(first["text"].(string)), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if len(payload.Pointers) == 0 || len(payload.Pointers) >= total {
		t.Fatalf("expected pointers cut well below %d at budget 300, got %d", total, len(payload.Pointers))
	}
	if payload.Note == "" {
		t.Error("expected a note disclosing the cut")
	}
	renderedLines := strings.Split(strings.TrimRight(payload.Rendered, "\n"), "\n")
	if len(renderedLines) != len(payload.Pointers)+1 {
		t.Fatalf("rendered has %d lines, want %d (pointers) + 1 (the inline \"…N more\" footer): %q",
			len(renderedLines), len(payload.Pointers), payload.Rendered)
	}
	for i, row := range payload.Pointers {
		if renderedLines[i] != row.Line {
			t.Errorf("rendered line %d = %q, want pointers[%d].Line %q — rows and rendered must describe the same set", i, renderedLines[i], i, row.Line)
		}
	}
}

// TestRenderDiscoveryPointer_IndexUnreachableIsNotStale is the MAJOR
// finding's unit-level proof: an index error must render its own honest
// line, never the "stale" text a reachable-but-not-found index makes.
func TestRenderDiscoveryPointer_IndexUnreachableIsNotStale(t *testing.T) {
	wantErr := errors.New("dial tcp: connection refused")
	line, res := renderDiscoveryPointer("pkg.Sym", 1, nil, nil, wantErr)
	if res != discoveryUnreachable {
		t.Fatalf("res = %v, want discoveryUnreachable", res)
	}
	if !strings.Contains(line, "index unreachable") || strings.Contains(line, "stale") {
		t.Errorf("line = %q, want it to name the index unreachable and never say stale", line)
	}
	// A sourceSet hit from a call that itself SUCCEEDED still resolves —
	// idxErr only matters once that cheap fast-path check has missed.
	line2, res2 := renderDiscoveryPointer("pkg/live.go", 1, nil, map[string]bool{"pkg/live.go": true}, wantErr)
	if res2 != discoveryResolved || !strings.HasPrefix(line2, "read ") {
		t.Errorf("line2 = %q, res2 = %v, want a resolved \"read\" line even with idxErr set", line2, res2)
	}
}

// indexErrorBackend wraps elisionFakeBackend, overriding ListAllSymbols to
// return an error — proves the discoveries TOOL end to end never renders
// an unreachable index's pointers as "stale".
type indexErrorBackend struct {
	elisionFakeBackend
	listErr error
}

func (b *indexErrorBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return nil, b.listErr
}

func TestDiscoveriesTool_IndexUnreachableIsNotRenderedStale(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "mcp-calls.jsonl")
	now := time.Now().UTC()
	rows := []string{
		`{"ts":"` + now.Format(time.RFC3339Nano) + `","service":"svc","pointers":["pkg.Sym"]}`,
	}
	if err := os.WriteFile(logPath, []byte(joinLines(rows)), 0o644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("store unavailable")
	backend := &indexErrorBackend{listErr: wantErr}
	l := NewCallLogger(mustOpenAppend(t, logPath))
	srv := NewServerWithLogger(backend, l)

	req := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "discoveries", "arguments": map[string]any{"service": "svc"}},
	}
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	resp, ok := srv.HandleRequest(context.Background(), raw)
	if !ok {
		t.Fatalf("discoveries: no response")
	}
	var env map[string]any
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, resp)
	}
	if env["error"] != nil {
		t.Fatalf("discoveries: rpc error: %v", env["error"])
	}
	result := env["result"].(map[string]any)
	content := result["content"].([]any)
	first := content[0].(map[string]any)
	var payload discoveriesResult
	if err := json.Unmarshal([]byte(first["text"].(string)), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	if payload.IndexError == "" {
		t.Fatal("expected index_error to name the ListAllSymbols failure")
	}
	if !strings.Contains(payload.IndexError, wantErr.Error()) {
		t.Errorf("index_error = %q, want it to contain %q", payload.IndexError, wantErr.Error())
	}
	if len(payload.Pointers) != 1 {
		t.Fatalf("got %d pointers, want 1: %+v", len(payload.Pointers), payload.Pointers)
	}
	row := payload.Pointers[0]
	if row.Resolved {
		t.Error("an unreachable index must never render resolved:true")
	}
	if strings.Contains(row.Line, "stale") {
		t.Errorf("line = %q, must not render \"stale\" for an unreachable index (a positive claim the index never made)", row.Line)
	}
	if !strings.Contains(row.Line, "index unreachable") {
		t.Errorf("line = %q, want it to name the index as unreachable", row.Line)
	}
}

func joinLines(lines []string) string {
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out
}

func mustOpenAppend(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
