// Surface tests: per-request tool surfaces carried in context.
//
// A surface is an immutable per-request view of the tool registry, selected
// by the HTTP path (/rpc/{surface}) on the shared daemon. Unlike the stdio
// profile (mutable, process-global, advisory), an HTTP surface hard-gates
// tools/call and batch sub-calls, and set_profile is rejected over HTTP.
package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/mcp"
)

// companionWant is the curated coding-companion allowlist, pinned exactly.
// Changing the surface is deliberate: update this list and docs/mcp-tools.md.
var companionWant = []string{
	"batch",
	"brief",
	"entrypoints",
	"find_callers",
	"find_symbol",
	"ingest",
	"list_services",
	"obligations",
	"outline",
	"pack_context",
	"query_symbols",
	"references",
	"repo_map",
	"staleness_report",
	"symbol_context",
}

// scopedRoundtrip drives a single request through HandleRequest with ctx,
// mirroring what the HTTP layer does (one JSON-RPC message per request).
func scopedRoundtrip(t *testing.T, srv *mcp.Server, ctx context.Context, req request) map[string]any {
	t.Helper()
	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	resp, hasResp := srv.HandleRequest(ctx, raw)
	if !hasResp {
		t.Fatalf("HandleRequest returned no response for %v", req)
	}
	var m map[string]any
	if err := json.NewDecoder(bytes.NewReader(resp)).Decode(&m); err != nil {
		t.Fatalf("decode response: %v (raw: %q)", err, resp)
	}
	return m
}

// toolNames extracts the sorted tool-name list from a tools/list response.
func toolNames(t *testing.T, resp map[string]any) []string {
	t.Helper()
	if resp["error"] != nil {
		t.Fatalf("tools/list error: %v", resp["error"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("result missing: %v", resp)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools missing: %v", result)
	}
	names := make([]string, len(tools))
	for i, raw := range tools {
		names[i] = raw.(map[string]any)["name"].(string)
	}
	sort.Strings(names)
	return names
}

// ─── companion surface: tools/list is exactly the allowlist ─────────────────

func TestCompanionSurfaceToolsList(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	names := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
	}))
	want := append([]string(nil), companionWant...)
	sort.Strings(want)
	if len(names) != len(want) {
		t.Fatalf("companion surface has %d tools, want %d\n got: %v\nwant: %v", len(names), len(want), names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("companion surface mismatch at %d: got %q want %q\nfull: %v", i, names[i], want[i], names)
		}
	}
}

// ─── companion surface: tools/call is hard-gated ─────────────────────────────

func TestCompanionSurfaceGatesToolsCall(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)

	// Off-surface tools are rejected — even shared-group ones like secret_scan
	// (the allowlist wins over group membership).
	for _, tool := range []string{"secret_scan", "blast_radius", "shell_out"} {
		resp := scopedRoundtrip(t, srv, ctx, request{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: map[string]any{"name": tool, "arguments": map[string]any{"service": "svc"}},
		})
		errObj, ok := resp["error"].(map[string]any)
		if !ok {
			t.Fatalf("calling %q on companion surface: want error, got %v", tool, resp)
		}
		if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
			t.Errorf("%q: code = %v, want %v", tool, code, mcp.ErrCodeNotFound)
		}
		if msg := errObj["message"].(string); !strings.Contains(msg, "surface") {
			t.Errorf("%q: error message %q should name the surface", tool, msg)
		}
	}

	// On-surface tools still dispatch.
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
	})
	if resp["error"] != nil {
		t.Fatalf("find_symbol on companion surface should dispatch, got error: %v", resp["error"])
	}
}

// ─── batch cannot smuggle off-surface tools ──────────────────────────────────

