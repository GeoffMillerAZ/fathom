// Package mcp_test — TDD tests for platform/fleet tools.
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/mcp"
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

// TestPlatformCallers_AliasRemoved: platform_callers was deleted as a
// zero-call alias (docs/plans/surfaces-week1.md SW-1) — the canonical
// cross_repo_callers stays.
func TestPlatformCallers_AliasRemoved(t *testing.T) {
	srv := newFleetServer(fleetFixture())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1031, Method: "tools/call",
		Params: map[string]any{
			"name":      "platform_callers",
			"arguments": map[string]any{"symbol": "Charge"},
		},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("platform_callers: want tool-not-found, got result: %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("platform_callers: code = %v, want ErrCodeNotFound (%d)", code, mcp.ErrCodeNotFound)
	}

	// The canonical name still works.
	canon := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1032, Method: "tools/call",
		Params: map[string]any{
			"name":      "cross_repo_callers",
			"arguments": map[string]any{"symbol": "Charge"},
		},
	})
	p := platformToolPayload(t, canon)
	if p["callers"] == nil {
		t.Fatalf("cross_repo_callers.callers missing: %v", p)
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

// ─── cross_repo_callers: plain pkg.Func edges (Go extractor format) ──────────

// goExtractorFleet builds a fleet where call edges are stored as plain "pkg.Func"
// strings — the format emitted by the Go extractor for cross-package calls, which
// has no "::" service prefix.
func goExtractorFleet() *fakeFleet {
	return &fakeFleet{
		svcs: map[string][]core.SymbolNode{
			// svc-a calls FormatCents (in svc-b) using the plain "output.FormatCents" edge
			// and NewLedger (in svc-b) using plain "eventstore.New" edge.
			"svc-a": {
				{
					Service: "svc-a", Package: "billing", Name: "ProcessInvoice", Kind: core.KindGoFunc, FilePath: "billing/invoice.go",
					Calls: []string{"output.FormatCents", "stripe.Charge"},
				},
				{
					Service: "svc-a", Package: "store", Name: "InitStore", Kind: core.KindGoFunc, FilePath: "store/init.go",
					Calls: []string{"eventstore.New"},
				},
			},
			// svc-b owns the symbols being called.
			"svc-b": {
				{Service: "svc-b", Package: "output", Name: "FormatCents", Kind: core.KindGoFunc, FilePath: "output/format.go"},
				{Service: "svc-b", Package: "eventstore", Name: "New", Kind: core.KindGoFunc, FilePath: "eventstore/store.go"},
			},
		},
		obs: map[string][]core.Observation{
			"svc-a": {
				{ServiceID: "svc-a", Symbol: "ProcessInvoice", Calls: []string{"output.FormatCents", "stripe.Charge"}},
				{ServiceID: "svc-a", Symbol: "InitStore", Calls: []string{"eventstore.New"}},
			},
			"svc-b": {},
		},
	}
}

func TestCrossRepoCallers_BareNameMatchesQualifiedEdge(t *testing.T) {
	srv := newFleetServer(goExtractorFleet())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1050, Method: "tools/call",
		Params: map[string]any{
			"name":      "cross_repo_callers",
			"arguments": map[string]any{"symbol": "FormatCents"},
		},
	})
	p := platformToolPayload(t, resp)
	callers, ok := p["callers"].([]any)
	if !ok {
		t.Fatalf("callers missing: %v", p)
	}
	// ProcessInvoice in svc-a calls "output.FormatCents" — bare "FormatCents" must match.
	if len(callers) == 0 {
		t.Fatal("cross_repo_callers(FormatCents): expected caller ProcessInvoice from svc-a, got 0")
	}
	found := false
	for _, raw := range callers {
		c := raw.(map[string]any)
		if c["symbol"] == "ProcessInvoice" {
			found = true
		}
	}
	if !found {
		t.Errorf("ProcessInvoice not in callers: %v", callers)
	}
}

func TestCrossRepoCallers_QualifiedNameMatchesQualifiedEdge(t *testing.T) {
	srv := newFleetServer(goExtractorFleet())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1051, Method: "tools/call",
		Params: map[string]any{
			"name":      "cross_repo_callers",
			"arguments": map[string]any{"symbol": "output.FormatCents"},
		},
	})
	p := platformToolPayload(t, resp)
	callers, ok := p["callers"].([]any)
	if !ok {
		t.Fatalf("callers missing: %v", p)
	}
	if len(callers) == 0 {
		t.Fatal("cross_repo_callers(output.FormatCents): expected at least one caller, got 0")
	}
}

func TestCrossRepoCallers_CrossServiceCallerReturned(t *testing.T) {
	// Verifies that callers from ALL services (including the symbol's home service)
	// appear in results.
	srv := newFleetServer(goExtractorFleet())
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1052, Method: "tools/call",
		Params: map[string]any{
			"name":      "cross_repo_callers",
			"arguments": map[string]any{"symbol": "New"},
		},
	})
	p := platformToolPayload(t, resp)
	callers, ok := p["callers"].([]any)
	if !ok {
		t.Fatalf("callers missing: %v", p)
	}
	// InitStore in svc-a calls "eventstore.New" — bare "New" must match.
	if len(callers) == 0 {
		t.Fatal("cross_repo_callers(New): expected InitStore from svc-a, got 0")
	}
	services := make(map[string]bool)
	for _, raw := range callers {
		c := raw.(map[string]any)
		services[c["service"].(string)] = true
	}
	if !services["svc-a"] {
		t.Errorf("expected svc-a in results, got services: %v", services)
	}
}

// ─── platform profile includes fleet tools ───────────────────────────────────

func TestPlatformProfile_ShowsFleetTools(t *testing.T) {
	srv := newFleetServer(fleetFixture())

	// set_profile is gone; select the platform surface via the HTTP path
	// scope instead (see TestSetProfileDirect for the stdio/surface
	// equivalence this relies on).
	ctx := mcp.WithRequestScope(context.Background(), "platform", true)
	listResp := scopedRoundtrip(t, srv, ctx, request{
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
	for _, want := range []string{"list_services", "query_cross_service", "fleet_drift", "cross_repo_callers"} {
		if !names[want] {
			t.Errorf("platform profile missing tool %q", want)
		}
	}
	if names["platform_callers"] {
		t.Error("platform profile still lists platform_callers — alias should be gone")
	}
}

// TestListServices_IncludesRecordedPath (dogfood O15): list_services carries
// each service's recorded corpus path — the authoritative repo↔service
// identity the hooks resolve against. A nil backend (the plain fleet server
// above) simply omits the field, which TestListServices_Basic still covers.
func TestListServices_IncludesRecordedPath(t *testing.T) {
	fleet := fleetFixture()
	srv := mcp.NewServerWithFleet(pathMetaBackend{}, fleet)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1001, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_services",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	svcs := p["services"].([]any)
	for _, raw := range svcs {
		entry := raw.(map[string]any)
		svc := entry["service"].(string)
		path, _ := entry["path"].(string)
		if want := "/corpora/" + svc; path != want {
			t.Errorf("service %q path = %q, want %q", svc, path, want)
		}
	}
}

// pathMetaBackend is a minimal QueryInterface whose only meaningful method is
// ServiceMeta — every other call is an honest zero value.
type pathMetaBackend struct{ mcp.QueryInterface }

func (pathMetaBackend) ServiceMeta(_ context.Context, service string) (string, string, bool, error) {
	return "/corpora/" + service, "deadbeef", true, nil
}
