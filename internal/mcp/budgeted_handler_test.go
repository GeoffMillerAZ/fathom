package mcp

// budgeted_handler_test.go — SU-3 (docs/plans/substrate.md §A, ADR-041
// decision 3): every tools/call result must pass through the same byte
// budget + elision seam fitToolBudget already gives disciplined handlers,
// so a handler that FORGOT to budget itself (O90's defect class) cannot
// regress past defaultCharBudget either.

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type synthPayload struct {
	Items []string `json:"items"`
}

// oversizedSyntheticTool registers a tool whose handler returns a payload
// well past defaultCharBudget (16000 bytes) with NO budgeting of its own —
// exactly the bug class O90 was.
func registerOversizedSyntheticTool(s *Server) {
	items := make([]string, 4000)
	for i := range items {
		// Each element is ~50 bytes; 4000 of them is ~200KB total, far past
		// defaultCharBudget.
		items[i] = strings.Repeat("x", 46) + "-item"
	}
	s.registerTool(toolDescriptor{
		Name:        "test_oversized_tool",
		Description: "synthetic oversized tool (SU-3 test only)",
		InputSchema: map[string]any{"type": "object"},
		group:       GroupCoding,
	}, func(_ context.Context, _ json.RawMessage) (any, *rpcError) {
		return &synthPayload{Items: items}, nil
	})
}

func callToolRaw(t *testing.T, s *Server, name string) map[string]any {
	t.Helper()
	req, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, hasResp := s.HandleRequest(context.Background(), req)
	if !hasResp {
		t.Fatal("HandleRequest returned hasResp=false")
	}
	var m map[string]any
	if err := json.Unmarshal(resp, &m); err != nil {
		t.Fatalf("unmarshal response: %v (raw=%s)", err, resp)
	}
	return m
}

func TestBudgetedHandler_ElidesOversizedResult(t *testing.T) {
	s := NewServer(nil)
	registerOversizedSyntheticTool(s)

	m := callToolRaw(t, s, "test_oversized_tool")
	if m["error"] != nil {
		t.Fatalf("unexpected error: %v", m["error"])
	}
	result, ok := m["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %v", m)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content missing: %v", result)
	}
	block, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] not an object: %v", content[0])
	}
	text, _ := block["text"].(string)
	if text == "" {
		t.Fatal("content[0].text empty")
	}
	if len(text) > defaultCharBudget+2000 {
		// A little slack for the wrapper object's own keys/braces — the
		// budget is a target over the trimmed field, not a hard byte cap
		// on the whole envelope (BA-4).
		t.Fatalf("elided text still %d bytes, want <= ~%d (budget %d)", len(text), defaultCharBudget+2000, defaultCharBudget)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal elided payload: %v (text=%s)", err, text)
	}
	elidedBlock, ok := payload["elided"].(map[string]any)
	if !ok {
		t.Fatalf("expected standard elided block in oversized result, got: %v", payload)
	}
	if elidedBlock["count"] == nil || elidedBlock["of"] == nil || elidedBlock["ordering"] == nil || elidedBlock["how_to_get_more"] == nil {
		t.Fatalf("elided block missing standard fields: %v", elidedBlock)
	}
}

// TestBudgetedHandler_UnderBudgetIsByteIdentical pins that a handler whose
// own result already fits the budget passes through the wrapper completely
// unchanged — SU-3's "existing handlers that already budget must render
// byte-identically" requirement, golden-pinned against a small synthetic
// tool registered the same way every real tool is (registerTool).
func TestBudgetedHandler_UnderBudgetIsByteIdentical(t *testing.T) {
	s := NewServer(nil)
	s.registerTool(toolDescriptor{
		Name:        "test_small_tool",
		Description: "synthetic small tool (SU-3 golden)",
		InputSchema: map[string]any{"type": "object"},
		group:       GroupCoding,
	}, func(_ context.Context, _ json.RawMessage) (any, *rpcError) {
		return map[string]any{"items": []string{"a", "b", "c"}}, nil
	})

	m := callToolRaw(t, s, "test_small_tool")
	if m["error"] != nil {
		t.Fatalf("unexpected error: %v", m["error"])
	}
	const want = `{"id":1,"jsonrpc":"2.0","result":{"content":[{"text":"{\"items\":[\"a\",\"b\",\"c\"]}","type":"text"}],"structuredContent":{"items":["a","b","c"]}}}`
	got, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("re-marshal response: %v", err)
	}
	if string(got) != want {
		t.Fatalf("small-tool response = %s, want %s", got, want)
	}
}
