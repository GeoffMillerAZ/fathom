package console

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// fixtureRows covers every row state the board and the bar distinguish.
func fixtureRows() []PlanRowFact {
	return []PlanRowFact{
		{ID: "S-1", Title: "Settle", Claimed: true, Symbol: "Settle", ReachableFrom: "TestSettle", Verified: true, Status: "pass", Label: "verified"},
		{ID: "S-2", Title: "Reconcile", Claimed: true, Symbol: "Reconcile", Verified: true, Status: "pass"},
		{ID: "S-3", Title: "Runbook", Claimed: true, Oracle: "lexical", OraclePath: "README.md", OraclePattern: "^## Window$", Verified: true, Status: "fail",
			Criteria: []CriterionFact{{Name: "lexical_oracle", Status: "fail", Detail: "pattern matches nothing in README.md"}}},
		{ID: "S-4", Title: "Unknown", Claimed: true, Verified: true, Status: "unknown", Label: "no-contract"},
		{ID: "S-5", Title: "Audit", Symbol: "Audit", Verified: true, Status: "fail"},
		{ID: "S-6", Title: "Batch", InProgress: true, Symbol: "Batch", Verified: true},
		{ID: "S-M1", Title: "(op) S-M1: latency", Op: true, Claimed: true, Oracle: "lexical", OraclePath: "README.md", Verified: true, Status: "pass"},
		{ID: "S-M2", Title: "(op) S-M2: drift", Op: true, SkipReason: "operator measurement: 30-day window", Verified: true},
		{ID: "S-7", Title: "Multi-currency", Skipped: true, Disposition: "deferred", Reason: "one currency", Trigger: "a second ships", Verified: true},
		{ID: "S-8", Title: "Anchoring", Skipped: true, Disposition: "refused", Reason: "out of scope", Verified: true},
	}
}

// TestPlanPortfolio_ProofBarsPerPlan pins OC-21: every plan across the
// fleet gets a proof bar counted from the gate's verdicts — proven,
// measured op rows, claimed-but-failing, undecidable, open, waiting op
// rows, deferred and refused each in their own segment — and the bar
// equals `plans status`'s rollup; a plan still verifying shows its claims
// as not yet verified (never proven), and the headline says it is a lower
// bound.
func TestPlanPortfolio_ProofBarsPerPlan(t *testing.T) {
	settlement := PlanFacts{Service: "proof", Path: "/r/docs/plans/settlement.md", Rel: "docs/plans/settlement.md", Title: "Settlement", Rows: fixtureRows(), Verified: true}
	audit := PlanFacts{Service: "proof", Path: "/r/docs/plans/audit.md", Rel: "docs/plans/audit.md", Title: "Audit",
		Rows: []PlanRowFact{{ID: "A-1", Claimed: true, Symbol: "validate"}, {ID: "A-2", Symbol: "Audit"}}, Pending: true}
	in := PortfolioInput{
		Registered: 3,
		Services: []ServicePlans{
			{Service: "proof", Root: "/r", Plans: []PlanFacts{audit, settlement}},
			{Service: "empty", Root: "/e"},
		},
	}
	p := PlanPortfolio(in)
	if len(p.Plans) != 2 || p.WithPlans != 1 || p.Registered != 3 {
		t.Fatalf("portfolio = %d plans, %d/%d services", len(p.Plans), p.WithPlans, p.Registered)
	}
	var got PortfolioPlan
	for _, pp := range p.Plans {
		if pp.Rel == "docs/plans/settlement.md" {
			got = pp
		}
	}
	want := map[string]int{RowProven: 2, RowOpMeasured: 1, RowClaimedFailing: 1, RowClaimedUnknown: 1, RowOpen: 1, RowInProgress: 1, RowOpWaiting: 1, RowDeferred: 1, RowSkipped: 1}
	for k, n := range want {
		if c := got.Bar.Count(k); c != n {
			t.Errorf("settlement bar %s = %d, want %d (%+v)", k, c, n, got.Bar)
		}
	}
	if got.Bar.Total != 10 || got.Href != "/plans/proof/settlement" || got.State != StatePresent {
		t.Errorf("settlement = %+v", got)
	}
	// The bar agrees with the verb's rollup: proven = proven + op measured,
	// open = open + in progress + op waiting, claimed-unproven = failing +
	// undecidable.
	r := got.Rollup
	if r == nil || r.Proven != 3 || r.OpProven != 1 || r.ClaimedUnproven != 2 || r.Open != 3 || r.OpOpen != 1 || r.DeferredWithTrigger != 1 || r.SkippedOther != 1 {
		t.Fatalf("rollup = %+v", r)
	}
	for _, seg := range got.Bar.Segments {
		if seg.Label == "" || seg.Role == "" {
			t.Errorf("segment %q must carry a label and a role — color is never the only channel", seg.Key)
		}
	}
	// A pending plan's claimed row is not yet verified — never proven.
	var pending PortfolioPlan
	for _, pp := range p.Plans {
		if pp.Rel == "docs/plans/audit.md" {
			pending = pp
		}
	}
	if pending.Rollup != nil || pending.State != StatePartial || pending.Bar.Count(RowPending) != 1 || pending.Bar.Count(RowProven) != 0 {
		t.Errorf("pending plan = %+v", pending)
	}
	if p.Unproven.State != StatePartial || p.Unproven.V != 2 || !strings.Contains(p.Unproven.Reason, "lower bound") {
		t.Errorf("headline with a pending plan = %+v, want a named lower bound of 2", p.Unproven)
	}
	// Once verified, exact.
	audit.Pending, audit.Verified = false, true
	audit.Rows[0].Verified, audit.Rows[0].Status = true, "pass"
	audit.Rows[1].Verified = true
	in.Services[0].Plans[0] = audit
	p = PlanPortfolio(in)
	if p.Unproven.State != StatePresent || p.Unproven.V != 2 || p.Proven.V != 4 {
		t.Errorf("headline, all verified = %+v / proven %+v", p.Unproven, p.Proven)
	}
	if p.Bar.Count(RowProven) != 3 {
		t.Errorf("fleet bar proven = %d, want 3 (2 settlement + 1 audit)", p.Bar.Count(RowProven))
	}
}

