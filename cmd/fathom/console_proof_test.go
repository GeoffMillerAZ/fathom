package main

// console_proof_test.go — console wave C's pages (the Proof group and the
// project overview's ladder) through the real handler on the fixture fleet,
// whose proof-service holds plans, tests, an intent corpus and a git
// history (console_proof_fixture_test.go). The projections' own contracts
// are pinned in internal/console; these pin what the pages render.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func mustContain(t *testing.T, where, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("%s: missing %q", where, w)
		}
	}
}

func h1Count(body string) int { return strings.Count(body, "<h1") }

func TestConsolePlansPortfolio_PageProofBarsAndHeadline(t *testing.T) {
	f := newConsoleFixture(t)
	code, _, page := f.get(t, "/plans")
	if code != http.StatusOK || h1Count(page) != 1 || !strings.Contains(page, ">Plans</h1>") {
		t.Fatalf("/plans = %d", code)
	}
	mustContain(t, "/plans", page,
		`aria-label="Filter plans by repository"`, `href="/plans?svc=proof-service"`,
		"claimed, not proven by the gate", "proven by the gate",
		`data-proof-bar`, `data-bar-legend`,
		`href="/plans/proof-service/settlement"`, `href="/plans/proof-service/audit"`,
		`data-console-copy="fathom plans status --service proof-service --json"`,
		`id="trend"`, "no verify-ledger rows yet",
	)
	// The fleet view lists one verb per repository behind one disclosure in
	// the heading, not a stack of chips.
	heading := page[strings.Index(page, "data-console-heading"):]
	heading = heading[:strings.Index(heading, "data-console-panel")]
	if i := strings.Index(heading, "data-verbs-disclosure"); i < 0 || strings.Contains(heading[:i], "data-verb-chip") {
		t.Error("the fleet portfolio's heading must list its per-repository verbs behind one disclosure")
	}
	if !strings.Contains(page, "The 2 verbs this page renders") {
		t.Error("the disclosure must name how many verbs it holds")
	}
	// The trend panel renders the verb that prints the fleet's trend; the
	// proof bar's proven segment never borrows the headline's words for a
	// smaller number (the headline counts the (op) rows, the segment not).
	mustContain(t, "/plans trend and labels", page,
		`data-console-verb="fathom plans history --json"`, "proven, not counting (op) measurements", "Proven, (op) included")
	if strings.Contains(page, "proven by the gate: ") {
		t.Error("a mini-bar segment must not be labelled with the headline's \"proven by the gate\"")
	}
	assertRoleUtilitiesOnly(t, page)

	env := f.pageJSON(t, "/plans")
	var m plansPortfolioModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Plans) != 2 || m.WithPlans != 1 || m.Registered != len(consoleFixtureServices) || m.Unproven.V != 1 || m.Proven.V != 5 {
		t.Errorf("portfolio = %d plans, %d/%d repos, unproven %+v, proven %+v", len(m.Plans), m.WithPlans, m.Registered, m.Unproven, m.Proven)
	}

	// Scoped to a repository: its own chip in the heading, and no fleet
	// metric reported under the fleet's name.
	code, _, scoped := f.get(t, "/plans?svc=proof-service")
	if code != http.StatusOK || !strings.Contains(scoped, `aria-current="page">proof-service</a>`) {
		t.Errorf("scoped portfolio = %d", code)
	}
	senv := f.pageJSON(t, "/plans?svc=proof-service")
	if _, ok := senv.Metrics[string(console.MetricPlansUnproven)]; ok {
		t.Error("a repository's gap must not be reported as the fleet metric")
	}
	// The scoped page's trend is the repository's, from its own verb.
	if _, ok := senv.Sections["fathom plans history --service proof-service --json"]; !ok || !strings.Contains(scoped, `data-console-verb="fathom plans history --service proof-service --json"`) {
		t.Errorf("the scoped trend must render `plans history --service proof-service`: sections %v", keysOf(senv.Sections))
	}
	if code, _, _ := f.get(t, "/plans?svc=no-such-repo"); code != http.StatusNotFound {
		t.Errorf("unknown repository = %d, want 404 on the shell", code)
	}
}

// While the gate's verdicts have not arrived the portfolio says so: the
// claimed rows are "not yet verified" (never proven), the headline is
// absent with the reason, the verb's section is named pending — and once
// they arrive the same page is exact.
func TestConsolePlansPortfolio_PendingIsNeverProven(t *testing.T) {
	gate := make(chan struct{})
	setConsolePlansGate(gate)
	prev := consolePlansPortfolioWait
	consolePlansPortfolioWait = 20 * time.Millisecond
	released := false
	release := func() {
		if !released {
			released = true
			close(gate)
		}
	}
	t.Cleanup(func() { release(); setConsolePlansGate(nil); consolePlansPortfolioWait = prev })

	f := newConsoleFixture(t)
	env := f.pageJSON(t, "/plans")
	var m plansPortfolioModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	if m.Unproven.State != console.StateAbsent || m.Bar.Total != 0 || len(m.PendingSections) != 1 {
		t.Fatalf("pending portfolio = headline %+v, bar %+v, pending %v", m.Unproven, m.Bar, m.PendingSections)
	}
	if _, ok := env.Sections["fathom plans status --service proof-service --json"]; ok {
		t.Error("a verb's section must not be served before its verdicts arrive")
	}
	for _, p := range m.Plans {
		if p.Rollup != nil || p.Bar.Count(console.RowProven) != 0 || p.State != console.StatePartial {
			t.Errorf("pending plan %s = %+v", p.Rel, p)
		}
	}
	_, _, page := f.get(t, "/plans")
	mustContain(t, "pending /plans", page, "verifying", "not measured")

	release()
	consolePlansPortfolioWait = time.Minute
	env = f.pageJSON(t, "/plans")
	m = plansPortfolioModel{}
	_ = json.Unmarshal(env.Model, &m)
	if m.Unproven.State != console.StatePresent || m.Unproven.V != 1 || len(m.PendingSections) != 0 {
		t.Errorf("after the verdicts arrive = %+v, pending %v", m.Unproven, m.PendingSections)
	}
}