func TestBatchGatedOnSurface(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{
			"name": "batch",
			"arguments": map[string]any{
				"calls": []map[string]any{
					{"name": "secret_scan", "arguments": map[string]any{"service": "svc"}},
					{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
				},
			},
		},
	})
	if resp["error"] != nil {
		t.Fatalf("batch itself should succeed: %v", resp["error"])
	}
	// The envelope wraps the JSON payload as text content.
	content := resp["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload struct {
		Results []struct {
			Name  string `json:"name"`
			Error string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("batch payload parse: %v (text=%q)", err, text)
	}
	if len(payload.Results) != 2 {
		t.Fatalf("batch results = %d, want 2", len(payload.Results))
	}
	if !strings.Contains(payload.Results[0].Error, "surface") {
		t.Errorf("secret_scan sub-call should be surface-gated, got error %q", payload.Results[0].Error)
	}
	if payload.Results[1].Error != "" {
		t.Errorf("find_symbol sub-call should succeed, got error %q", payload.Results[1].Error)
	}
}

// ─── set_profile is rejected over HTTP ───────────────────────────────────────

func TestSetProfileRejectedOverHTTP(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	// Even the full (unscoped) HTTP surface rejects set_profile: the daemon is
	// shared, and the profile is process-global.
	ctx := mcp.WithRequestScope(context.Background(), "", true)
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{"name": "set_profile", "arguments": map[string]any{"profile": "coding"}},
	})
	errObj, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("set_profile over HTTP: want error, got %v", resp)
	}
	if msg := errObj["message"].(string); !strings.Contains(msg, "/rpc/") {
		t.Errorf("error message %q should point at path-scoped endpoints", msg)
	}
}

// ─── stdio: set_profile still works, now including companion ────────────────

func TestSetProfileCompanionOnStdio(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	setResp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 6, Method: "tools/call",
		Params: map[string]any{"name": "set_profile", "arguments": map[string]any{"profile": "companion"}},
	})
	if setResp["error"] != nil {
		t.Fatalf("set_profile(companion) on stdio: %v", setResp["error"])
	}
	names := toolNames(t, roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 7, Method: "tools/list", Params: map[string]any{},
	}))
	if len(names) != len(companionWant) {
		t.Fatalf("stdio companion profile lists %d tools, want %d: %v", len(names), len(companionWant), names)
	}
}

// ─── capabilities reports the active surface ─────────────────────────────────

func TestCapabilitiesReportsActiveSurface(t *testing.T) {
	// capabilities is deliberately NOT on the companion surface (tools/list is
	// the discovery there); exercise it on a group surface where it is visible.
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "coding", true)

	listed := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/list", Params: map[string]any{},
	}))

	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 9, Method: "tools/call",
		Params: map[string]any{"name": "capabilities", "arguments": map[string]any{}},
	})
	if resp["error"] != nil {
		t.Fatalf("capabilities: %v", resp["error"])
	}
	content := resp["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("capabilities payload parse: %v", err)
	}
	if payload["active_surface"] != "coding" {
		t.Errorf("active_surface = %v, want coding", payload["active_surface"])
	}
	if count := payload["tool_count"].(float64); count != float64(len(listed)) {
		t.Errorf("tool_count = %v, want %d (the tools/list count on the same surface)", count, len(listed))
	}

	// And the companion surface gates capabilities itself.
	compCtx := mcp.WithRequestScope(context.Background(), "companion", true)
	gated := scopedRoundtrip(t, srv, compCtx, request{
		JSONRPC: "2.0", ID: 10, Method: "tools/call",
		Params: map[string]any{"name": "capabilities", "arguments": map[string]any{}},
	})
	if gated["error"] == nil {
		t.Error("capabilities should be gated on the companion surface")
	}
}

// ─── group surfaces match the stdio profile view ─────────────────────────────

