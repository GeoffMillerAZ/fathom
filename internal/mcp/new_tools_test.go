// Package mcp_test — TDD tests for new coding/investigation tools,
// meta-tools, prompts, resources, and call logging.
package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── extendedFakeBackend ─────────────────────────────────────────────────────
//
// extendedFakeBackend extends richFakeBackend to also satisfy the new
// Services method required by QueryInterface.

type extendedFakeBackend struct {
	nodes      []core.SymbolNode
	services   []string
	typedEdges []core.Edge
	// svcPath, when non-empty, is returned as ServiceMeta's corpus-root path
	// (arch_conformance's C4-3 project-rules lookup uses it to find
	// .fathom/rules/archconformance.cue under a service's corpus root).
	svcPath string
}

func (e *extendedFakeBackend) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (e *extendedFakeBackend) FindCallers(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (e *extendedFakeBackend) FindSymbol(_ context.Context, _, _ string) ([]core.SymbolNode, error) {
	return nil, nil
}

func (e *extendedFakeBackend) ListAllSymbols(_ context.Context, _ string) ([]core.SymbolNode, error) {
	return e.nodes, nil
}

func (e *extendedFakeBackend) Services(_ context.Context) ([]string, error) {
	return e.services, nil
}

func (e *extendedFakeBackend) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}

func (e *extendedFakeBackend) IngestService(_ context.Context, _ string) error { return nil }

func (e *extendedFakeBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	if e.svcPath == "" {
		return "", "", false, nil
	}
	return e.svcPath, "", true, nil
}

func (e *extendedFakeBackend) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func (e *extendedFakeBackend) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return e.typedEdges, nil
}

// compile-time check.
var _ mcp.QueryInterface = (*extendedFakeBackend)(nil)

// fixture: A→B→C, TestFoo→A, Orphan
func extFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "A", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Line: 10, Calls: []string{"svc::pkg.B"}, Complexity: 3},
		{Service: "svc", Package: "pkg", Name: "B", Kind: core.KindGoFunc, FilePath: "pkg/a.go", Line: 20, Calls: []string{"C"}, Complexity: 5},
		{Service: "svc", Package: "pkg", Name: "C", Kind: core.KindGoFunc, FilePath: "pkg/c.go", Line: 5, Calls: nil, Complexity: 1},
		{Service: "svc", Package: "pkg", Name: "TestFoo", Kind: core.KindGoFunc, FilePath: "pkg/a_test.go", Line: 1, Calls: []string{"svc::pkg.A"}, Complexity: 2},
		{Service: "svc", Package: "pkg", Name: "Orphan", Kind: core.KindGoFunc, FilePath: "pkg/orphan.go", Line: 1, Calls: nil, Complexity: 1},
		{Service: "svc", Package: "pkg", Name: "MyType", Kind: core.KindGoType, FilePath: "pkg/types.go", Line: 1},
	}
}

func newExtSrv() *mcp.Server {
	return mcp.NewServer(&extendedFakeBackend{
		nodes:    extFixtureNodes(),
		services: []string{"svc", "other-svc"},
	})
}

// dispatchFixtureNodes mirrors the ADR-021 §3 O9 residue shape: RunAdapter
// dispatches to Complete on two receivers (AdapterA, AdapterB) — an
// ambiguous method name the syntax-only resolver refuses (no Calls entry at
// all, matching graph.go:218-240's honesty rule), reachable ONLY through
// typedEdges.
func dispatchFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "svc", Package: "pkg", Name: "RunAdapter", Kind: core.KindGoFunc, FilePath: "pkg/adapter.go", Line: 1},
		{Service: "svc", Package: "AdapterA", Name: "AdapterA.Complete", Kind: core.KindGoMethod, FilePath: "pkg/adaptera.go", Line: 1},
		{Service: "svc", Package: "AdapterB", Name: "AdapterB.Complete", Kind: core.KindGoMethod, FilePath: "pkg/adapterb.go", Line: 1},
	}
}