// The Now page's plans headline (plans.proven, OC-H3) is the portfolio's,
// but Now never waits on verification for it: with the verdicts held back
// it answers at once, the headline not measured (pending), never proven.
func TestConsoleNow_PlansHeadlineNeverWaitsOnVerification(t *testing.T) {
	gate := make(chan struct{})
	setConsolePlansGate(gate)
	prev := consolePlansHeadlineWait
	consolePlansHeadlineWait = 0
	t.Cleanup(func() { close(gate); setConsolePlansGate(nil); consolePlansHeadlineWait = prev })

	f := newConsoleFixture(t)
	start := time.Now()
	env := f.pageJSON(t, "/")
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("/ took %s with verification held back: it must not wait", took)
	}
	raw, ok := env.Metrics[string(console.MetricPlansProven)]
	if !ok {
		t.Fatalf("/ must show %s; metrics = %v", console.MetricPlansProven, env.Metrics)
	}
	var v console.Value[int]
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if v.State == console.StatePresent {
		t.Errorf("plans.proven before any verdict = %+v: pending is never proven", v)
	}
}

func TestConsolePlanPage_BoardDrawerAndBurnup(t *testing.T) {
	f := newConsoleFixture(t)
	code, _, page := f.get(t, "/plans/proof-service/settlement")
	if code != http.StatusOK || h1Count(page) != 1 || !strings.Contains(page, ">Settlement ledger (fixture plan)</h1>") {
		t.Fatalf("plan page = %d", code)
	}
	if !regexp.MustCompile(`href="/plans"[^>]*>Plans</a>.*href="/plans\?svc=proof-service"[^>]*>proof-service</a>`).MatchString(page) {
		t.Error("the plan page's trail must run Observatory / Plans / its repository / the plan")
	}
	mustContain(t, "plan page", page,
		`data-proof-bar`, `data-plan-board`,
		`data-kanban-col="remaining"`, `data-kanban-col="claimed"`, `data-kanban-col="proven"`,
		`data-kanban-col="op-measured"`, `data-kanban-col="op-waiting"`, `data-kanban-col="deferred"`,
		`href="/plans/proof-service/settlement/rows/S-2"`, "symbol + test reach", "lexical oracle",
		"The other 6 rows as a table", `id="row-drawer"`, `data-row-drawer-body`, "dlg.showModal()", `new CustomEvent("console:content",{bubbles:true})`,
		`id="burnup"`, `data-table-row="rev-`, "Proven by the gate over time", "data-cross-check",
	)
	// The ledger is read-only: nothing on the board looks draggable.
	for _, bad := range []string{`draggable="true"`, "cursor-grab", "neon-pink"} {
		if strings.Contains(page, bad) {
			t.Errorf("the read-only board must not carry %q", bad)
		}
	}
	if n := len(regexp.MustCompile(`data-table-row="rev-`).FindAllString(page, -1)); n != len(proofServiceCommits)-1 {
		t.Errorf("burn-up rows = %d, want one per commit of the plan (%d)", n, len(proofServiceCommits)-1)
	}
	assertRoleUtilitiesOnly(t, page)

	env := f.pageJSON(t, "/plans/proof-service/settlement")
	if len(env.Sections) != 3 {
		t.Errorf("plan page sections = %d, want status, next and history", len(env.Sections))
	}
	var v planPageModel
	if err := json.Unmarshal(env.Model, &v); err != nil {
		t.Fatal(err)
	}
	if v.Rollup == nil || v.Rollup.Proven != 4 || v.Rollup.ClaimedUnproven != 1 || v.Burnup.Commits != 3 || v.Burnup.Proven.State != console.StateAbsent {
		t.Errorf("plan page model = rollup %+v, burn-up %+v", v.Rollup, v.Burnup)
	}
	if code, _, body := f.get(t, "/plans/proof-service/no-such-plan"); code != http.StatusNotFound || !strings.Contains(body, "Unknown plan") {
		t.Errorf("unknown plan = %d", code)
	}
}

