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
const (
	agentSurfaceSchemaByteFloor   = 10000
	agentSurfaceSchemaByteCeiling = 12500
)

// TestNarrowProfileSchemaBytes is BA-7: the narrow/agent surface profile
// carries only its own tools' schemas, with the byte count asserted so an
// accidental widening fails the build.
func TestNarrowProfileSchemaBytes(t *testing.T) {
	ts := httptest.NewServer(newTestHandler(t))
	defer ts.Close()
	hc := &http.Client{}

	tools, schemaBytes := surfaceSchemaTax(hc, ts.URL)
	if tools != 16 {
		t.Errorf("agent surface tools = %d, want 16 (TestAgentSurfaceDiet's own pin)", tools)
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