func dispatchFixtureTypedEdges() []core.Edge {
	return []core.Edge{
		{FromService: "svc", FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::AdapterA.AdapterA.Complete", Kind: core.EdgeIfaceDispatch},
		{FromService: "svc", FromSymbol: "svc::pkg.RunAdapter", ToSymbol: "svc::AdapterB.AdapterB.Complete", Kind: core.EdgeIfaceDispatch},
	}
}

func newDispatchSrv() *mcp.Server {
	return mcp.NewServer(&extendedFakeBackend{
		nodes:      dispatchFixtureNodes(),
		services:   []string{"svc"},
		typedEdges: dispatchFixtureTypedEdges(),
	})
}

// ─── helper ──────────────────────────────────────────────────────────────────

func extToolPayload(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	if resp["error"] != nil {
		t.Fatalf("unexpected RPC error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	content := result["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("payload unmarshal: %v (text=%q)", err, text)
	}
	return payload
}

// ─── find_callees ─────────────────────────────────────────────────────────────

func TestFindCallees_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 200, Method: "tools/call",
		Params: map[string]any{
			"name":      "find_callees",
			"arguments": map[string]any{"service": "svc", "symbol": "A"},
		},
	})
	p := extToolPayload(t, resp)
	callees, ok := p["callees"].([]any)
	if !ok {
		t.Fatalf("callees missing: %v", p)
	}
	if len(callees) == 0 {
		t.Error("find_callees(A): expected B as callee")
	}
}

func TestFindCallees_MissingService(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 201, Method: "tools/call",
		Params: map[string]any{
			"name":      "find_callees",
			"arguments": map[string]any{"symbol": "A"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── call_hierarchy ───────────────────────────────────────────────────────────

func TestCallHierarchy_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 210, Method: "tools/call",
		Params: map[string]any{
			"name":      "call_hierarchy",
			"arguments": map[string]any{"service": "svc", "symbol": "A", "depth": 2},
		},
	})
	p := extToolPayload(t, resp)
	if p["key"] == nil {
		t.Fatalf("hierarchy.key missing: %v", p)
	}
}

func TestCallHierarchy_DefaultDepth(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 211, Method: "tools/call",
		Params: map[string]any{
			"name":      "call_hierarchy",
			"arguments": map[string]any{"service": "svc", "symbol": "A"},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestCallHierarchy_MissingParams(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 212, Method: "tools/call",
		Params: map[string]any{
			"name":      "call_hierarchy",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── symbol_context ───────────────────────────────────────────────────────────

func TestSymbolContext_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 220, Method: "tools/call",
		Params: map[string]any{
			"name":      "symbol_context",
			"arguments": map[string]any{"service": "svc", "symbol": "A"},
		},
	})
	p := extToolPayload(t, resp)
	if p["callers"] == nil || p["callees"] == nil {
		t.Fatalf("context missing callers/callees: %v", p)
	}
}

// TestSymbolContext_TypedEdges_DispatchLabeled is the CT-6 proof for
// symbol_context: the typed dispatch fan-out from RunAdapter surfaces as a
// kind-labeled "typed_edges" block, distinct from (and never mixed into)
// the existing syntax-only "callers"/"callees" fields.
func TestSymbolContext_TypedEdges_DispatchLabeled(t *testing.T) {
	srv := newDispatchSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 222, Method: "tools/call",
		Params: map[string]any{
			"name":      "symbol_context",
			"arguments": map[string]any{"service": "svc", "symbol": "RunAdapter"},
		},
	})
	p := extToolPayload(t, resp)

	// Plain syntax-only callees stay empty: RunAdapter has no Calls entry
	// at all (the ambiguous-method refusal never even produces a raw string
	// to point at).
	if callees, ok := p["callees"].([]any); !ok || len(callees) != 0 {
		t.Errorf("plain callees = %v, want empty", p["callees"])
	}

	typedEdges, ok := p["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("typed_edges missing or wrong shape: %v", p)
	}
	callees, ok := typedEdges["callees"].([]any)
	if !ok || len(callees) != 2 {
		t.Fatalf("typed_edges.callees = %v, want 2 entries", typedEdges["callees"])
	}
	seen := map[string]string{}
	for _, c := range callees {
		m := c.(map[string]any)
		seen[m["key"].(string)] = m["kind"].(string)
	}
	if seen["svc::AdapterA.AdapterA.Complete"] != "iface-dispatch" || seen["svc::AdapterB.AdapterB.Complete"] != "iface-dispatch" {
		t.Errorf("typed_edges.callees = %v, want both AdapterA/AdapterB Complete kind=iface-dispatch", seen)
	}
}