// "Which rows of plan P remain?" is answered on the plan page itself, at
// the third click (UX-M1 T1): a Remaining panel lists every row `fathom
// plans next` lists — none cut by the board's lane cap, none behind a
// closed disclosure — each linking to its row page, and the page's JSON
// lists the same IDs as the verb, counted as `plans status` counts them.
func TestConsolePlanPage_RemainingAnswersWhichRowsRemain(t *testing.T) {
	f := newConsoleFixture(t)
	const plan = "/plans/proof-service/settlement"
	_, _, page := f.get(t, plan)
	at, board := strings.Index(page, "data-plan-remaining"), strings.Index(page, "data-plan-board")
	if at < 0 || board < 0 || at > board {
		t.Fatalf("the plan page must lead with its remaining rows, before the board (remaining at %d, board at %d)", at, board)
	}
	panel := page[at:board]
	if strings.Contains(panel, "<details") {
		t.Error("the remaining rows must not sit behind a disclosure")
	}

	next := "fathom plans next --service proof-service --plan docs/plans/settlement.md --json"
	status := "fathom plans status --service proof-service --plan docs/plans/settlement.md --json"
	verbIDs := func(v any) []string {
		var ids []string
		for _, r := range v.([]any) {
			ids = append(ids, r.(map[string]any)["id"].(string))
		}
		return ids
	}
	want := verbIDs(runVerbJSON(t, f, next))
	if strings.Join(want, ",") != "S-4,S-5,S-6,S-M2" {
		t.Fatalf("plans next on the fixture = %v", want)
	}
	for _, id := range want {
		if !strings.Contains(panel, `href="`+plan+`/rows/`+id+`"`) {
			t.Errorf("remaining row %s is not linked in the panel", id)
		}
	}

	env := f.pageJSON(t, plan)
	var v planPageModel
	if err := json.Unmarshal(env.Model, &v); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range v.Remaining.Rows {
		got = append(got, r.ID)
	}
	var section any
	if err := json.Unmarshal(env.Sections[next], &section); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") || strings.Join(verbIDs(section), ",") != strings.Join(want, ",") {
		t.Errorf("remaining: model %v, page section %v, verb %v", got, verbIDs(section), want)
	}
	st := runVerbJSON(t, f, status).([]any)[0].(map[string]any)
	n := int(st["open"].(float64) + st["claimed_unproven"].(float64))
	if ms, ok := st["measured_stale"].(float64); ok {
		n += int(ms)
	}
	if n != len(got) || v.Remaining.State != console.StatePresent {
		t.Errorf("remaining %d (%s), plans status counts %d", len(got), v.Remaining.State, n)
	}
}

// The remaining panel lists every row however many there are — the board's
// per-lane cap (boardLaneCards) never applies to it.
func TestConsolePlanRemaining_ListsEveryRow(t *testing.T) {
	var rows []console.PlanRowView
	for i := 0; i < 3*boardLaneCards; i++ {
		id := fmt.Sprintf("R-%d", i)
		rows = append(rows, console.PlanRowView{PlanRowFact: console.PlanRowFact{ID: id, Title: "open row " + id}, State: console.RowOpen, Contract: console.ContractReach, Href: "/plans/s/p/rows/" + id})
	}
	out := renderPlanRemaining(console.PlanRemaining{State: console.StatePresent, Rows: rows})
	for _, r := range rows {
		if !strings.Contains(out, `href="`+r.Href+`"`) {
			t.Errorf("row %s missing", r.ID)
		}
	}
	if strings.Contains(out, "<details") {
		t.Error("no disclosure")
	}
	if none := renderPlanRemaining(console.PlanRemaining{State: console.StateZero, Rows: []console.PlanRowView{}}); !strings.Contains(none, "Nothing remains") {
		t.Errorf("nothing remaining must say so:\n%s", none)
	}
}

// Every row sits in exactly one table — a remaining row in the Remaining
// table, any other behind the board's disclosure — so listing every
// remaining row adds nothing per row to the page: the 250 KB budget holds
// on a plan of any size as it did before (listing them twice measured
// 205 KB of body alone for 120 remaining rows).
func TestConsolePlanPage_EachRowInExactlyOneTable(t *testing.T) {
	var rows []console.PlanRowFact
	for i := 0; i < 30; i++ {
		rows = append(rows, console.PlanRowFact{ID: fmt.Sprintf("R-%d", i), Title: "row", Claimed: i%2 == 0, Verified: true, Status: "pass"})
	}
	v := console.PlanLedger(console.PlanLedgerInput{Plan: console.PlanFacts{Rows: rows, Verified: true}, RowHref: func(id string) string { return "/plans/s/p/rows/" + id }})
	page := renderPlanPage(v, "s", []string{"fathom plans status", "fathom plans next", "fathom plans history"})
	for _, r := range rows {
		if n := strings.Count(page, `data-table-row="remaining-`+r.ID+`"`) + strings.Count(page, `data-table-row="row-`+r.ID+`"`); n != 1 {
			t.Errorf("row %s is in %d tables, want exactly one", r.ID, n)
		}
	}
	mustContain(t, "plan page", page, "The other 15 rows as a table")
}

