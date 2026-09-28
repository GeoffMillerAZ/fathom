// Surface tests: per-request tool surfaces carried in context.
//
// A surface is an immutable per-request view of the tool registry, selected
// by the HTTP path (/rpc/{surface}) on the shared daemon. Gate semantics are
// execute-and-flag (ADR-014 amendment, 2026-07-16): an off-surface tools/call
// or batch sub-call still executes, logged with an off_surface flag, unless
// Server.strictSurfaces restores the pre-amendment hard gate.
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

// companionWant is the curated agent allowlist, pinned exactly
// (docs/vision/05-surfaces.md §3: companion 15, minus batch and brief, plus
// verify_plan and capabilities; T5-4, docs/plans/trust-and-tokenomics.md,
// added read_source as the program's second net-new tool — 16 total;
// AM-8 added memory_search — 17; RC-24, docs/plans/receipts.md, added
// search, the grep-with-a-receipt tool — 18 total; RC-27, docs/vision/
// 28-receipts.md §4, added discoveries, the hand-off receipt tool — 19
// total; ADR-035 decision 1 (docs/vision/29-verbs-not-lookups.md §3 wave
// A, plan VB-1/VB-2) promotes blast_radius, review_diff, test_impact and
// rename_impact — the answers grep cannot give, never another lookup
// pitched on token savings — and adds safe_delete_check, rename_impact's
// read-only sibling — 24 total; wave G (plan VB-26) promotes localize,
// the bounded/budgeted/logged localization walk — 25 total,
// companionWantCount below).
// Changing the surface is deliberate: update this list and docs/mcp-tools.md
// together. Served identically at both /rpc/agent (the primary path) and
// /rpc/companion (a compatible alias).
var companionWant = []string{
	"blast_radius",
	"capabilities",
	"discoveries",
	"entrypoints",
	"find_callers",
	"find_symbol",
	"ingest",
	"list_services",
	"localize",
	"memory_search",
	"obligations",
	"outline",
	"pack_context",
	"query_symbols",
	"read_source",
	"references",
	"rename_impact",
	"repo_map",
	"review_diff",
	"safe_delete_check",
	"search",
	"staleness_report",
	"symbol_context",
	"test_impact",
	"verify_plan",
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

// companionWantCount pins the agent-surface tool count (V-review MINOR:
// prose docs drifted to "17-tool"/"18-tool" after RC-27 added discoveries
// without this assertion catching it) — the ONE place the number lives as
// a checked fact; docs/mcp-tools.md, docs/CLAUDE.md, internal/CLAUDE.md and
// every other prose mention are asserted equal to companionWant's own
// length here, never independently counted by hand.
const companionWantCount = 25

func TestCompanionSurfaceToolsList(t *testing.T) {
	if len(companionWant) != companionWantCount {
		t.Fatalf("companionWant has %d entries, want %d — update companionWantCount, docs/mcp-tools.md, docs/CLAUDE.md and internal/CLAUDE.md together", len(companionWant), companionWantCount)
	}
	srv := mcp.NewServer(&fakeBackend{})
	want := append([]string(nil), companionWant...)
	sort.Strings(want)

	// /rpc/agent is the primary surface; /rpc/companion is a compatible
	// alias — both must serve the identical allowlist.
	for _, surface := range []string{mcp.SurfaceAgent, mcp.SurfaceCompanion} {
		ctx := mcp.WithRequestScope(context.Background(), surface, true)
		names := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
			JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
		}))
		if len(names) != len(want) {
			t.Fatalf("%s surface has %d tools, want %d\n got: %v\nwant: %v", surface, len(names), len(want), names, want)
		}
		for i := range want {
			if names[i] != want[i] {
				t.Fatalf("%s surface mismatch at %d: got %q want %q\nfull: %v", surface, i, names[i], want[i], names)
			}
		}
	}
}

// ─── companion surface: tools/call is execute-and-flag, not hard-gated ───────

