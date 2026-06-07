// Package mcp_test — TDD tests for platform/fleet tools.
package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
	"github.com/geoffmilleraz/sextant/internal/mcp"
)

// ─── fake fleet backend ───────────────────────────────────────────────────────

type fakeFleet struct {
	svcs map[string][]core.SymbolNode
	obs  map[string][]core.Observation
}

func (f *fakeFleet) Services() []string {
	out := make([]string, 0, len(f.svcs))
	for k := range f.svcs {
		out = append(out, k)
	}
	// sort for determinism
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i] > out[j] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func (f *fakeFleet) SymbolsFor(service string) []core.SymbolNode {
	return f.svcs[service]
}

func (f *fakeFleet) ObservationsFor(service string) []core.Observation {
	return f.obs[service]
}

// compile-time check
var _ mcp.FleetBackend = (*fakeFleet)(nil)

// fleetFixture builds a small two-service fleet with a shared symbol to exercise
// cross-service queries.
func fleetFixture() *fakeFleet {
	return &fakeFleet{
		svcs: map[string][]core.SymbolNode{
			"svc-a": {
				{Service: "svc-a", Package: "auth", Name: "Login", Kind: core.KindGoFunc, FilePath: "auth/login.go", Calls: []string{"svc-b::payment.Charge"}},
				{Service: "svc-a", Package: "auth", Name: "Logout", Kind: core.KindGoFunc, FilePath: "auth/login.go"},
			},
			"svc-b": {
				{Service: "svc-b", Package: "payment", Name: "Charge", Kind: core.KindGoFunc, FilePath: "payment/charge.go"},
				// Same symbol name as svc-a but different kind — triggers drift.
				{Service: "svc-b", Package: "payment", Name: "Login", Kind: core.KindGoType, FilePath: "payment/types.go"},
			},
		},
		obs: map[string][]core.Observation{
			"svc-a": {
				{ServiceID: "svc-a", Symbol: "Login", Claim: "func Login() handles authentication", Calls: []string{"svc-b::payment.Charge"}},
			},
			"svc-b": {},
		},
	}
}

// newFleetServer builds an mcp.Server with platform tools registered.
func newFleetServer(fleet mcp.FleetBackend) *mcp.Server {
	return mcp.NewServerWithFleet(nil, fleet)
}

// platformToolPayload extracts the inner JSON payload from a tools/call response.
func platformToolPayload(t *testing.T, resp map[string]any) map[string]any {
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

// ─── list_services ────────────────────────────────────────────────────────────

func TestListServices_Basic(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1000, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_services",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	svcs, ok := p["services"].([]any)
	if !ok {
		t.Fatalf("list_services.services missing: %v", p)
	}
	if len(svcs) != 2 {
		t.Errorf("list_services: expected 2 services, got %d", len(svcs))
	}
	// Each entry must have service, symbol_count, languages.
	for _, raw := range svcs {
		entry := raw.(map[string]any)
		if entry["service"] == nil {
			t.Error("list_services entry missing 'service'")
		}
		if entry["symbol_count"] == nil {
			t.Error("list_services entry missing 'symbol_count'")
		}
		if entry["languages"] == nil {
			t.Error("list_services entry missing 'languages'")
		}
	}
}

func TestListServices_Empty(t *testing.T) {
	srv := newFleetServer(&fakeFleet{svcs: map[string][]core.SymbolNode{}, obs: map[string][]core.Observation{}})
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1001, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_services",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	if count, _ := p["count"].(float64); count != 0 {
		t.Errorf("list_services count = %v, want 0", p["count"])
	}
}

// ─── query_cross_service ──────────────────────────────────────────────────────

func TestQueryCrossService_MatchesSymbols(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1010, Method: "tools/call",
		Params: map[string]any{
			"name":      "query_cross_service",
			"arguments": map[string]any{"keyword": "login"},
		},
	})
	p := platformToolPayload(t, resp)
	matches, ok := p["matches"].([]any)
	if !ok {
		t.Fatalf("query_cross_service.matches missing: %v", p)
	}
	// "Login" appears in both svc-a and svc-b.
	if len(matches) < 2 {
		t.Errorf("expected at least 2 matches for 'login', got %d: %v", len(matches), matches)
	}
}