// TestPlanPortfolio_FleetAndPerRepoHeadlineGap pins SD-2: the SDD panel
// is computed from the plans-status engine for the fleet and per
// repository, and its headline is the claimed-vs-proven gap — a service
// whose plans cannot be read makes the fleet headline an error naming it,
// never a smaller gap (I38).
func TestPlanPortfolio_FleetAndPerRepoHeadlineGap(t *testing.T) {
	a := PlanFacts{Service: "alpha", Path: "/a/docs/plans/x.md", Rel: "docs/plans/x.md", Verified: true,
		Rows: []PlanRowFact{{ID: "1", Claimed: true, Verified: true, Status: "fail"}, {ID: "2", Claimed: true, Verified: true, Status: "pass"}}}
	b := PlanFacts{Service: "beta", Path: "/b/docs/plans/y.md", Rel: "docs/plans/y.md", Verified: true,
		Rows: []PlanRowFact{{ID: "1", Claimed: true, Verified: true, Status: "unknown"}, {ID: "2", Claimed: true, Verified: true, Status: "fail"}}}
	in := PortfolioInput{Registered: 2, Services: []ServicePlans{{Service: "alpha", Plans: []PlanFacts{a}}, {Service: "beta", Plans: []PlanFacts{b}}}}
	p := PlanPortfolio(in)
	if p.Unproven.V != 3 || p.Unproven.State != StatePresent || p.Proven.V != 1 {
		t.Fatalf("fleet gap = %+v, proven %+v", p.Unproven, p.Proven)
	}
	per := map[string]int{}
	for _, s := range p.Services {
		per[s.Service] = s.Unproven.V
	}
	if per["alpha"] != 1 || per["beta"] != 2 {
		t.Errorf("per-repo gaps = %v", per)
	}
	// One unreadable repository: the fleet headline is an error naming it.
	in.Services = append(in.Services, ServicePlans{Service: "gamma", Err: "operation not permitted"})
	p = PlanPortfolio(in)
	if p.Unproven.State != StateError || !strings.Contains(p.Unproven.Reason, "gamma") {
		t.Errorf("an unreadable repository must make the gap an error naming it, got %+v", p.Unproven)
	}
	raw, _ := json.Marshal(p.Unproven)
	if strings.Contains(string(raw), `"v"`) {
		t.Errorf("an errored gap must carry no number: %s", raw)
	}
	// No plans anywhere: absent, never zero.
	p = PlanPortfolio(PortfolioInput{Registered: 1, Services: []ServicePlans{{Service: "empty"}}})
	if p.Unproven.State != StateAbsent {
		t.Errorf("no plans = %+v, want absent", p.Unproven)
	}
}

