package console

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/intent"
)

// ledgerLine is one schema-3 verify-ledger line for the fixtures.
func ledgerLine(at, commit, origin string, rows ...string) string {
	claimed, verified := 0, 0
	var arr []string
	for _, r := range rows {
		f := strings.SplitN(r, ":", 3) // id:state[:weight]
		w := "1"
		if len(f) == 3 {
			w = f[2]
		}
		if f[1] != "open" && f[1] != "dispositioned" {
			claimed++
		}
		if f[1] == "claimed-proven" {
			verified++
		}
		crit := ""
		if f[1] == "claimed-unproven" {
			crit = `,"criterion":"test_reaches: no test reaches X"`
		}
		arr = append(arr, fmt.Sprintf(`{"id":%q,"state":%q%s,"weight":%s}`, f[0], f[1], crit, w))
	}
	rowsJSON := ""
	if rows != nil {
		rowsJSON = `,"rows":[` + strings.Join(arr, ",") + `]`
	}
	return fmt.Sprintf(`{"schema_version":3,"at":%q,"repo_root":"/r/svc","service":"svc","plan":"docs/plans/p.md","commit":%q,"origin":%q,"total":%d,"claimed":%d,"verified":%d,"failing":%d,"digest":"d"%s}`,
		at, commit, origin, len(rows), claimed, verified, claimed-verified, rowsJSON)
}