func TestQueryCrossService_MissingKeyword(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1011, Method: "tools/call",
		Params: map[string]any{
			"name":      "query_cross_service",
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

func TestQueryCrossService_NoMatches(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1012, Method: "tools/call",
		Params: map[string]any{
			"name":      "query_cross_service",
			"arguments": map[string]any{"keyword": "xyzzy_nonexistent"},
		},
	})
	p := platformToolPayload(t, resp)
	matches, ok := p["matches"].([]any)
	if !ok {
		// An empty JSON array may be nil after unmarshal; treat nil as ok.
		if p["matches"] != nil {
			t.Fatalf("matches unexpected type: %T", p["matches"])
		}
		matches = nil
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches for nonexistent keyword, got %d", len(matches))
	}
}

// ─── fleet_drift ──────────────────────────────────────────────────────────────

func TestFleetDrift_DetectsDrift(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1020, Method: "tools/call",
		Params: map[string]any{
			"name":      "fleet_drift",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	drift, ok := p["drift"].([]any)
	if !ok {
		t.Fatalf("fleet_drift.drift missing: %v", p)
	}
	// "Login" appears in both services with different kinds, so at least one drift item.
	if len(drift) == 0 {
		t.Error("fleet_drift: expected at least one drift item (Login has conflicting kinds)")
	}
	// Each item must have key, service, value, majority.
	for _, raw := range drift {
		item := raw.(map[string]any)
		for _, field := range []string{"key", "service", "value", "majority"} {
			if item[field] == nil {
				t.Errorf("drift item missing field %q: %v", field, item)
			}
		}
	}
}

func TestFleetDrift_NoDrift(t *testing.T) {
	// Fleet where no symbol appears in more than one service — no drift possible.
	fleet := &fakeFleet{
		svcs: map[string][]core.SymbolNode{
			"a": {{Service: "a", Name: "Foo", Kind: core.KindGoFunc}},
			"b": {{Service: "b", Name: "Bar", Kind: core.KindGoFunc}},
		},
		obs: map[string][]core.Observation{},
	}
	srv := newFleetServer(fleet)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1021, Method: "tools/call",
		Params: map[string]any{
			"name":      "fleet_drift",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	drift, _ := p["drift"].([]any)
	if len(drift) != 0 {
		t.Errorf("expected no drift when symbols don't overlap; got %v", drift)
	}
}

// ─── cross_repo_callers / platform_callers ────────────────────────────────────

func TestCrossRepoCallers_FindsCallers(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1030, Method: "tools/call",
		Params: map[string]any{
			"name":      "cross_repo_callers",
			"arguments": map[string]any{"symbol": "Charge"},
		},
	})
	p := platformToolPayload(t, resp)
	callers, ok := p["callers"].([]any)
	if !ok {
		t.Fatalf("cross_repo_callers.callers missing: %v", p)
	}
	// svc-a Login calls svc-b::payment.Charge so Login should appear.
	if len(callers) == 0 {
		t.Error("cross_repo_callers(Charge): expected at least one caller (Login in svc-a)")
	}
}

func TestPlatformCallers_Alias(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1031, Method: "tools/call",
		Params: map[string]any{
			"name":      "platform_callers",
			"arguments": map[string]any{"symbol": "Charge"},
		},
	})
	p := platformToolPayload(t, resp)
	if p["callers"] == nil {
		t.Fatalf("platform_callers.callers missing: %v", p)
	}
}

func TestCrossRepoCallers_MissingSymbol(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1032, Method: "tools/call",
		Params: map[string]any{
			"name":      "cross_repo_callers",
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

// ─── platform profile includes fleet tools ───────────────────────────────────

func TestPlatformProfile_ShowsFleetTools(t *testing.T) {
	srv := newFleetServer(fleetFixture())

	// Switch to platform profile.
	setResp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1040, Method: "tools/call",
		Params: map[string]any{
			"name":      "set_profile",
			"arguments": map[string]any{"profile": "platform"},
		},
	})
	if setResp["error"] != nil {
		t.Fatalf("set_profile(platform): %v", setResp["error"])
	}

	listResp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1041, Method: "tools/list",
	})
	if listResp["error"] != nil {
		t.Fatalf("tools/list error: %v", listResp["error"])
	}
	result := listResp["result"].(map[string]any)
	tools := result["tools"].([]any)
	names := make(map[string]bool, len(tools))
	for _, raw := range tools {
		names[raw.(map[string]any)["name"].(string)] = true
	}
	for _, want := range []string{"list_services", "query_cross_service", "fleet_drift", "cross_repo_callers", "platform_callers"} {
		if !names[want] {
			t.Errorf("platform profile missing tool %q", want)
		}
	}
}
