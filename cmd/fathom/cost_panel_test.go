package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── BA-7: the narrow (agent) surface profile's schema byte ceiling ────────

// agentSurfaceSchemaByteCeiling is an ASSERTED ceiling on the agent
// surface's serialized tool schema bytes (docs/design/budgeted-answers.md
// §4): measured at 11,839 bytes / 16 tools when this test was written
// (matching the design doc's own cited figure exactly). A tool added to the
// surface without a measured case (§4 point 2) pushes past this and fails
// the build; the lower bound catches the opposite defect — an empty or
// broken surface silently reading as "cheap".
//
// Re-pinned 2026-09-02 for the served contract (docs/plans/reach-parity.md
// RP-9..RP-11), measured on the same 17 tools: 15,980 bytes with the
// per-tool "Reach for it when …" / "Required: …" sentences (RP-9) and
// 28,705 bytes with annotations (RP-10) and generated outputSchema (RP-11)
// on top — the schema budget more than doubled, and that number is the
// price of a contract a non-Claude client can machine-check; the
// capabilities tool now reports it live as schema_bytes so the next change
// is measured too. The band below brackets that measurement, not a hope.
//
// BP-22 re-pin, 2026-09-12, CORRECTED 2026-09-13 (O32). The band is
// unchanged. The 2026-09-12 probe reported 0 ToolSearch turns with and
// without alwaysLoad, but it read the stream's `init` tool list — which
// names every MCP tool whether or not its schema is deferred — and its
// prompt never made the model call a fathom tool, so deferral had nothing
// to show. The 2026-09-13 probe that DID demand a fathom call, headless
// `claude -p --output-format stream-json` on 2.1.270 against the same
// 17-tool / 28.7 kB surface:
//
//	alwaysLoad unset → 1 ToolSearch turn before the first fathom call
//	alwaysLoad:true  → 0 ToolSearch turns
//
// Claude Code's MCP docs (2.1.270) say the same: with ENABLE_TOOL_SEARCH
// unset EVERY MCP tool schema is deferred behind ToolSearch, no threshold;
// `alwaysLoad: true` exempts a server. The full home run of 2026-09-13
// paid ≥1 ToolSearch turn in every fathom cell (11% of the arm's tokens).
// So the O24 tax is real in headless sessions, the remedy is the
// alwaysLoad bit (written by `fathom plugin render`, the bench config and
// the descriptor note), and this band stays a BUDGET — the schema size
// was never what decided deferral.
//
// The band is deliberately not tightened toward the ~16 kB July figure.
// Reaching it would mean deleting either the generated outputSchemas
// (10,693 bytes — the machine-checkable contract RP-11 exists to provide)
// or the per-tool "Reach for it when …" sentences and argument
// descriptions that make the surface discoverable, to chase a threshold
// that is measurably not binding. `$ref`/`$defs` compaction was evaluated
// and rejected on evidence: every repeated sub-schema repeats ACROSS tools,
// and tools/list gives each tool its own independent schema document, so
// in-document refs save exactly 0 bytes (measured).
const (
	agentSurfaceSchemaByteFloor = 26000
	// agentSurfaceSchemaByteCeiling was 30000 before SB-1..SB-5
	// (docs/plans/strengths-bench.md): references/read_source/find_symbol
	// gained longer, more specific descriptions and new response fields
	// (implementations, elided, occurrence_tier/depth/service, max_bytes),
	// and five tools gained a "this result IS the answer" sentence — a
	// measured, deliberate widening (O31/O32's over-payload findings), not
	// a new tool (still 17, TestNarrowProfileSchemaBytes' own pin).
	// Re-pinned 2026-09-13: `references` gained `exact`, `find_callers`
	// gained `with_signatures`, `find_symbol` gained `implementations`
	// (SB-45/47/5) — 34,821 bytes measured. Size never decided deferral
	// (O32: alwaysLoad does), so this band is a budget, not a threshold.
	agentSurfaceSchemaByteCeiling = 36000
)

// TestNarrowProfileSchemaBytes is BA-7: the narrow/agent surface profile
// carries only its own tools' schemas, with the byte count asserted so an
// accidental widening fails the build.
func TestNarrowProfileSchemaBytes(t *testing.T) {
	ts := httptest.NewServer(newTestHandler(t))
	defer ts.Close()
	hc := &http.Client{}

	tools, schemaBytes := surfaceSchemaTax(hc, ts.URL)
	if tools != 17 {
		t.Errorf("agent surface tools = %d, want 17 (TestAgentSurfaceDiet's own pin: 16 + memory_search, AM-8)", tools)
	}
	if schemaBytes < agentSurfaceSchemaByteFloor || schemaBytes > agentSurfaceSchemaByteCeiling {
		t.Errorf("agent surface schema bytes = %d, want between %d and %d — "+
			"either the surface widened (add a tool only with a measured case) or shrank unexpectedly",
			schemaBytes, agentSurfaceSchemaByteFloor, agentSurfaceSchemaByteCeiling)
	}
}

