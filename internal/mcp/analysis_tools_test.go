// Package mcp_test — analysis tool tests (TDD, fake backend, no DuckDB).
//
// Tests cover: rename_impact, reachable, test_impact, dead_code.  Each test
// builds a richFakeBackend (defined below) with a
// small call graph fixture and drives the MCP server exactly like the existing
// tools_test.go tests — synchronous Serve call over a finite in-memory buffer.
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── richFakeBackend ─────────────────────────────────────────────────────────
//
// richFakeBackend returns full SymbolNodes (including Calls) from
// ListAllSymbols so the analysis tools can build a real callgraph.Graph.  All
// other QueryInterface methods are no-ops.

type richFakeBackend struct {
	nodes []core.SymbolNode
}

func (r *richFakeBackend) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (r *richFakeBackend) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (r *richFakeBackend) FindSymbol(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (r *richFakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return r.nodes, nil
}

func (r *richFakeBackend) Services(_ context.Context) ([]string, error) {
	return nil, nil
}

func (r *richFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}

func (r *richFakeBackend) IngestService(_ context.Context, _ string) error { return nil }

func (r *richFakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (r *richFakeBackend) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func (r *richFakeBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return nil, nil
}

// compile-time: richFakeBackend must satisfy mcp.QueryInterface.
var _ mcp.QueryInterface = (*richFakeBackend)(nil)

// ─── shared fixture ──────────────────────────────────────────────────────────
//
//	A → B → C
//	TestFoo → A
//	Orphan   (no callers, no callees)

func richFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, Calls: []string{"svc::pkg.B"}},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, Calls: []string{"C"}},
		{Service: "svc", Package: "pkg", Name: "C", Kind: core.KindGoFunc, Calls: nil},
		{Service: "svc", Package: "pkg", Name: "TestFoo", Kind: core.KindGoFunc, Calls: []string{"svc::pkg.A"}},
		{Service: "svc", Package: "pkg", Name: "Orphan", Kind: core.KindGoFunc, Calls: nil},
	}
}

func newAnalysisSrv() *mcp.Server {
	return mcp.NewServer(&richFakeBackend{nodes: richFixtureNodes()})
}

// ─── helper: extract text payload from tools/call response ───────────────────

func toolPayload(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	if resp["error"] != nil {
		t.Fatalf("unexpected RPC error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing or wrong type: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content missing or empty: %v", result)
	}
	text, ok := content[0].(map[string]any)["text"].(string)
	if !ok {
		t.Fatalf("content[0].text missing: %v", content[0])
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v (text=%q)", err, text)
	}
	return payload
}

// ─── rename_impact ────────────────────────────────────────────────────────────

func TestRenameImpact_DirectCallers(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 100, Method: "tools/call",
		Params: map[string]any{
			"name":      "rename_impact",
			"arguments": map[string]any{"service": "svc", "symbol": "B"},
		},
	})
	payload := toolPayload(t, resp)
	callers, ok := payload["callers"].([]any)
	if !ok {
		t.Fatalf("payload.callers missing: %v", payload)
	}
	// B is called by A and (bare "C" resolution aside) also by nothing else —
	// in the fixture B is called by A (svc::pkg.A) and TestFoo does not call B
	// directly. Actually in the fixture TestBar does not exist here. Let's
	// verify we get at least one caller (svc::pkg.A).
	if len(callers) == 0 {
		t.Errorf("rename_impact(B): expected callers, got empty slice")
	}
}

func TestRenameImpact_MissingSymbol(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 101, Method: "tools/call",
		Params: map[string]any{
			"name":      "rename_impact",
			"arguments": map[string]any{"service": "svc", "symbol": "NoSuchFunc"},
		},
	})
	payload := toolPayload(t, resp)
	callers, ok := payload["callers"].([]any)
	if !ok {
		t.Fatalf("payload.callers missing: %v", payload)
	}
	if len(callers) != 0 {
		t.Errorf("rename_impact(NoSuchFunc): expected empty, got %v", callers)
	}
}

