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
	// Re-pinned again 2026-09-13: `find_callers`/`references`/`outline`/
	// `symbol_context`/`staleness_report`/`repo_map` gained budget/
	// `scope_required` fields and descriptions (RC-19/RC-20), and
	// `references` gained `implementations` (RC-21) — 36,948 bytes
	// measured (vision 28, receipts).
	// RC-24 (docs/plans/receipts.md) then added `search` — the
	// grep-with-a-receipt tool, ships SB-48 — as an 18th agent-surface
	// tool: 40,204 bytes measured with RC-19/20/21 combined. A genuinely
	// new tool, not a widened description (TestAgentSurfaceDiet's own pin
	// moved to 18 alongside it).
	// Re-pinned again for `discoveries` (RC-27, docs/vision/
	// 28-receipts.md §4) — a second deliberate, pre-registered exception to
	// the agent-surface diet (surface_diet_test.go), alongside `search`,
	// carrying its own kill criterion: 41,733 bytes measured at 19 tools
	// combined with RC-19/20/21/24.
	// Re-pinned again for ADR-035 decision 1 (docs/vision/
	// 29-verbs-not-lookups.md §3 wave A, plan VB-1/VB-2): `blast_radius`,
	// `review_diff`, `test_impact` and `rename_impact` join the surface —
	// the answers grep cannot give, never another lookup pitched on token
	// savings — plus `safe_delete_check`, a genuinely new tool: 52,273
	// bytes measured at 24 tools (TestAgentSurfaceDiet's own pin moved to
	// 24 alongside it).
	// Re-pinned again for VB-19 (ADR-035 decision 4, docs/vision/
	// 29-verbs-not-lookups.md §3 wave E): every result schema for
	// find_callers/references/blast_radius/symbol_context/safe_delete_check/
	// rename_impact/test_impact/review_diff gained `resolution: {typed,
	// heuristic}` and `index_commit` — still 24 tools, a widened result
	// shape, not a new one: 54,872 bytes measured.
	// Re-pinned once more for wave G (plan VB-26): `localize` joins the
	// surface — where a bug lives before anyone knows the file, R10's own
	// case (LocAgent/SHERLOC) — 56,655 bytes measured at 25 tools
	// (TestAgentSurfaceDiet's own pin moved to 25 alongside it).
	// Re-pinned once more integrating wave N5 with the concurrently merged
	// waves N8/N10/N11 (O54 addendum, O60's `layer_coverage` unknown-status
	// shape on `obligations`, and other sibling-wave description growth) —
	// still 25 tools, no new tool: 59,094 bytes measured, then 60,918 by the
	// time fl-wave-10 (below) started.
	//
	// Re-pinned 2026-09-18 (fl-wave-10, docs/vision/05-surfaces.md's own
	// cost complaint): a description DIET, not a tool change — still 25
	// tools (TestAgentSurfaceDiet's own pin, unmoved). Every agent-surface
	// tool description was cut to the grammar `docs/mcp-tools.md` already
	// documented in full: one opening sentence, at most two sentences on
	// the result shape a caller must know (budget/elision, tiers,
	// checked:false), the "Reach for it when …" routing sentence, and the
	// "Required: …" line — plan-row ids (SB-xx/VB-xx/RC-xx/O-numbers), ADR
	// cross-references and migration notes moved to docs/mcp-tools.md
	// (nothing lost, address only). Five tools (find_symbol/find_callers/
	// symbol_context/verify_plan/obligations) keep a short "this result IS
	// the answer … a follow-up call only …" stop sentence as one of their
	// two result-shape sentences — SB-4's own measured H1 finding
	// (agent_surface_stop_test.go's TestAgentSurfaceDescriptionsStateWhenToStop),
	// not design history, so the diet trims it, never cuts it. Argument
	// schemas (names/types/enums/required) are BYTE-IDENTICAL — pinned by
	// internal/mcp's TestAgentSurfaceArgSchemaUnchanged against a pre-diet
	// golden; only prose shrank. Measured 60,918 → 47,480 bytes: a 22% cut
	// of the SERVED total, and a 43% cut of the actually in-scope prose
	// (description + argument-description bytes: 31,273 → 17,787). The
	// remaining ~29.6 kB is a fixed floor this wave never touched —
	// generated `outputSchema` (18,626 B, RP-11's machine-checkable
	// contract) + `annotations` (2,227 B, RP-10) + argument-schema
	// STRUCTURE (~6,700 B: property names/types/enums/required, protected
	// by the golden above) — so docs/vision/05-surfaces.md's aspirational
	// ≤36,000 B target is not reachable by a description diet alone without
	// cutting one of those three, which this brief put out of scope.
	// TestAgentSurfaceDescriptionsWithinCaps pins the per-tool cap (700 B;
	// 1000 B for review_diff/obligations/references/search) and the
	// per-argument cap (200 B) going forward.
	agentSurfaceSchemaByteCeiling = 49480 // 47,480 measured 2026-09-18 (fl-wave-10) + 2,000
)

