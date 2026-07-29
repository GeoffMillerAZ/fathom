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
	want := []string{"list_modules", "module_obligations", "module_staleness"}
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
		if n == "list_modules" || n == "module_obligations" || n == "module_staleness" {
			t.Errorf("module tool %q present on a server with no module backend wired in", n)
		}
	}
}