// TestFindCallers_TypedEdges_DispatchLabeled is the CT-6 proof for
// find_callers: AdapterA.Complete has no syntax-only caller at all, but a
// typed dispatch edge from RunAdapter surfaces via "typed_callers".
func TestFindCallers_TypedEdges_DispatchLabeled(t *testing.T) {
	srv := newDispatchSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 223, Method: "tools/call",
		Params: map[string]any{
			"name":      "find_callers",
			"arguments": map[string]any{"service": "svc", "symbol": "AdapterA.Complete"},
		},
	})
	p := extToolPayload(t, resp)
	typedCallers, ok := p["typed_callers"].([]any)
	if !ok || len(typedCallers) != 1 {
		t.Fatalf("typed_callers = %v, want 1 entry", p["typed_callers"])
	}
	m := typedCallers[0].(map[string]any)
	if m["key"] != "svc::pkg.RunAdapter" || m["kind"] != "iface-dispatch" {
		t.Errorf("typed_callers[0] = %v, want key=svc::pkg.RunAdapter kind=iface-dispatch", m)
	}
}

func TestSymbolContext_MissingService(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 221, Method: "tools/call",
		Params: map[string]any{
			"name":      "symbol_context",
			"arguments": map[string]any{"symbol": "A"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── outline ──────────────────────────────────────────────────────────────────

func TestOutline_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 230, Method: "tools/call",
		Params: map[string]any{
			"name":      "outline",
			"arguments": map[string]any{"service": "svc", "file": "pkg/a.go"},
		},
	})
	p := extToolPayload(t, resp)
	symbols, ok := p["symbols"].([]any)
	if !ok {
		t.Fatalf("outline.symbols missing: %v", p)
	}
	// pkg/a.go has A (line 10) and B (line 20)
	if len(symbols) < 2 {
		t.Errorf("outline(pkg/a.go): expected at least 2 symbols, got %d", len(symbols))
	}
}

