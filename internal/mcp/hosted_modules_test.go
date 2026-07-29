package mcp_test

// hosted_modules_test.go — the module host's hosted-deployment mode
// (docs/plans/gke-module-host.md GKE-MH-7, critic finding 2): when
// SetHostedModules(true) is active (mirrors cmd/fathom's serve-http
// enabling it exactly when --modules-token-env names a configured token),
// /rpc/modules serves ONLY its own curated allowlist — never GroupShared's
// fleet-backend reads or write/spend tools (ingest, compile_layer, batch)
// — off-surface calls there are hard-gated (never execute-and-flag), and
// module tools are unreachable via bare /rpc or a batch sub-call. The
// NON-hosted default (no token configured) is unchanged from before this
// fix: GroupShared-inclusive, execute-and-flag.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func newHostedModuleTestServer() *mcp.Server {
	srv, _ := newModuleTestServer()
	srv.SetHostedModules(true)
	return srv
}

// ─── (a) /rpc/modules serves ONLY the curated allowlist while hosted ─────

func TestHostedModules_ModulesSurfaceServesOnlyAllowlist(t *testing.T) {
	srv := newHostedModuleTestServer()
	ctx := mcp.WithRequestScope(context.Background(), mcp.SurfaceModules, true)
	names := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
	}))
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	for _, want := range []string{"list_modules", "module_obligations", "module_query", "module_staleness", "capabilities", "server_stats"} {
		if !nameSet[want] {
			t.Errorf("hosted /rpc/modules tools/list missing %q: got %v", want, names)
		}
	}
	// The whole point: GroupShared's write/spend/fleet-backend tools are
	// NOT on the hosted allowlist, even though they'd be visible on the
	// non-hosted "modules" surface (default GroupShared-inclusive rule).
	for _, forbidden := range []string{"ingest", "compile_layer", "batch", "repo_map", "pack_context", "staleness_report", "find_symbol", "estimate_cost", "cost_report", "list_sources"} {
		if nameSet[forbidden] {
			t.Errorf("hosted /rpc/modules tools/list must NOT include %q (fleet-backend/write/spend leak): got %v", forbidden, names)
		}
	}
}

// ─── (b) off-allowlist calls on /rpc/modules are hard-gated while hosted ──

func TestHostedModules_OffAllowlistCallsHardGated(t *testing.T) {
	srv := newHostedModuleTestServer()
	ctx := mcp.WithRequestScope(context.Background(), mcp.SurfaceModules, true)

	for _, tool := range []string{"ingest", "batch", "find_symbol", "staleness_report"} {
		resp := scopedRoundtrip(t, srv, ctx, request{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: map[string]any{"name": tool, "arguments": map[string]any{"service": "svc"}},
		})
		errObj, ok := resp["error"].(map[string]any)
		if !ok {
			t.Errorf("hosted /rpc/modules calling %q: want a hard-gate error (never execute-and-flag), got %v", tool, resp)
			continue
		}
		if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
			t.Errorf("%q: code = %v, want ErrCodeNotFound", tool, code)
		}
	}

	// An on-allowlist tool still dispatches normally.
	ok := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": "list_modules", "arguments": map[string]any{}},
	})
	if ok["error"] != nil {
		t.Errorf("list_modules on hosted /rpc/modules should dispatch: %v", ok["error"])
	}
}

// ─── (c) module tools unreachable outside /rpc/modules while hosted ──────

func TestHostedModules_ModuleToolsHiddenFromBareRPC(t *testing.T) {
	srv := newHostedModuleTestServer()
	ctx := mcp.WithRequestScope(context.Background(), "", true) // bare /rpc
	names := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/list", Params: map[string]any{},
	}))
	for _, n := range names {
		if n == "list_modules" || n == "module_obligations" || n == "module_query" || n == "module_staleness" {
			t.Errorf("hosted mode: %q must be hidden from bare /rpc tools/list, got it in %v", n, names)
		}
	}
}

func TestHostedModules_ModuleToolsRefusedOnBareRPC(t *testing.T) {
	srv := newHostedModuleTestServer()
	ctx := mcp.WithRequestScope(context.Background(), "", true) // bare /rpc — enforced()==false here
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{"name": "list_modules", "arguments": map[string]any{}},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("list_modules via bare /rpc while hosted: want a refusal (no unauthenticated path to module data), got %v", resp)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("code = %v, want ErrCodeNotFound", code)
	}
}

func TestHostedModules_ModuleToolsRefusedViaBatchSubCall(t *testing.T) {
	srv := newHostedModuleTestServer()
	ctx := mcp.WithRequestScope(context.Background(), "", true) // bare /rpc
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: map[string]any{
			"name": "batch",
			"arguments": map[string]any{
				"calls": []map[string]any{
					{"name": "list_modules", "arguments": map[string]any{}},
				},
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("batch itself should succeed: %v", resp["error"])
	}
	content := resp["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload struct {
		Results []struct {
			Name  string `json:"name"`
			Error string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("batch payload parse: %v", err)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("batch results = %d, want 1", len(payload.Results))
	}
	if payload.Results[0].Error == "" {
		t.Error("list_modules batch sub-call must be refused while hosted, not smuggled through")
	}
}

// ─── (d) non-hosted default is unchanged ──────────────────────────────────

func TestHostedModules_NonHostedDefaultUnchanged(t *testing.T) {
	srv, _ := newModuleTestServer() // SetHostedModules never called — default false
	ctx := mcp.WithRequestScope(context.Background(), mcp.SurfaceModules, true)
	names := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/list", Params: map[string]any{},
	}))
	nameSet := map[string]bool{}
	for _, n := range names {
		nameSet[n] = true
	}
	// Non-hosted "modules" surface is GroupShared-inclusive, same as every
	// other named group surface — batch/ingest/etc. ARE visible here.
	for _, want := range []string{"list_modules", "module_query", "batch", "ingest"} {
		if !nameSet[want] {
			t.Errorf("non-hosted modules surface missing %q (should be GroupShared-inclusive): got %v", want, names)
		}
	}

	// Bare /rpc still shows module tools when not hosted.
	bareCtx := mcp.WithRequestScope(context.Background(), "", true)
	bareNames := toolNames(t, scopedRoundtrip(t, srv, bareCtx, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/list", Params: map[string]any{},
	}))
	found := false
	for _, n := range bareNames {
		if n == "list_modules" {
			found = true
		}
	}
	if !found {
		t.Error("non-hosted bare /rpc should still list module tools")
	}
}
