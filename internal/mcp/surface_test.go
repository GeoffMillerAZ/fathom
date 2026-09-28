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

// defaultWant is POST /rpc/agent's floor, pinned exactly (surface reckoning
// F-86, design §1.1, plan SR-33): the eight answers an agent gets before
// opting into anything else. Changing it is deliberate: update this list,
// extendedWant below and docs/mcp-tools.md together.
var defaultWant = []string{
	"blast_radius",
	"find_symbol",
	"obligations",
	"read_source",
	"review_diff",
	"search",
	"symbol_context",
	"verify_plan",
}

// extendedWant is POST /rpc/extended's (and its /rpc/companion compatible
// alias's) allowlist, pinned exactly (SR-33): the default eight plus the
// navigation/context/freshness tools and the remaining promoted verbs.
// "status" (design §1.1's capabilities/server_stats/list_sources merge,
// SR-23) joined this list once that row registered the tool — 21 total.
var extendedWant = []string{
	"blast_radius",
	"find_callers",
	"find_symbol",
	"findings_near",
	"ingest",
	"list_services",
	"localize",
	"obligations",
	"outline",
	"pack_context",
	"query_symbols",
	"read_source",
	"references",
	"repo_map",
	"review_diff",
	"search",
	"staleness_report",
	"status",
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

// defaultWantCount and extendedWantCount pin the two surfaces' tool counts
// (V-review MINOR: prose docs drifted to "17-tool"/"18-tool" after RC-27
// added discoveries without this assertion catching it) — the ONE place
// each number lives as a checked fact; docs/mcp-tools.md, docs/CLAUDE.md,
// internal/CLAUDE.md and every other prose mention are asserted equal to
// the two lists' own lengths here, never independently counted by hand.
const (
	defaultWantCount  = 8
	extendedWantCount = 21
)

// TestDefaultSurfaceToolsList pins POST /rpc/agent's served allowlist
// exactly (surface reckoning F-86, design §1.1, plan SR-33): the
// eight-tool floor, not the wider set /rpc/companion and /rpc/extended
// carry (TestExtendedSurfaceToolsList below).
func TestDefaultSurfaceToolsList(t *testing.T) {
	if len(defaultWant) != defaultWantCount {
		t.Fatalf("defaultWant has %d entries, want %d — update defaultWantCount, docs/mcp-tools.md, docs/CLAUDE.md and internal/CLAUDE.md together", len(defaultWant), defaultWantCount)
	}
	srv := mcp.NewServer(&fakeBackend{})
	want := append([]string(nil), defaultWant...)
	sort.Strings(want)

	ctx := mcp.WithRequestScope(context.Background(), mcp.SurfaceAgent, true)
	names := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/list", Params: map[string]any{},
	}))
	if len(names) != len(want) {
		t.Fatalf("agent surface has %d tools, want %d\n got: %v\nwant: %v", len(names), len(want), names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("agent surface mismatch at %d: got %q want %q\nfull: %v", i, names[i], want[i], names)
		}
	}
}