func TestCompanionSurfaceGatesToolsCall(t *testing.T) {
	var buf bytes.Buffer
	srv := mcp.NewServerWithLogger(&fakeBackend{}, mcp.NewCallLogger(&buf))
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)

	// Off-surface tools still execute — even shared-group ones like
	// secret_scan (ADR-014 amendment: an error only teaches abandonment,
	// which looks identical to no-demand in the call log). The call log
	// records the pass-through with off_surface:true.
	for _, tool := range []string{"secret_scan", "dead_code", "shell_out"} {
		buf.Reset()
		resp := scopedRoundtrip(t, srv, ctx, request{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: map[string]any{"name": tool, "arguments": map[string]any{"service": "svc"}},
		})
		if resp["error"] != nil {
			t.Fatalf("off-surface %q should execute-and-flag, got error: %v", tool, resp["error"])
		}
		line := strings.TrimSpace(buf.String())
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line parse for %q: %v (line=%q)", tool, err, line)
		}
		if off, _ := entry["off_surface"].(bool); !off {
			t.Errorf("%q: call-log entry should carry off_surface:true, got %v", tool, entry)
		}
	}

	// On-surface tools dispatch too, without the flag.
	buf.Reset()
	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: map[string]any{"name": "find_symbol", "arguments": map[string]any{"service": "svc", "name": "Foo"}},
	})
	if resp["error"] != nil {
		t.Fatalf("find_symbol on companion surface should dispatch, got error: %v", resp["error"])
	}
	var onSurfaceEntry map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &onSurfaceEntry); err != nil {
		t.Fatalf("log line parse: %v", err)
	}
	if off, _ := onSurfaceEntry["off_surface"].(bool); off {
		t.Error("find_symbol is on the companion surface; off_surface should be false/absent")
	}

	// Hard gating stays available behind strictSurfaces for any future
	// multi-tenant shape — flipping it restores the pre-amendment error.
	srv.SetStrictSurfaces(true)
	strict := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 4, Method: "tools/call",
		Params: map[string]any{"name": "secret_scan", "arguments": map[string]any{"service": "svc"}},
	})
	errObj, ok := strict["error"].(map[string]any)
	if !ok {
		t.Fatalf("secret_scan under strictSurfaces: want error, got %v", strict)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("strictSurfaces: code = %v, want %v", code, mcp.ErrCodeNotFound)
	}
	if msg := errObj["message"].(string); !strings.Contains(msg, "surface") {
		t.Errorf("strictSurfaces: error message %q should name the surface", msg)
	}
}

// ─── batch cannot smuggle off-surface tools — but they still execute-and-flag ─

func TestBatchGatedOnSurface(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	batchReq := request{
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
	}
	resp := scopedRoundtrip(t, srv, ctx, batchReq)
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
	// Off-surface sub-calls execute-and-flag too: secret_scan runs instead of
	// being gated (ADR-014 amendment).
	if payload.Results[0].Error != "" {
		t.Errorf("secret_scan sub-call should execute-and-flag, got error %q", payload.Results[0].Error)
	}
	if payload.Results[1].Error != "" {
		t.Errorf("find_symbol sub-call should succeed, got error %q", payload.Results[1].Error)
	}

	// strictSurfaces restores the pre-amendment gate for batch sub-calls too.
	// batch itself is GroupShared, which dropped off the companion allowlist
	// in this slice (companionTools omits batch/brief); under strict
	// mode a companion-scoped batch call would be rejected outright before
	// even reaching its sub-calls. Use the "shared" surface instead, where
	// batch is NOT on the agent allowlist (surfaces week-1 dropped it), so
	// under strictSurfaces the outer call itself is refused — HF-5 deleted
	// the group surfaces this case used to ride ("shared"), and the agent
	// surface is the only curated one left.
	srv.SetStrictSurfaces(true)
	sharedCtx := mcp.WithRequestScope(context.Background(), mcp.SurfaceAgent, true)
	strictResp := scopedRoundtrip(t, srv, sharedCtx, request{
		JSONRPC: "2.0", ID: 5, Method: "tools/call",
		Params: map[string]any{
			"name": "batch",
			"arguments": map[string]any{
				"calls": []map[string]any{
					{"name": "find_callers", "arguments": map[string]any{"service": "svc", "symbol": "Foo"}},
					{"name": "secret_scan", "arguments": map[string]any{"service": "svc"}},
				},
			},
		},
	})
	if strictResp["error"] == nil {
		t.Fatalf("batch is off the agent allowlist; strictSurfaces must refuse the outer call")
	}
	if msg, _ := strictResp["error"].(map[string]any)["message"].(string); !strings.Contains(msg, "batch") {
		t.Errorf("strict refusal should name the tool: %q", msg)
	}
	return
}

