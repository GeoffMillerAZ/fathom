package mcp

import (
	"sort"
	"testing"
)

// TestAgentSurfaceDiet pins TD-2 (docs/plans/exploration-ledger.md): the
// agent surface is the curated set — every schema on it taxes EVERY
// MCP-connected session's context before any work happens, so growing it
// requires a measured case (a TD-1 panel reading, not a hunch) and the
// long tail belongs on bare /rpc. `search` (RC-24) and `discoveries`
// (RC-27, docs/vision/28-receipts.md §4) are the two deliberate,
// pre-registered exceptions since AM-8: discoveries carries its own kill
// criterion (M5 at K=3 — facts_reused >= 3 and tokens-to-first-edit below
// control on two of three tasks, else it reverts to pull-only and the cap
// drops back to 18).
//
// ADR-035 decision 1 (docs/vision/29-verbs-not-lookups.md §3 wave A, plan
// VB-1/VB-2) raises the cap again, on its own measured case: the verbs
// lead because grep cannot give their answer at all (blast radius,
// review, test reach, rename impact) — never another lookup pitched on
// token savings — plus safe_delete_check, rename_impact's read-only
// sibling. 19 + 5 = 24.
//
// Wave G (plan VB-26) raises it once more: localize — where a bug lives
// before anyone knows the file, R10's own case (LocAgent/SHERLOC) for a
// verb grep cannot substitute for. 24 + 1 = 25.
func TestAgentSurfaceDiet(t *testing.T) {
	tools := AgentSurfaceTools()
	if len(tools) > 25 {
		t.Fatalf("agent surface carries %d tools — the diet caps it at 25 (19 + the four ADR-035 verbs + safe_delete_check + localize); the long tail belongs on /rpc (TD-2)", len(tools))
	}
	on := map[string]bool{}
	for _, name := range tools {
		on[name] = true
	}
	// The golden discovery loop: losing any of these breaks the session
	// brief's promise, so the diet may never cut them.
	for _, want := range []string{"find_symbol", "references", "outline", "obligations", "verify_plan", "read_source", "ingest"} {
		if !on[want] {
			t.Errorf("golden tool %q missing from the agent surface", want)
		}
	}
}

// TestDefaultSurfaceIsEight is the surface reckoning's own diet test
// (F-86, design §1.1, plan SR-33): POST /rpc/agent — the primary path the
// user-scope MCP registration points at — serves exactly the eight-tool
// floor, not the wider companion/extended allowlist AgentSurfaceTools
// still describes. Growing this set is the same measured-case discipline
// TestAgentSurfaceDiet already documents for the wider surface; the long
// tail lives one opt-in hop away at /rpc/extended (and /rpc/companion, its
// compatible alias), never silently back on the default.
func TestDefaultSurfaceIsEight(t *testing.T) {
	srv := NewServer(nil)
	descs := srv.visibleToolsFor(SurfaceAgent)
	if len(descs) != 8 {
		names := make([]string, len(descs))
		for i, d := range descs {
			names[i] = d.Name
		}
		sort.Strings(names)
		t.Fatalf("/rpc/agent serves %d tools, want exactly 8: %v", len(descs), names)
	}
	want := []string{
		"blast_radius", "find_symbol", "obligations", "read_source",
		"review_diff", "search", "symbol_context", "verify_plan",
	}
	sort.Strings(want)
	got := make([]string, len(descs))
	for i, d := range descs {
		got[i] = d.Name
	}
	sort.Strings(got)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("/rpc/agent surface mismatch at %d: got %q want %q\nfull: %v", i, got[i], want[i], got)
		}
	}
	// DefaultSurfaceTools is the same set exported for callers outside the
	// package (SR-34's stanza generation among them).
	if exported := DefaultSurfaceTools(); len(exported) != 8 {
		t.Errorf("DefaultSurfaceTools() = %v, want 8 entries", exported)
	}
}
