// Package mcp_test — status (SR-23, F-11, design §4): one tool answering
// what the retired capabilities/server_stats/list_sources tools each
// answered, byte-compatible per section.
package mcp_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// TestStatusSectionsMatchRetiredTools proves each of status' three
// sections answers exactly what its retired tool used to: "tools" mirrors
// capabilities' {tools, active_surface, active_profile, tool_count,
// extraction_engines, schema_bytes}, "daemon" mirrors server_stats'
// {uptime_seconds, request_count, started_at, ...}, and "sources" mirrors
// list_sources' {sources: [...]}.
func TestStatusSectionsMatchRetiredTools(t *testing.T) {
	srv := newExtSrv()

	// ─── section: "tools" (byte-compatible with the retired capabilities) ──
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "tools"}},
	})
	p := extToolPayload(t, resp)
	for _, key := range []string{"tools", "active_surface", "active_profile", "tool_count", "extraction_engines", "schema_bytes"} {
		if _, ok := p[key]; !ok {
			t.Errorf(`status(section:"tools") missing %q: %v`, key, p)
		}
	}
	names, ok := p["tools"].([]any)
	if !ok {
		t.Fatalf(`status(section:"tools").tools is not a list: %v`, p["tools"])
	}
	if count, ok := p["tool_count"].(float64); !ok || int(count) != len(names) {
		t.Errorf(`status(section:"tools").tool_count = %v, want %d (len(tools))`, p["tool_count"], len(names))
	}

	// ─── section: "daemon" (byte-compatible with the retired server_stats) ─
	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "daemon"}},
	})
	p = extToolPayload(t, resp)
	for _, key := range []string{"uptime_seconds", "request_count", "started_at"} {
		if _, ok := p[key]; !ok {
			t.Errorf(`status(section:"daemon") missing %q: %v`, key, p)
		}
	}
	// A daemon-section call never demands a service (server_stats' was
	// optional, adding typed_edges only when present).
	if _, ok := p["typed_edges"]; ok {
		t.Errorf(`status(section:"daemon") without a service should omit typed_edges: %v`, p)
	}

	// ─── section: "sources" (byte-compatible with the retired list_sources) ─
	resp = roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "sources", "service": "svc"}},
	})
	p = extToolPayload(t, resp)
	sources, ok := p["sources"].([]any)
	if !ok {
		t.Fatalf(`status(section:"sources").sources missing or wrong shape: %v`, p)
	}
	want := map[string]bool{"pkg/a.go": true, "pkg/c.go": true, "pkg/a_test.go": true, "pkg/orphan.go": true, "pkg/types.go": true}
	if len(sources) != len(want) {
		t.Errorf(`status(section:"sources").sources = %v, want %d entries`, sources, len(want))
	}
	// "sources" is the one section that carries a fleet/service pivot risk
	// (SR-23): it requires a service, like list_sources did.
	errResp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "sources"}},
	})
	if errResp["error"] == nil {
		t.Fatalf(`status(section:"sources") with no service should error, got %v`, errResp)
	}

	// ─── an unknown/missing section is a params error ──────────────────────
	for _, args := range []map[string]any{{}, {"section": "bogus"}} {
		bad := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 5, Method: "tools/call",
			Params: map[string]any{"name": "status", "arguments": args},
		})
		errObj, ok := bad["error"].(map[string]any)
		if !ok {
			t.Fatalf("status(%v) should error, got %v", args, bad)
		}
		if code := errObj["code"].(float64); code != float64(mcp.ErrCodeParams) {
			t.Errorf("status(%v) code = %v, want %v", args, code, mcp.ErrCodeParams)
		}
	}

	// ─── the retired tool names are gone outright ──────────────────────────
	for _, retired := range []string{"capabilities", "server_stats", "list_sources"} {
		gone := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 6, Method: "tools/call",
			Params: map[string]any{"name": retired, "arguments": map[string]any{"service": "svc"}},
		})
		if gone["error"] == nil {
			t.Errorf("retired tool %q should no longer be callable, got %v", retired, gone)
		}
	}
}

// TestStatus_ToolsSectionExtractionEngines pins the retired capabilities
// tool's per-language extraction engine provenance (ADR-021 §4 / XT-5) —
// tree-sitter-wasm vs the languages that kept their incumbent — is
// discoverable through status(section:"tools") without reading source.
func TestStatus_ToolsSectionExtractionEngines(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 9, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "tools"}},
	})
	p := extToolPayload(t, resp)
	engines, ok := p["extraction_engines"].(map[string]any)
	if !ok || len(engines) == 0 {
		t.Fatalf(`status(section:"tools").extraction_engines missing/empty: %v`, p)
	}
	if got := engines[".py"]; got != "tree-sitter-wasm" {
		t.Errorf("extraction_engines[.py] = %v, want tree-sitter-wasm", got)
	}
}

// TestStatus_DaemonTypedEdges is CT-1's "counters in server_stats" proof
// (ADR-021 §3), moved to status(section:"daemon") by SR-23: passing a
// service adds a typed_edges block with honest kind counts — the visible
// signal for whether a store's typed pass has run at all.
func TestStatus_DaemonTypedEdges(t *testing.T) {
	srv := newDispatchSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 10, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "daemon", "service": "svc"}},
	})
	p := extToolPayload(t, resp)
	typedEdges, ok := p["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf(`status(section:"daemon").typed_edges missing or wrong shape: %v`, p)
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

// TestStatus_DaemonTypedEdgesZeroForUntypedService confirms a service with
// no typed edges yet (predates ADR-021 §3, or hasn't been re-ingested)
// reports all-zero counts rather than omitting the block — the honest
// "the pass hasn't run" signal.
func TestStatus_DaemonTypedEdgesZeroForUntypedService(t *testing.T) {
	srv := newExtSrv()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 11, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "daemon", "service": "svc"}},
	})
	p := extToolPayload(t, resp)
	typedEdges, ok := p["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf(`status(section:"daemon").typed_edges missing or wrong shape: %v`, p)
	}
	if typedEdges["total"].(float64) != 0 {
		t.Errorf("typed_edges.total = %v, want 0", typedEdges["total"])
	}
}

// TestStatusToolsSectionCountMatchesToolsList: status(section:"tools")'
// tool_count must equal a real tools/list on the SAME surface — the
// capabilities contract TestCapabilitiesReportsActiveSurface used to pin.
func TestStatusToolsSectionCountMatchesToolsList(t *testing.T) {
	srv := newExtSrv()
	listResp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/list", Params: map[string]any{},
	})
	names := toolNames(t, listResp)

	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "tools"}},
	})
	p := extToolPayload(t, resp)
	if count := p["tool_count"].(float64); int(count) != len(names) {
		t.Errorf("status(section:tools).tool_count = %v, want %d (tools/list count)", count, len(names))
	}
}