// batchStrictTail is the pre-HF-5 remainder of TestBatchGatedOnSurface,
// kept out of the compiled path: it asserted per-sub-call gating on a group
// surface that no longer exists. The outer-call refusal above is the
// surviving contract.
func batchStrictTail(t *testing.T, strictResp map[string]any) {
	strictContent := strictResp["result"].(map[string]any)["content"].([]any)
	strictText := strictContent[0].(map[string]any)["text"].(string)
	var strictPayload struct {
		Results []struct {
			Name  string `json:"name"`
			Error string `json:"error"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(strictText), &strictPayload); err != nil {
		t.Fatalf("strict batch payload parse: %v (text=%q)", err, strictText)
	}
	if !strings.Contains(strictPayload.Results[0].Error, "surface") {
		t.Errorf("find_callers sub-call (GroupInvestigation) under strictSurfaces on shared should be gated, got error %q", strictPayload.Results[0].Error)
	}
	if strictPayload.Results[1].Error != "" {
		t.Errorf("secret_scan sub-call (GroupShared) should still succeed on the shared surface, got error %q", strictPayload.Results[1].Error)
	}
}

// set_profile no longer exists (removed entirely, SW-2): stdio always serves
// the full registry now, and there is no tool for anything to reject over
// HTTP. The remaining internal profile machinery (setProfile/activeProfile,
// still used by effectiveSurface for stdio) is exercised directly in
// TestSetProfileDirect (profile_test.go, package mcp) since no external
// caller can reach it any more; that test also covers the stdio-profile vs.
// HTTP-surface view equivalence and stdio's tools/call staying advisory.

// ─── capabilities reports the active surface ─────────────────────────────────

func TestCapabilitiesReportsActiveSurface(t *testing.T) {
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

	// capabilities was promoted onto the companion/agent allowlist
	// (docs/vision/05-surfaces.md §3) — the discovery escape hatch a small,
	// curated surface needs — so it dispatches there too, on both surface
	// names.
	for _, surface := range []string{mcp.SurfaceCompanion, mcp.SurfaceAgent} {
		scCtx := mcp.WithRequestScope(context.Background(), surface, true)
		got := scopedRoundtrip(t, srv, scCtx, request{
			JSONRPC: "2.0", ID: 10, Method: "tools/call",
			Params: map[string]any{"name": "capabilities", "arguments": map[string]any{}},
		})
		if got["error"] != nil {
			t.Errorf("capabilities on %s surface: %v", surface, got["error"])
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
	// set_profile no longer exists, so there is nothing left to strip from
	// the HTTP view: the unscoped HTTP registry is exactly the stdio registry.
	if len(viaHTTP) != len(full) {
		t.Fatalf("unscoped HTTP lists %d tools, want %d (same as stdio)", len(viaHTTP), len(full))
	}
	for i := range full {
		if viaHTTP[i] != full[i] {
			t.Fatalf("mismatch at %d: HTTP %q vs stdio %q", i, viaHTTP[i], full[i])
		}
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

// TestPromptsScopedOnSurface pins SR-20 (surface reckoning F-78): the four
// canned prompts are gone from every surface, scoped or not.
// registerPromptsHandlers stays wired (the protocol capability is kept),
// serving an empty list everywhere, and prompts/get for any name — a
// former canned prompt or not — errors identically on every surface.
func TestPromptsScopedOnSurface(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})

	unscoped := scopedRoundtrip(t, srv, context.Background(), request{
		JSONRPC: "2.0", ID: 33, Method: "prompts/list", Params: map[string]any{},
	})
	if unscoped["error"] != nil {
		t.Fatalf("prompts/list: %v", unscoped["error"])
	}
	all := unscoped["result"].(map[string]any)["prompts"].([]any)
	if len(all) != 0 {
		t.Fatalf("unscoped prompts/list = %d prompts, want 0 (SR-20 removed the canned prompts)", len(all))
	}

	ctx := mcp.WithRequestScope(context.Background(), "companion", true)
	scoped := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 34, Method: "prompts/list", Params: map[string]any{},
	})
	if scoped["error"] != nil {
		t.Fatalf("prompts/list on companion: %v", scoped["error"])
	}
	visible := scoped["result"].(map[string]any)["prompts"].([]any)
	if len(visible) != 0 {
		t.Fatalf("companion prompts/list = %d prompts, want 0: %v", len(visible), visible)
	}
	got := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 35, Method: "prompts/get",
		Params: map[string]any{"name": "investigate_incident", "arguments": map[string]any{"service": "svc", "symptom": "x"}},
	})
	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("prompts/get on companion: want error, got %v", got)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("code = %v, want ErrCodeNotFound (prompt gone, not surface-gated)", code)
	}
}

// ─── surface name validation ─────────────────────────────────────────────────

func TestValidSurface(t *testing.T) {
	for _, name := range []string{"agent", "companion", "modules"} {
		if !mcp.ValidSurface(name) {
			t.Errorf("ValidSurface(%q) = false, want true", name)
		}
	}
	// HF-5 (ADR-014 amendment 2): the four group surfaces are DELETED. They
	// were reachable only by typing the URL — no client registration, stanza,
	// skill, or doc pointed at one — so their zero traffic proved
	// exposure-zero, not demand-zero (law 8 forbids reading that as a kill
	// clock); they go on law 4's ground, a surface with no reachable consumer.
	for _, name := range []string{"coding", "investigation", "shared", "platform"} {
		if mcp.ValidSurface(name) {
			t.Errorf("ValidSurface(%q) = true — the group surfaces were deleted (HF-5)", name)
		}
	}
	for _, name := range []string{"", "bogus", "Companion", "Agent", "full"} {
		if mcp.ValidSurface(name) {
			t.Errorf("ValidSurface(%q) = true, want false", name)
		}
	}
	names := mcp.SurfaceNames()
	if len(names) != 3 {
		t.Fatalf("SurfaceNames() = %v, want 3 entries (agent, companion, modules)", names)
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
	// secret_scan is off the companion allowlist but execute-and-flags: it
	// runs (no error) and its log entry carries off_surface:true.
	offSurface, ok := byTool["secret_scan"]
	if !ok {
		t.Fatalf("off-surface secret_scan sub-call not logged: %v", byTool)
	}
	if errMsg, _ := offSurface["error"].(string); errMsg != "" {
		t.Errorf("off-surface sub-call should execute-and-flag, not error, got %v", offSurface["error"])
	}
	if off, _ := offSurface["off_surface"].(bool); !off {
		t.Errorf("off-surface sub-call should log off_surface:true, got %v", offSurface)
	}
	if outer, ok := byTool["batch"]; !ok {
		t.Error("outer batch entry not logged")
	} else if outer["via"] != nil {
		t.Errorf("outer batch entry should have no via, got %v", outer["via"])
	}
}

// TestGroupSurfacesRetired (HF-5, ADR-014 amendment 2) pins the deletion of
// the four path-scoped group surfaces. They were reachable only by typing
// the URL — no client registration, stanza, skill, or doc ever pointed at
// one — so their zero traffic proved exposure-zero, not demand-zero, and
// law 8 forbids reading that as a kill clock. They go on law 4's ground
// instead: a surface no wired client can reach is registry weight with no
// consumer. The group TAXONOMY they were named after is untouched.
func TestGroupSurfacesRetired(t *testing.T) {
	for _, gone := range []string{"coding", "investigation", "shared", "platform"} {
		if mcp.ValidSurface(gone) {
			t.Errorf("ValidSurface(%q) = true — HF-5 deleted the group surfaces", gone)
		}
	}
	for _, kept := range []string{mcp.SurfaceAgent, mcp.SurfaceCompanion, mcp.SurfaceModules} {
		if !mcp.ValidSurface(kept) {
			t.Errorf("ValidSurface(%q) = false — agent, companion and modules survive", kept)
		}
	}
	if got := mcp.SurfaceNames(); len(got) != 3 {
		t.Errorf("SurfaceNames() = %v, want exactly the three surviving surfaces", got)
	}

	// The taxonomy survives the surfaces: a tool still carries its group.
	srv := mcp.NewServer(&fakeBackend{})
	if got := srv.GroupOfForTest("find_callers"); got != mcp.GroupInvestigation {
		t.Errorf("group taxonomy must survive surface deletion: find_callers group = %q", got)
	}
}