// ─── BA-8: the Cost panel ───────────────────────────────────────────────────

func intp(n int) *int { return &n }

// TestCostPanel is BA-8: the Cost panel folds already-read call-log entries
// (BA-6's response_bytes/elided_count) into per-tool token estimates and
// elision rates, carries the schema tax (BA-7's number), and renders the
// tokens-spent/discovery-served ratio labeled OBSERVATIONAL — never a
// savings claim (law 12).
func TestCostPanel(t *testing.T) {
	entries := []adoptionCallEntry{
		{Tool: "query_symbols", ResponseBytes: 4000, ElidedCount: intp(3)},
		{Tool: "query_symbols", ResponseBytes: 2000, ElidedCount: intp(0)},
		{Tool: "find_symbol", ResponseBytes: 400}, // no elided field: not a truncating call
		{Tool: "", ResponseBytes: 999},            // no tool name: excluded
	}

	cp := observatoryCostPanel(entries, 16, 11839, intp(100))

	if cp.SchemaTaxTools != 16 || cp.SchemaTaxBytes != 11839 {
		t.Errorf("schema tax = %d tools / %d bytes, want 16/11839 (BA-7's own number)", cp.SchemaTaxTools, cp.SchemaTaxBytes)
	}
	if cp.SchemaTaxTrendNote == "" {
		t.Error("schema tax trend must be a NAMED absence (law 3), not blank")
	}
	if cp.TotalBytes != 6400 {
		t.Errorf("TotalBytes = %d, want 6400 (4000+2000+400)", cp.TotalBytes)
	}
	if cp.TotalEstTokens != 6400/costEstBytesPerToken {
		t.Errorf("TotalEstTokens = %d, want %d", cp.TotalEstTokens, 6400/costEstBytesPerToken)
	}
	if len(cp.Tools) != 2 {
		t.Fatalf("Tools = %+v, want 2 rows (query_symbols, find_symbol) — the blank-tool row must be excluded", cp.Tools)
	}
	// Sorted by TotalBytes descending: query_symbols (6000) before find_symbol (400).
	if cp.Tools[0].Tool != "query_symbols" || cp.Tools[1].Tool != "find_symbol" {
		t.Fatalf("Tools order = %v, want [query_symbols find_symbol]", []string{cp.Tools[0].Tool, cp.Tools[1].Tool})
	}
	qs := cp.Tools[0]
	if qs.Calls != 2 || qs.TotalBytes != 6000 {
		t.Errorf("query_symbols row = %+v, want Calls=2 TotalBytes=6000", qs)
	}
	if pct, ok := qs.ElisionRatePct(); !ok || pct != 50 {
		t.Errorf("query_symbols elision rate = %d/%v, want 50/true (1 of 2 truncating calls elided)", pct, ok)
	}
	fs := cp.Tools[1]
	if _, ok := fs.ElisionRatePct(); ok {
		t.Errorf("find_symbol carried no elided field on its one call — rate must be \"not meaningful\", got ok=true")
	}

	if cp.RatioTokensPerDiscoveryX10 == nil {
		t.Fatal("expected a computed ratio (discovery total was provided and nonzero)")
	}
	if !strings.Contains(strings.ToUpper(cp.RatioNote), "OBSERVATIONAL") {
		t.Errorf("RatioNote = %q, must say OBSERVATIONAL (law 12)", cp.RatioNote)
	}

	// No discovery total measured: the ratio is absent, named, never a
	// fabricated number.
	cpNoDiscovery := observatoryCostPanel(entries, 16, 11839, nil)
	if cpNoDiscovery.RatioTokensPerDiscoveryX10 != nil {
		t.Error("ratio must be nil when discovery total is not measured")
	}
	if cpNoDiscovery.RatioNote == "" {
		t.Error("absent ratio must still carry a named reason")
	}

	// Rendering must not fabricate a trend or a percent for a non-truncating row.
	html := renderCostPanel(cp)
	if !strings.Contains(html, "query_symbols") || !strings.Contains(html, "find_symbol") {
		t.Error("rendered panel must list both tools")
	}
	if !strings.Contains(html, "—") {
		t.Error("rendered panel must show \"—\" (not 0%) for find_symbol's non-truncating elision rate")
	}
}
