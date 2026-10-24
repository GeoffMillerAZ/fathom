package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/internal/verify"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// progressHistoryFixture is a schema-3 proof history: A-2 proven then
// regressed, A-1 proven on the second reading.
func progressHistoryFixture() []byte {
	line := func(at, commit, rows string) string {
		return fmt.Sprintf(`{"schema_version":3,"at":%q,"repo_root":"/r/payment-service","service":"payment-service","plan":"docs/plans/p.md","commit":%q,"origin":"cli","total":3,"claimed":2,"verified":1,"digest":%q,"rows":[%s]}`, at, commit, commit, rows)
	}
	return []byte(strings.Join([]string{
		line("2026-09-20T10:00:00Z", "c1", `{"id":"A-1","state":"open","weight":1},{"id":"A-2","state":"claimed-proven","weight":2},{"id":"A-3","state":"dispositioned","weight":1}`),
		line("2026-09-24T10:00:00Z", "c2", `{"id":"A-1","state":"claimed-proven","weight":1},{"id":"A-2","state":"claimed-unproven","criterion":"test_reaches: no test reaches Charge","weight":2},{"id":"A-3","state":"dispositioned","weight":1}`),
	}, "\n") + "\n")
}

// PE-4: the weighted table renders every level of the rollup, each with its
// proven weight over total weight.
func TestConsoleWeightedTable(t *testing.T) {
	proj := weightedProject("payment-service", []string{"/r/payment-service/docs/plans/p.md"}, [][]planRow{{
		{Task: taskW("A-1", 3, true), Verdict: passVerdict("A-1"), Section: "Wave 1"},
		{Task: taskW("A-2", 1, false), Section: "Wave 2"},
	}})
	html := renderWeightedTable(weightedFleet([]weightedScope{proj}))
	for _, want := range []string{"progress-weighted", "fleet", "payment-service", "docs/plans/p.md", "Wave 1", "Wave 2", "3/4 (75%)", "3/3 (100%)", "0/1 (0%)"} {
		if !strings.Contains(html, want) {
			t.Errorf("weighted table missing %q", want)
		}
	}
	if !strings.Contains(renderWeightedTable(weightedFleet(nil)), "data-weighted-empty") {
		t.Errorf("an empty selection must say so, never draw 0/0")
	}
}

// PE-5: the derived kanban has the four columns, is read-only (no card is
// draggable), and a claimed-unproven card names its failing criterion and
// links to the row.
func TestConsoleDerivedKanbanReadOnly(t *testing.T) {
	h := console.ParseProofHistory(progressHistoryFixture(), false)
	cols := console.DerivedBoard(h, console.HistoryFilter{})
	if len(cols) != 4 || cols[0].Label != "Open" || cols[1].Label != "Claimed, unproven" || cols[2].Label != "Proven" || cols[3].Label != "Refused or deferred" {
		t.Fatalf("columns wrong: %+v", cols)
	}
	if len(cols[1].Cards) != 1 || cols[1].Cards[0].ID != "A-2" || len(cols[2].Cards) != 1 || len(cols[3].Cards) != 1 {
		t.Fatalf("cards in the wrong lanes: %+v", cols)
	}
	out := renderDerivedKanban(cols)
	if strings.Contains(out, `draggable="true"`) || strings.Contains(out, "cursor-grab") {
		t.Fatalf("the derived board must be read-only")
	}
	for _, want := range []string{"data-read-only", "test_reaches: no test reaches Charge", "failing criterion", "/plans/payment-service/p/rows/A-2"} {
		if !strings.Contains(out, want) {
			t.Errorf("board missing %q", want)
		}
	}
	for _, verb := range []string{"<form", "method=\"post\"", "hx-post"} {
		if strings.Contains(out, verb) {
			t.Errorf("a read-only board carries no write affordance, found %q", verb)
		}
	}
}