func TestOutline_MissingParams(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 231, Method: "tools/call",
		Params: map[string]any{
			"name":      "outline",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── entrypoints ──────────────────────────────────────────────────────────────

func TestEntrypoints_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 240, Method: "tools/call",
		Params: map[string]any{
			"name":      "entrypoints",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["entrypoints"] == nil {
		t.Fatalf("entrypoints missing: %v", p)
	}
}

func TestEntrypoints_MissingService(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 241, Method: "tools/call",
		Params: map[string]any{
			"name":      "entrypoints",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── complexity ───────────────────────────────────────────────────────────────

func TestComplexityTool_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 250, Method: "tools/call",
		Params: map[string]any{
			"name":      "complexity",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["hotspots"] == nil || p["maintainability_index"] == nil {
		t.Fatalf("complexity missing fields: %v", p)
	}
}

func TestComplexityTool_MissingService(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 251, Method: "tools/call",
		Params: map[string]any{
			"name":      "complexity",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── god_object ───────────────────────────────────────────────────────────────

func TestGodObject_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 260, Method: "tools/call",
		Params: map[string]any{
			"name":      "god_object",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["god_objects"] == nil {
		t.Fatalf("god_objects missing: %v", p)
	}
}

// ─── api_surface ──────────────────────────────────────────────────────────────

func TestAPISurface_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 270, Method: "tools/call",
		Params: map[string]any{
			"name":      "api_surface",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["symbols"] == nil {
		t.Fatalf("api_surface.symbols missing: %v", p)
	}
}

// ─── community ────────────────────────────────────────────────────────────────

func TestCommunity_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 280, Method: "tools/call",
		Params: map[string]any{
			"name":      "community",
			"arguments": map[string]any{"service": "svc", "symbol": "A"},
		},
	})
	p := extToolPayload(t, resp)
	members, ok := p["members"].([]any)
	if !ok {
		t.Fatalf("community.members missing: %v", p)
	}
	// A is in pkg, so it should be related to B, C, TestFoo, Orphan (same package)
	if len(members) == 0 {
		t.Error("community(A): expected non-empty members (same-package nodes)")
	}
}

func TestCommunity_MissingParams(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 281, Method: "tools/call",
		Params: map[string]any{
			"name":      "community",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── type_hierarchy ───────────────────────────────────────────────────────────

func TestTypeHierarchy_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 290, Method: "tools/call",
		Params: map[string]any{
			"name":      "type_hierarchy",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["hierarchy"] == nil {
		t.Fatalf("type_hierarchy.hierarchy missing: %v", p)
	}
}

// ─── meta: capabilities ───────────────────────────────────────────────────────

func TestCapabilities_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 300, Method: "tools/call",
		Params: map[string]any{
			"name":      "capabilities",
			"arguments": map[string]any{},
		},
	})
	p := extToolPayload(t, resp)
	if p["tools"] == nil {
		t.Fatalf("capabilities.tools missing: %v", p)
	}
	if p["active_profile"] == nil {
		t.Fatalf("capabilities.active_profile missing: %v", p)
	}
	// ADR-021 §4 / XT-5: per-language extraction engine provenance (tree-
	// sitter-wasm vs the languages that kept their incumbent) is discoverable
	// without reading source.
	engines, ok := p["extraction_engines"].(map[string]any)
	if !ok || len(engines) == 0 {
		t.Fatalf("capabilities.extraction_engines missing/empty: %v", p)
	}
	if got := engines[".py"]; got != "tree-sitter-wasm" {
		t.Errorf("extraction_engines[.py] = %v, want tree-sitter-wasm", got)
	}
}

// ─── meta: server_stats ───────────────────────────────────────────────────────

func TestServerStats_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 310, Method: "tools/call",
		Params: map[string]any{
			"name":      "server_stats",
			"arguments": map[string]any{},
		},
	})
	p := extToolPayload(t, resp)
	if p["uptime_seconds"] == nil {
		t.Fatalf("server_stats.uptime_seconds missing: %v", p)
	}
	if p["request_count"] == nil {
		t.Fatalf("server_stats.request_count missing: %v", p)
	}
	// No service argument: typed_edges must be entirely absent (backward
	// compatible with the pre-ADR-021-§3 shape).
	if _, ok := p["typed_edges"]; ok {
		t.Errorf("server_stats without a service should omit typed_edges, got %v", p["typed_edges"])
	}
}

