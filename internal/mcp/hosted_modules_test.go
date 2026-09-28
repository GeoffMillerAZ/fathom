package mcp_test

// hosted_modules_test.go — the module host's hosted-deployment mode
// (docs/plans/gke-module-host.md GKE-MH-7, critic finding 2): when
// SetHostedModules(true) is active (mirrors cmd/fathom's serve-http
// enabling it exactly when --modules-token-env names a configured token),
// /rpc/modules serves ONLY its own curated allowlist — never GroupShared's
// fleet-backend reads or write/spend tools (ingest, compile_layer) — off-
// surface calls there are hard-gated (never execute-and-flag), and module
// tools are unreachable via bare /rpc. (batch, and the sub-call-smuggling
// gate it needed, were unregistered by SR-19, surface reckoning F-77.) The
// NON-hosted default (no token configured) is unchanged from before this
// fix: GroupShared-inclusive, execute-and-flag.

import (
	"context"
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
	for _, want := range []string{"list_modules", "module_obligations", "module_query", "module_staleness", "status"} {
		if !nameSet[want] {
			t.Errorf("hosted /rpc/modules tools/list missing %q: got %v", want, names)
		}
	}
	// The whole point: GroupShared's write/spend/fleet-backend tools are
	// NOT on the hosted allowlist, even though they'd be visible on the
	// non-hosted "modules" surface (default GroupShared-inclusive rule).
	for _, forbidden := range []string{"ingest", "compile_layer", "batch", "repo_map", "pack_context", "staleness_report", "find_symbol", "estimate_cost", "cost_report"} {
		if nameSet[forbidden] {
			t.Errorf("hosted /rpc/modules tools/list must NOT include %q (fleet-backend/write/spend leak): got %v", forbidden, names)
		}
	}
}

// TestHostedModules_StatusSourcesSectionRefused (SR-23): the merged status
// tool is on the hosted allowlist for its "tools"/"daemon" sections — the
// two harmless, dependency-free meta reads capabilities/server_stats used
// to be — but its "sources" section takes an arbitrary caller-supplied
// service and lists THAT service's own files, exactly the fleet-backend
// pivot list_sources was excluded from hostedModulesTools for before the
// merge; the merge must not silently widen hosted-modules exposure.
func TestHostedModules_StatusSourcesSectionRefused(t *testing.T) {
	srv := newHostedModuleTestServer()
	ctx := mcp.WithRequestScope(context.Background(), mcp.SurfaceModules, true)

	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "sources", "service": "svc"}},
	})
	if resp["error"] == nil {
		t.Fatalf("status(section:\"sources\") under hosted-modules mode should be refused, got %v", resp)
	}

	for _, section := range []string{"tools", "daemon"} {
		got := scopedRoundtrip(t, srv, ctx, request{
			JSONRPC: "2.0", ID: 6, Method: "tools/call",
			Params: map[string]any{"name": "status", "arguments": map[string]any{"section": section}},
		})
		if got["error"] != nil {
			t.Errorf("status(section:%q) under hosted-modules mode should still dispatch: %v", section, got["error"])
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

// TestHostedModules_ModuleToolsRefusedViaBatchSubCall was retired: batch
// (RT-60) was unregistered by SR-19 (surface reckoning, F-77), so the
// sub-call-smuggling gate it proved (hostedModuleBlocked inside
// registerBatch's handler) no longer has a caller to prove it through.

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
	// other named group surface — ingest/etc. ARE visible here.
	for _, want := range []string{"list_modules", "module_query", "ingest"} {
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
