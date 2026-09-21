// surface_description_caps_test.go — fl-wave-10, tightened by O101/TD-2
// (the wave-diet brief): per-tool and per-argument byte caps on the agent
// surface's schema prose (docs/vision/05-surfaces.md,
// cmd/fathom/cost_panel_test.go's agentSurfaceSchemaByteCeiling). Every byte
// on this surface is paid by every connected session before its first call
// (BA-7); design history, plan-row ids and migration notes belong in
// docs/mcp-tools.md, not in the schema.
//
// O101 dropped the wider 1000-byte allowance the four composed/fleet-wide
// tools (review_diff/obligations/references/search) carried under
// fl-wave-10: measured against the "one question sentence, one reach-for-it
// clause" shape, every tool — including those four — fits the same 260-byte
// cap without losing the machine-checkable contract (Required:/"Reach for
// it when"/the five tools' stop sentence); the extra moving parts (coverage
// layers, exact-mode fallback, receipt shape) live in docs/mcp-tools.md.
package mcp

import "testing"

// agentSurfaceDescriptionCap is the per-tool Description byte cap (O101):
// one sentence naming the question the tool answers, one clause naming
// when to reach for it over grep or another fathom tool.
const agentSurfaceDescriptionCap = 260

// agentSurfaceArgDescriptionCap is the per-argument-property "description"
// byte cap, applied recursively (array item properties included). O101
// drops alias/history prose from argument descriptions; the aliases
// themselves still work (paramalias.go).
const agentSurfaceArgDescriptionCap = 120

// collectSchemaDescriptions walks an InputSchema (or any nested object/array
// within it) and appends every "description" string found, tagged with a
// dotted path for a readable failure message.
func collectSchemaDescriptions(v any, path string, out *[]struct{ path, text string }) {
	switch t := v.(type) {
	case map[string]any:
		if d, ok := t["description"].(string); ok {
			*out = append(*out, struct{ path, text string }{path, d})
		}
		for k, val := range t {
			if k == "description" {
				continue
			}
			collectSchemaDescriptions(val, path+"."+k, out)
		}
	case []any:
		for i, val := range t {
			collectSchemaDescriptions(val, path, out)
			_ = i
		}
	}
}

// TestAgentSurfaceDescriptionsWithinCaps is the invariant-2/5 pin
// (fl-wave-10, tightened by O101): every agent-surface tool's top-level
// Description fits 260 bytes, and every argument-property "description"
// string fits 120 bytes.
func TestAgentSurfaceDescriptionsWithinCaps(t *testing.T) {
	s := NewServer(nil)
	for _, name := range AgentSurfaceTools() {
		te, ok := s.findTool(name)
		if !ok {
			t.Errorf("agent surface tool %q not found in the registry", name)
			continue
		}
		if n := len(te.desc.Description); n > agentSurfaceDescriptionCap {
			t.Errorf("tool %q description = %d bytes, want <= %d (move design history/plan-row ids/migration notes to docs/mcp-tools.md)", name, n, agentSurfaceDescriptionCap)
		}

		var descs []struct{ path, text string }
		collectSchemaDescriptions(te.desc.InputSchema, name, &descs)
		for _, d := range descs {
			if n := len(d.text); n > agentSurfaceArgDescriptionCap {
				t.Errorf("tool %q arg description at %s = %d bytes, want <= %d: %q", name, d.path, n, agentSurfaceArgDescriptionCap, d.text)
			}
		}
	}
}