// TestNarrowProfileSchemaBytes is BA-7: the narrow/agent surface profile
// carries only its own tools' schemas, with the byte count asserted so an
// accidental widening fails the build.
func TestNarrowProfileSchemaBytes(t *testing.T) {
	ts := httptest.NewServer(newTestHandler(t))
	defer ts.Close()
	hc := &http.Client{}

	tools, schemaBytes := surfaceSchemaTax(hc, ts.URL)
	if tools != 25 {
		t.Errorf("agent surface tools = %d, want 25 (TestAgentSurfaceDiet's own pin: 19 + the four ADR-035 verbs + safe_delete_check + localize)", tools)
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
	html := renderCostPanel(cp, "")
	if !strings.Contains(html, "query_symbols") || !strings.Contains(html, "find_symbol") {
		t.Error("rendered panel must list both tools")
	}
	if !strings.Contains(html, "—") {
		t.Error("rendered panel must show \"—\" (not 0%) for find_symbol's non-truncating elision rate")
	}
}

func impactMarkupBetween(t *testing.T, body, start, end string) string {
	t.Helper()
	from := strings.Index(body, start)
	if from < 0 {
		t.Fatalf("impact markup missing section start %q", start)
	}
	to := strings.Index(body[from+len(start):], end)
	if to < 0 {
		t.Fatalf("impact markup missing section end %q", end)
	}
	return body[from : from+len(start)+to]
}

func impactStatMarkup(t *testing.T, section, label string) string {
	t.Helper()
	needle := ">" + label + "</dt>"
	from := strings.Index(section, needle)
	if from < 0 {
		t.Fatalf("impact section missing stat %q", label)
	}
	to := strings.Index(section[from:], "</dd>")
	if to < 0 {
		t.Fatalf("impact stat %q has no closing description", label)
	}
	return section[from : from+to+len("</dd>")]
}

func TestObservatoryImpactCallLogUnavailableSuppressesDerivedCosts(t *testing.T) {
	const sourceReason = `<call log> & unavailable`
	ratio := 987
	m := observatoryModel{
		CallLogNote: sourceReason,
		Misses:      []obsMissRow{{Tool: "poisoned_outcome", Calls: 99, Hits: 98, Misses: 1}},
		Cost: costPanel{
			Tools:                      []costToolRow{{Tool: "poisoned_cost", Calls: 9, TotalBytes: 8000, EstTokens: 2000, TruncatingCalls: 2, ElidedCalls: 1}},
			TotalBytes:                 8000,
			TotalEstTokens:             2000,
			SchemaTaxBytes:             1234,
			SchemaTaxTools:             7,
			SchemaTaxTrendNote:         "Independent schema trend unavailable.",
			RatioTokensPerDiscoveryX10: &ratio,
			RatioNote:                  "poisoned numeric ratio",
		},
	}

	body := renderObsImpact(m)
	friction := impactMarkupBetween(t, body, "Q3 · where does the agent still flounder?", "cost · what fathom put into your context")
	cost := impactMarkupBetween(t, body, "cost · what fathom put into your context", "Q4 · is the knowledge compounding?")
	if strings.Contains(friction, "<table") || strings.Contains(friction, "poisoned_outcome") {
		t.Error("unavailable call log rendered poisoned per-tool outcomes")
	}
	if strings.Contains(cost, "<table") || strings.Contains(cost, "poisoned_cost") || strings.Contains(cost, "poisoned numeric ratio") {
		t.Error("unavailable call log rendered poisoned response-cost data")
	}
	escapedReason := "&lt;call log&gt; &amp; unavailable"
	if !strings.Contains(friction, escapedReason) || strings.Contains(body, sourceReason) {
		t.Error("per-tool outcome absence did not safely retain the source reason")
	}
	for _, label := range []string{"total response cost (est.)", "tokens per discovery action"} {
		stat := impactStatMarkup(t, cost, label)
		if !strings.Contains(stat, ">—</span>") || !strings.Contains(stat, escapedReason) {
			t.Errorf("%s must be absent with the escaped call-log reason: %s", label, stat)
		}
	}
	schemaTax := impactStatMarkup(t, cost, "agent surface schema tax")
	if !strings.Contains(schemaTax, ">1,234 B</span>") || !strings.Contains(schemaTax, "7 tools carried by every connected session") {
		t.Errorf("independent schema tax was lost: %s", schemaTax)
	}
	schemaTrend := impactStatMarkup(t, cost, "schema tax trend")
	if !strings.Contains(schemaTrend, ">—</span>") || !strings.Contains(schemaTrend, "Independent schema trend unavailable.") {
		t.Errorf("independent schema trend absence was lost: %s", schemaTrend)
	}
}

func TestObservatoryImpactCallLogUnavailableWithEmptyRowsUsesSourceReason(t *testing.T) {
	const sourceReason = `<call log> & unavailable`
	m := observatoryModel{
		CallLogNote: sourceReason,
		Cost: costPanel{
			SchemaTaxBytes:     2345,
			SchemaTaxTools:     8,
			SchemaTaxTrendNote: "Empty-source independent trend.",
		},
	}
	body := renderObsImpact(m)
	friction := impactMarkupBetween(t, body, "Q3 · where does the agent still flounder?", "cost · what fathom put into your context")
	cost := impactMarkupBetween(t, body, "cost · what fathom put into your context", "Q4 · is the knowledge compounding?")
	escapedReason := "&lt;call log&gt; &amp; unavailable"
	if !strings.Contains(friction, "per-tool outcomes") || !strings.Contains(friction, escapedReason) {
		t.Errorf("empty per-tool outcomes did not name the failed source: %s", friction)
	}
	if !strings.Contains(cost, "response cost by tool") || !strings.Contains(cost, escapedReason) {
		t.Errorf("empty response-cost rows did not name the failed source: %s", cost)
	}
	for _, label := range []string{"total response cost (est.)", "tokens per discovery action"} {
		stat := impactStatMarkup(t, cost, label)
		if !strings.Contains(stat, ">—</span>") || !strings.Contains(stat, escapedReason) || strings.Contains(stat, ">~0 tok</span>") {
			t.Errorf("%s must be unavailable, never measured zero: %s", label, stat)
		}
	}
	if strings.Contains(friction, "no per-tool outcomes recorded in the current log") || strings.Contains(cost, "no accounted response-cost rows in the current log") {
		t.Error("failed source was mislabeled as a readable empty log")
	}
	schemaTax := impactStatMarkup(t, cost, "agent surface schema tax")
	if !strings.Contains(schemaTax, ">2,345 B</span>") || !strings.Contains(schemaTax, "8 tools carried by every connected session") {
		t.Errorf("known schema tax was lost with empty call-log rows: %s", schemaTax)
	}
	schemaTrend := impactStatMarkup(t, cost, "schema tax trend")
	if !strings.Contains(schemaTrend, "Empty-source independent trend.") {
		t.Errorf("independent schema trend note was lost: %s", schemaTrend)
	}
}

func TestObservatoryImpactCallLogReadableCostStates(t *testing.T) {
	t.Run("measured zero", func(t *testing.T) {
		m := observatoryModel{Cost: costPanel{
			SchemaTaxBytes:     1200,
			SchemaTaxTools:     6,
			SchemaTaxTrendNote: "Independent trend is not measured.",
			RatioNote:          "no accounted response tokens",
		}}
		body := renderObsImpact(m)
		friction := impactMarkupBetween(t, body, "Q3 · where does the agent still flounder?", "cost · what fathom put into your context")
		cost := impactMarkupBetween(t, body, "cost · what fathom put into your context", "Q4 · is the knowledge compounding?")
		total := impactStatMarkup(t, cost, "total response cost (est.)")
		if !strings.Contains(total, ">~0 tok</span>") || !strings.Contains(total, "0 B measured across accounted calls") {
			t.Errorf("readable measured zero changed: %s", total)
		}
		ratio := impactStatMarkup(t, cost, "tokens per discovery action")
		if !strings.Contains(ratio, ">—</span>") || !strings.Contains(ratio, "no accounted response tokens") {
			t.Errorf("readable zero ratio absence changed: %s", ratio)
		}
		if !strings.Contains(cost, "no accounted response-cost rows in the current log") {
			t.Error("readable empty cost rows lost their exact explanation")
		}
		if !strings.Contains(friction, "no per-tool outcomes recorded in the current log") {
			t.Error("readable empty outcomes lost their exact explanation")
		}
	})

	t.Run("positive cost and ratio", func(t *testing.T) {
		ratio := 25
		m := observatoryModel{
			Misses: []obsMissRow{{Tool: "find_symbol", Calls: 2, Hits: 1, Misses: 1}},
			Cost: costPanel{
				Tools:                      []costToolRow{{Tool: "find_symbol", Calls: 2, TotalBytes: 400, EstTokens: 100}},
				TotalBytes:                 400,
				TotalEstTokens:             100,
				SchemaTaxBytes:             1200,
				SchemaTaxTools:             6,
				SchemaTaxTrendNote:         "Independent trend is not measured.",
				RatioTokensPerDiscoveryX10: &ratio,
				RatioNote:                  "OBSERVATIONAL positive fixture",
			},
		}
		body := renderObsImpact(m)
		friction := impactMarkupBetween(t, body, "Q3 · where does the agent still flounder?", "cost · what fathom put into your context")
		cost := impactMarkupBetween(t, body, "cost · what fathom put into your context", "Q4 · is the knowledge compounding?")
		if !strings.Contains(friction, "<table") || !strings.Contains(friction, "find_symbol") || !strings.Contains(cost, "response cost by tool") || !strings.Contains(cost, "find_symbol") {
			t.Fatal("readable positive call-log tables changed")
		}
		total := impactStatMarkup(t, cost, "total response cost (est.)")
		if !strings.Contains(total, ">~100 tok</span>") || !strings.Contains(total, "400 B measured across accounted calls") {
			t.Errorf("readable positive total changed: %s", total)
		}
		ratioStat := impactStatMarkup(t, cost, "tokens per discovery action (est.)")
		if !strings.Contains(ratioStat, ">2.5</span>") || !strings.Contains(ratioStat, "OBSERVATIONAL positive fixture") {
			t.Errorf("readable positive ratio changed: %s", ratioStat)
		}
	})
}