func TestGroupSurfaceMatchesProfileView(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})

	ctx := mcp.WithRequestScope(context.Background(), "coding", true)
	viaSurface := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 9, Method: "tools/list", Params: map[string]any{},
	}))

	srv2 := mcp.NewServer(&fakeBackend{})
	setResp := roundtrip(t, srv2, request{
		JSONRPC: "2.0", ID: 10, Method: "tools/call",
		Params: map[string]any{"name": "set_profile", "arguments": map[string]any{"profile": "coding"}},
	})
	if setResp["error"] != nil {
		t.Fatalf("set_profile(coding): %v", setResp["error"])
	}
	viaProfile := toolNames(t, roundtrip(t, srv2, request{
		JSONRPC: "2.0", ID: 11, Method: "tools/list", Params: map[string]any{},
	}))

	// The HTTP view drops set_profile (rejected over HTTP); otherwise the two
	// views must be identical.
	withoutSetProfile := make([]string, 0, len(viaProfile))
	for _, name := range viaProfile {
		if name != "set_profile" {
			withoutSetProfile = append(withoutSetProfile, name)
		}
	}
	if len(viaSurface) != len(withoutSetProfile) {
		t.Fatalf("coding surface (%d tools) != coding profile minus set_profile (%d tools)", len(viaSurface), len(withoutSetProfile))
	}
	for i := range viaSurface {
		if viaSurface[i] != withoutSetProfile[i] {
			t.Fatalf("mismatch at %d: surface %q vs profile %q", i, viaSurface[i], withoutSetProfile[i])
		}
	}
}

// ─── unscoped HTTP shows the full registry ───────────────────────────────────

func TestUnscopedHTTPShowsAllTools(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	full := toolNames(t, roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 12, Method: "tools/list", Params: map[string]any{},
	}))
	ctx := mcp.WithRequestScope(context.Background(), "", true)
	viaHTTP := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 13, Method: "tools/list", Params: map[string]any{},
	}))
	// The HTTP view is the full registry minus set_profile (rejected over HTTP,
	// so its schema is not advertised).
	if len(viaHTTP) != len(full)-1 {
		t.Fatalf("unscoped HTTP lists %d tools, want %d (stdio %d minus set_profile)", len(viaHTTP), len(full)-1, len(full))
	}
	for _, name := range viaHTTP {
		if name == "set_profile" {
			t.Error("set_profile should not be listed over HTTP")
		}
	}
}

// ─── stdio profile stays advisory; bare HTTP stays ungated ───────────────────

func TestStdioProfileDoesNotGateToolsCall(t *testing.T) {
	// The old contract: a stdio profile only filters tools/list. Calling an
	// off-profile tool (find_callers is GroupInvestigation) must still work
	// after set_profile("coding").
	srv := mcp.NewServer(&fakeBackend{})
	setResp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 30, Method: "tools/call",
		Params: map[string]any{"name": "set_profile", "arguments": map[string]any{"profile": "coding"}},
	})
	if setResp["error"] != nil {
		t.Fatalf("set_profile(coding): %v", setResp["error"])
	}
	resp := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 31, Method: "tools/call",
		Params: map[string]any{"name": "find_callers", "arguments": map[string]any{"service": "svc", "symbol": "Foo"}},
	})
	if resp["error"] != nil {
		t.Fatalf("stdio profile must stay advisory: off-profile find_callers failed: %v", resp["error"])
	}
}

func TestBareHTTPDoesNotGateToolsCall(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "", true)
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 32, Method: "tools/call",
		Params: map[string]any{"name": "find_callers", "arguments": map[string]any{"service": "svc", "symbol": "Foo"}},
	})
	if resp["error"] != nil {
		t.Fatalf("bare /rpc must serve the full surface: find_callers failed: %v", resp["error"])
	}
}

// ─── canned prompts are scoped to the surface ────────────────────────────────

