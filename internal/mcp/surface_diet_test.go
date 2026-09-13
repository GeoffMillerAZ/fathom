package mcp

import "testing"

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
func TestAgentSurfaceDiet(t *testing.T) {
	tools := AgentSurfaceTools()
	if len(tools) > 19 {
		t.Fatalf("agent surface carries %d tools — the diet caps it at 19 (16 + memory_search AM-8 + search RC-24 + discoveries RC-27); the long tail belongs on /rpc (TD-2)", len(tools))
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