// TestServerStats_TypedEdges is CT-1's "counters in server_stats" proof
// (ADR-021 §3): passing a service adds a typed_edges block with honest
// kind counts — the visible signal for whether a store's typed pass has
// run at all.
func TestServerStats_TypedEdges(t *testing.T) {
	srv := newDispatchSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 311, Method: "tools/call",
		Params: map[string]any{
			"name":      "server_stats",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	typedEdges, ok := p["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("server_stats.typed_edges missing or wrong shape: %v", p)
	}
	if typedEdges["total"].(float64) != 2 {
		t.Errorf("typed_edges.total = %v, want 2", typedEdges["total"])
	}
	if typedEdges["iface_dispatch"].(float64) != 2 {
		t.Errorf("typed_edges.iface_dispatch = %v, want 2", typedEdges["iface_dispatch"])
	}
	if typedEdges["direct"].(float64) != 0 || typedEdges["unresolved"].(float64) != 0 {
		t.Errorf("typed_edges direct/unresolved = %v/%v, want 0/0", typedEdges["direct"], typedEdges["unresolved"])
	}
}

// TestServerStats_TypedEdges_ZeroForUntypedService confirms a service with
// no typed edges yet (predates ADR-021 §3, or hasn't been re-ingested)
// reports all-zero counts rather than omitting the block — the honest
// "the pass hasn't run" signal.
func TestServerStats_TypedEdges_ZeroForUntypedService(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 312, Method: "tools/call",
		Params: map[string]any{
			"name":      "server_stats",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	typedEdges, ok := p["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("server_stats.typed_edges missing or wrong shape: %v", p)
	}
	if typedEdges["total"].(float64) != 0 {
		t.Errorf("typed_edges.total = %v, want 0", typedEdges["total"])
	}
}

// ─── meta: staleness_report ───────────────────────────────────────────────────

func TestStalenessReport_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 320, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	if p["service"] == nil {
		t.Fatalf("staleness_report.service missing: %v", p)
	}
}

func TestStalenessReport_MissingService(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 321, Method: "tools/call",
		Params: map[string]any{
			"name":      "staleness_report",
			"arguments": map[string]any{},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeParams)
	}
}

// ─── meta: list_sources ───────────────────────────────────────────────────────

func TestListSources_Basic(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 330, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_sources",
			"arguments": map[string]any{"service": "svc"},
		},
	})
	p := extToolPayload(t, resp)
	sources, ok := p["sources"].([]any)
	if !ok {
		t.Fatalf("list_sources.sources missing: %v", p)
	}
	// fixture has files: pkg/a.go, pkg/c.go, pkg/a_test.go, pkg/orphan.go, pkg/types.go
	if len(sources) == 0 {
		t.Error("list_sources: expected non-empty sources")
	}
}

// ─── MCP protocol: prompts/list ───────────────────────────────────────────────

