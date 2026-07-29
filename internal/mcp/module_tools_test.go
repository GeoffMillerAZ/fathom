package mcp_test

// module_tools_test.go — the /rpc/modules scoped surface (docs/plans/
// gke-module-host.md GKE-MH-5): list_modules/module_obligations/
// module_staleness dispatch correctly, are visible on the "modules" surface
// (and bare /rpc) but absent from every server built WITHOUT a module
// backend, and module_obligations/module_staleness surface a not-found
// error for an unmounted module rather than an empty silent success.

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
	"github.com/geoffmilleraz/fathom/internal/modulehost"
	"github.com/geoffmilleraz/fathom/internal/semantic"
)

// fakeModuleBackend is a scripted moduleBackend fake — internal/mcp's
// moduleBackend interface is unexported, but this package (mcp_test) can
// still satisfy it structurally by implementing the same three methods.
type fakeModuleBackend struct {
	modules     []modulehost.ModuleInfo
	obligations []modulehost.FederatedObligation
	obligErr    error
	queryResult []modulehost.FederatedQueryEntry
	staleness   map[string][]modulehost.ModuleStaleness
}

func (f *fakeModuleBackend) ListModules(context.Context) ([]modulehost.ModuleInfo, error) {
	return f.modules, nil
}

func (f *fakeModuleBackend) FederatedObligations(_ context.Context, module, _ string, _ int) ([]modulehost.FederatedObligation, error) {
	if f.obligErr != nil {
		return nil, f.obligErr
	}
	if module == "" {
		return f.obligations, nil
	}
	var out []modulehost.FederatedObligation
	for _, o := range f.obligations {
		if o.Module == module {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeModuleBackend) FederatedQuery(_ context.Context, module, _ string, _ int) ([]modulehost.FederatedQueryEntry, error) {
	if module == "" {
		return f.queryResult, nil
	}
	var out []modulehost.FederatedQueryEntry
	for _, e := range f.queryResult {
		if e.Module == module {
			out = append(out, e)
		}
	}
	return out, nil
}

func (f *fakeModuleBackend) ModuleStaleness(_ context.Context, module string) ([]modulehost.ModuleStaleness, bool, error) {
	report, ok := f.staleness[module]
	return report, ok, nil
}

func newModuleTestServer() (*mcp.Server, *fakeModuleBackend) {
	fb := &fakeModuleBackend{
		modules: []modulehost.ModuleInfo{
			{Module: "alpha", Path: "/mods/alpha.sqlite", Service: "svc-a", IndexedCommit: "deadbeef"},
		},
		obligations: []modulehost.FederatedObligation{
			{Module: "alpha", Service: "svc-a", Obligations: semantic.Obligations{
				Service: "svc-a", File: "x.go",
				Rules: []semantic.Obligation{{Rule: "do the thing", Src: "RULES.md#L1-L1", Tier: "doc-claimed", Layer: "obligations"}},
			}},
		},
		queryResult: []modulehost.FederatedQueryEntry{
			{Module: "alpha", Service: "svc-a", Watermark: "deadbeef", Claims: []modulehost.QueryClaim{
				{ClaimID: "claim-abc123", Text: "matched rule", Src: "RULES.md#L1-L1", Anchor: "the cited text", Tier: "doc-claimed"},
			}},
		},
		staleness: map[string][]modulehost.ModuleStaleness{
			"alpha": {{Module: "alpha", Service: "svc-a", IndexedCommit: "deadbeef", Status: modulehost.StatusUnknownDetached}},
		},
	}
	srv := mcp.NewServerWithModules(&fakeBackend{}, nil, nil, fb)
	return srv, fb
}

func TestModuleTools_ListModules(t *testing.T) {
	srv, _ := newModuleTestServer()
	resp := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "list_modules", "arguments": map[string]any{}},
	})
	if resp["error"] != nil {
		t.Fatalf("list_modules: %v", resp["error"])
	}
	text := resp["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	var payload struct {
		Modules []modulehost.ModuleInfo `json:"modules"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("decode: %v (text=%q)", err, text)
	}
	if len(payload.Modules) != 1 || payload.Modules[0].Module != "alpha" {
		t.Fatalf("list_modules = %+v, want exactly module=alpha", payload.Modules)
	}
}

func TestModuleTools_ModuleObligations(t *testing.T) {
	srv, _ := newModuleTestServer()
	resp := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: map[string]any{"name": "module_obligations", "arguments": map[string]any{"file": "x.go"}},
	})
	if resp["error"] != nil {
		t.Fatalf("module_obligations: %v", resp["error"])
	}
	text := resp["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	var payload struct {
		Results []modulehost.FederatedObligation `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("decode: %v (text=%q)", err, text)
	}
	if len(payload.Results) != 1 || payload.Results[0].Module != "alpha" {
		t.Fatalf("module_obligations = %+v, want exactly one alpha result", payload.Results)
	}
	if len(payload.Results[0].Obligations.Rules) != 1 || payload.Results[0].Obligations.Rules[0].Src == "" {
		t.Errorf("federated rule missing citation: %+v", payload.Results[0].Obligations.Rules)
	}

	// Missing required "file" is a params error, not a silent empty result.
	missing := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": "module_obligations", "arguments": map[string]any{}},
	})
	if missing["error"] == nil {
		t.Fatal("module_obligations without file: want a params error")
	}
}