func fixtureHistory() []byte {
	lines := []string{
		`{"schema_version":2,"at":"2026-09-01T00:00:00Z","repo_root":"/r/svc","service":"svc","plan":"docs/plans/p.md","total":3,"claimed":1,"verified":1}`,
		ledgerLine("2026-09-02T10:00:00Z", "c1", "cli", "A-1:open", "A-2:claimed-proven:2", "A-3:open"),
		ledgerLine("2026-09-02T11:00:00Z", "c1", "cli"), // digest unchanged: no array
		ledgerLine("2026-09-04T09:00:00Z", "c2", "stop", "A-1:claimed-proven", "A-2:claimed-proven:2", "A-3:claimed-unproven"),
		ledgerLine("2026-09-06T09:00:00Z", "c3", "cli", "A-1:claimed-proven", "A-2:claimed-unproven:2", "A-3:claimed-unproven", "A-4:open"),
		`not json`,
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

// PE-2: per row, its state over time, projected from the ledger; v2 rows
// count as plan-level readings only; the read is a bounded tail cached in
// a memo group keyed by the ledger's size.
func TestProofHistoryProjection(t *testing.T) {
	h := ParseProofHistory(fixtureHistory(), false)
	if h.CountOnly != 1 || h.RowReadings != 3 || h.Unchanged != 1 || h.Malformed != 1 {
		t.Fatalf("reading counts wrong: %+v", h)
	}
	if len(h.Readings) != 5 {
		t.Fatalf("every parseable reading is a plan-level reading, got %d", len(h.Readings))
	}
	if len(h.Rows) != 4 {
		t.Fatalf("want 4 row histories, got %d", len(h.Rows))
	}
	a2 := h.Rows[1]
	if a2.ID != "A-2" || a2.Plan != "docs/plans/p.md" || a2.Service != "svc" {
		t.Fatalf("row identity wrong: %+v", a2)
	}
	// Changes only: proven at c1, unproven at c3.
	if len(a2.Readings) != 2 || a2.Readings[0].State != HistProven || a2.Readings[1].State != HistUnproven || a2.Readings[1].Commit != "c3" || a2.Readings[0].Weight != 2 {
		t.Fatalf("A-2 history wrong: %+v", a2.Readings)
	}
	if st, ok := a2.StateAt("2026-09-05T00:00:00Z"); !ok || st.State != HistProven {
		t.Fatalf("A-2 state on 09-05 should be proven: %+v", st)
	}

	// The file reader: absent is absent, a present ledger is read and memoized.
	path := filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	if _, read := ProofHistory(context.Background(), path, 1<<20); read.State != StateAbsent {
		t.Fatalf("no ledger must read absent, got %+v", read)
	}
	if err := os.WriteFile(path, fixtureHistory(), 0o644); err != nil {
		t.Fatal(err)
	}
	got, read := ProofHistory(context.Background(), path, 1<<20)
	if read.State != StatePresent || len(got.Rows) != 4 {
		t.Fatalf("present ledger not read: %+v %+v", read, got)
	}
	// A tail shorter than the file is named as truncated.
	short, _ := ProofHistory(context.Background(), path, 600)
	if !short.Truncated {
		t.Fatalf("a bounded tail that misses the start must say so")
	}
}

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t.Add(23 * time.Hour)
}

// PE-6: the projection needs 7 days of data, is a linear fit over 14 days,
// and carries its interval.
func TestBurnUpProjectionNeedsSevenDays(t *testing.T) {
	h := ParseProofHistory(fixtureHistory(), false)
	b := BurnUp(h, HistoryFilter{}, day("2026-09-06"))
	if len(b.Points) != 5 { // 09-02 .. 09-06
		t.Fatalf("want a point per day from the first reading, got %+v", b.Points)
	}
	if b.Points[0].Proven != 2 || b.Points[0].Total != 4 || b.Points[2].Proven != 3 {
		t.Fatalf("points wrong: %+v", b.Points)
	}
	if b.Projection.State != StateAbsent || !strings.Contains(b.Projection.Reason, "7 days") {
		t.Fatalf("under 7 days the projection is withheld with its reason: %+v", b.Projection)
	}

	// Ten days of steady progress on a 20-row plan.
	var lines []string
	for d := 0; d < 10; d++ {
		var rows []string
		for i := 0; i < 20; i++ {
			st := "open"
			if i < d+1 {
				st = "claimed-proven"
			}
			rows = append(rows, fmt.Sprintf("R-%02d:%s", i, st))
		}
		lines = append(lines, ledgerLine(fmt.Sprintf("2026-09-%02dT12:00:00Z", d+1), "c", "cli", rows...))
	}
	h = ParseProofHistory([]byte(strings.Join(lines, "\n")), false)
	b = BurnUp(h, HistoryFilter{}, day("2026-09-10"))
	p := b.Projection
	if p.State != StatePresent {
		t.Fatalf("10 days of data should project: %+v", p)
	}
	if p.V.Finish != "2026-09-20" || p.V.Slope < 0.99 || p.V.Slope > 1.01 {
		t.Fatalf("linear fit wrong: %+v", p.V)
	}
	if p.V.Low == "" || p.V.High == "" || p.V.Low > p.V.Finish || p.V.High < p.V.Finish {
		t.Fatalf("the projection carries its interval: %+v", p.V)
	}
	if p.V.Days != 10 {
		t.Fatalf("fit window names its days: %+v", p.V)
	}
}

// PE-8: rows once proven that now read otherwise, with the first changed
// reading and its commit.
func TestProofRegressionsWithCommit(t *testing.T) {
	h := ParseProofHistory(fixtureHistory(), false)
	regs := ProofRegressions(h, HistoryFilter{})
	if len(regs) != 1 {
		t.Fatalf("want one regression (A-2), got %+v", regs)
	}
	r := regs[0]
	if r.ID != "A-2" || r.ChangedCommit != "c3" || r.ChangedAt != "2026-09-06T09:00:00Z" || r.ProvenCommit != "c1" || r.State != HistUnproven {
		t.Fatalf("regression wrong: %+v", r)
	}
	if !strings.Contains(r.Criterion, "test_reaches") {
		t.Fatalf("regression names the failing criterion: %+v", r)
	}
}

// PE-9: first appearance to first proven reading; rows present at the
// history's first reading are lower bounds, never exact.
func TestTimeToProof(t *testing.T) {
	h := ParseProofHistory(fixtureHistory(), false)
	tt := TimeToProof(h, HistoryFilter{})
	by := map[string]RowTimeToProof{}
	for _, r := range tt.Rows {
		by[r.ID] = r
	}
	if a1 := by["A-1"]; a1.FirstProven != "2026-09-04T09:00:00Z" || a1.Hours != 47 || !a1.LowerBound {
		t.Fatalf("A-1 wrong: %+v", a1)
	}
	if a4 := by["A-4"]; a4.FirstProven != "" || a4.LowerBound {
		t.Fatalf("A-4 never proven and first seen after the start: %+v", a4)
	}
	if tt.Proven.V != 2 || tt.Proven.State != StatePresent {
		t.Fatalf("proven count wrong: %+v", tt.Proven)
	}
}

// PE-10: claimed-unproven per agent identity (harness, model) — never a
// session id or a person; readings no session covers are named.
func TestClaimHonestyByAgentIdentity(t *testing.T) {
	h := ParseProofHistory(fixtureHistory(), false)
	sessions := []AgentSession{
		{ID: "s-1", Harness: "claude-code", Model: "opus", Repo: "svc", Start: "2026-09-04T08:00:00Z", End: "2026-09-04T10:00:00Z"},
		{ID: "s-2", Harness: "codex", Repo: "/r/svc", Start: "2026-09-06T08:00:00Z", End: "2026-09-06T09:30:00Z"},
	}
	ch := ClaimHonesty(h, sessions, HistoryFilter{})
	if len(ch.Rows) != 2 {
		t.Fatalf("want 2 identities, got %+v", ch.Rows)
	}
	cc, cx := ch.Rows[0], ch.Rows[1]
	if cc.Harness != "claude-code" || cc.Model != "opus" || cc.Claimed != 3 || cc.Unproven != 1 {
		t.Fatalf("claude-code row wrong: %+v", cc)
	}
	if cx.Model != ModelNotRecorded || cx.Claimed != 3 || cx.Unproven != 2 {
		t.Fatalf("codex row wrong: %+v", cx)
	}
	if ch.Unattributed != 3 {
		t.Fatalf("readings no session covers are named: %+v", ch)
	}
	for _, r := range ch.Rows {
		if strings.Contains(fmt.Sprintf("%+v", r), "s-1") {
			t.Fatalf("a session id leaked into an identity row (I58): %+v", r)
		}
	}
	only := ClaimHonesty(h, sessions, HistoryFilter{Agent: "codex"})
	if len(only.Rows) != 1 || only.Rows[0].Harness != "codex" {
		t.Fatalf("the agent filter narrows: %+v", only.Rows)
	}
}

// PE-12: a gauge per goal bet from its measurement rows' newest readings,
// and the review date; filters narrow every view.
func TestGoalGaugesFromBets(t *testing.T) {
	lines := string(fixtureHistory()) + ledgerLine("2026-09-07T09:00:00Z", "c4", "cli", "PE-M1:claimed-proven", "PE-M2:open") + "\n"
	h := ParseProofHistory([]byte(lines), false)
	bets := []intent.GoalBet{
		{Goal: intent.Goal{ID: "G22", Title: "Progress."}, Condition: "G22#1", Name: "progress-economics", ReviewRaw: "2026-12-31"},
		{Goal: intent.Goal{ID: "G9", Title: "Other."}, Condition: "G9#1", ReviewRaw: "2027-01-01"},
	}
	conds := []intent.Condition{
		{ID: "G22#1", Text: "`docs/plans/p.md` is proven and PE-M1 (every number) and PE-M2 (30 days) and PE-M3 read PASS."},
		{ID: "G9#1", Text: "no measurement rows named here"},
	}
	g := goalGauges(bets, conds, h)
	if len(g) != 2 {
		t.Fatalf("want a gauge per goal: %+v", g)
	}
	if g[0].Goal != "G22" || g[0].Review != "2026-12-31" || g[0].Proven.V != 1 || g[0].Total != 3 || g[0].Proven.State != StatePartial {
		t.Fatalf("G22 gauge wrong (PE-M3 has no reading: partial): %+v", g[0])
	}
	if g[1].Proven.State != StateAbsent {
		t.Fatalf("a goal naming no measurement row is absent, never 0: %+v", g[1])
	}

	// Filters: service, plan, date range, goal.
	f := HistoryFilter{Service: "svc", Plan: "docs/plans/p.md", From: "2026-09-05", To: "2026-09-06"}
	if regs := ProofRegressions(h, f); len(regs) != 1 {
		t.Fatalf("regression inside the range should survive the filter: %+v", regs)
	}
	if regs := ProofRegressions(h, HistoryFilter{To: "2026-09-05"}); len(regs) != 0 {
		t.Fatalf("a range ending before the change hides it: %+v", regs)
	}
	if regs := ProofRegressions(h, HistoryFilter{Service: "other"}); len(regs) != 0 {
		t.Fatalf("the service filter narrows: %+v", regs)
	}
	gf := GoalFilter(bets, conds, "G22")
	if len(gf.Plans) != 1 || gf.Plans[0] != "docs/plans/p.md" {
		t.Fatalf("a goal filter names its plans: %+v", gf)
	}
	if regs := ProofRegressions(h, HistoryFilter{GoalPlans: []string{"docs/plans/q.md"}}); len(regs) != 0 {
		t.Fatalf("the goal filter narrows to the goal's plans: %+v", regs)
	}
}
