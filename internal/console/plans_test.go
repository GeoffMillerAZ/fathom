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

// TestPlansTrend_CountsTheScopesCurrentPlans pins the portfolio's trend to
// the plans the portfolio shows (SD-3, vision 35 §5 "filter state lives in
// the URL"): the verify-ledger runs of each in-scope service's CURRENT
// plans, matched by service and file name — never a scratch service the
// registry does not hold, a plan file since deleted, or README.md, all of
// which the ledger keeps for good — and a service whose plans could not be
// read makes the trend an error, never a smaller line.
func TestPlansTrend_CountsTheScopesCurrentPlans(t *testing.T) {
	var runs []LedgerRun
	for d := 1; d <= 14; d++ {
		at := time.Date(2026, 9, d, 10, 0, 0, 0, time.UTC)
		runs = append(runs,
			LedgerRun{At: at, Service: "proof", Plan: "docs/plans/a.md", Claimed: 5, Verified: 4, Total: 9},
			LedgerRun{At: at, Service: "proof", Plan: "docs/plans/gone.md", Claimed: 50, Verified: 50},   // deleted since
			LedgerRun{At: at, Service: "proof-sv", Plan: "docs/plans/a.md", Claimed: 7, Verified: 7},     // a scratch service
			LedgerRun{At: at, Service: "proof", Plan: "docs/plans/README.md", Claimed: 9},                // never a plan
			LedgerRun{At: at, Service: "other", Plan: "/tmp/x/docs/plans/b.md", Claimed: 3, Verified: 1}, // a clone's run of a current plan
		)
	}
	svcs := []ServicePlans{
		{Service: "proof", Root: "/r", Plans: []PlanFacts{{Service: "proof", Path: "/r/docs/plans/a.md"}}},
		{Service: "other", Root: "/o", Plans: []PlanFacts{{Service: "other", Path: "/o/docs/plans/b.md"}}},
		{Service: "empty", Root: "/e"},
	}
	tr := PlansTrend(runs, "/l.jsonl", "", 14, svcs)
	if tr.State != StatePresent || len(tr.Points) != 14 {
		t.Fatalf("fleet trend = %+v", tr)
	}
	if last := tr.Points[13]; last.Plans != 2 || last.Claimed != 8 || last.Proven != 5 {
		t.Errorf("fleet trend's last day = %+v, want the two current plans only (2 plans, 8 claimed, 5 proven)", last)
	}
	if !strings.Contains(tr.Scope, "2 current plans") {
		t.Errorf("the trend names what it counts: %q", tr.Scope)
	}
	// Scoped to one repository: its plans alone.
	tr = PlansTrend(runs, "/l.jsonl", "", 14, svcs[:1])
	if last := tr.Points[len(tr.Points)-1]; tr.State != StatePresent || last.Plans != 1 || last.Claimed != 5 || last.Proven != 4 {
		t.Errorf("proof's trend = %+v", tr)
	}
	// The portfolio draws exactly this trend.
	pf := PlanPortfolio(PortfolioInput{Services: svcs, Registered: 3, Ledger: runs, LedgerPath: "/l.jsonl", MinTrendDays: 14})
	if got := pf.Trend.Points[13]; got.Plans != 2 || got.Claimed != 8 {
		t.Errorf("the portfolio's trend = %+v", got)
	}
	// An unreadable repository: an error naming it.
	bad := append(append([]ServicePlans{}, svcs...), ServicePlans{Service: "gamma", Err: "operation not permitted"})
	if tr := PlansTrend(runs, "/l.jsonl", "", 14, bad); tr.State != StateError || !strings.Contains(tr.Reason, "gamma") || len(tr.Points) != 0 {
		t.Errorf("an unreadable repository = %+v", tr)
	}
	// No plans in scope: absent, never an empty line.
	if tr := PlansTrend(runs, "/l.jsonl", "", 14, svcs[2:]); tr.State != StateAbsent || !strings.Contains(tr.Reason, "no plans") {
		t.Errorf("no plans = %+v", tr)
	}
}