func TestModuleTools_ModuleQuery(t *testing.T) {
	srv, _ := newModuleTestServer()
	resp := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 20, Method: "tools/call",
		Params: map[string]any{"name": "module_query", "arguments": map[string]any{"text": "matched"}},
	})
	if resp["error"] != nil {
		t.Fatalf("module_query: %v", resp["error"])
	}
	text := resp["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	var payload struct {
		Results []modulehost.FederatedQueryEntry `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("decode: %v (text=%q)", err, text)
	}
	if len(payload.Results) != 1 || payload.Results[0].Module != "alpha" {
		t.Fatalf("module_query = %+v, want exactly one alpha result", payload.Results)
	}
	if len(payload.Results[0].Claims) != 1 || payload.Results[0].Claims[0].Src == "" {
		t.Errorf("module_query claim missing citation: %+v", payload.Results[0].Claims)
	}
	if payload.Results[0].Watermark != "deadbeef" {
		t.Errorf("module_query watermark = %q, want %q", payload.Results[0].Watermark, "deadbeef")
	}
	if payload.Results[0].Claims[0].ClaimID != "claim-abc123" {
		t.Errorf("module_query claim_id = %q, want %q", payload.Results[0].Claims[0].ClaimID, "claim-abc123")
	}
	if payload.Results[0].Claims[0].Anchor != "the cited text" {
		t.Errorf("module_query anchor = %q, want %q", payload.Results[0].Claims[0].Anchor, "the cited text")
	}

	// Raw-key pin (CONTRACT module_query v0): decode into a bare
	// map[string]any so this test fails if the WIRE field names ever drift
	// from what the kit consumer decodes against, independent of whatever
	// Go struct tags this package happens to declare — claim_id/text/src/
	// anchor/tier/status/kind on each claim, watermark/nodes/staleness/
	// error alongside module/service on each result entry.
	var raw struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatalf("raw decode: %v", err)
	}
	if len(raw.Results) != 1 {
		t.Fatalf("raw results = %d, want 1", len(raw.Results))
	}
	entry := raw.Results[0]
	for _, key := range []string{"module", "service", "watermark", "claims", "staleness"} {
		if _, ok := entry[key]; !ok {
			t.Errorf("module_query result entry missing wire key %q: %v", key, entry)
		}
	}
	claims, ok := entry["claims"].([]any)
	if !ok || len(claims) != 1 {
		t.Fatalf("raw claims = %v, want exactly 1 entry", entry["claims"])
	}
	claim, ok := claims[0].(map[string]any)
	if !ok {
		t.Fatalf("raw claim entry is not an object: %v", claims[0])
	}
	for _, key := range []string{"claim_id", "text", "src", "anchor", "tier"} {
		if _, ok := claim[key]; !ok {
			t.Errorf("module_query claim entry missing wire key %q: %v", key, claim)
		}
	}
	// status/kind are omitempty (fresh/default "obligation" both omit) —
	// this fixture's claim is fresh with no non-default kind, so their
	// ABSENCE here is itself the pinned, correct behavior; presence is
	// exercised by TestFederatedQuery_SymbolsAndClaimsPerModule's stale/
	// non-default-kind cases at the modulehost layer (internal/modulehost).
	if _, ok := claim["status"]; ok {
		t.Errorf("fresh claim's \"status\" key should be omitted (zero-output-on-clean), got %v", claim["status"])
	}
	if claim["claim_id"] != "claim-abc123" {
		t.Errorf("raw claim_id = %v, want %q", claim["claim_id"], "claim-abc123")
	}

	// The "concern" alias (P1-1 pattern, query_symbols' own field name)
	// works identically to "text".
	viaAlias := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 21, Method: "tools/call",
		Params: map[string]any{"name": "module_query", "arguments": map[string]any{"concern": "matched"}},
	})
	if viaAlias["error"] != nil {
		t.Fatalf("module_query via concern alias: %v", viaAlias["error"])
	}

	// Missing both "text" and "concern" is a params error.
	missing := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 22, Method: "tools/call",
		Params: map[string]any{"name": "module_query", "arguments": map[string]any{}},
	})
	if missing["error"] == nil {
		t.Fatal("module_query without text/concern: want a params error")
	}
}

func TestModuleTools_ModuleStaleness(t *testing.T) {
	srv, _ := newModuleTestServer()
	resp := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{"name": "module_staleness", "arguments": map[string]any{"module": "alpha"}},
	})
	if resp["error"] != nil {
		t.Fatalf("module_staleness: %v", resp["error"])
	}
	text := resp["result"].(map[string]any)["content"].([]any)[0].(map[string]any)["text"].(string)
	var payload struct {
		Module  string                       `json:"module"`
		Corpora []modulehost.ModuleStaleness `json:"corpora"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("decode: %v (text=%q)", err, text)
	}
	if len(payload.Corpora) != 1 || payload.Corpora[0].Status != modulehost.StatusUnknownDetached {
		t.Fatalf("module_staleness = %+v, want status=%q", payload.Corpora, modulehost.StatusUnknownDetached)
	}

	// An unmounted module is refused loudly, not a silent empty success.
	notFound := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{"name": "module_staleness", "arguments": map[string]any{"module": "not-mounted"}},
	})
	errObj, ok := notFound["error"].(map[string]any)
	if !ok {
		t.Fatalf("module_staleness for an unmounted module: want an error, got %v", notFound)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("code = %v, want ErrCodeNotFound", code)
	}
}

