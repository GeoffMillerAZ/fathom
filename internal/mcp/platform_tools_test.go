// Package mcp_test — TDD tests for platform/fleet tools.
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// ─── fake fleet backend ───────────────────────────────────────────────────────

type fakeFleet struct {
	svcs map[string][]core.SymbolNode
	obs  map[string][]core.Observation
	// unsupportedLangs is O69's fixture hook for UnsupportedLanguagesFor —
	// service -> language display name -> file count. A nil/absent entry
	// (the zero value every existing fixture gets for free) means "none",
	// matching a real fleet backend with nothing unsupported to report.
	unsupportedLangs map[string]map[string]int
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

func (f *fakeFleet) UnsupportedLanguagesFor(service string) map[string]int {
	return f.unsupportedLangs[service]
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

// TestListServices_IncludesUnsupportedLanguages (O69, wave P1): a service
// with source in a known-but-unextracted language (the Long Code Arena bench
// run's bumble-tech/appyx finding — a real Kotlin repo indexed as near-empty
// with no signal anywhere that Kotlin was the reason) must say so by name
// and file count, not stay indistinguishable from a service that classifies
// everything.
func TestListServices_IncludesUnsupportedLanguages(t *testing.T) {
	fleet := fleetFixture()
	fleet.unsupportedLangs = map[string]map[string]int{
		"svc-a": {"kotlin": 412},
	}
	srv := newFleetServer(fleet)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1002, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_services",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	svcs := p["services"].([]any)
	var sawA, sawB bool
	for _, raw := range svcs {
		entry := raw.(map[string]any)
		switch entry["service"] {
		case "svc-a":
			sawA = true
			langs, ok := entry["unsupported_languages"].([]any)
			if !ok || len(langs) != 1 {
				t.Fatalf("svc-a unsupported_languages = %v, want one entry", entry["unsupported_languages"])
			}
			row := langs[0].(map[string]any)
			if row["language"] != "kotlin" {
				t.Errorf("svc-a unsupported_languages[0].language = %v, want %q", row["language"], "kotlin")
			}
			if got, _ := row["files"].(float64); got != 412 {
				t.Errorf("svc-a unsupported_languages[0].files = %v, want 412", row["files"])
			}
		case "svc-b":
			sawB = true
			if _, ok := entry["unsupported_languages"]; ok {
				t.Errorf("svc-b has no unsupported languages — the key must be omitted, got %v", entry["unsupported_languages"])
			}
		}
	}
	if !sawA || !sawB {
		t.Fatalf("expected both svc-a and svc-b in results: %v", svcs)
	}
}

// ─── list_services: typed_edges outcome (wave P3 / O82) ──────────────────────

// perServiceTypedStatusBackend is a minimal QueryInterface whose only
// meaningful method is TypedEdgeStatus, keyed per service — list_services'
// typed_edges row must name the SAME per-service outcome doctor's own
// typed_edges row reads (both ultimately read Pipeline.typedEdgeStatus).
type perServiceTypedStatusBackend struct {
	mcp.QueryInterface
	statuses map[string]fusion.TypedEdgeStatus
}

func (b perServiceTypedStatusBackend) TypedEdgeStatus(service string) (fusion.TypedEdgeStatus, bool) {
	st, ok := b.statuses[service]
	return st, ok
}

// ServiceMeta is overridden (never delegated to the nil embedded
// QueryInterface, which would panic) — no recorded root, matching a real
// backend with nothing to report for a service list_services renders from
// the fleet backend alone.
func (perServiceTypedStatusBackend) ServiceMeta(_ context.Context, _ string) (string, string, bool, error) {
	return "", "", false, nil
}

func TestListServices_IncludesTypedEdgesOutcome(t *testing.T) {
	fleet := fleetFixture()
	backend := perServiceTypedStatusBackend{statuses: map[string]fusion.TypedEdgeStatus{
		"svc-a": {PackagesSeen: 3, Packages: 3, EdgesTotal: 7},
		"svc-b": {PackagesSeen: 2, Failed: true, Reason: "go toolchain missing", Fallback: "GOWORK=off"},
	}}
	srv := mcp.NewServerWithFleet(backend, fleet)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1003, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_services",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	svcs := p["services"].([]any)
	byName := map[string]map[string]any{}
	for _, raw := range svcs {
		e := raw.(map[string]any)
		byName[e["service"].(string)] = e
	}

	teA, ok := byName["svc-a"]["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("svc-a: no typed_edges block, got %v", byName["svc-a"])
	}
	if got, _ := teA["packages_seen"].(float64); int(got) != 3 {
		t.Errorf("svc-a typed_edges.packages_seen = %v, want 3", teA["packages_seen"])
	}
	if got, _ := teA["edges"].(float64); int(got) != 7 {
		t.Errorf("svc-a typed_edges.edges = %v, want 7", teA["edges"])
	}
	if teA["failed"] != false {
		t.Errorf("svc-a typed_edges.failed = %v, want false", teA["failed"])
	}

	teB, ok := byName["svc-b"]["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("svc-b: no typed_edges block, got %v", byName["svc-b"])
	}
	if teB["failed"] != true {
		t.Errorf("svc-b typed_edges.failed = %v, want true", teB["failed"])
	}
	if teB["error"] != "go toolchain missing" {
		t.Errorf("svc-b typed_edges.error = %v, want the named reason (I24)", teB["error"])
	}
	if teB["fallback"] != "GOWORK=off" {
		t.Errorf("svc-b typed_edges.fallback = %v, want %q", teB["fallback"], "GOWORK=off")
	}
}

// TestListServices_PersistedTypedEdgesCarriesRecordedAt is the finding [4]
// regression test: TypedEdgeStatus.Persisted/RecordedAt (DG-10) exist
// specifically so a caller can tell a store-replayed (pre-restart) answer
// apart from a fresh in-process one — but before this fix, list_services'
// typed_edges block dropped both fields on the floor, so a restart-stale
// PASS read identically to a genuinely fresh one.
func TestListServices_PersistedTypedEdgesCarriesRecordedAt(t *testing.T) {
	fleet := fleetFixture()
	recordedAt := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	backend := perServiceTypedStatusBackend{statuses: map[string]fusion.TypedEdgeStatus{
		"svc-a": {Packages: 5, Persisted: true, RecordedAt: recordedAt},
	}}
	srv := mcp.NewServerWithFleet(backend, fleet)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1004, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_services",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	svcs := p["services"].([]any)
	byName := map[string]map[string]any{}
	for _, raw := range svcs {
		e := raw.(map[string]any)
		byName[e["service"].(string)] = e
	}

	teA, ok := byName["svc-a"]["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("svc-a: no typed_edges block, got %v", byName["svc-a"])
	}
	if teA["persisted"] != true {
		t.Errorf("svc-a typed_edges.persisted = %v, want true", teA["persisted"])
	}
	got, _ := teA["recorded_at"].(string)
	if got != recordedAt.Format(time.RFC3339) {
		t.Errorf("svc-a typed_edges.recorded_at = %q, want %q", got, recordedAt.Format(time.RFC3339))
	}
}

// TestListServices_SurfacesDiscoveryGaps is DG-9: a discovery gap (a call
// target file that EXISTS on disk despite being excluded from the
// connector's allowed set) was computed and deduped by EnrichTypedEdges
// every run, but before this fix, none of noteTypedEdgeStatus's call sites
// copied it onto TypedEdgeStatus — list_services could never see it even
// though the daemon's own stderr line named it (docs/intent/invariants.md
// I24's documented gap).
func TestListServices_SurfacesDiscoveryGaps(t *testing.T) {
	fleet := fleetFixture()
	backend := perServiceTypedStatusBackend{statuses: map[string]fusion.TypedEdgeStatus{
		"svc-a": {
			PackagesSeen:      3,
			Packages:          3,
			EdgesTotal:        7,
			DiscoveryGaps:     1,
			DiscoveryGapFiles: []string{"adapter.go"},
		},
		"svc-b": {PackagesSeen: 2, Packages: 2, EdgesTotal: 4}, // no gaps
	}}
	srv := mcp.NewServerWithFleet(backend, fleet)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1005, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_services",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	byName := map[string]map[string]any{}
	for _, raw := range p["services"].([]any) {
		e := raw.(map[string]any)
		byName[e["service"].(string)] = e
	}

	teA, ok := byName["svc-a"]["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("svc-a: no typed_edges block, got %v", byName["svc-a"])
	}
	if got, _ := teA["discovery_gaps"].(float64); int(got) != 1 {
		t.Errorf("svc-a typed_edges.discovery_gaps = %v, want 1", teA["discovery_gaps"])
	}
	gapFiles, _ := teA["discovery_gap_files"].([]any)
	if len(gapFiles) != 1 || gapFiles[0] != "adapter.go" {
		t.Errorf("svc-a typed_edges.discovery_gap_files = %v, want [adapter.go]", teA["discovery_gap_files"])
	}

	teB, ok := byName["svc-b"]["typed_edges"].(map[string]any)
	if !ok {
		t.Fatalf("svc-b: no typed_edges block, got %v", byName["svc-b"])
	}
	if _, present := teB["discovery_gaps"]; present {
		t.Errorf("svc-b typed_edges.discovery_gaps = %v, want omitted (zero gaps)", teB["discovery_gaps"])
	}
}

// TestListServices_OmitsTypedEdgesWhenUnknown: a service this daemon process
// has never ingested must omit typed_edges entirely — never a fabricated
// clean pass (BP-35's own known:false discipline, carried into this row).
func TestListServices_OmitsTypedEdgesWhenUnknown(t *testing.T) {
	fleet := fleetFixture()
	backend := perServiceTypedStatusBackend{statuses: map[string]fusion.TypedEdgeStatus{}}
	srv := mcp.NewServerWithFleet(backend, fleet)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1004, Method: "tools/call",
		Params: map[string]any{
			"name":      "list_services",
			"arguments": map[string]any{},
		},
	})
	p := platformToolPayload(t, resp)
	for _, raw := range p["services"].([]any) {
		e := raw.(map[string]any)
		if _, ok := e["typed_edges"]; ok {
			t.Errorf("service %v: typed_edges present for a service never ingested this process, want omitted: %v", e["service"], e["typed_edges"])
		}
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

	// HF-5 deleted the /rpc/platform surface (ADR-014 amendment 2). The
	// GroupPlatform taxonomy still classifies these tools; that is the
	// contract this test defends.
	for _, name := range []string{"cross_repo_callers"} {
		if got := srv.GroupOfForTest(name); got != mcp.GroupPlatform {
			t.Errorf("tool %q registered in group %q, want %q", name, got, mcp.GroupPlatform)
		}
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

// TestServiceSummaries_MatchesListServices pins the in-process registry the
// daemon's console reads to what list_services returns, field for field.
func TestServiceSummaries_MatchesListServices(t *testing.T) {
	fleet := fleetFixture()
	srv := newFleetServer(fleet)
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1002, Method: "tools/call",
		Params: map[string]any{"name": "list_services", "arguments": map[string]any{}},
	})
	p := platformToolPayload(t, resp)
	entries, _ := p["services"].([]any)
	sums := mcp.ServiceSummaries(context.Background(), nil, fleet)
	if len(sums) != len(entries) {
		t.Fatalf("ServiceSummaries returned %d rows, list_services %d", len(sums), len(entries))
	}
	for i, raw := range entries {
		e := raw.(map[string]any)
		s := sums[i]
		if e["service"] != s.Service {
			t.Errorf("row %d: service %v vs %q", i, e["service"], s.Service)
		}
		if int(e["symbol_count"].(float64)) != s.SymbolCount {
			t.Errorf("row %d: symbol_count %v vs %d", i, e["symbol_count"], s.SymbolCount)
		}
		langs, _ := e["languages"].([]any)
		if len(langs) != len(s.Languages) {
			t.Fatalf("row %d: languages %v vs %v", i, langs, s.Languages)
		}
		for j := range langs {
			if langs[j] != s.Languages[j] {
				t.Errorf("row %d: language %d %v vs %q", i, j, langs[j], s.Languages[j])
			}
		}
	}
}
