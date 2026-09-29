package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/verify"
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
