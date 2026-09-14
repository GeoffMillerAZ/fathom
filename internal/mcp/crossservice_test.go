// Package mcp_test — P1-2 (cross-service call stitching for symbol_context
// and find_callers against a fleet store) tests.
// docs/plans/pulled-tools-excellence.md; reproduction from
// docs/dogfood-observations.md O12(a).
package mcp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// fleetAsQuery adapts a *fakeFleet (platform_tools_test.go) to
// mcp.QueryInterface, so a single fixture is visible through BOTH roles a
// fleet daemon actually wires: the per-service QueryInterface backend AND
// the FleetBackend consulted for cross-service stitching — exactly like
// production's client.Client + fleetAdapter (cmd/fathom/fleetadapter.go,
// cmd/fathom/servehttp.go), both backed by the same store. A single Go
// type can't implement both interfaces directly (FleetBackend's
// Services() []string and QueryInterface's Services(ctx) ([]string,
// error) collide on the method name), hence the adapter.
type fleetAsQuery struct{ f *fakeFleet }

func (q fleetAsQuery) QuerySymbols(_ context.Context, _ core.Query) (core.QueryResult, error) {
	return core.QueryResult{}, nil
}

func (q fleetAsQuery) FindCallers(_ context.Context, serviceID, symbol string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range q.f.svcs[serviceID] {
		for _, c := range n.Calls {
			if c == symbol {
				out = append(out, n)
				break
			}
		}
	}
	return out, nil
}

func (q fleetAsQuery) FindSymbol(_ context.Context, serviceID, name string) ([]core.SymbolNode, error) {
	var out []core.SymbolNode
	for _, n := range q.f.svcs[serviceID] {
		if n.Name == name {
			out = append(out, n)
		}
	}
	return out, nil
}

func (q fleetAsQuery) ListAllSymbols(_ context.Context, serviceID string) ([]core.SymbolNode, error) {
	return q.f.svcs[serviceID], nil
}

func (q fleetAsQuery) Services(_ context.Context) ([]string, error) { return q.f.Services(), nil }

func (q fleetAsQuery) Sources(_ context.Context, _ string) ([]core.SourceFile, error) {
	return nil, nil
}

func (q fleetAsQuery) IngestService(_ context.Context, _ string) error { return nil }

func (q fleetAsQuery) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func (q fleetAsQuery) ListCommits(_ context.Context, _ string) ([]mcp.CommitInfo, error) {
	return nil, nil
}

func (q fleetAsQuery) TypedEdges(_ context.Context, _ string) ([]core.Edge, error) {
	return nil, nil
}

var _ mcp.QueryInterface = fleetAsQuery{}

// heartbeatFleetFixture reproduces O12(a): fathom's cmdInstrument
// (cmd/fathom/instrument.go) calls agentic-kit's heartbeat.Load /
// CheckLiveness / Rollup — package-qualified callee keys, the Go
// extractor's cross-package call form (no "::" service prefix — see
// internal/extractor/goextract.go collectCalls). agentic-kit also has an
// entirely same-service caller of CheckLiveness (its own test) so the
// same-service/cross-service split has something real to prove.
func heartbeatFleetFixture() *fakeFleet {
	return &fakeFleet{
		svcs: map[string][]core.SymbolNode{
			"fathom": {
				{
					Service: "fathom", Package: "main", Name: "cmdInstrument", Kind: core.KindGoFunc,
					FilePath: "cmd/fathom/instrument.go", Line: 30,
					Calls: []string{"heartbeat.Load", "heartbeat.CheckLiveness", "heartbeat.Rollup"},
				},
			},
			"agentic-kit": {
				{Service: "agentic-kit", Package: "heartbeat", Name: "Load", Kind: core.KindGoFunc, FilePath: "foundations/telemetry/heartbeat/heartbeat.go", Line: 10},
				{Service: "agentic-kit", Package: "heartbeat", Name: "CheckLiveness", Kind: core.KindGoFunc, FilePath: "foundations/telemetry/heartbeat/heartbeat.go", Line: 20},
				{Service: "agentic-kit", Package: "heartbeat", Name: "Rollup", Kind: core.KindGoFunc, FilePath: "foundations/telemetry/heartbeat/heartbeat.go", Line: 30},
				{
					Service: "agentic-kit", Package: "heartbeat_test", Name: "TestCheckLiveness", Kind: core.KindGoFunc,
					FilePath: "foundations/telemetry/heartbeat/heartbeat_test.go", Line: 5,
					Calls: []string{"CheckLiveness"},
				},
			},
		},
		obs: map[string][]core.Observation{},
	}
}

func newHeartbeatFleetServer() *mcp.Server {
	fleet := heartbeatFleetFixture()
	return mcp.NewServerWithFleet(fleetAsQuery{f: fleet}, fleet)
}

// ─── symbol_context: cross-service callees ────────────────────────────────────