// renderPlanBoard caps each lane and says where the rest are.
func TestConsolePlanBoard_LanesAreBounded(t *testing.T) {
	var cards []console.BoardCard
	for i := 0; i < boardLaneCards+3; i++ {
		id := "R-" + strings.Repeat("x", i+1)
		cards = append(cards, console.BoardCard{ID: id, Title: id, State: console.RowOpen, Contract: console.ContractReach, Href: "/plans/s/p/rows/" + id})
	}
	out := renderPlanBoard(console.PlanLedgerView{Columns: []console.BoardColumn{{ID: "remaining", Label: "Remaining", Cards: cards}}})
	if n := strings.Count(out, "data-kanban-card="); n != boardLaneCards {
		t.Errorf("lane shows %d cards, want %d", n, boardLaneCards)
	}
	mustContain(t, "bounded board", out, "data-board-cut", "3 rows more are in the Remaining list above or the table below", ">15</span>")

	// An empty lane is named, not drawn — except claimed-not-proven, whose
	// emptiness is the news.
	out = renderPlanBoard(console.PlanLedgerView{Columns: []console.BoardColumn{
		{ID: "remaining", Label: "Remaining", Cards: cards[:1]},
		{ID: "claimed", Label: "Claimed, not proven"},
		{ID: "op-measured", Label: "Measured (op)"},
	}})
	mustContain(t, "empty lanes", out, `data-kanban-col="remaining"`, `data-kanban-col="claimed"`, "data-board-empty", "No rows are measured (op)")
	if strings.Contains(out, `data-kanban-col="op-measured"`) {
		t.Error("an empty lane other than claimed-not-proven must not be drawn")
	}

	// Lanes share the row from md up (FATHOM-C-KIT-05): five fixed 16rem
	// lanes overflowed a 1440 content column and hid the last behind the
	// board's scroll. A kit change that renames the classes this rewrites
	// must fail here, not silently bring the scroll back.
	var five []console.BoardColumn
	for _, id := range []string{"remaining", "claimed", "proven", "op-waiting", "deferred"} {
		five = append(five, console.BoardColumn{ID: id, Label: id, Cards: cards[:1]})
	}
	out = renderPlanBoard(console.PlanLedgerView{Columns: five})
	if n := strings.Count(out, laneClass+" "+laneFitClass); n != 5 || !strings.Contains(out, boardClass+" "+boardFitClass) {
		t.Errorf("the board must fit its lanes to the column from md up (%d of 5 lanes fitted)", n)
	}
	// Each card's title link, the keyboard path into the drawer, is a 44 px
	// target with a visible focus ring.
	if !strings.Contains(out, "inline-flex min-h-11 items-center focus-visible:outline-2") {
		t.Error("a card's link must be a 44 px target with a focus ring")
	}
}

// Every console pager lifts the kit's ~30 px page links to 44 px targets.
func TestConsolePager_FortyFourPixelTargets(t *testing.T) {
	out := consolePager(chrome.Pagination{Page: 2, TotalPages: 3, AriaLabel: "Plans pages", HrefFor: func(n int) string { return fmt.Sprintf("/plans?page=%d", n) }})
	mustContain(t, "pager", out, "data-console-pager", "[&_a]:min-h-11", "[&_span]:min-h-11", `href="/plans?page=3"`)
	if consolePager(chrome.Pagination{Page: 1, TotalPages: 1}) != "" {
		t.Error("one page needs no pager")
	}
}

func TestConsolePlanRow_EvidenceReachOracleAndFragment(t *testing.T) {
	f := newConsoleFixture(t)
	code, _, page := f.get(t, "/plans/proof-service/settlement/rows/S-2")
	if code != http.StatusOK || h1Count(page) != 1 {
		t.Fatalf("row page = %d", code)
	}
	if !regexp.MustCompile(`href="/plans/proof-service/settlement"[^>]*>Settlement ledger \(fixture plan\)</a>`).MatchString(page) {
		t.Error("the row page's trail must lead back to its plan")
	}
	if !regexp.MustCompile(`aria-current="page"[^>]*>\s*S-2\s*<`).MatchString(page) {
		t.Error("the row page's own crumb is its id, current")
	}
	mustContain(t, "S-2", page,
		`id="row-contract"`, `id="row-verdict"`, `id="row-reach"`, `id="row-history"`,
		`data-reach-path`, "proof.TestReconcile_Matches", "proof.reconcileBoth", "proof.Reconcile", "2 hops from proof.TestReconcile_Matches",
		"first claimed", "refuted by the gate", "per-row verdicts",
		`data-console-copy="fathom verify --service proof-service --plan docs/plans/settlement.md --row S-2 --json"`,
	)
	_, _, oracle := f.get(t, "/plans/proof-service/settlement/rows/S-3")
	mustContain(t, "S-3", oracle, `id="row-oracle"`, "## Settlement window", "First match", "first at line 5")
	if strings.Contains(oracle, `id="row-reach"`) {
		t.Error("a lexical row makes no reach claim, so it has no reach panel")
	}
	assertRoleUtilitiesOnly(t, page)
	assertRoleUtilitiesOnly(t, oracle)

	code, hdr, frag := f.get(t, "/plans/proof-service/settlement/rows/S-1?format=fragment")
	if code != http.StatusOK || !strings.Contains(hdr.Get("Content-Type"), "text/html") || strings.Contains(frag, "<html") ||
		!strings.Contains(frag, `data-row-evidence data-row-id="S-1"`) || strings.Contains(frag, "data-live-source") {
		t.Errorf("fragment = %d:\n%.300s", code, frag)
	}
	if code, _, body := f.get(t, "/plans/proof-service/settlement/rows/NOPE"); code != http.StatusNotFound || !strings.Contains(body, "Unknown row") {
		t.Errorf("unknown row = %d", code)
	}
}

