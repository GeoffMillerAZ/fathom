// Package mcp_test — additional tool-level tests.
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// compile-time check: fakeBackend must implement QueryInterface.
var _ mcp.QueryInterface = (*fakeBackend)(nil)

// ─── fake backend ────────────────────────────────────────────────────────────

type fakeNode struct {
	name string
	kind string
	file string
}

type fakeQueryResult struct {
	nodes []fakeNode
}

type fakeBackend struct {
	queryResult fakeQueryResult
	callers     []fakeNode
	symbol      []fakeNode
	listAll     []fakeNode
}

func (f *fakeBackend) QuerySymbols(_ context.Context, q core.Query) (core.QueryResult, error) {
	nodes := make([]core.SymbolNode, len(f.queryResult.nodes))
	for i, n := range f.queryResult.nodes {
		nodes[i] = core.SymbolNode{
			Service:  q.ServiceID,
			Name:     n.name,
			Kind:     core.SourceKind(n.kind),
			FilePath: n.file,
		}
	}
	return core.QueryResult{Nodes: nodes}, nil
}

func (f *fakeBackend) FindCallers(_ context.Context, serviceID, symbol string) ([]core.SymbolNode, error) {
	nodes := make([]core.SymbolNode, len(f.callers))
	for i, n := range f.callers {
		nodes[i] = core.SymbolNode{
			Service:  serviceID,
			Name:     n.name,
			Kind:     core.SourceKind(n.kind),
			FilePath: n.file,
		}
	}
	_ = symbol
	return nodes, nil
}

func (f *fakeBackend) FindSymbol(_ context.Context, serviceID, name string) ([]core.SymbolNode, error) {
	nodes := make([]core.SymbolNode, len(f.symbol))
	for i, n := range f.symbol {
		nodes[i] = core.SymbolNode{
			Service:  serviceID,
			Name:     n.name,
			Kind:     core.SourceKind(n.kind),
			FilePath: n.file,
		}
	}
	_ = name
	return nodes, nil
}

func (f *fakeBackend) ListAllSymbols(_ context.Context, serviceID string) ([]core.SymbolNode, error) {
	nodes := make([]core.SymbolNode, len(f.listAll))
	for i, n := range f.listAll {
		nodes[i] = core.SymbolNode{
			Service:  serviceID,
			Name:     n.name,
			Kind:     core.SourceKind(n.kind),
			FilePath: n.file,
		}
	}
	return nodes, nil
}

func (f *fakeBackend) Services(_ context.Context) ([]string, error) {
	return nil, nil
}

func (f *fakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}

func (f *fakeBackend) IngestService(_ context.Context, _ string) error { return nil }

func (f *fakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (f *fakeBackend) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

// ─── find_callers ────────────────────────────────────────────────────────────

func TestFindCallers(t *testing.T) {
	fake := &fakeBackend{
		callers: []fakeNode{
			{name: "ProcessPayment", kind: "GoFunc", file: "payment/payment.go"},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      30,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "find_callers",
			"arguments": map[string]any{
				"service": "payment-service",
				"symbol":  "ValidateToken",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	callers, ok := payload["callers"].([]any)
	if !ok {
		t.Fatalf("payload.callers missing: %v", payload)
	}
	if len(callers) != 1 {
		t.Errorf("callers count = %d, want 1", len(callers))
	}
}

// ─── find_symbol ─────────────────────────────────────────────────────────────

func TestFindSymbol(t *testing.T) {
	fake := &fakeBackend{
		symbol: []fakeNode{
			{name: "ChargeRequest", kind: "GoType", file: "payment/types.go"},
		},
	}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      40,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "find_symbol",
			"arguments": map[string]any{
				"service": "payment-service",
				"name":    "ChargeRequest",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["symbols"] == nil {
		t.Error("payload.symbols missing")
	}
}

// ─── missing required param → -32602 ─────────────────────────────────────────

func TestQuerySymbolsMissingService(t *testing.T) {
	fake := &fakeBackend{}
	srv := mcp.NewServer(fake)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      50,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "query_symbols",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	code := errObj["code"].(float64)
	if code != -32602 {
		t.Errorf("error.code = %v, want -32602", code)
	}
}

// ─── set_profile → unknown profile → -32602 ──────────────────────────────────

// ─── tools/call with unknown tool name → -32601 ───────────────────────────────

func TestCallUnknownTool(t *testing.T) {
	srv := mcp.NewServer(nil)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0",
		ID:      70,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "no_such_tool",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	code := errObj["code"].(float64)
	if code != -32601 {
		t.Errorf("error.code = %v, want -32601", code)
	}
}

// ─── investigation profile: find_callers must appear ─────────────────────────

func TestInvestigationProfileIncludesFindCallers(t *testing.T) {
	srv := mcp.NewServer(nil)

	// set_profile is gone; select the investigation surface via the HTTP path
	// scope instead — toolOnSurface resolves named groups identically whether
	// reached via stdio profile or HTTP surface (see TestSetProfileDirect).
	ctx := mcp.WithRequestScope(context.Background(), "investigation", true)
	listResp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0",
		ID:      81,
		Method:  "tools/list",
		Params:  map[string]any{},
	})
	if listResp["error"] != nil {
		t.Fatalf("tools/list error: %v", listResp["error"])
	}
	result := listResp["result"].(map[string]any)
	tools := result["tools"].([]any)

	found := false
	for _, raw := range tools {
		if raw.(map[string]any)["name"] == "find_callers" {
			found = true
		}
	}
	if !found {
		t.Error("find_callers not found in investigation profile")
	}
}