func ledgerRun(day int, plan string, claimed, verified int) LedgerRun {
	return LedgerRun{At: time.Date(2026, 9, day, 10, 0, 0, 0, time.UTC), Service: "proof", Plan: plan, Claimed: claimed, Verified: verified, Total: 10}
}

// TestLedgerTrend_AbsentUntilFourteenDays pins SD-3: the trend chart is
// absent, with the count of days the ledger holds, until the ledger has
// rows on 14 distinct days; then it is one point per day, each plan's last
// run carried forward.
func TestLedgerTrend_AbsentUntilFourteenDays(t *testing.T) {
	var runs []LedgerRun
	for d := 1; d <= 13; d++ {
		runs = append(runs, ledgerRun(d, "docs/plans/a.md", d, d-1))
	}
	tr := LedgerTrend(runs, "/l.jsonl", "", 14, nil)
	if tr.State != StateAbsent || tr.Days != 13 || !strings.Contains(tr.Reason, "13 days") || len(tr.Points) != 0 {
		t.Fatalf("13 days = %+v, want absent naming 13 days", tr)
	}
	runs = append(runs, ledgerRun(14, "docs/plans/a.md", 14, 13), ledgerRun(14, "/other/root/docs/plans/b.md", 4, 4))
	tr = LedgerTrend(runs, "/l.jsonl", "", 14, nil)
	if tr.State != StatePresent || len(tr.Points) != 14 {
		t.Fatalf("14 days = %+v", tr)
	}
	last := tr.Points[13]
	if last.Plans != 2 || last.Claimed != 18 || last.Proven != 17 {
		t.Errorf("last point = %+v, want both plans' latest runs summed", last)
	}
	// A filter selects one plan by service and file name, whatever root
	// the run came from.
	tr = LedgerTrend(runs, "/l.jsonl", "", 14, func(r LedgerRun) bool { return strings.HasSuffix(r.Plan, "b.md") })
	if tr.State != StateAbsent || tr.Days != 1 {
		t.Errorf("one plan with one day = %+v", tr)
	}
	if tr := LedgerTrend(nil, "/l.jsonl", "permission denied", 14, nil); tr.State != StateError {
		t.Errorf("unreadable ledger = %+v, want error", tr)
	}
}