func TestConsoleCertifications_MatrixGapsAndScope(t *testing.T) {
	f := newConsoleFixture(t)
	pinConsoleVerbClocks(t, f)
	code, _, page := f.get(t, "/certifications")
	if code != http.StatusOK || h1Count(page) != 1 || !strings.Contains(page, ">Certifications</h1>") {
		t.Fatalf("/certifications = %d", code)
	}
	mustContain(t, "/certifications", page,
		`id="cert-matrix"`, `role="img"`, "The matrix as a table", `data-table="cert-matrix"`,
		`data-console-copy="fathom certify status --service proof-service --store `+consoleFixtureStorePath(t)+` --json"`,
		`data-table-row="gap-proof-service-incident-INC-1"`, `data-table-row="gap-proof-service-invariant-I2"`,
		"// certifies: invariant I2", "docs/intent/invariants.md:6",
	)
	assertRoleUtilitiesOnly(t, page)
	env := f.pageJSON(t, "/certifications")
	var m certificationsModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	if m.Declared.V != 3 || m.Live.V != 1 || m.GapCount.V != 2 || len(m.Rows) != 2 {
		t.Errorf("fleet certifications = declared %+v live %+v gaps %+v rows %d", m.Declared, m.Live, m.GapCount, len(m.Rows))
	}

	// A repository's scope names both verbs; its gaps section is the verb's.
	senv := f.pageJSON(t, "/certifications?svc=proof-service")
	gapsVerb := "fathom certify gaps --service proof-service --store " + consoleFixtureStorePath(t) + " --json"
	if len(senv.Verbs) != 2 || senv.Sections[gapsVerb] == nil {
		t.Fatalf("scoped verbs = %v", senv.Verbs)
	}
	args, dir := consoleVerbArgs(t, f, gapsVerb)
	var ob, eb bytes.Buffer
	if code := runIn(t, dir, args, &ob, &eb); code != 0 {
		t.Fatalf("certify gaps: %d %s", code, eb.String())
	}
	var fromVerb, fromPage any
	_ = json.Unmarshal(ob.Bytes(), &fromVerb)
	_ = json.Unmarshal(senv.Sections[gapsVerb], &fromPage)
	a, _ := json.Marshal(fromVerb)
	b, _ := json.Marshal(fromPage)
	if string(a) != string(b) {
		t.Errorf("the gaps section differs from the verb:\nverb %s\npage %s", a, b)
	}

	// A repository that declares nothing is named with its skip, not zero.
	_, _, quiet := f.get(t, "/certifications?svc=payment-service")
	mustContain(t, "payment-service", quiet, "nothing declared", "no targets, so no gaps to list")
}

func TestConsoleRefactors_PlanesNeverConservedByDefault(t *testing.T) {
	f := newConsoleFixture(t)
	code, _, chooser := f.get(t, "/refactors")
	if code != http.StatusOK || !strings.Contains(chooser, `data-refactor-form`) || !strings.Contains(chooser, `<option value="proof-service"`) {
		t.Fatalf("/refactors chooser = %d", code)
	}
	code, _, page := f.get(t, "/refactors?svc=proof-service&since=HEAD~1")
	if code != http.StatusOK || h1Count(page) != 1 {
		t.Fatalf("refactor page = %d", code)
	}
	// The kit's form controls are 36–38 px: the form lifts them to 44.
	mustContain(t, "refactor form", page, "[&_select]:min-h-11", "[&_input]:min-h-11", "[&_button]:min-h-11")
	mustContain(t, "refactor", page,
		`data-plane="generated" data-verdict="not run"`, "no plane by this name is registered",
		`data-plane="integrity" data-verdict="conserved"`,
		`id="reading-set"`, "changed functions", `data-table-row="item-0"`,
		`data-console-copy="fathom conserve --since HEAD~1 --service proof-service --store `+consoleFixtureStorePath(t)+` --json"`,
	)
	assertRoleUtilitiesOnly(t, page)
	env := f.pageJSON(t, "/refactors?svc=proof-service&since=HEAD~1")
	var m refactorsModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	for _, c := range m.Planes {
		if c.Plane == "generated" && c.Verdict != "not run" {
			t.Errorf("an unregistered plane reads %q, never conserved", c.Verdict)
		}
	}
	if m.Residue.Changed.V != 2 {
		t.Errorf("reading set = %+v", m.Residue.Changed)
	}
	// proof-service loads no coverage profile: which changed lines ran is
	// not measured, never a colored 0.
	if m.Residue.Partial.State != console.StateAbsent || m.Residue.Unexecuted.State != console.StateAbsent || !strings.Contains(m.Residue.Partial.Reason, "no execution evidence") {
		t.Errorf("partial/unexecuted with no execution evidence = %+v / %+v", m.Residue.Partial, m.Residue.Unexecuted)
	}
	for _, bad := range []string{"--help", "-x", "a b", "HEAD%3Brm"} {
		if code, _, _ := f.get(t, "/refactors?svc=proof-service&since="+strings.ReplaceAll(bad, " ", "%20")); code != http.StatusBadRequest {
			t.Errorf("since=%q = %d, want 400", bad, code)
		}
	}
	if code, _, body := f.get(t, "/refactors?svc=proof-service&since=no-such-ref"); code != http.StatusOK || !strings.Contains(body, "could not be made") {
		t.Errorf("an unresolvable ref = %d, want the page naming the failure", code)
	}
}

