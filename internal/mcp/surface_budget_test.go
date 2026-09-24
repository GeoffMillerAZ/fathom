// surface_budget_test.go — O101/TD-2 (the wave-diet brief): a single
// machine-checked budget on the agent surface's tools/list, rendered
// exactly as served (default ctx: no WithContracts, so outputSchema is
// off — see dispatch.go's toolsListEntries). Every byte here is paid by
// every MCP-connected session before its first call (BA-7).
//
// agentSurfaceTotalByteCeiling is a MEASURED pin, not the brief's own
// ≤16,000 B aspiration: dropping outputSchema (18,827 B measured
// 2026-09-21) plus the description/property diet (this file's caps, and
// surface_description_caps_test.go's 260/120) cut the served payload
// 48,341 -> 18,008 B (62.7%), but three components form a floor a
// description diet alone cannot cross without cutting something this
// brief put out of scope, the same shape as fl-wave-10's outputSchema
// floor before it (cost_panel_test.go's agentSurfaceSchemaByteCeiling
// history): annotations (RP-10, ~2,227 B, unconditionally served —
// untouched, no client asked to drop it), the argument-schema STRUCTURE
// golden-pinned by TestAgentSurfaceArgSchemaUnchanged (~4,900 B: property
// names/types/enums/required, protected because it is a real contract,
// not prose), and the JSON wire overhead of 25 {name,description,
// inputSchema,annotations} envelopes (~3,600 B, pure punctuation). Those
// three sum to ~10.7 kB before a single byte of description or argument
// prose is counted — leaving ~5.3 kB of real budget for 25 tool
// descriptions (each carrying a mandatory "Required: …" sentence, a
// "Reach for it when …" sentence, and five carrying SB-4's stop
// sentence) plus 100+ argument descriptions, which is what this wave
// actually spent. Re-pin this ceiling (with the number that moved and
// why) whenever a genuine content change moves the measurement; never
// widen it to make an accidental regression pass.
package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"testing"
)

// agentSurfaceTotalByteCeiling is the served (contracts=false) tools/list
// total, measured + headroom for float/build noise.
const agentSurfaceTotalByteCeiling = 18700

// agentSurfacePerToolByteCeiling is the per-tool served-entry byte cap
// (name + description + inputSchema + annotations, contracts=false).
// Re-pinned 1200 -> 1260 (ADR-044 §1.6, CV-13): review_diff's genuinely
// new "since" argument measures 1240 B served; the terse description
// above leaves no further slack to cut from an already-dense six-property
// tool without losing real information. The total ceiling above did not
// move (18106 B measured, still inside 18700).
const agentSurfacePerToolByteCeiling = 1260

// TestAgentSurfaceBudget is the O101/TD-2 acceptance test: render the
// agent surface's tools/list exactly as a connected session receives it
// (no contracts=true — outputSchema stays off) and assert the served
// budget: total bytes, per-tool bytes, per-tool description bytes, and
// per-argument description bytes. The per-tool byte table is logged
// (t.Logf) so `go test -v` shows the number, not just the verdict.
func TestAgentSurfaceBudget(t *testing.T) {
	s := NewServer(nil)
	ctx := WithRequestScope(context.Background(), SurfaceAgent, true)
	entries := s.toolsListEntries(ctx)

	type row struct {
		name  string
		bytes int
	}
	rows := make([]row, 0, len(entries))
	total := 0
	for _, e := range entries {
		m := e.(map[string]any)
		name, _ := m["name"].(string)
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatalf("%s: marshal entry: %v", name, err)
		}
		n := len(b)
		total += n
		rows = append(rows, row{name, n})

		if m["outputSchema"] != nil {
			t.Errorf("%s: served outputSchema with contracts=false (the diet's default must stay off)", name)
		}
		if n > agentSurfacePerToolByteCeiling {
			t.Errorf("%s: served entry = %d bytes, want <= %d", name, n, agentSurfacePerToolByteCeiling)
		}

		desc, _ := m["description"].(string)
		if n := len(desc); n > agentSurfaceDescriptionCap {
			t.Errorf("%s: description = %d bytes, want <= %d", name, n, agentSurfaceDescriptionCap)
		}
		var descs []struct{ path, text string }
		collectSchemaDescriptions(m["inputSchema"], name, &descs)
		for _, d := range descs {
			if n := len(d.text); n > agentSurfaceArgDescriptionCap {
				t.Errorf("%s: arg description at %s = %d bytes, want <= %d: %q", name, d.path, n, agentSurfaceArgDescriptionCap, d.text)
			}
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].bytes > rows[j].bytes })
	for _, r := range rows {
		t.Logf("%-20s %5d bytes", r.name, r.bytes)
	}
	t.Logf("agent surface tools/list (contracts=false): %d tools, %d bytes summed per-entry", len(rows), total)

	// wireTotal is the ACTUAL served payload — json.Marshal({"tools":
	// [...]})  — the same computation capabilities' schema_bytes uses
	// (toolsListBytes, dispatch.go). It differs from the per-entry sum
	// above by the array/wrapper punctuation (commas, brackets, the
	// "tools" key), so the budget itself is asserted against this number,
	// not the sum.
	wireTotal := s.toolsListBytes(ctx)
	t.Logf("agent surface tools/list (contracts=false): %d bytes as served (with wrapper)", wireTotal)
	if wireTotal > agentSurfaceTotalByteCeiling {
		t.Errorf("agent surface tools/list = %d bytes, want <= %d", wireTotal, agentSurfaceTotalByteCeiling)
	}

	// End-to-end: the capabilities TOOL's own schema_bytes field (what a
	// connected session actually reads) must equal wireTotal — not a
	// separately-tracked or stale number — proving the diet is visible
	// through the same call a session would make to check its own cost.
	raw, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "capabilities", "arguments": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("marshal capabilities call: %v", err)
	}
	resp, hasResp := s.HandleRequest(ctx, raw)
	if !hasResp {
		t.Fatal("capabilities call produced no response")
	}
	var env struct {
		Result struct {
			StructuredContent struct {
				SchemaBytes int `json:"schema_bytes"`
			} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(resp, &env); err != nil {
		t.Fatalf("decode capabilities response: %v (raw: %s)", err, resp)
	}
	if env.Result.StructuredContent.SchemaBytes != wireTotal {
		t.Errorf("capabilities schema_bytes = %d, want %d (the served, diet-sized tools/list total)", env.Result.StructuredContent.SchemaBytes, wireTotal)
	}
}
