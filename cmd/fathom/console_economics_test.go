package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// appendLedgerRows appends session-ledger rows to the fixture's ledger and
// drops the console's cached reads, so the next page sees them.
func appendLedgerRows(t *testing.T, f *consoleFixture, rows ...string) {
	t.Helper()
	fh, err := os.OpenFile(f.ledger, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if _, err := fh.WriteString(r + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	fh.Close()
	resetObsCache()
}

// buildDirect runs page id's Build for target against the fixture, with
// the route's path values set, and returns its rendered body — the page's
// builder called directly, as the router calls it.
func buildDirect(t *testing.T, f *consoleFixture, id, target string, pathValues ...string) (consoleOut, string) {
	t.Helper()
	p, ok := defaultConsoleRegistry().page(id)
	if !ok {
		t.Fatalf("page %s is not registered", id)
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for i := 0; i+1 < len(pathValues); i += 2 {
		req.SetPathValue(pathValues[i], pathValues[i+1])
	}
	cr := &consoleRequest{w: httptest.NewRecorder(), r: req, deps: f.deps, page: p, now: f.now}
	var out consoleOut
	var err error
	switch id {
	case "session":
		out, err = buildSessionPage(cr)
	case "sessions":
		out, err = buildSessionsPage(cr)
	case "plan":
		out, err = buildPlanPage(cr)
	case "progress":
		out, err = buildProgressPage(cr)
	default:
		t.Fatalf("buildDirect: no builder for %s", id)
	}
	if err != nil {
		t.Fatalf("build %s: %v", target, err)
	}
	if out.Body == nil {
		return out, ""
	}
	return out, out.Body()
}

// proofServiceVerifyLedger is a schema-3 verify ledger for proof-service's
// settlement plan: S-1 moves to claimed-proven at `at`.
func proofServiceVerifyLedger(t *testing.T, at string) {
	t.Helper()
	line := func(when, rows string) string {
		return fmt.Sprintf(`{"schema_version":3,"at":%q,"repo_root":"/r/proof-service","service":"proof-service","plan":"docs/plans/settlement.md","commit":"","origin":"cli","total":2,"claimed":1,"verified":1,"digest":%q,"rows":[%s]}`, when, when, rows)
	}
	body := strings.Join([]string{
		line("2026-09-15T08:00:00Z", `{"id":"S-1","state":"open","weight":1},{"id":"S-2","state":"open","weight":1}`),
		line(at, `{"id":"S-1","state":"claimed-proven","weight":1},{"id":"S-2","state":"open","weight":1}`),
	}, "\n") + "\n"
	if err := os.WriteFile(os.Getenv("FATHOM_VERIFY_LEDGER"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// PE-18w: the transcript fold counts tool calls by name (the top 20 names
// plus `other` on the ledger row); the session page renders that row's
// tool-call mix and cache hit rate, and the plan page renders the same two
// figures summed over the sessions the join attributes to the plan.
func TestSessionPageShowsToolMixAndCacheHit(t *testing.T) {
	// The fold: counts by name, bounded to the top 20 plus other, and the
	// counts sum to the session's tool_calls.
	var lines []string
	for i := 0; i < 25; i++ {
		n := 1
		if i < 3 {
			n = 10 - i
		}
		for j := 0; j < n; j++ {
			lines = append(lines, fmt.Sprintf(`{"type":"assistant","timestamp":"2026-09-20T10:%02d:%02dZ","message":{"content":[{"type":"tool_use","id":"u%d-%d","name":"Tool%02d"}]}}`, i, j, i, j, i))
		}
	}
	ts := transcriptSummaryFrom(writeTranscript(t, lines...))
	if len(ts.ToolCallsByName) != 21 || ts.ToolCallsByName["Tool00"] != 10 || ts.ToolCallsByName["other"] != 5 {
		t.Fatalf("by-name counts = %v, want the top 20 names plus other (5 calls)", ts.ToolCallsByName)
	}
	sum := 0
	for _, n := range ts.ToolCallsByName {
		sum += n
	}
	if sum != ts.ToolCallsTotal {
		t.Errorf("by-name counts sum to %d, the fold counted %d", sum, ts.ToolCallsTotal)
	}
	row := ledgerRowMap(t, buildSessionLedgerEntry("s", "r", ts, nil, time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)))
	if byName, ok := row["tool_calls_by_name"].(map[string]any); !ok || byName["Tool01"] != float64(9) {
		t.Errorf("the ledger row carries tool_calls_by_name: %v", row["tool_calls_by_name"])
	}

	f := newConsoleFixture(t)
	appendLedgerRows(t, f,
		`{"schema_version":2,"session_id":"sess-mix-1","arm":"on","repo":"proof-service","at":"2026-09-20T12:00:00Z","duration_ms":10800000,"tokens_in":80,"tokens_out":40,"tokens_cache_read":300,"tokens_cache_create":20,"tokens_note":"","tool_calls":10,"tool_calls_by_name":{"Read":6,"Edit":2,"mcp__fathom__search":2}}`,
		`{"schema_version":2,"session_id":"sess-mix-2","arm":"on","repo":"proof-service","at":"2026-09-21T12:00:00Z","tokens_note":"unavailable: x"}`,
	)
	_, direct := buildDirect(t, f, "session", "/sessions/sess-mix-1", "id", "sess-mix-1")
	mustContain(t, "buildSessionPage", direct, `id="tool-mix"`, "mcp__fathom__search", "60%", "75%")
	code, _, page := f.get(t, "/sessions/sess-mix-1")
	if code != http.StatusOK {
		t.Fatalf("GET /sessions/sess-mix-1 = %d", code)
	}
	mustContain(t, "session page", page, `id="tool-mix"`, "Tool-call mix and cache hit rate", "mcp__fathom__search", "60%", "75%", "10 tool calls")
	assertRoleUtilitiesOnly(t, page)
	env := f.pageJSON(t, "/sessions/sess-mix-1")
	var rp struct {
		Ledger *sessionLedgerEntry `json:"ledger"`
	}
	if err := json.Unmarshal(env.Model, &rp); err != nil || rp.Ledger == nil || rp.Ledger.ToolCallsByName["Read"] != 6 {
		t.Fatalf("the session JSON carries the row's per-name counts: %v %s", err, env.Model)
	}

	// A row without per-name counts names the absence, never 0%.
	_, _, bare := f.get(t, "/sessions/sess-mix-2")
	mustContain(t, "bare session page", bare, `id="tool-mix"`, "no per-tool counts on this session")
	if strings.Contains(bare, ">0%<") {
		t.Error("an unrecorded mix must not render as 0%")
	}

	// The plan page sums the sessions the join attributes to the plan.
	proofServiceVerifyLedger(t, "2026-09-20T11:00:00Z")
	resetObsCache()
	_, directPlan := buildDirect(t, f, "plan", "/plans/proof-service/settlement", "svc", "proof-service", "plan", "settlement")
	mustContain(t, "buildPlanPage", directPlan, `id="plan-tool-mix"`, "mcp__fathom__search", "75%")
	code, _, plan := f.get(t, "/plans/proof-service/settlement")
	if code != http.StatusOK {
		t.Fatalf("plan page = %d", code)
	}
	mustContain(t, "plan page", plan, `id="plan-tool-mix"`, "mcp__fathom__search", "60%", "75%", "1 session")
	assertRoleUtilitiesOnly(t, plan)
}

// PE-19w: /sessions serves the live session feed as a read-only panel fed
// from the session ledger joined to the proof history, following the
// `sessions` live source over the SSE channel; no feed cell is a session id
// or any per-person figure (I58).
func TestSessionsPageServesLiveFeed(t *testing.T) {
	f := newConsoleFixture(t)
	appendLedgerRows(t, f,
		`{"schema_version":2,"session_id":"sess-feed-aaaa1111","arm":"on","repo":"proof-service","at":"2026-09-20T12:00:00Z","duration_ms":10800000,"tokens_note":"","harness":"claude-code","model":"claude-opus-4-5"}`,
		`{"schema_version":2,"session_id":"sess-feed-bbbb2222","arm":"on","repo":"proof-service","at":"2026-09-22T12:00:00Z","duration_ms":600000,"tokens_note":"","harness":"codex","model":"gpt-5-codex"}`,
	)
	proofServiceVerifyLedger(t, "2026-09-20T11:00:00Z")
	resetObsCache()

	_, body := buildDirect(t, f, "sessions", "/sessions")
	start := strings.Index(body, `id="live-feed"`)
	end := strings.Index(body, `id="sessions-table"`)
	if start < 0 || end < start {
		t.Fatalf("/sessions lacks the live feed panel before the sessions table:\n%s", body)
	}
	feed := body[start:end]
	mustContain(t, "live feed", feed, `data-live-source="sessions"`, `data-table="session-feed"`, "Live session feed",
		"claude-code", "claude-opus-4-5", "codex", "gpt-5-codex", "proof-service", "2026-09-20T09:00:00Z – 2026-09-20T12:00:00Z")
	// The session the join attributed S-1 to shows 1; the other names none.
	if !strings.Contains(feed, "<td") || !regexp.MustCompile(`>\s*1\s*</td>`).MatchString(feed) {
		t.Errorf("the attributed session shows its proven row count:\n%s", feed)
	}
	for _, id := range []string{"sess-feed", "aaaa1111", "bbbb2222", "sess-fee"} {
		if strings.Contains(feed, id) {
			t.Errorf("the feed must never show a session id (I58), found %q", id)
		}
	}
	for _, bad := range []string{"<form", "<button", `method="post"`} {
		if strings.Contains(feed, bad) {
			t.Errorf("the feed is read-only, found %q", bad)
		}
	}

	// Over HTTP the page starts the live stream the panel follows.
	code, _, page := f.get(t, "/sessions")
	if code != http.StatusOK {
		t.Fatalf("GET /sessions = %d", code)
	}
	mustContain(t, "/sessions", page, `data-live-page data-live-url="/events"`, `id="live-feed"`)
	assertRoleUtilitiesOnly(t, page)
}

// PE-16w: /progress prices each joined session by its recorded model
// through the user's price table (internal/cost LoadPriceTable), and shows
// USD and tokens per proven row per plan and per language (the proven
// row's anchor file, as the index resolved it). A session whose model is
// not in the table is unpriced, never $0.00, and the PE-24 caveat is named.
func TestProgressPagePricesSessionsThroughPriceTable(t *testing.T) {
	f := newConsoleFixture(t)
	prices := filepath.Join(t.TempDir(), "prices.toml")
	mustWrite(t, prices, "[models.\"claude-opus-4-5\"]\ninput_per_mtok = 15.0\noutput_per_mtok = 75.0\ncache_read_per_mtok = 1.5\ncache_write_per_mtok = 18.75\n")
	t.Setenv("FATHOM_PRICE_TABLE", prices)
	appendLedgerRows(t, f,
		`{"schema_version":2,"session_id":"sess-priced","arm":"on","repo":"proof-service","at":"2026-09-20T12:00:00Z","duration_ms":10800000,"tokens_in":1000000,"tokens_out":100000,"tokens_cache_read":0,"tokens_cache_create":0,"tokens_note":"","harness":"claude-code","model":"claude-opus-4-5"}`,
		`{"schema_version":2,"session_id":"sess-unpriced","arm":"on","repo":"proof-service","at":"2026-09-22T12:00:00Z","duration_ms":10800000,"tokens_in":500,"tokens_out":50,"tokens_note":"","harness":"codex","model":"gpt-not-in-table"}`,
	)
	line := func(when, rows string) string {
		return fmt.Sprintf(`{"schema_version":3,"at":%q,"repo_root":"/r/proof-service","service":"proof-service","plan":"docs/plans/settlement.md","commit":"","origin":"cli","total":2,"claimed":1,"verified":1,"digest":%q,"rows":[%s]}`, when, when, rows)
	}
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), strings.Join([]string{
		line("2026-09-15T08:00:00Z", `{"id":"S-1","state":"open","weight":1},{"id":"S-2","state":"open","weight":1}`),
		line("2026-09-20T11:00:00Z", `{"id":"S-1","state":"claimed-proven","weight":1},{"id":"S-2","state":"open","weight":1}`),
		line("2026-09-22T11:00:00Z", `{"id":"S-1","state":"claimed-proven","weight":1},{"id":"S-2","state":"claimed-proven","weight":1}`),
	}, "\n")+"\n")
	resetObsCache()
	// The plan's verdicts (each proven row's anchor file) come from the
	// plans cache; the plan page waits for them.
	if code, _, _ := f.get(t, "/plans/proof-service/settlement"); code != http.StatusOK {
		t.Fatalf("plan page = %d", code)
	}

	out, body := buildDirect(t, f, "progress", "/progress?svc=proof-service")
	pm, ok := out.Model.(progressPageModel)
	if !ok {
		t.Fatalf("progress model is %T", out.Model)
	}
	e := pm.Economics
	if e.PriceTable != prices || e.JoinedRows != 2 {
		t.Fatalf("economics = %+v", e)
	}
	var plan *progressCostRow
	for i := range e.PerPlan {
		if e.PerPlan[i].Plan == "docs/plans/settlement.md" {
			plan = &e.PerPlan[i]
		}
	}
	if plan == nil || plan.Rows != 2 || plan.Priced != 1 {
		t.Fatalf("per-plan cost = %+v", e.PerPlan)
	}
	// $15/MTok in × 1M + $75/MTok out × 0.1M = $22.50 over the one priced row.
	if !plan.USDPerRow.Known() || plan.USDPerRow.V != 22.5 {
		t.Errorf("USD per proven row = %+v, want 22.50 over the priced row", plan.USDPerRow)
	}
	if !plan.TokensPerRow.Known() || plan.TokensPerRow.V != (1100000+550)/2.0 {
		t.Errorf("tokens per proven row = %+v, want the mean of both rows' in+out", plan.TokensPerRow)
	}
	langs := map[string]progressCostRow{}
	for _, r := range e.PerLanguage {
		langs[r.Language] = r
	}
	if r, ok := langs["go"]; !ok || r.Rows != 2 {
		t.Errorf("per-language cost = %+v, want both rows under go (settle.go)", e.PerLanguage)
	}
	if !strings.Contains(strings.Join(e.Unpriced, " "), "gpt-not-in-table") {
		t.Errorf("the unpriced model is named: %v", e.Unpriced)
	}
	mustContain(t, "progress", body, `id="cost-per-row"`, "$22.50", "gpt-not-in-table", "unpriced", "PE-24", "docs/plans/settlement.md")
	if strings.Contains(body, "$0.00") {
		t.Error("an unpriced session must never render as $0.00")
	}

	// Every session unpriced: the per-row USD is absent, named, not $0.00.
	mustWrite(t, prices, "")
	resetObsCache()
	out, body = buildDirect(t, f, "progress", "/progress?svc=proof-service")
	for _, r := range out.Model.(progressPageModel).Economics.PerPlan {
		if r.USDPerRow.Known() {
			t.Errorf("an empty price table priced %s: %+v", r.Plan, r.USDPerRow)
		}
	}
	if strings.Contains(body, "$0.00") {
		t.Error("an empty price table must never render $0.00")
	}
}
