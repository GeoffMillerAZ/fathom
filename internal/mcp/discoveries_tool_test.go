package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