// A partial item's excerpt marks exactly the changed lines that never ran.
func TestConsoleRefactors_PartialItemExcerpt(t *testing.T) {
	r := console.ResidueView{
		State:   console.StatePresent,
		Changed: console.Measured(1, "x"), Attested: console.Measured(0, "x"), Partial: console.Measured(1, "x"),
		Unexecuted: console.Measured(0, "x"), Unattested: console.Measured(0, "x"), Unreached: console.Measured(0, "x"),
		Items: []console.ResidueItemView{{
			ResidueItem: console.ResidueItem{Symbol: "Settle", File: "settle.go", Line: 3, Class: "partial", Evidence: "executed", UnexecutedLines: []string{"5-6"}},
			Excerpt:     []console.ExcerptLine{{Number: 3, Text: "func Settle() {"}, {Number: 4, Text: "\tif x {"}, {Number: 5, Text: "\t\treturn", Unexecuted: true}, {Number: 6, Text: "\t}", Unexecuted: true}},
		}},
	}
	out := renderReadingSet(r)
	mustContain(t, "excerpt", out, "Settle</span> — changed lines that never ran", "Changed, never ran", `id="item0-L5"`, "unexecuted changed lines: 5-6")
	if strings.Count(out, "Changed, never ran") < 2 {
		t.Error("each unexecuted line must carry its label (and the legend its count)")
	}
	assertRoleUtilitiesOnly(t, out)
}

func TestConsoleProjectOverview_RealizationLadder(t *testing.T) {
	f := newConsoleFixture(t)
	_, _, page := f.get(t, "/projects/proof-service")
	i := strings.Index(page, "data-ladder")
	if i < 0 {
		t.Fatal("the project overview must lead with the realization ladder")
	}
	order := regexp.MustCompile(`data-rung="([a-z]+)"`).FindAllStringSubmatch(page, -1)
	var names []string
	for _, m := range order {
		names = append(names, m[1])
	}
	if strings.Join(names, ",") != "wired,fresh,queried,preventing,gating" {
		t.Errorf("rungs = %v", names)
	}
	mustContain(t, "ladder", page, `data-ladder-remedy`, "Remedy for <strong>wired</strong>", "fathom fleet reconcile --apply",
		`data-console-copy="fathom fleet realization --service proof-service --json"`, "grades idle, not failing")
	if strings.Count(page, "data-ladder-remedy") != 1 {
		t.Error("only the first unmet rung carries a remedy")
	}
	assertRoleUtilitiesOnly(t, page)
}

// SD-3 on the pages: with 14 days of this plan's gate runs in the verify
// ledger the burn-up draws the proven trend (and the portfolio its fleet
// trend), and the runs on a revision of the plan are cross-checked with it.
func TestConsolePlanBurnup_ProvenTrendAfterFourteenDays(t *testing.T) {
	f := newConsoleFixture(t)
	root := proofServiceRoot(t)
	out, err := exec.Command("git", "-C", root, "log", "-1", "--format=%H", "--", "docs/plans/settlement.md").Output()
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(out))[:8]
	var ledger strings.Builder
	for d := 0; d < 14; d++ {
		at := time.Date(2026, 9, 11+d, 8, 0, 0, 0, time.UTC).Format(time.RFC3339)
		fmt.Fprintf(&ledger, `{"schema_version":2,"at":%q,"repo_root":%q,"service":"proof-service","plan":"docs/plans/settlement.md","commit":%q,"total":10,"claimed":5,"verified":%d,"failing":%d,"origin":"cli"}`+"\n", at, root, commit, 3+d%2, 2-d%2)
		// What the ledger also keeps and the portfolio's trend must never
		// sum: a scratch service re-running the plan, a plan since
		// deleted, and README.md.
		fmt.Fprintf(&ledger, `{"schema_version":2,"at":%q,"repo_root":"/tmp/scratch","service":"proof-service-sv","plan":"docs/plans/settlement.md","total":10,"claimed":9,"verified":9,"failing":0}`+"\n", at)
		fmt.Fprintf(&ledger, `{"schema_version":2,"at":%q,"repo_root":%q,"service":"proof-service","plan":"docs/plans/retired.md","total":40,"claimed":40,"verified":40,"failing":0}`+"\n", at, root)
		fmt.Fprintf(&ledger, `{"schema_version":2,"at":%q,"repo_root":%q,"service":"proof-service","plan":"docs/plans/README.md","total":3,"claimed":3,"verified":0,"failing":0}`+"\n", at, root)
	}
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), ledger.String())

	env := f.pageJSON(t, "/plans/proof-service/settlement")
	var v planPageModel
	if err := json.Unmarshal(env.Model, &v); err != nil {
		t.Fatal(err)
	}
	if v.Burnup.Proven.State != console.StatePresent || v.Burnup.Proven.Days != 14 || len(v.Burnup.Proven.Points) != 14 {
		t.Fatalf("proven trend = %+v", v.Burnup.Proven)
	}
	if cc := v.Burnup.CrossCheck; cc.Matched != 14 || cc.Agree != 14 {
		t.Errorf("cross-check = %+v, want every run on the plan's revision agreeing", cc)
	}
	_, _, page := f.get(t, "/plans/proof-service/settlement")
	mustContain(t, "trend", page, `data-table="trend"`, "14 of 14 gate runs on a revision of this plan")
	penv := f.pageJSON(t, "/plans")
	var pm plansPortfolioModel
	_ = json.Unmarshal(penv.Model, &pm)
	if pm.Trend.State != console.StatePresent || pm.Trend.Days != 14 {
		t.Fatalf("fleet trend = %+v", pm.Trend)
	}
	if last := pm.Trend.Points[len(pm.Trend.Points)-1]; last.Plans != 1 || last.Claimed != 5 {
		t.Errorf("the fleet trend's last day = %+v, want settlement.md alone (the only current plan with ledger rows)", last)
	}
}