func TestPromptsList(t *testing.T) {
	srv := newExtSrv()
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      400,
		"method":  "prompts/list",
		"params":  map[string]any{},
	})
	if !hasResp {
		t.Fatal("prompts/list: no response")
	}
	if resp["error"] != nil {
		t.Fatalf("prompts/list error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	prompts, ok := result["prompts"].([]any)
	if !ok || len(prompts) == 0 {
		t.Fatalf("prompts/list: missing or empty prompts: %v", result)
	}
	// Expect at least 4 canned prompts.
	if len(prompts) < 4 {
		t.Errorf("prompts/list: expected at least 4 prompts, got %d", len(prompts))
	}
	// Each prompt must have name and description.
	names := make(map[string]bool)
	for _, raw := range prompts {
		p := raw.(map[string]any)
		if p["name"] == nil {
			t.Error("prompt missing name")
		}
		names[p["name"].(string)] = true
	}
	for _, want := range []string{"investigate_incident", "review_change", "onboard_service", "plan_requirement"} {
		if !names[want] {
			t.Errorf("missing prompt %q", want)
		}
	}
}

// ─── MCP protocol: prompts/get ────────────────────────────────────────────────

func TestPromptsGet_InvestigateIncident(t *testing.T) {
	srv := newExtSrv()
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      401,
		"method":  "prompts/get",
		"params": map[string]any{
			"name": "investigate_incident",
			"arguments": map[string]any{
				"service": "payment-service",
				"symptom": "high latency on /charge endpoint",
			},
		},
	})
	if !hasResp {
		t.Fatal("prompts/get: no response")
	}
	if resp["error"] != nil {
		t.Fatalf("prompts/get error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	messages, ok := result["messages"].([]any)
	if !ok || len(messages) == 0 {
		t.Fatalf("prompts/get: messages missing or empty: %v", result)
	}
	// Content of first message should mention the service or symptom.
	text := messages[0].(map[string]any)["content"].(map[string]any)["text"].(string)
	if !strings.Contains(text, "payment-service") && !strings.Contains(text, "high latency") {
		t.Errorf("prompt text does not mention service or symptom: %q", text)
	}
}

func TestPromptsGet_ReviewChange(t *testing.T) {
	srv := newExtSrv()
	resp, _ := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      402,
		"method":  "prompts/get",
		"params": map[string]any{
			"name": "review_change",
			"arguments": map[string]any{
				"service": "auth-service",
				"change":  "added rate limiting to login endpoint",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("review_change error: %v", resp["error"])
	}
}

func TestPromptsGet_OnboardService(t *testing.T) {
	srv := newExtSrv()
	resp, _ := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      403,
		"method":  "prompts/get",
		"params": map[string]any{
			"name": "onboard_service",
			"arguments": map[string]any{
				"service": "ledger-service",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("onboard_service error: %v", resp["error"])
	}
}

func TestPromptsGet_PlanRequirement(t *testing.T) {
	srv := newExtSrv()
	resp, _ := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      404,
		"method":  "prompts/get",
		"params": map[string]any{
			"name": "plan_requirement",
			"arguments": map[string]any{
				"service":     "payment-service",
				"requirement": "support multi-currency charges",
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("plan_requirement error: %v", resp["error"])
	}
}

func TestPromptsGet_UnknownPrompt(t *testing.T) {
	srv := newExtSrv()
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      405,
		"method":  "prompts/get",
		"params": map[string]any{
			"name":      "no_such_prompt",
			"arguments": map[string]any{},
		},
	})
	if !hasResp {
		t.Fatal("prompts/get: no response")
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for unknown prompt, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeNotFound)
	}
}

// ─── MCP protocol: resources/list ────────────────────────────────────────────

func TestResourcesList(t *testing.T) {
	srv := newExtSrv()
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      500,
		"method":  "resources/list",
		"params":  map[string]any{},
	})
	if !hasResp {
		t.Fatal("resources/list: no response")
	}
	if resp["error"] != nil {
		t.Fatalf("resources/list error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	resources, ok := result["resources"].([]any)
	if !ok || len(resources) == 0 {
		t.Fatalf("resources/list: missing or empty resources: %v", result)
	}
	// Must include fathom:///manifest.
	uris := make(map[string]bool)
	for _, raw := range resources {
		r := raw.(map[string]any)
		if uri, ok := r["uri"].(string); ok {
			uris[uri] = true
		}
	}
	if !uris["fathom:///manifest"] {
		t.Errorf("fathom:///manifest not in resources list: %v", uris)
	}
	// Must include fathom:///service/<id> for each known service.
	for _, svc := range []string{"svc", "other-svc"} {
		uri := "fathom:///service/" + svc
		if !uris[uri] {
			t.Errorf("resource %q not in list; have: %v", uri, uris)
		}
	}
}

// ─── MCP protocol: resources/read ────────────────────────────────────────────

func TestResourcesRead_Manifest(t *testing.T) {
	srv := newExtSrv()
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      510,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": "fathom:///manifest",
		},
	})
	if !hasResp {
		t.Fatal("resources/read manifest: no response")
	}
	if resp["error"] != nil {
		t.Fatalf("resources/read manifest error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	contents, ok := result["contents"].([]any)
	if !ok || len(contents) == 0 {
		t.Fatalf("resources/read manifest: contents missing: %v", result)
	}
}

func TestResourcesRead_Service(t *testing.T) {
	srv := newExtSrv()
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      511,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": "fathom:///service/svc",
		},
	})
	if !hasResp {
		t.Fatal("resources/read service: no response")
	}
	if resp["error"] != nil {
		t.Fatalf("resources/read service error: %v", resp["error"])
	}
	result := resp["result"].(map[string]any)
	contents, ok := result["contents"].([]any)
	if !ok || len(contents) == 0 {
		t.Fatalf("resources/read service: contents missing: %v", result)
	}
}