func TestPromptsScopedOnSurface(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})

	// Unscoped: all four canned prompts.
	unscoped := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 33, Method: "prompts/list", Params: map[string]any{},
	})
	if unscoped["error"] != nil {
		t.Fatalf("prompts/list: %v", unscoped["error"])
	}
	all := unscoped["result"].(map[string]any)["prompts"].([]any)
	if len(all) != 4 {
		t.Fatalf("unscoped prompts/list = %d prompts, want 4", len(all))
	}

	// Companion surface: every canned prompt references gated tools, so none
	// are advertised and prompts/get refuses them.
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	scoped := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 34, Method: "prompts/list", Params: map[string]any{},
	})
	if scoped["error"] != nil {
		t.Fatalf("prompts/list on companion: %v", scoped["error"])
	}
	visible := scoped["result"].(map[string]any)["prompts"].([]any)
	if len(visible) != 0 {
		t.Fatalf("companion prompts/list = %d prompts, want 0 (all reference gated tools): %v", len(visible), visible)
	}
	got := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 35, Method: "prompts/get",
		Params: map[string]any{"name": "investigate_incident", "arguments": map[string]any{"service": "svc", "symptom": "x"}},
	})
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("prompts/get on companion: want error, got %v", got)
	}
	if msg := errObj["message"].(string); !strings.Contains(msg, "surface") {
		t.Errorf("prompts/get error %q should name the surface", msg)
	}
}

// ─── surface name validation ─────────────────────────────────────────────────

func TestValidSurface(t *testing.T) {
	for _, name := range []string{"companion", "coding", "investigation", "shared", "platform"} {
		if !mcp.ValidSurface(name) {
			t.Errorf("ValidSurface(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"", "bogus", "Companion", "full"} {
		if mcp.ValidSurface(name) {
			t.Errorf("ValidSurface(%q) = true, want false", name)
		}
	}
	names := mcp.SurfaceNames()
	if len(names) != 5 {
		t.Fatalf("SurfaceNames() = %v, want 5 entries", names)
	}
	if !sort.StringsAreSorted(names) {
		t.Errorf("SurfaceNames() not sorted: %v", names)
	}
}

// ─── call log records the surface ────────────────────────────────────────────

func TestCallLoggerRecordsSurface(t *testing.T) {
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(&fakeBackend{}, mcp.NewCallLogger(&buf))
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	_ = scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 14, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
	})
	line := strings.TrimSpace(buf.String())
	if line == "" {
		t.Fatal("no call-log line written")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(line), &entry); err != nil {
		t.Fatalf("log line parse: %v (line=%q)", err, line)
	}
	if entry["surface"] != "companion" {
		t.Errorf("log entry surface = %v, want companion", entry["surface"])
	}
}

// ─── batch sub-calls are individually logged ─────────────────────────────────

func TestCallLoggerRecordsBatchSubCalls(t *testing.T) {
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(&fakeBackend{}, mcp.NewCallLogger(&buf))
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	_ = scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 15, Method: "tools/call",
		Params: map[string]any{
			"name": "batch",
			"arguments": map[string]any{
				"calls": []map[string]any{
					{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
					{"name": "secret_scan", "arguments": map[string]any{"service": "svc"}},
				},
			},
		},
	})
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	// Two sub-call entries (via:"batch") plus the outer batch entry.
	if len(lines) != 3 {
		t.Fatalf("expected 3 log lines (2 sub-calls + batch), got %d: %q", len(lines), buf.String())
	}
	byTool := map[string]map[string]any{}
	for _, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line parse: %v (line=%q)", err, line)
		}
		byTool[entry["tool"].(string)] = entry
	}
	sub, ok := byTool["find_symbol"]
	if !ok {
		t.Fatalf("find_symbol sub-call not logged: %v", byTool)
	}
	if sub["via"] != "batch" {
		t.Errorf("sub-call via = %v, want batch", sub["via"])
	}
	if sub["surface"] != "companion" {
		t.Errorf("sub-call surface = %v, want companion", sub["surface"])
	}
	gated, ok := byTool["secret_scan"]
	if !ok {
		t.Fatalf("gated secret_scan sub-call not logged: %v", byTool)
	}
	if errMsg, _ := gated["error"].(string); !strings.Contains(errMsg, "surface") {
		t.Errorf("gated sub-call should log the gate error, got %v", gated["error"])
	}
	if outer, ok := byTool["batch"]; !ok {
		t.Error("outer batch entry not logged")
	} else if outer["via"] != nil {
		t.Errorf("outer batch entry should have no via, got %v", outer["via"])
	}
}