// TestPlanLedger_BoardAndBurnup pins OC-22: a plan page carries the proof
// bar, a board whose lanes sort rows by the state the gate computed with
// each card naming its contract kind, and a burn-up of scope against
// claimed rows from the plan's own git history — cross-checked with the
// verify-ledger runs whose commit is a revision of the plan, the proven
// trend gated on 14 days of ledger rows (SD-3).
func TestPlanLedger_BoardAndBurnup(t *testing.T) {
	plan := PlanFacts{Service: "proof", Path: "/r/docs/plans/settlement.md", Rel: "docs/plans/settlement.md", Title: "Settlement", Rows: fixtureRows(), Verified: true, StoreCommit: "1ddacbe5b07165d6"}
	revs := []PlanRevision{
		{Commit: "aaaaaaaa11", At: time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC), Total: 3},
		{Commit: "bbbbbbbb22", At: time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC), Total: 5, Claimed: 1, Skipped: 1},
		{Commit: "cccccccc33", At: time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC), Total: 10, Claimed: 5, Skipped: 2},
	}
	ledger := []LedgerRun{
		{At: time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC), Service: "proof", Plan: "docs/plans/settlement.md", Commit: "bbbbbbbb", Total: 5, Claimed: 1, Verified: 1},
		{At: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), Service: "proof", Plan: "/clone/docs/plans/settlement.md", Commit: "cccccccc", Total: 9, Claimed: 5, Verified: 3},
		{At: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), Service: "proof", Plan: "docs/plans/settlement.md", Commit: "dddddddd", Total: 10},
		{At: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), Service: "other", Plan: "docs/plans/settlement.md", Commit: "bbbbbbbb", Total: 99},
	}
	v := PlanLedger(PlanLedgerInput{
		Plan:    plan,
		Burnup:  BurnupInput{Revisions: revs, Ledger: ledger, LedgerPath: "/l.jsonl", MinDays: 14, MaxPoints: 2},
		RowHref: func(id string) string { return "/plans/proof/settlement/rows/" + id },
	})
	if v.State != StatePresent || v.Rollup == nil || v.Rollup.Total != 10 || v.Bar.Total != 10 {
		t.Fatalf("plan page = %+v", v)
	}
	lanes := map[string][]string{}
	contracts := map[string]string{}
	for _, c := range v.Columns {
		for _, card := range c.Cards {
			lanes[c.ID] = append(lanes[c.ID], card.ID)
			contracts[card.ID] = card.Contract
			if card.Href != "/plans/proof/settlement/rows/"+card.ID {
				t.Errorf("card %s href = %q", card.ID, card.Href)
			}
		}
	}
	want := map[string]string{"remaining": "S-5,S-6", "claimed": "S-3,S-4", "proven": "S-1,S-2", "op-measured": "S-M1", "op-waiting": "S-M2", "deferred": "S-7,S-8"}
	for lane, ids := range want {
		if got := strings.Join(lanes[lane], ","); got != ids {
			t.Errorf("lane %s = %q, want %q", lane, got, ids)
		}
	}
	if contracts["S-1"] != ContractReach || contracts["S-3"] != ContractLexical || contracts["S-7"] != ContractDisposition || contracts["S-4"] != ContractNone {
		t.Errorf("contract kinds = %v", contracts)
	}
	for _, c := range v.Columns {
		if c.ID == "claimed" && (len(c.Cards) == 0 || !strings.Contains(c.Cards[0].Detail, "lexical_oracle")) {
			t.Errorf("a failing card must say why: %+v", c.Cards)
		}
	}
	if len(v.Rows) != 10 {
		t.Errorf("the table alternative must list every row, got %d", len(v.Rows))
	}
	b := v.Burnup
	if b.State != StatePresent || b.Commits != 3 || len(b.Points) != 2 || b.Omitted != 1 || b.MaxScope != 10 {
		t.Fatalf("burn-up = %+v", b)
	}
	if b.Points[1].Scope != 10 || b.Points[1].Claimed != 5 || b.Points[1].Commit != "cccccccc" {
		t.Errorf("last point = %+v", b.Points[1])
	}
	// Cross-check: two runs sit on revisions (one agrees, one counted 9 of
	// 10 rows), one ran on a commit that did not touch the plan, and
	// another service's run is not this plan's.
	if b.CrossCheck.Matched != 2 || b.CrossCheck.Agree != 1 || len(b.CrossCheck.Disagree) != 1 || b.CrossCheck.Unmatched != 1 {
		t.Errorf("cross-check = %+v", b.CrossCheck)
	}
	if b.Proven.State != StateAbsent || b.Proven.Days != 3 {
		t.Errorf("the proven trend must stay absent under 14 days of this plan's rows: %+v", b.Proven)
	}
	// Not yet verified: partial, and no claimed row reads proven.
	plan.Verified, plan.Pending = false, true
	for i := range plan.Rows {
		plan.Rows[i].Verified, plan.Rows[i].Status = false, ""
	}
	v = PlanLedger(PlanLedgerInput{Plan: plan})
	if v.State != StatePartial || v.Rollup != nil || v.Bar.Count(RowProven) != 0 || v.Bar.Count(RowPending) != 5 {
		t.Errorf("pending plan page = %+v", v.Bar)
	}
	// No git history: the burn-up says so.
	if v.Burnup.State != StateAbsent || v.Burnup.Reason == "" {
		t.Errorf("no history = %+v", v.Burnup)
	}
}