func TestResourcesRead_UnknownURI(t *testing.T) {
	srv := newExtSrv()
	resp, hasResp := handleReq(t, srv, map[string]any{
		"jsonrpc": "2.0",
		"id":      512,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": "fathom:///unknown/path",
		},
	})
	if !hasResp {
		t.Fatal("resources/read unknown: no response")
	}
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error for unknown resource, got: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("code = %v, want %v", code, mcp.ErrCodeNotFound)
	}
}

// ─── call logging ─────────────────────────────────────────────────────────────

func TestCallLogger_WritesEntries(t *testing.T) {
	var buf bytes.Buffer
	logger := mcp.NewCallLogger(&buf)

	backend := &extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}
	srv := mcp.NewServerWithLogger(backend, logger)

	// Make a few calls.
	for _, req := range []request{
		{
			JSONRPC: "2.0", ID: 600, Method: "tools/call",
			Params: map[string]any{
				"name":      "find_callees",
				"arguments": map[string]any{"service": "svc", "symbol": "A"},
			},
		},
		{
			JSONRPC: "2.0", ID: 601, Method: "tools/call",
			Params: map[string]any{
				"name":      "complexity",
				"arguments": map[string]any{"service": "svc"},
			},
		},
	} {
		_ = roundtrip(t, srv, req)
	}

	// Parse written JSONL lines.
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 log lines, got %d: %q", len(lines), buf.String())
	}
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line parse: %v (line=%q)", err, line)
		}
		if entry["ts"] == nil {
			t.Error("log entry missing ts")
		}
		if entry["tool"] == nil {
			t.Error("log entry missing tool")
		}
		if entry["latency_ms"] == nil {
			t.Error("log entry missing latency_ms")
		}
	}
}

func TestCallLogger_TimestampValid(t *testing.T) {
	var buf bytes.Buffer
	logger := mcp.NewCallLogger(&buf)
	backend := &extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}}
	srv := mcp.NewServerWithLogger(backend, logger)

	_ = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 700, Method: "tools/call",
		Params: map[string]any{
			"name":      "entrypoints",
			"arguments": map[string]any{"service": "svc"},
		},
	})

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("no log lines written")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("parse: %v", err)
	}
	tsStr, ok := entry["ts"].(string)
	if !ok {
		t.Fatalf("ts is not a string: %T %v", entry["ts"], entry["ts"])
	}
	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		t.Fatalf("ts not RFC3339: %v (value=%q)", err, tsStr)
	}
	if time.Since(ts) > 10*time.Second {
		t.Errorf("ts too far in the past: %v", ts)
	}
}

// ─── profile membership for new tools ────────────────────────────────────────

func TestNewToolsInCorrectProfile(t *testing.T) {
	srv := newExtSrv()

	// set_profile is gone; select each group via the HTTP path scope instead
	// (see TestSetProfileDirect for the stdio/surface equivalence this relies
	// on).
	toolsInProfile := func(profile string) map[string]bool {
		ctx := mcp.WithRequestScope(context.Background(), profile, true)
		listResp := scopedRoundtrip(t, srv, ctx, request{
			JSONRPC: "2.0", ID: 801, Method: "tools/list",
		})
		if listResp["error"] != nil {
			t.Fatalf("tools/list: %v", listResp["error"])
		}
		result := listResp["result"].(map[string]any)
		tools := result["tools"].([]any)
		m := make(map[string]bool, len(tools))
		for _, raw := range tools {
			m[raw.(map[string]any)["name"].(string)] = true
		}
		return m
	}

	codingTools := toolsInProfile("coding")
	for _, name := range []string{"find_callees", "call_hierarchy", "symbol_context", "outline", "entrypoints", "complexity", "god_object", "api_surface"} {
		if !codingTools[name] {
			t.Errorf("coding profile missing tool %q", name)
		}
	}

	investigationTools := toolsInProfile("investigation")
	for _, name := range []string{"community", "type_hierarchy"} {
		if !investigationTools[name] {
			t.Errorf("investigation profile missing tool %q", name)
		}
	}
}