// TestPlanPortfolio_MeasuredStaleIsALowerBound pins PC-5 on the headline:
// claimed rows a stale daemon did not prove are kept out of
// claimed_unproven, and they are not proven either — so the headline is a
// named lower bound with the remedy, never a clean measured count (a
// stale reading may not be hidden, cmd/fathom/measuredstale.go).
func TestPlanPortfolio_MeasuredStaleIsALowerBound(t *testing.T) {
	stale := PlanFacts{Service: "fathom", Path: "/f/docs/plans/x.md", Rel: "docs/plans/x.md", Verified: true, DaemonStale: true,
		Rows: []PlanRowFact{{ID: "1", Claimed: true, Verified: true, Status: "fail"}, {ID: "2", Claimed: true, Verified: true, Status: "pass"}, {ID: "3", Claimed: true, Verified: true, Status: "unknown"}}}
	pf := PlanPortfolio(PortfolioInput{Registered: 1, Services: []ServicePlans{{Service: "fathom", Plans: []PlanFacts{stale}}}})
	if pf.Bar.Count(RowMeasuredStale) != 2 {
		t.Fatalf("bar = %+v", pf.Bar)
	}
	for name, v := range map[string]Value[int]{"fleet": pf.Unproven, "fathom": pf.Services[0].Unproven} {
		if v.State != StatePartial || v.V != 0 || !strings.Contains(v.Reason, "2 claimed rows") || !strings.Contains(v.Reason, "stale") ||
			!strings.Contains(v.Reason, "lower bound") || !strings.Contains(v.Reason, "restart the daemon") {
			t.Errorf("%s headline with stale rows = %+v, want a named lower bound", name, v)
		}
	}
	if pf.Proven.V != 1 || pf.Proven.State != StatePresent {
		t.Errorf("proven = %+v", pf.Proven)
	}
}

// One label, one number (OC-9, UX-M3): the headline's "proven by the gate"
// counts every proven row, the (op) measurements among them; the bar's
// proven segment leaves those to their own segment, so it may not carry
// the headline's words for a smaller number.
func TestProofBar_ProvenSegmentIsNotTheHeadlinesLabel(t *testing.T) {
	bar := ProofBarOf(fixtureRows(), false)
	for _, s := range bar.Segments {
		if s.Key == RowProven && (s.Label == "proven by the gate" || !strings.Contains(s.Label, "(op)")) {
			t.Errorf("the proven segment = %q: it excludes the (op) rows the headline counts, and must say so", s.Label)
		}
	}
	pf := PlanPortfolio(PortfolioInput{Registered: 1, Services: []ServicePlans{{Service: "proof", Plans: []PlanFacts{{Service: "proof", Path: "/r/docs/plans/s.md", Rows: fixtureRows(), Verified: true}}}}})
	if pf.Proven.V != 3 || !strings.Contains(pf.Proven.Denominator, "1 of them a measurement row (op)") {
		t.Errorf("proven headline = %+v, want its (op) share named", pf.Proven)
	}
}

// A claimed row the gate cannot decide says why on its card — never
// runtime_pass, which is unknown on every row, proven ones included, and
// never decides one: a no-contract row is undecidable because it names no
// contract, and ingesting CI would not change that.
func TestCardDetail_UndecidableNamesTheRealCause(t *testing.T) {
	runtime := CriterionFact{Name: "runtime_pass", Status: "unknown", Detail: "no CI/coverage evidence ingested — static checks cannot prove the test passes"}
	noContract := PlanRowFact{ID: "md-001", Title: "x", Claimed: true, Verified: true, Status: "unknown", Label: "no-contract", Criteria: []CriterionFact{runtime}}
	d := cardDetail(noContract, RowClaimedUnknown)
	if strings.Contains(d, "runtime_pass") || !strings.Contains(d, "no contract") {
		t.Errorf("no-contract card detail = %q", d)
	}
	oracle := PlanRowFact{ID: "S-3", Claimed: true, Oracle: "lexical", Verified: true, Status: "unknown", Label: "oracle-unevaluated",
		Criteria: []CriterionFact{runtime, {Name: "lexical_oracle", Status: "unknown", Detail: "no repo file access on this evidence source"}}}
	if d := cardDetail(oracle, RowClaimedUnknown); !strings.HasPrefix(d, "lexical_oracle: no repo file access") {
		t.Errorf("an unevaluated oracle's card detail = %q", d)
	}
}

