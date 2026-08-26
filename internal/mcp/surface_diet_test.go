package mcp

import "testing"

// TestAgentSurfaceDiet pins TD-2 (docs/plans/exploration-ledger.md): the
// agent surface is the curated set — every schema on it taxes EVERY
// MCP-connected session's context before any work happens, so growing it
// requires a measured case (a TD-1 panel reading, not a hunch) and the
// long tail belongs on bare /rpc.
func TestAgentSurfaceDiet(t *testing.T) {
	tools := AgentSurfaceTools()
	if len(tools) > 17 {
		t.Fatalf("agent surface carries %d tools — the diet caps it at 17 (16 + memory_search, AM-8); the long tail belongs on /rpc (TD-2)", len(tools))
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
