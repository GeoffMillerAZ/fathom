package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