// TestSymbolContext_CrossServiceCallees is the O12(a) proof target:
// symbol_context(fathom, cmdInstrument) must surface the heartbeat.Load /
// CheckLiveness / Rollup callees into agentic-kit — invisible to the
// per-service backend alone.
func TestSymbolContext_CrossServiceCallees(t *testing.T) {
	srv := newHeartbeatFleetServer()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 100, Method: "tools/call",
		Params: map[string]any{
			"name":      "symbol_context",
			"arguments": map[string]any{"service": "fathom", "symbol": "cmdInstrument"},
		},
	})
	p := platformToolPayload(t, resp)
	raw, ok := p["cross_service_callees"].([]any)
	if !ok {
		t.Fatalf("cross_service_callees missing: %v", p)
	}
	want := map[string]bool{"Load": false, "CheckLiveness": false, "Rollup": false}
	for _, r := range raw {
		entry := r.(map[string]any)
		if entry["service"] != "agentic-kit" {
			t.Errorf("cross_service_callees entry service = %v, want agentic-kit: %v", entry["service"], entry)
		}
		if entry["scope"] != "cross-service" {
			t.Errorf("cross_service_callees entry scope = %v, want cross-service: %v", entry["scope"], entry)
		}
		if sym, ok := entry["symbol"].(string); ok {
			if _, known := want[sym]; known {
				want[sym] = true
			}
		}
	}
	for sym, found := range want {
		if !found {
			t.Errorf("cross_service_callees missing %q: %v", sym, raw)
		}
	}
	// Same-service "callees" must stay exactly its existing (string-key,
	// same-service-only) shape — never silently merged with the
	// cross-service hits.
	callees, _ := p["callees"].([]any)
	for _, c := range callees {
		if s, ok := c.(string); ok && (s == "heartbeat.Load" || s == "heartbeat.CheckLiveness" || s == "heartbeat.Rollup") {
			t.Errorf("cross-service callee leaked into same-service \"callees\": %v", callees)
		}
	}
	// review F4 (VB-19): resolution must count the 3 cross-service
	// callees too — every one heuristic, since no typed pass crosses a
	// service boundary — not just the (empty) same-service tally.
	resolution, ok := p["resolution"].(map[string]any)
	if !ok {
		t.Fatalf("resolution missing or wrong type: %v", p)
	}
	if typed, _ := resolution["typed"].(float64); typed != 0 {
		t.Errorf("resolution.typed = %v, want 0", resolution["typed"])
	}
	if heuristic, _ := resolution["heuristic"].(float64); heuristic != 3 {
		t.Errorf("resolution.heuristic = %v, want 3 (the 3 cross-service callees, same-service tally is empty)", resolution["heuristic"])
	}
}

// ─── find_callers: cross-service callers ──────────────────────────────────────

// TestFindCallers_CrossServiceCallers is O12(a)'s reverse direction:
// find_callers(agentic-kit, CheckLiveness) must surface fathom's
// cmdInstrument alongside agentic-kit's own TestCheckLiveness.
func TestFindCallers_CrossServiceCallers(t *testing.T) {
	srv := newHeartbeatFleetServer()
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 101, Method: "tools/call",
		Params: map[string]any{
			"name":      "find_callers",
			"arguments": map[string]any{"service": "agentic-kit", "symbol": "CheckLiveness"},
		},
	})
	p := platformToolPayload(t, resp)

	// Same-service caller (TestCheckLiveness) stays in the existing
	// "callers" field, unaffected.
	callers, ok := p["callers"].([]any)
	if !ok || len(callers) == 0 {
		t.Fatalf("callers missing same-service TestCheckLiveness: %v", p)
	}
	// SB-47: callers is the compact {loc, c} wire shape — "loc" ends in the
	// symbol name ("file:line name"), there is no separate "name" field.
	foundSameService := false
	for _, c := range callers {
		entry := c.(map[string]any)
		if loc, _ := entry["loc"].(string); strings.HasSuffix(loc, " TestCheckLiveness") {
			foundSameService = true
		}
	}
	if !foundSameService {
		t.Errorf("callers missing TestCheckLiveness: %v", callers)
	}

	// Cross-service caller: fathom's cmdInstrument, in its OWN field,
	// scope-labeled.
	cross, ok := p["cross_service_callers"].([]any)
	if !ok || len(cross) == 0 {
		t.Fatalf("cross_service_callers missing fathom::main.cmdInstrument: %v", p)
	}
	found := false
	for _, r := range cross {
		entry := r.(map[string]any)
		if entry["service"] == "fathom" && entry["symbol"] == "cmdInstrument" {
			if entry["scope"] != "cross-service" {
				t.Errorf("cross_service_callers entry scope = %v, want cross-service", entry["scope"])
			}
			found = true
		}
	}
	if !found {
		t.Errorf("cross_service_callers missing fathom::main.cmdInstrument: %v", cross)
	}
}

// TestSymbolContext_NoFleetBackend_OmitsCrossServiceFields proves a
// non-fleet server (nil FleetBackend — stdio serve-mcp without
// NewServerWithFleet) never adds the cross_service_* fields at all,
// rather than an empty/zero-value shape.
func TestSymbolContext_NoFleetBackend_OmitsCrossServiceFields(t *testing.T) {
	srv := mcp.NewServer(&extendedFakeBackend{nodes: extFixtureNodes(), services: []string{"svc"}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 102, Method: "tools/call",
		Params: map[string]any{
			"name":      "symbol_context",
			"arguments": map[string]any{"service": "svc", "symbol": "A"},
		},
	})
	p := platformToolPayload(t, resp)
	if _, ok := p["cross_service_callees"]; ok {
		t.Errorf("cross_service_callees should be omitted with no fleet backend: %v", p)
	}
	if _, ok := p["cross_service_callers"]; ok {
		t.Errorf("cross_service_callers should be omitted with no fleet backend: %v", p)
	}
}