// rowOracle names a file that does not exist as the gate's path-absent (a
// measured nothing), never as a file that could not be read.
func TestRowOracle_MissingFileIsPathAbsent(t *testing.T) {
	m := rowOracle(t.TempDir(), console.PlanRowFact{OraclePath: "cmd/fathom/console_palette.go", OraclePattern: "palette"})
	if !m.Missing || m.Err == "" {
		t.Errorf("a missing oracle file = %+v", m)
	}
}

// TestConsoleProofPages_UnreachableRegistryIsNamed pins the fleet-scope
// Proof pages' answer when the registry cannot be read (the daemon busy
// verifying every plan timed its own list_services out in the browser
// proof): a 503 naming the registry, never "project  cannot be looked up"
// with no project, and never an empty portfolio.
func TestConsoleProofPages_UnreachableRegistryIsNamed(t *testing.T) {
	h := newUnreachableConsoleHandler(t)
	for _, path := range []string{"/plans", "/certifications"} {
		resetObsCache()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		body := rec.Body.String()
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(body, "Registry unreachable") || !strings.Contains(body, "no repository can be listed") || strings.Contains(body, "project  cannot") {
			t.Errorf("%s with an unreachable registry = %d:\n%.400s", path, rec.Code, body)
		}
	}
	resetObsCache()
}

// TestConsoleProjectLadder_WindowAndVerifyLedger pins the ladder's inputs
// to its design (docs/design/realization-grade.md §2): every rung reads the
// last fourteen days, never all time, and gating reads the verify ledger
// (SD-1), the ledger a pre-push gate's runs reach — a repository whose gate
// runs on every push is never told to add the gate. A ledger that does not
// exist is not measured, never "0 runs".
func TestConsoleProjectLadder_WindowAndVerifyLedger(t *testing.T) {
	f := newConsoleFixture(t)
	ladder := func() console.Ladder {
		t.Helper()
		resetObsCache()
		env := f.pageJSON(t, "/projects/"+proofServiceName)
		var l console.Ladder
		if err := json.Unmarshal(env.Sections[realizationVerb(proofServiceName)], &l); err != nil {
			t.Fatal(err)
		}
		return l
	}
	if g := ladder().Rungs[4]; g.State != console.RungUnknown || !strings.Contains(g.Evidence, "no verify ledger at") {
		t.Errorf("no verify ledger = %+v, want unknown naming the missing ledger", g)
	}

	root := proofServiceRoot(t)
	row := func(at, service, plan string) string {
		return fmt.Sprintf(`{"schema_version":2,"at":%q,"repo_root":%q,"service":%q,"plan":%q,"total":3,"claimed":2,"verified":2,"failing":0,"origin":"cli"}`+"\n", at, root, service, plan)
	}
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"),
		row("2026-08-01T09:00:00Z", proofServiceName, "docs/plans/settlement.md")+ // before the window
			row("2026-09-20T09:00:00Z", proofServiceName, "docs/plans/settlement.md")+
			row("2026-09-20T09:00:00Z", proofServiceName, "docs/plans/audit.md")+ // the same run's second plan
			row("2026-09-24T09:00:00Z", proofServiceName, "docs/plans/settlement.md")+
			row("2026-09-24T10:00:00Z", proofServiceName+"-sv", "docs/plans/settlement.md")) // a scratch service
	appendTo := func(path, text string) {
		t.Helper()
		fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatal(err)
		}
		defer fh.Close()
		if _, err := fh.WriteString(text); err != nil {
			t.Fatal(err)
		}
	}
	appendTo(f.callLog, `{"ts":"2026-08-01T10:00:00Z","tool":"obligations","service":"proof-service","surface":"","latency_ms":2}
{"ts":"2026-08-02T10:00:00Z","tool":"obligations","service":"proof-service","surface":"","latency_ms":2}
{"ts":"2026-09-23T10:00:00Z","tool":"obligations","service":"proof-service","surface":"","latency_ms":2}
`)
	appendTo(f.ledger, `{"schema_version":1,"session_id":"s-july","arm":"on","repo":"proof-service","at":"2026-07-20T10:00:00Z","turn_count":2}
`)

	l := ladder()
	byName := map[string]console.Rung{}
	for _, r := range l.Rungs {
		byName[r.Name] = r
	}
	if g := byName["gating"]; g.State != console.RungMet || !strings.Contains(g.Evidence, "2 verify run(s) in the last 14 days, recorded in the verify ledger") {
		t.Errorf("gating = %+v, want met on the verify ledger's two runs in the window", g)
	}
	if p := byName["preventing"]; p.State != console.RungMet || !strings.Contains(p.Evidence, "1 rule injection(s) in the last 14 days") {
		t.Errorf("preventing = %+v, want the window's one injection, not all three", p)
	}
	if q := byName["queried"]; !strings.Contains(q.Evidence, "1 call(s) in the last 14 days") {
		t.Errorf("queried = %+v", q)
	}
	// Its only session is from July: idle in the window, however many
	// sessions it had ever.
	if !l.Idle || l.Sessions != 0 || l.Grade != "idle" {
		t.Errorf("a repository silent for the window = idle %v, sessions %d, grade %q", l.Idle, l.Sessions, l.Grade)
	}
	var fleet observatoryModel
	resetObsCache()
	if err := json.Unmarshal([]byte(mustGetBody(t, f, "/observatory?format=json")), &fleet); err != nil {
		t.Fatal(err)
	}
	for _, s := range fleet.Services {
		if s.Service == proofServiceName && (s.SessionCount == nil || *s.SessionCount != 1) {
			t.Errorf("the fleet's all-time session column = %v, want 1 (the ladder's window is its own)", s.SessionCount)
		}
	}
}