// ─── surface visibility ───────────────────────────────────────────────────

func TestModuleTools_VisibleOnModulesSurfaceAndBareRPC(t *testing.T) {
	srv, _ := newModuleTestServer()
	want := []string{"list_modules", "module_obligations", "module_query", "module_staleness"}
	sort.Strings(want)

	for _, surface := range []string{mcp.SurfaceModules, ""} {
		ctx := mcp.WithRequestScope(context.Background(), surface, true)
		names := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
			JSONRPC: "2.0", ID: 6, Method: "tools/list", Params: map[string]any{},
		}))
		nameSet := map[string]bool{}
		for _, n := range names {
			nameSet[n] = true
		}
		for _, w := range want {
			if !nameSet[w] {
				t.Errorf("surface %q: %q missing from tools/list (got %v)", surface, w, names)
			}
		}
	}

	// Absent from the curated agent/companion allowlist (doctrine 4: no
	// net-new tools on /rpc/agent).
	agentCtx := mcp.WithRequestScope(context.Background(), mcp.SurfaceAgent, true)
	agentNames := toolNames(t, scopedRoundtrip(t, srv, agentCtx, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/list", Params: map[string]any{},
	}))
	for _, n := range agentNames {
		for _, w := range want {
			if n == w {
				t.Errorf("module tool %q must not be on the agent surface", w)
			}
		}
	}
}

// TestModuleTools_AbsentWithoutModuleBackend proves a plain server (no
// modules wired in) never registers these tools at all — every existing
// pinned tool-list assertion elsewhere in this package is unaffected by
// this file's additions.
func TestModuleTools_AbsentWithoutModuleBackend(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	names := toolNames(t, roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/list", Params: map[string]any{},
	}))
	for _, n := range names {
		if n == "list_modules" || n == "module_obligations" || n == "module_query" || n == "module_staleness" {
			t.Errorf("module tool %q present on a server with no module backend wired in", n)
		}
	}
}