// TestExtendedSurfaceToolsList pins POST /rpc/extended and its
// POST /rpc/companion compatible alias exactly (SR-33): both must serve
// the identical, wider allowlist — the default eight plus the opt-in set —
// never the pre-reckoning companion allowlist AgentSurfaceTools still
// names.
func TestExtendedSurfaceToolsList(t *testing.T) {
	if len(extendedWant) != extendedWantCount {
		t.Fatalf("extendedWant has %d entries, want %d — update extendedWantCount, docs/mcp-tools.md, docs/CLAUDE.md and internal/CLAUDE.md together", len(extendedWant), extendedWantCount)
	}
	srv := mcp.NewServer(&fakeBackend{})
	want := append([]string(nil), extendedWant...)
	sort.Strings(want)

	// /rpc/extended is the primary opt-in surface; /rpc/companion is a
	// compatible alias — both must serve the identical allowlist.
	for _, surface := range []string{mcp.SurfaceExtended, mcp.SurfaceCompanion} {
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
	srv := mcp.NewServerWithFleetAndLogger(&fakeBackend{}, fleetFixture(), mcp.NewCallLogger(&buf))
	ctx := mcp.WithRequestScope(context.Background(), "companion", true)

	// Off-surface tools still execute — even shared-group ones like
	// cross_repo_callers (ADR-014 amendment: an error only teaches
	// abandonment, which looks identical to no-demand in the call log).
	// The call log records the pass-through with off_surface:true.
	// (secret_scan, dead_code and shell_out, the original three
	// off-surface examples, were unregistered by SR-12/SR-19, surface
	// reckoning F-22/F-77; status joined the companion/extended surface
	// itself at SR-23 and is no longer an off-surface example.)
	for _, tool := range []string{"cross_repo_callers"} {
		buf.Reset()
		resp := scopedRoundtrip(t, srv, ctx, request{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: map[string]any{"name": tool, "arguments": map[string]any{"symbol": "Foo"}},
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
		Params: map[string]any{"name": "cross_repo_callers", "arguments": map[string]any{"symbol": "Foo"}},
	})
	errObj, ok := strict["error"].(map[string]any)
	if !ok {
		t.Fatalf("cross_repo_callers under strictSurfaces: want error, got %v", strict)
	}
	if code := errObj["code"].(float64); code != float64(mcp.ErrCodeNotFound) {
		t.Errorf("strictSurfaces: code = %v, want %v", code, mcp.ErrCodeNotFound)
	}
	if msg := errObj["message"].(string); !strings.Contains(msg, "surface") {
		t.Errorf("strictSurfaces: error message %q should name the surface", msg)
	}
}

// set_profile no longer exists (removed entirely, SW-2): stdio always serves
// the full registry now, and there is no tool for anything to reject over
// HTTP. The remaining internal profile machinery (setProfile/activeProfile,
// still used by effectiveSurface for stdio) is exercised directly in
// TestSetProfileDirect (profile_test.go, package mcp) since no external
// caller can reach it any more; that test also covers the stdio-profile vs.
// HTTP-surface view equivalence and stdio's tools/call staying advisory.

// ─── status(section:"tools") reports the active surface ──────────────────────

// TestStatusToolsSectionReportsActiveSurface is the SR-23 successor to the
// retired capabilities tool's own TestCapabilitiesReportsActiveSurface.
func TestStatusToolsSectionReportsActiveSurface(t *testing.T) {
	srv := mcp.NewServer(&fakeBackend{})
	ctx := mcp.WithRequestScope(context.Background(), "coding", true)

	listed := toolNames(t, scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 8, Method: "tools/list", Params: map[string]any{},
	}))

	resp := scopedRoundtrip(t, srv, ctx, request{
		JSONRPC: "2.0", ID: 9, Method: "tools/call",
		Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "tools"}},
	})
	if resp["error"] != nil {
		t.Fatalf(`status(section:"tools"): %v`, resp["error"])
	}
	content := resp["result"].(map[string]any)["content"].([]any)
	text := content[0].(map[string]any)["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf(`status(section:"tools") payload parse: %v`, err)
	}
	if payload["active_surface"] != "coding" {
		t.Errorf("active_surface = %v, want coding", payload["active_surface"])
	}
	if count := payload["tool_count"].(float64); count != float64(len(listed)) {
		t.Errorf("tool_count = %v, want %d (the tools/list count on the same surface)", count, len(listed))
	}

	// status is on /rpc/extended and its /rpc/companion alias (extendedTools,
	// SR-23) but not the /rpc/agent floor — dispatches on both anyway,
	// on-surface on companion and execute-and-flag (ADR-014 amendment) on
	// agent.
	for _, surface := range []string{mcp.SurfaceCompanion, mcp.SurfaceAgent} {
		scCtx := mcp.WithRequestScope(context.Background(), surface, true)
		got := scopedRoundtrip(t, srv, scCtx, request{
			JSONRPC: "2.0", ID: 10, Method: "tools/call",
			Params: map[string]any{"name": "status", "arguments": map[string]any{"section": "tools"}},
		})
		if got["error"] != nil {
			t.Errorf(`status(section:"tools") on %s surface: %v`, surface, got["error"])
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
		Params: map[string]any{"name": "no_such_prompt", "arguments": map[string]any{"service": "svc"}},
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
	for _, name := range []string{"agent", "companion", "extended", "modules"} {
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
	if len(names) != 4 {
		t.Fatalf("SurfaceNames() = %v, want 4 entries (agent, companion, extended, modules)", names)
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

// TestCallLoggerRecordsBatchSubCalls was retired: batch and secret_scan,
// its demonstration vehicles, were both unregistered by SR-19 (surface
// reckoning, F-77).

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
	for _, kept := range []string{mcp.SurfaceAgent, mcp.SurfaceCompanion, mcp.SurfaceExtended, mcp.SurfaceModules} {
		if !mcp.ValidSurface(kept) {
			t.Errorf("ValidSurface(%q) = false — agent, companion, extended and modules survive", kept)
		}
	}
	if got := mcp.SurfaceNames(); len(got) != 4 {
		t.Errorf("SurfaceNames() = %v, want exactly the four surviving surfaces", got)
	}

	// The taxonomy survives the surfaces: a tool still carries its group.
	srv := mcp.NewServer(&fakeBackend{})
	if got := srv.GroupOfForTest("find_callers"); got != mcp.GroupInvestigation {
		t.Errorf("group taxonomy must survive surface deletion: find_callers group = %q", got)
	}
}