// TestPlanLedger_RemainingIsWhatPlansNextLists pins the plan page's answer
// to "which rows of this plan remain" (UX-M1 T1): every row `fathom plans
// next` lists — open, in progress, claimed but failing or undecidable,
// claimed and measured stale, and (op) rows still waiting — in document
// order with its row page, however many there are; its count is `plans
// status`'s open + claimed_unproven + measured_stale. Before the verdicts
// arrive the list is a named lower bound: the claimed rows are counted as
// pending, never listed as remaining nor dropped as proven.
func TestPlanLedger_RemainingIsWhatPlansNextLists(t *testing.T) {
	plan := PlanFacts{Service: "proof", Path: "/r/docs/plans/settlement.md", Rel: "docs/plans/settlement.md", Title: "Settlement", Rows: fixtureRows(), Verified: true}
	href := func(id string) string { return "/plans/proof/settlement/rows/" + id }
	v := PlanLedger(PlanLedgerInput{Plan: plan, RowHref: href})
	var ids []string
	for _, r := range v.Remaining.Rows {
		ids = append(ids, r.ID)
		if r.Href != href(r.ID) {
			t.Errorf("remaining %s href = %q", r.ID, r.Href)
		}
	}
	if got := strings.Join(ids, ","); got != "S-3,S-4,S-5,S-6,S-M2" {
		t.Errorf("remaining = %q, want every row plans next lists, in document order", got)
	}
	if r := v.Rollup; r == nil || len(v.Remaining.Rows) != r.Open+r.ClaimedUnproven+r.MeasuredStale {
		t.Errorf("remaining %d != plans status open+claimed_unproven+measured_stale (%+v)", len(v.Remaining.Rows), v.Rollup)
	}
	if v.Remaining.State != StatePresent || v.Remaining.Pending != 0 || v.Remaining.Reason != "" {
		t.Errorf("verified remaining = %+v", v.Remaining)
	}

	// More remaining rows than a board lane shows: every one is listed.
	var many []PlanRowFact
	for i := 0; i < 40; i++ {
		many = append(many, PlanRowFact{ID: "R-" + strings.Repeat("x", i+1), Title: "open row", Verified: true})
	}
	if v := PlanLedger(PlanLedgerInput{Plan: PlanFacts{Rows: many, Verified: true}}); len(v.Remaining.Rows) != 40 {
		t.Errorf("remaining lists %d of 40 open rows", len(v.Remaining.Rows))
	}

	// A stale daemon's unproven claims still remain (plans next lists them).
	stale := PlanFacts{Rows: []PlanRowFact{{ID: "1", Claimed: true, Verified: true, Status: "fail"}, {ID: "2", Claimed: true, Verified: true, Status: "pass"}}, Verified: true, DaemonStale: true}
	if v := PlanLedger(PlanLedgerInput{Plan: stale}); len(v.Remaining.Rows) != 1 || v.Remaining.Rows[0].ID != "1" || v.Remaining.Rows[0].State != RowMeasuredStale {
		t.Errorf("stale remaining = %+v", v.Remaining.Rows)
	}

	// Nothing remains: zero, an empty list — never absent.
	done := PlanFacts{Rows: []PlanRowFact{{ID: "1", Claimed: true, Verified: true, Status: "pass"}}, Verified: true}
	if v := PlanLedger(PlanLedgerInput{Plan: done}); v.Remaining.State != StateZero || v.Remaining.Rows == nil || len(v.Remaining.Rows) != 0 {
		t.Errorf("nothing remaining = %+v", v.Remaining)
	}

	// Still verifying: the unclaimed rows are listed; the 5 claimed rows
	// wait for their verdicts, counted and named.
	plan.Verified, plan.Pending = false, true
	for i := range plan.Rows {
		plan.Rows[i].Verified, plan.Rows[i].Status = false, ""
	}
	v = PlanLedger(PlanLedgerInput{Plan: plan, RowHref: href})
	ids = ids[:0]
	for _, r := range v.Remaining.Rows {
		ids = append(ids, r.ID)
	}
	if got := strings.Join(ids, ","); got != "S-5,S-6,S-M2" || v.Remaining.State != StatePartial || v.Remaining.Pending != 5 || !strings.Contains(v.Remaining.Reason, "5 claimed rows") {
		t.Errorf("pending remaining = %q %+v", got, v.Remaining)
	}
}
