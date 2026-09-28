// Package mcp_test — blast_radius's `kind` merge (SR-24, F-13, design §4):
// rename_impact and safe_delete_check are unregistered; blast_radius
// answers all three shapes through a `kind` argument, byte-compatible with
// what each retired tool returned.
package mcp_test

import (
	"testing"
)

// TestBlastRadiusKindsMatchRetiredVerbs is SR-24's anchor: kind:"change"
// (the default) is unaffected, kind:"rename" is byte-compatible with the
// retired rename_impact, kind:"delete" is byte-compatible with the retired
// safe_delete_check (proven-zero receipt included), and both retired tool
// names are gone outright.
func TestBlastRadiusKindsMatchRetiredVerbs(t *testing.T) {
	srv := newExtSrv()

	// ─── kind:"change" (default) — unaffected by the merge ─────────────────
	changeResp := callTool(t, srv, "blast_radius", map[string]any{"service": "svc", "changed_files": []string{"pkg/a.go"}})
	if changeResp["symbols"] == nil {
		t.Fatalf(`blast_radius(kind:"change" implicit).symbols missing: %v`, changeResp)
	}
	if _, ok := changeResp["by_depth"]; !ok {
		t.Errorf(`blast_radius(kind:"change").by_depth missing: %v`, changeResp)
	}

	// ─── kind:"rename" — byte-compatible with the retired rename_impact ────
	// extFixtureNodes: A calls B, so renaming B needs A updated.
	renameResp := callTool(t, srv, "blast_radius", map[string]any{"service": "svc", "kind": "rename", "symbol": "B"})
	callers, ok := renameResp["callers"].([]any)
	if !ok || len(callers) != 1 {
		t.Fatalf(`blast_radius(kind:"rename").callers = %v, want exactly 1 (A)`, renameResp["callers"])
	}
	if _, ok := renameResp["resolution"]; !ok {
		t.Errorf(`blast_radius(kind:"rename") missing "resolution": %v`, renameResp)
	}
	if _, ok := renameResp["summary"]; !ok {
		t.Errorf(`blast_radius(kind:"rename") missing "summary": %v`, renameResp)
	}
	// kind:"change" never returns "callers" — proves the two shapes are
	// genuinely distinct, not one shape with extra fields.
	if _, ok := renameResp["by_depth"]; ok {
		t.Errorf(`blast_radius(kind:"rename") should not carry "by_depth" (that is kind:"change"'s field): %v`, renameResp)
	}

	// ─── kind:"delete" — byte-compatible with the retired safe_delete_check ─
	// extFixtureNodes: Orphan has no callers — the proven-zero receipt.
	deleteResp := callTool(t, srv, "blast_radius", map[string]any{"service": "svc", "kind": "delete", "symbol": "Orphan"})
	referents, ok := deleteResp["referents"].([]any)
	if !ok || len(referents) != 0 {
		t.Fatalf(`blast_radius(kind:"delete").referents = %v, want an empty array (Orphan has no callers)`, deleteResp["referents"])
	}
	if count, _ := deleteResp["referent_count"].(float64); count != 0 {
		t.Errorf(`blast_radius(kind:"delete").referent_count = %v, want 0`, deleteResp["referent_count"])
	}
	if ce, _ := deleteResp["checked_edges"].(string); ce != "typed" && ce != "heuristic" {
		t.Errorf(`blast_radius(kind:"delete").checked_edges = %v, want typed or heuristic`, deleteResp["checked_edges"])
	}

	// ─── rename/delete require symbol; change requires changed_files ───────
	for _, kind := range []string{"rename", "delete"} {
		resp := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: map[string]any{"name": "blast_radius", "arguments": map[string]any{"service": "svc", "kind": kind}},
		})
		if resp["error"] == nil {
			t.Errorf(`blast_radius(kind:%q) with no symbol should error, got %v`, kind, resp)
		}
	}

	// ─── an unknown kind is a params error ──────────────────────────────────
	bad := roundtrip(t, srv, request{
		JSONRPC: "2.0", ID: 1, Method: "tools/call",
		Params: map[string]any{"name": "blast_radius", "arguments": map[string]any{"service": "svc", "kind": "bogus", "symbol": "A"}},
	})
	if bad["error"] == nil {
		t.Errorf(`blast_radius(kind:"bogus") should error, got %v`, bad)
	}

	// ─── the retired tool names are gone outright ───────────────────────────
	for _, retired := range []string{"rename_impact", "safe_delete_check"} {
		gone := roundtrip(t, srv, request{
			JSONRPC: "2.0", ID: 1, Method: "tools/call",
			Params: map[string]any{"name": retired, "arguments": map[string]any{"service": "svc", "symbol": "A"}},
		})
		if gone["error"] == nil {
			t.Errorf("retired tool %q should no longer be callable, got %v", retired, gone)
		}
	}
}
