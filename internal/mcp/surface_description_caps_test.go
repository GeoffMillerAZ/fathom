// surface_description_caps_test.go — fl-wave-10: per-tool and per-argument
// byte caps on the agent surface's schema prose (docs/vision/05-surfaces.md,
// cmd/fathom/cost_panel_test.go's agentSurfaceSchemaByteCeiling). Every byte
// on this surface is paid by every connected session before its first call
// (BA-7); design history, plan-row ids and migration notes belong in
// docs/mcp-tools.md, not in the schema.
package mcp

import "testing"

// agentSurfaceDescriptionCap is the default per-tool Description byte cap.
// The four composed/fleet-wide tools below carry a wider one — their answer
// genuinely has more moving parts a caller must know before calling
// correctly (dimensions, coverage layers, exact-mode fallback, receipt
// shape) — set explicitly rather than raising the default for everyone.
const agentSurfaceDescriptionCap = 700

// agentSurfaceDescriptionCapWide names the tools whose Description gets the
// wider 1000-byte cap.
var agentSurfaceDescriptionCapWide = map[string]int{
	"review_diff": 1000,
	"obligations": 1000,
	"references":  1000,
	"search":      1000,
}

// agentSurfaceArgDescriptionCap is the per-argument-property "description"
// byte cap, applied recursively (array item properties included).
const agentSurfaceArgDescriptionCap = 200

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
// (fl-wave-10): every agent-surface tool's top-level Description fits its
// cap (700 bytes, 1000 for the four composed/fleet-wide tools), and every
// argument-property "description" string fits 200 bytes.
func TestAgentSurfaceDescriptionsWithinCaps(t *testing.T) {
	s := NewServer(nil)
	for _, name := range AgentSurfaceTools() {
		te, ok := s.findTool(name)
		if !ok {
			t.Errorf("agent surface tool %q not found in the registry", name)
			continue
		}
		capBytes := agentSurfaceDescriptionCap
		if wide, ok := agentSurfaceDescriptionCapWide[name]; ok {
			capBytes = wide
		}
		if n := len(te.desc.Description); n > capBytes {
			t.Errorf("tool %q description = %d bytes, want <= %d (move design history/plan-row ids/migration notes to docs/mcp-tools.md)", name, n, capBytes)
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