func mustGetBody(t *testing.T, f *consoleFixture, path string) string {
	t.Helper()
	code, _, body := f.get(t, path)
	if code != http.StatusOK {
		t.Fatalf("GET %s = %d", path, code)
	}
	return body
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// A store-direct verb's chip names the daemon's store, quoted for a shell
// when its path needs it: copied as shown it reads what the page read.
func TestConsoleStoreArg_NamesTheDaemonsStore(t *testing.T) {
	if got := (&consoleDeps{storePath: "/v/workspace/db/fleet.sqlite"}).storeArg(); got != " --store /v/workspace/db/fleet.sqlite" {
		t.Errorf("storeArg = %q", got)
	}
	if got := (&consoleDeps{storePath: "/Users/a b/it's.sqlite"}).storeArg(); got != ` --store '/Users/a b/it'\''s.sqlite'` {
		t.Errorf("a path with a space and a quote = %q", got)
	}
	if got := (&consoleDeps{}).storeArg(); got != "" {
		t.Errorf("no store path = %q", got)
	}
}

// A row's evidence opened in the plan page's drawer is a person's view of
// the row page (its ?format=fragment is what the drawer fetches), so it is
// counted for UX-M4 like the page itself; the palette's fragment reads are
// keystrokes, not views, and a JSON read never is.
func TestConsolePageViews_DrawerFragmentIsAView(t *testing.T) {
	f := newConsoleFixture(t)
	route := "/plans/{svc}/{plan}/rows/{id}"
	for i := 0; i < 2; i++ {
		if code, _, frag := f.get(t, "/plans/proof-service/settlement/rows/S-1?format=fragment"); code != http.StatusOK || !strings.Contains(frag, "data-row-evidence") {
			t.Fatalf("drawer fragment = %d", code)
		}
	}
	f.pageJSON(t, "/plans/proof-service/settlement/rows/S-1")
	f.get(t, "/search?q=settle&format=fragment")
	_, _, metrics := f.get(t, "/metrics")
	if !strings.Contains(metrics, `fathom_console_page_views_total{route="`+route+`"} 2`) {
		t.Errorf("two drawer opens must count two views of %s:\n%s", route, metrics)
	}
	if strings.Contains(metrics, `fathom_console_page_views_total{route="/search"}`) {
		t.Error("a palette fragment read is not a page view")
	}
	logged, _ := os.ReadFile(f.viewsLog)
	if n := strings.Count(string(logged), `"route":"`+route+`"`); n != 2 {
		t.Errorf("durable log holds %d row-evidence views, want 2:\n%s", n, logged)
	}
}

// The drawer's inserted evidence gets its Copy buttons: console.js reveals
// the chips of any content announced with 'console:content', which the
// drawer script dispatches once the row's fragment is in place.
func TestConsoleJS_RevealsAnnouncedContent(t *testing.T) {
	f := newConsoleFixture(t)
	code, _, js := f.get(t, "/assets/console.js")
	if code != http.StatusOK || !strings.Contains(js, "d.addEventListener('console:content'") || !strings.Contains(js, "reveal(e.target)") {
		t.Errorf("console.js must reveal announced content's chips (%d)", code)
	}
	_, _, frag := f.get(t, "/plans/proof-service/settlement/rows/S-1?format=fragment")
	if !strings.Contains(frag, "data-console-copy=") {
		t.Error("the drawer's fragment carries the verdict's verb chip")
	}
}
