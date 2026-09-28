package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── BA-7: the narrow (agent) surface profile's schema byte ceiling ────────
//
// This band tracked POST /rpc/agent from BA-7 (11,839 B / 16 tools) through
// O101 (18,008 B / 25 tools, docs/design/budgeted-answers.md §4,
// docs/plans/reach-parity.md RP-9..RP-11, docs/vision/05-surfaces.md's cost
// complaint) — the full re-pin history, tool by tool and byte by byte, is
// in git blame on this comment, not repeated here. Surface reckoning F-86
// (design §1.1, plan SR-33) ends that history: /rpc/agent is no longer the
// 25-tool allowlist this band measured. It now serves the eight-tool floor
// below, TestAgentSurfaceBudget/TestAgentSurfaceArgSchemaUnchanged
// (internal/mcp) keep gating the WIDER companion/extended surface's own
// per-tool prose and argument-schema structure, and nothing here still
// asserts against the retired 25-tool number.
const (
	// The eight-tool floor: blast_radius, find_symbol, obligations,
	// read_source, review_diff, search, symbol_context, verify_plan.
	agentDefaultSurfaceToolCount = 8
	// 7,715-7,725 measured 2026-09-27 (SR-33), well inside the design doc's
	// own ≤9,000 B target (docs/design/surface-reckoning.md §1); + headroom.
	// Re-pinned 2026-09-28 (roadmap wave R4b): 8,144 B at the wave's base,
	// 8,329 B after it — search's `pkg` argument (KR-12, +174 B) and
	// review_diff's `as_of` (GL-17, +11 B). Still under the ≤9,000 B target.
	agentDefaultSurfaceSchemaByteFloor   = 7000
	agentDefaultSurfaceSchemaByteCeiling = 8600
)

// TestNarrowProfileSchemaBytes is BA-7: the narrow/agent surface profile
// carries only its own tools' schemas, with the byte count asserted so an
// accidental widening fails the build.
func TestNarrowProfileSchemaBytes(t *testing.T) {
	ts := httptest.NewServer(newTestHandler(t))
	defer ts.Close()
	hc := &http.Client{}

	tools, schemaBytes := surfaceSchemaTax(hc, ts.URL)
	if tools != agentDefaultSurfaceToolCount {
		t.Errorf("agent surface tools = %d, want %d (surface reckoning F-86/SR-33: TestDefaultSurfaceIsEight's own pin)", tools, agentDefaultSurfaceToolCount)
	}
	if schemaBytes < agentDefaultSurfaceSchemaByteFloor || schemaBytes > agentDefaultSurfaceSchemaByteCeiling {
		t.Errorf("agent surface schema bytes = %d, want between %d and %d — "+
			"either the surface widened (add a tool only with a measured case) or shrank unexpectedly",
			schemaBytes, agentDefaultSurfaceSchemaByteFloor, agentDefaultSurfaceSchemaByteCeiling)
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