// PE-7: the frontier lists `plans next --eligible-only` rows, each linked
// to its row page; none is a named empty state.
func TestConsoleFrontier(t *testing.T) {
	rows := []planRow{
		{PlanFile: "docs/plans/p.md", Task: taskNeeds("B-1", ""), Section: "W"},
		{PlanFile: "docs/plans/p.md", Task: taskNeeds("B-2", "B-1"), Section: "W"},
	}
	var got []progressFrontierRow
	for _, fr := range frontierForPlan(rows, nil, time.Now()) {
		if fr.Eligible {
			got = append(got, progressFrontierRow{Service: "payment-service", frontierRow: fr})
		}
	}
	if len(got) != 1 || got[0].ID != "B-1" {
		t.Fatalf("only B-1 is eligible (B-2 needs it): %+v", got)
	}
	out := renderFrontier(got)
	for _, want := range []string{"progress-frontier", "B-1", "/plans/payment-service/p/rows/B-1"} {
		if !strings.Contains(out, want) {
			t.Errorf("frontier missing %q", want)
		}
	}
	if strings.Contains(out, "B-2") {
		t.Errorf("a blocked row is not on the frontier")
	}
	if !strings.Contains(renderFrontier(nil), "data-frontier-empty") {
		t.Errorf("an empty frontier must say so")
	}
}

// PE-11: the heatmap draws newly proven weight by plan and week, and the
// streak and "proved today" are counts with their denominators.
func TestConsoleProofHeatmap(t *testing.T) {
	h := console.ParseProofHistory(progressHistoryFixture(), false)
	now := time.Date(2026, 9, 24, 18, 0, 0, 0, time.UTC)
	hm := console.ProofHeatmap(h, console.HistoryFilter{}, now)
	if len(hm.Cells) != 1 || hm.Cells[0].Week != "2026-09-21" || hm.Cells[0].Weight != 1 {
		t.Fatalf("heatmap cells wrong (A-1 proven in the week of 09-21; A-2 was proven at the history's start): %+v", hm.Cells)
	}
	if hm.ProvedToday.V != 1 || hm.Streak.V != 1 {
		t.Fatalf("today and streak wrong: %+v %+v", hm.ProvedToday, hm.Streak)
	}
	out := renderProofHeatmap(hm)
	for _, want := range []string{"data-proof-heatmap", "<svg", "day streak", "proved today", "09-21"} {
		if !strings.Contains(out, want) {
			t.Errorf("heatmap missing %q", want)
		}
	}
	empty := console.ProofHeatmap(console.History{}, console.HistoryFilter{}, now)
	if !strings.Contains(renderProofHeatmap(empty), "data-heatmap-empty") || empty.Streak.State != console.StateAbsent {
		t.Fatalf("no history is absent, never a zero streak: %+v", empty.Streak)
	}
}