func TestRenameImpact_MissingService(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 102, Method: "tools/call",
		Params: map[string]any{
			"name":      "rename_impact",
			"arguments": map[string]any{"symbol": "B"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for missing service, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

func TestRenameImpact_MissingSymbolParam(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 103, Method: "tools/call",
		Params: map[string]any{
			"name":      "rename_impact",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for missing symbol, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── reachable ────────────────────────────────────────────────────────────────

func TestReachable_True(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 110, Method: "tools/call",
		Params: map[string]any{
			"name":      "reachable",
			"arguments": map[string]any{"service": "svc", "from": "A", "to": "C"},
		},
	})
	payload := toolPayload(t, resp)
	reach, ok := payload["reachable"].(bool)
	if !ok {
		t.Fatalf("payload.reachable missing or wrong type: %v", payload)
	}
	if !reach {
		t.Error("reachable(A,C) should be true")
	}
}

func TestReachable_False(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 111, Method: "tools/call",
		Params: map[string]any{
			"name":      "reachable",
			"arguments": map[string]any{"service": "svc", "from": "C", "to": "A"},
		},
	})
	payload := toolPayload(t, resp)
	reach, ok := payload["reachable"].(bool)
	if !ok {
		t.Fatalf("payload.reachable missing or wrong type: %v", payload)
	}
	if reach {
		t.Error("reachable(C,A) should be false")
	}
}

func TestReachable_MissingParams(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 112, Method: "tools/call",
		Params: map[string]any{
			"name":      "reachable",
			"arguments": map[string]any{"service": "svc", "from": "A"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for missing 'to', got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── test_impact ──────────────────────────────────────────────────────────────

func TestTestImpact_ReachesTestFoo(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 130, Method: "tools/call",
		Params: map[string]any{
			"name":      "test_impact",
			"arguments": map[string]any{"service": "svc", "symbol": "A"},
		},
	})
	payload := toolPayload(t, resp)
	tests, ok := payload["tests"].([]any)
	if !ok {
		t.Fatalf("payload.tests missing or wrong type: %v", payload)
	}
	if len(tests) == 0 {
		t.Error("test_impact(A): expected at least one test (TestFoo)")
	}
	found := false
	for _, raw := range tests {
		if s, ok := raw.(string); ok && s == "svc::pkg.TestFoo" {
			found = true
		}
	}
	if !found {
		t.Errorf("test_impact(A): TestFoo not in results: %v", tests)
	}
}

func TestTestImpact_Orphan(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 131, Method: "tools/call",
		Params: map[string]any{
			"name":      "test_impact",
			"arguments": map[string]any{"service": "svc", "symbol": "Orphan"},
		},
	})
	payload := toolPayload(t, resp)
	tests, ok := payload["tests"].([]any)
	if !ok {
		t.Fatalf("payload.tests missing or wrong type: %v", payload)
	}
	if len(tests) != 0 {
		t.Errorf("test_impact(Orphan): expected empty, got %v", tests)
	}
}

func TestTestImpact_MissingParams(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 132, Method: "tools/call",
		Params: map[string]any{
			"name":      "test_impact",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for missing symbol, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── dead_code ────────────────────────────────────────────────────────────────

func TestDeadCode_FindsOrphan(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 140, Method: "tools/call",
		Params: map[string]any{
			"name": "dead_code",
			"arguments": map[string]any{
				"service":     "svc",
				"entrypoints": []string{"svc::pkg.TestFoo"},
			},
		},
	})
	payload := toolPayload(t, resp)
	dead, ok := payload["dead"].([]any)
	if !ok {
		t.Fatalf("payload.dead missing or wrong type: %v", payload)
	}
	found := false
	for _, raw := range dead {
		if s, ok := raw.(string); ok && s == "svc::pkg.Orphan" {
			found = true
		}
	}
	if !found {
		t.Errorf("dead_code: Orphan not in results: %v", dead)
	}
}

func TestDeadCode_EmptyEntrypoints(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 141, Method: "tools/call",
		Params: map[string]any{
			"name": "dead_code",
			"arguments": map[string]any{
				"service":     "svc",
				"entrypoints": []string{},
			},
		},
	})
	payload := toolPayload(t, resp)
	if payload["dead"] == nil {
		t.Error("dead_code: 'dead' key missing")
	}
}

func TestDeadCode_MissingService(t *testing.T) {
	srv := newAnalysisSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 142, Method: "tools/call",
		Params: map[string]any{
			"name":      "dead_code",
			"arguments": map[string]any{"entrypoints": []string{"A"}},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for missing service, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── profile membership ───────────────────────────────────────────────────────

func TestAnalysisToolsInCorrectProfile(t *testing.T) {
	srv := newAnalysisSrv()

	// HF-5 deleted the path-scoped group surfaces (ADR-014 amendment 2); the
	// group taxonomy still classifies every tool, which is what this test
	// was always asserting.
	for _, tc := range []struct {
		name  string
		group mcp.ToolGroup
	}{
		{"rename_impact", mcp.GroupCoding},
		{"reachable", mcp.GroupCoding},
		{"dead_code", mcp.GroupCoding},
		{"test_impact", mcp.GroupInvestigation},
	} {
		if got := srv.GroupOfForTest(tc.name); got != tc.group {
			t.Errorf("tool %q registered in group %q, want %q", tc.name, got, tc.group)
		}
	}
}
