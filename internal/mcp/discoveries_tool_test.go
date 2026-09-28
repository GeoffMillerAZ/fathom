package mcp

import (
	"encoding/json"
	"testing"
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

// The discoveries MCP tool (registerDiscoveries) was unregistered by SR-29
// (surface reckoning F-47): 0 lifetime calls, and the SubagentStart hook's
// push-form render (subagentDiscoveries, cmd/fathom/discoveries.go) already
// covers the need. renderDiscoveryPointer/discoveriesResult's rendering
// logic is exercised through that hook's own tests now
// (TestSubagentDiscoveries_*, cmd/fathom/discoveries_test.go).