// The page serves every view, filters included, over a real ledger.
func TestConsoleProgressPage(t *testing.T) {
	f := newConsoleFixture(t)
	// PE-24: the page reads only the registered root's readings.
	ledger := strings.ReplaceAll(string(progressHistoryFixture()), `"/r/payment-service"`, strconv.Quote(fixtureServiceRoot(t, f, "payment-service")))
	if err := os.WriteFile(os.Getenv("FATHOM_VERIFY_LEDGER"), []byte(ledger), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, body := f.get(t, "/progress?svc=payment-service&from=2026-09-01")
	if code != http.StatusOK {
		t.Fatalf("GET /progress = %d: %s", code, body)
	}
	for _, id := range []string{`id="goals"`, `id="weighted"`, `id="board"`, `id="frontier"`, `id="burnup"`, `id="regressions"`, `id="time-to-proof"`, `id="honesty"`, `id="heatmap"`, "data-progress-filters"} {
		if !strings.Contains(body, id) {
			t.Errorf("/progress missing %s", id)
		}
	}
	env := f.pageJSON(t, "/progress")
	if env.Page != "progress" || len(env.Model) == 0 {
		t.Fatalf("progress JSON envelope wrong: %+v", env)
	}
	var model struct {
		Regressions []console.Regression `json:"regressions"`
	}
	if err := json.Unmarshal(env.Model, &model); err != nil || len(model.Regressions) != 1 || model.Regressions[0].ChangedCommit != "c2" {
		t.Fatalf("the model carries the regression with its commit: %v %s", err, env.Model)
	}
}

func TestConsoleProgressInvariantRegressionsFromStore(t *testing.T) {
	dir := hermeticObsEnv(t)
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(dir, "verify-ledger.jsonl"))
	const service = "regression-service"
	st := store.NewMemoryStore()
	c := client.New(nil, st)
	t.Cleanup(func() { c.Close() })
	ctx := t.Context()
	meta, err := json.Marshal(certify.Meta{
		TargetKind: "invariant", TargetID: "I1", Test: "TestInvariant", TestSymbolKey: "k:TestInvariant",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.PutClaims(ctx, []core.SemanticClaim{{
		ClaimID: "invariant-regression", ServiceID: service, Layer: certify.Layer, Kind: certify.Kind,
		Subject: "invariant:I1", Tier: core.TierPreScan, Status: core.ClaimFresh, Meta: meta,
	}}, nil); err != nil {
		t.Fatal(err)
	}
	var runs []store.TestRun
	for _, r := range []struct{ at, commit, status string }{
		{"2026-09-01T00:00:00Z", "c1", "pass"},
		{"2026-09-02T00:00:00Z", "c2", "fail"},
	} {
		runs = append(runs, store.TestRun{
			ServiceID: service, RunDigest: r.commit, Component: "p", Suite: "unit", StartedAt: r.at, Commit: r.commit, Verdict: r.status,
			Cases: []store.TestCase{{CaseID: "1", Name: "TestInvariant", Status: r.status, Attempts: 1, TestSymbolKey: "k:TestInvariant"}},
		})
	}
	if err := st.PutTestRuns(ctx, service, runs); err != nil {
		t.Fatal(err)
	}

	// The handler reads a generated service's real claim/run history. No
	// conservation-plane history exists to seed: conserve persists no readings.
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	resetObsCache()
	t.Cleanup(resetObsCache)
	obsCache.Lock()
	obsCache.model = observatoryModel{GeneratedAt: now.Format(time.RFC3339), Services: []obsServiceRow{{
		fleetServiceRow: fleetServiceRow{Service: service, Path: dir},
	}}}
	obsCache.at = now
	obsCache.Unlock()
	deps := newConsoleDeps(c, "", now)
	deps.now = func() time.Time { return now }
	page, ok := deps.reg.page("progress")
	if !ok {
		t.Fatal("progress page is not registered")
	}
	w := httptest.NewRecorder()
	deps.pageHandler(page).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/progress?svc="+service+"&format=json", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /progress = %d: %s", w.Code, w.Body.String())
	}
	var env struct {
		Model progressPageModel `json:"model"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Model.Regressions) != 1 {
		t.Fatalf("want one stored invariant regression, got %+v", env.Model.Regressions)
	}
	r := env.Model.Regressions[0]
	if r.Service != service || r.Kind != "invariant" || r.ID != "I1" || r.Test != "TestInvariant" || r.ProvenCommit != "c1" || r.ChangedCommit != "c2" || r.State != "broken" {
		t.Errorf("stored invariant regression lost identity or evidence: %+v", r)
	}
	if env.Model.RegressionsChecked.Invariants != 1 || len(env.Model.RegressionsChecked.Unread) != 0 {
		t.Errorf("stored invariant was not counted as read: %+v", env.Model.RegressionsChecked)
	}
}

func taskW(id string, weight int, claimed bool) verify.Task {
	return verify.Task{ID: id, Title: id, Weight: weight, Claimed: claimed}
}

func passVerdict(id string) verify.TaskVerdict {
	return verify.TaskVerdict{TaskID: id, Status: verify.Pass}
}

func taskNeeds(id, needs string) verify.Task {
	t := verify.Task{ID: id, Title: id, Weight: 1}
	if needs != "" {
		t.Needs = []string{needs}
	}
	return t
}

// WI-20/WI-21: certificate and invariant regressions render with their
// commit and an evidence link; an empty list says what was checked.
func TestConsoleRegressionsRenderCertificatesAndInvariants(t *testing.T) {
	regs := []console.Regression{
		{Service: "svc", Kind: "certificate", ID: "spec:S1", Test: "TestS", ProvenAt: "2026-09-01", ProvenCommit: "c1", ChangedAt: "2026-09-02", ChangedCommit: "c2", State: "broken"},
		{Service: "svc", Kind: "invariant", ID: "I1", Test: "TestI", ProvenAt: "2026-09-01", ProvenCommit: "c1", ChangedAt: "2026-09-02", ChangedCommit: "c3", State: "unproven"},
	}
	out := renderRegressions(regs, regressionsChecked{})
	for _, want := range []string{"spec:S1 (TestS)", "c2", "c3", "certificate", "invariant", `href="/certifications?svc=svc"`} {
		if !strings.Contains(out, want) {
			t.Errorf("regressions table missing %q", want)
		}
	}
	empty := renderRegressions(nil, regressionsChecked{PlanRows: 4, Certificates: 3, Invariants: 2, Unread: []string{"svc: boom"}})
	for _, want := range []string{"data-regressions-empty", "4 plan rows", "3 certificates", "2 invariant enforcements", "conservation planes", "svc: boom"} {
		if !strings.Contains(empty, want) {
			t.Errorf("empty regressions missing %q: %s", want, empty)
		}
	}
}
