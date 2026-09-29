package main

// console_progress.go — /progress (PE-4, PE-5, PE-7, PE-11; design
// docs/design/progress-and-economics.md §4): the Progress views over
// computed state. Every view is read-only (ADR-049 decision 4): the
// weighted table and the frontier are the gate's verdicts on the current
// plans (the plans cache the /plans pages share), the derived kanban,
// burn-up, regressions, time-to-proof, claim honesty, heatmap and goal
// gauges are projections of the verify ledger's proof history
// (console.ProofHistory — a bounded tail read, cached). Filters (service,
// plan, wave, goal, agent identity, date range) live in the URL.

import (
	"fmt"
	"html"
	"math"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"html/template"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/kanban"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/intent"
)

func init() {
	declarePage(consolePage{
		ID: "progress", Route: "/progress", Href: "/progress", Group: "Proof", Order: 2, Nav: true,
		Title:    "Progress",
		Question: "How much of the planned weight does the gate prove, what regressed, what is next, and when will it finish?",
		Build:    buildProgressPage,
	})
}

// progressPageModel is /progress's JSON model.
type progressPageModel struct {
	Filter      console.HistoryFilter     `json:"filter"`
	History     console.SourceRead        `json:"history_read"`
	Weighted    weightedScope             `json:"weighted"`
	Pending     []string                  `json:"pending_plans,omitempty"`
	Board       []console.ProgressColumn  `json:"board"`
	BurnUp      console.BurnUpModel       `json:"burn_up"`
	Frontier    []progressFrontierRow     `json:"frontier"`
	Regressions []console.Regression      `json:"regressions"`
	TimeToProof console.TimeToProofModel  `json:"time_to_proof"`
	Honesty     console.ClaimHonestyModel `json:"claim_honesty"`
	Heatmap     console.HeatmapModel      `json:"heatmap"`
	Goals       []console.GoalGauge       `json:"goals"`
	GoalsNote   string                    `json:"goals_note,omitempty"`
	Waves       []string                  `json:"waves"`
	Rows        int                       `json:"history_rows"`
	Readings    int                       `json:"history_readings"`
}

// progressFrontierRow is a frontier row with its repository.
type progressFrontierRow struct {
	Service string `json:"service"`
	frontierRow
}

// progressFilterFrom reads the view filter from the URL.
func progressFilterFrom(q url.Values) console.HistoryFilter {
	day := func(s string) string {
		if _, err := time.Parse("2006-01-02", s); err == nil {
			return s
		}
		return ""
	}
	return console.HistoryFilter{
		Service: q.Get("svc"), Plan: q.Get("plan"), Wave: q.Get("wave"), Goal: q.Get("goal"),
		Agent: q.Get("agent"), From: day(q.Get("from")), To: day(q.Get("to")),
	}
}

// progressGoals is one root's goal bets and conditions, cached while its
// scope.md and HEAD are unchanged.
type progressGoals struct {
	bets  []intent.GoalBet
	conds []intent.Condition
	note  string
}

var progressGoalsMemo = consoleMemo[progressGoals]{max: 16}

func goalsFor(root string, now time.Time) progressGoals {
	key := root + "\x00" + fileWatermark(filepath.Join(root, "docs", "intent", "scope.md")) + "\x00" + repoHeadNoFork(root)
	g, _ := progressGoalsMemo.do(key, now, func() (progressGoals, time.Duration) {
		c, err := intent.Load(root)
		if err != nil {
			return progressGoals{note: err.Error()}, 30 * time.Second
		}
		bets, _ := intent.LoadBets(c, nil)
		return progressGoals{bets: bets, conds: c.Conditions}, 5 * time.Minute
	})
	return g
}

func buildProgressPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	f := progressFilterFrom(cr.r.URL.Query())
	var roots []serviceRoot
	for _, r := range modelServiceRoots(m) {
		if f.Service == "" || strings.EqualFold(r.Service, f.Service) {
			roots = append(roots, r)
		}
	}
	model := progressPageModel{Filter: f}

	// Goals: the scope bets of the roots in view; a goal filter narrows
	// every view to the plans its condition names.
	var bets []intent.GoalBet
	var conds []intent.Condition
	for _, r := range roots {
		if r.Root == "" {
			continue
		}
		g := goalsFor(r.Root, cr.now)
		bets, conds = append(bets, g.bets...), append(conds, g.conds...)
		if g.note != "" {
			model.GoalsNote = strings.TrimSpace(model.GoalsNote + " " + r.Service + ": " + g.note)
		}
	}
	if f.Goal != "" {
		f.GoalPlans = console.GoalFilter(bets, conds, f.Goal).Plans
		if len(f.GoalPlans) == 0 {
			f.GoalPlans = []string{"\x00no plan"} // a goal naming no plan selects nothing
		}
	}

	// The gate's verdicts on the current plans: the weighted table, the
	// frontier, and the wave labels the wave filter resolves against.
	pc := plansFor(cr.deps)
	sps := snapshotPlans(cr.r.Context(), pc, roots, "", consolePlansPortfolioWait)
	var projects []weightedScope
	waveRows := map[string]bool{}
	waves := map[string]bool{}
	leasesByRoot := map[string][]leaseRow{}
	for i, sp := range sps {
		var files []string
		var sets [][]planRow
		for _, plan := range sp.Plans {
			if f.Plan != "" && f.Plan != plan.Rel && filepath.Base(f.Plan) != filepath.Base(plan.Rel) {
				continue
			}
			if len(f.GoalPlans) > 0 && !containsBase(f.GoalPlans, plan.Rel) {
				continue
			}
			rows, ok := planRowsAs(pc, plan)
			if !ok {
				model.Pending = append(model.Pending, sp.Service+" "+plan.Rel)
				continue
			}
			for _, r := range rows {
				label := r.Section
				if label == "" {
					label = noSection
				}
				waves[label] = true
				if f.Wave == label {
					waveRows[console.RowKey(sp.Service, plan.Rel, r.Task.ID)] = true
				}
			}
			if f.Wave != "" {
				rows = filterRows(rows, func(r planRow) bool { return r.Section == f.Wave })
			}
			files, sets = append(files, plan.Rel), append(sets, rows)
			root := roots[i].Root
			if _, ok := leasesByRoot[root]; !ok {
				leasesByRoot[root], _ = readLeases(root)
			}
			for _, fr := range frontierForPlan(rows, leasesByRoot[root], cr.now) {
				if fr.Eligible {
					model.Frontier = append(model.Frontier, progressFrontierRow{Service: sp.Service, frontierRow: fr})
				}
			}
		}
		if len(files) > 0 {
			projects = append(projects, weightedProject(sp.Service, files, sets))
		}
	}
	if model.Frontier == nil {
		model.Frontier = []progressFrontierRow{}
	}
	model.Weighted = weightedFleet(projects)
	model.Waves = progressSorted(waves)
	if f.Wave != "" {
		f.WaveRows = waveRows
	}

	// The proof history and its projections.
	h, read := console.ProofHistory(cr.r.Context(), verifyLedgerPath(), console.DefaultHistoryTail, m.ledgerRoots())
	model.History, model.Rows, model.Readings = read, len(h.Rows), len(h.Readings)
	model.Board = console.DerivedBoard(h, f)
	model.BurnUp = console.BurnUp(h, f, cr.now)
	model.Regressions = console.ProofRegressions(h, f)
	if model.Regressions == nil {
		model.Regressions = []console.Regression{}
	}
	model.TimeToProof = console.TimeToProof(h, f)
	model.Honesty = console.ClaimHonesty(h, agentSessionsOf(m), f)
	model.Heatmap = console.ProofHeatmap(h, f, cr.now)
	model.Goals = console.GoalGauges(bets, conds, h)
	if f.Goal != "" {
		model.Goals = filterGoals(model.Goals, f.Goal)
	}
	model.Filter = f

	meta := []string{fmt.Sprintf("%s of proof history", plural(len(h.Rows), "row")), "generated " + m.GeneratedAt}
	return consoleOut{
		Meta: meta, Model: model, Verbs: []string{},
		Body: func() string { return renderProgressPage(model) },
	}, nil
}

func containsBase(plans []string, rel string) bool {
	for _, p := range plans {
		if p == rel || filepath.Base(p) == filepath.Base(rel) {
			return true
		}
	}
	return false
}

func filterRows(rows []planRow, keep func(planRow) bool) []planRow {
	out := rows[:0:0]
	for _, r := range rows {
		if keep(r) {
			out = append(out, r)
		}
	}
	return out
}

func filterGoals(gs []console.GoalGauge, goal string) []console.GoalGauge {
	var out []console.GoalGauge
	for _, g := range gs {
		if g.Goal == goal {
			out = append(out, g)
		}
	}
	return out
}

func progressSorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// agentSessionsOf windows each session from its last ledger row: the row's
// time back over its recorded duration. The session ledger records neither
// harness nor model today, so both read "not recorded" — named, never
// guessed (I58: the identity is an agent's, never a person's).
func agentSessionsOf(m observatoryModel) []console.AgentSession {
	if m.ledgerErr != nil || m.ledger == nil {
		return nil
	}
	out := make([]console.AgentSession, 0, len(m.ledger.order))
	for _, k := range m.ledger.order {
		e := m.ledger.latest[k]
		end, err := time.Parse(time.RFC3339, e.At)
		if err != nil || e.Repo == "" {
			continue
		}
		start := end
		if e.DurationMS != nil {
			start = end.Add(-time.Duration(*e.DurationMS) * time.Millisecond)
		}
		out = append(out, console.AgentSession{ID: e.SessionID, Repo: e.Repo,
			Start: start.UTC().Format(time.RFC3339), End: end.UTC().Format(time.RFC3339)})
	}
	return out
}

// ─── rendering ──────────────────────────────────────────────────────────────

const progressLive = "plans"

func renderProgressPage(m progressPageModel) string {
	var b strings.Builder
	b.WriteString(renderProgressFilters(m))
	src := "the verify ledger's proof history (" + m.History.Path + ")"
	histState, histReason := m.History.State, m.History.Reason
	if histState == console.StatePresent && m.Rows == 0 {
		histState, histReason = console.StateAbsent, "the verify ledger has no schema-3 reading yet — the per-row views draw once `fathom verify` records per-row verdicts"
	}
	panel := func(p consolePanel) {
		b.WriteString(`<div class="mt-4">` + renderPanel(p) + `</div>`)
	}
	panel(consolePanel{ID: "goals", Title: "Goal gauges", SourceKey: progressLive,
		Question: "For each scope bet, how many of its measurement rows does the gate prove, and when is it reviewed?",
		Source:   "docs/intent/scope.md bets; " + src, Denominator: plural(len(m.Goals), "goal"),
		State: histState, Reason: histReason, Body: renderGoalGauges(m.Goals, m.GoalsNote)})
	weightedBody := renderWeightedTable(m.Weighted)
	if len(m.Pending) > 0 {
		weightedBody += `<p class="mt-2 text-xs text-ink-muted" data-weighted-pending>` + html.EscapeString("Not yet verified at the current index, so not counted: "+strings.Join(m.Pending, ", ")) + `</p>`
	}
	panel(consolePanel{ID: "weighted", Title: "Weighted progress", SourceKey: progressLive,
		Question: "Of the planned weight, how much does the gate prove — by fleet, repository, plan and wave?",
		Verb:     "fathom plans status --service <svc> --weighted --json",
		Source:   "the verify engine at the current index; each row's weight: (1 when unset)", Denominator: fmt.Sprintf("%d weight across %s", m.Weighted.Total, plural(m.Weighted.Rows, "row")),
		Body: weightedBody})
	panel(consolePanel{ID: "board", Title: "Rows by computed state", SourceKey: progressLive,
		Question: "Which rows are open, claimed but unproven, proven, or refused — as the last gate reading says?",
		Source:   src, Denominator: plural(boardCards(m.Board), "row"), State: histState, Reason: histReason,
		Body: renderDerivedKanban(m.Board)})
	panel(consolePanel{ID: "frontier", Title: "Frontier", SourceKey: progressLive,
		Question: "Which rows can be worked now — every needs: target satisfied?",
		Verb:     "fathom plans next --service <svc> --eligible-only --json",
		Source:   "the verify engine at the current index; .fathom/leases.jsonl", Denominator: plural(len(m.Frontier), "eligible row"),
		Body: renderFrontier(m.Frontier)})
	panel(consolePanel{ID: "burnup", Title: "Burn-up", SourceKey: progressLive,
		Question: "How has proven weight grown per day, and when does the trend finish?",
		Source:   src, Denominator: plural(len(m.BurnUp.Points), "day"), State: histState, Reason: histReason,
		Body: renderProgressBurnUp(m.BurnUp)})
	panel(consolePanel{ID: "regressions", Title: "Proof regressions", SourceKey: progressLive,
		Question: "Which rows were once proven and now read otherwise — and at which commit did that change?",
		Source:   src, Denominator: plural(len(m.Regressions), "row"), State: histState, Reason: histReason,
		Body: renderRegressions(m.Regressions)})
	panel(consolePanel{ID: "time-to-proof", Title: "Time to proof", SourceKey: progressLive,
		Question: "From a row's first appearance to its first proven reading, how long did proof take?",
		Source:   src, Denominator: plural(len(m.TimeToProof.Rows), "row"), State: histState, Reason: histReason,
		Body: renderTimeToProof(m.TimeToProof)})
	panel(consolePanel{ID: "honesty", Title: "Claim honesty", SourceKey: progressLive + " sessions",
		Question: "Of the rows each agent identity claimed, how many did the gate not prove? Per harness and model, never per person.",
		Source:   src + " joined to the session ledger's session windows", Denominator: plural(m.Honesty.Readings, "reading"), State: histState, Reason: histReason,
		Body: renderClaimHonesty(m.Honesty)})
	panel(consolePanel{ID: "heatmap", Title: "Proof heatmap", SourceKey: progressLive,
		Question: "Where did proof land — newly proven weight by plan and week — and is there a streak?",
		Source:   src, Denominator: plural(len(m.Heatmap.Cells), "plan-week"), State: histState, Reason: histReason,
		Body: renderProofHeatmap(m.Heatmap)})
	return b.String()
}

func boardCards(cols []console.ProgressColumn) int {
	n := 0
	for _, c := range cols {
		n += len(c.Cards)
	}
	return n
}

// renderProgressFilters is the kit's form over GET: every filter is a URL
// parameter, so a filtered view is a link.
func renderProgressFilters(m progressPageModel) string {
	f := m.Filter
	input := func(id, name, label, val, typ string) string {
		return form.RenderField(form.Field{Label: label, For: id, Control: template.HTML(form.RenderInput(form.Input{ID: id, Name: name, Value: val, Type: typ}))})
	}
	waves := []form.Option{{Value: "", Label: "every wave"}}
	for _, w := range m.Waves {
		waves = append(waves, form.Option{Value: w, Label: w})
	}
	var b strings.Builder
	b.WriteString(`<form method="get" action="/progress" class="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4 [&_input]:min-h-11 [&_select]:min-h-11" aria-label="filter every view" data-progress-filters>`)
	b.WriteString(input("progress-svc", "svc", "Service", f.Service, "text"))
	b.WriteString(input("progress-plan", "plan", "Plan", f.Plan, "text"))
	b.WriteString(form.RenderField(form.Field{Label: "Wave", For: "progress-wave", Control: template.HTML(form.RenderSelect(form.Select{ID: "progress-wave", Name: "wave", Value: f.Wave, Options: waves}))}))
	b.WriteString(input("progress-goal", "goal", "Goal (G-id)", f.Goal, "text"))
	b.WriteString(input("progress-agent", "agent", "Agent (harness or harness/model)", f.Agent, "text"))
	b.WriteString(input("progress-from", "from", "From", f.From, "date"))
	b.WriteString(input("progress-to", "to", "To", f.To, "date"))
	b.WriteString(`<div class="flex items-end gap-2 [&>a]:min-h-11 [&>button]:min-h-11">` + form.RenderButton(form.Button{Label: "Filter", Variant: form.ButtonPrimary}) +
		form.RenderButton(form.Button{Label: "Clear", Href: "/progress", Variant: form.ButtonGhost}) + `</div>`)
	b.WriteString(`</form>`)
	return b.String()
}

// renderWeightedTable renders the weighted rollup (PE-4): one table row
// per scope, indented fleet → repository → plan → wave, each with its
// proven weight over total weight as a meter.
func renderWeightedTable(root weightedScope) string {
	if root.Total == 0 && len(root.Children) == 0 {
		return `<p class="text-ink-muted" data-weighted-empty>No verified plan in this selection — nothing to weigh.</p>`
	}
	var rows []table.Row
	var walk func(s weightedScope, depth int, id string)
	walk = func(s weightedScope, depth int, id string) {
		name := s.Name
		if s.Scope == "plan" {
			name = shortPlanName(name)
		}
		label := `<span class="` + indentClass(depth) + `"><span class="text-xs text-ink-muted">` + html.EscapeString(s.Scope) + `</span> ` + html.EscapeString(name) + `</span>`
		meter := `<div class="w-40 max-w-full">` + viz.MeterRow(float64(s.Proven), math.Max(float64(s.Total), 1), viz.MeterOptions{AriaLabel: fmt.Sprintf("%s %s: %d of %d weight proven", s.Scope, name, s.Proven, s.Total)}) + `</div>`
		rows = append(rows, table.Row{ID: "w-" + id,
			Cells:     []string{s.Scope + " " + name, "", fmtThousands(s.Rows), weightedShare(s)},
			HTMLCells: []templ.Component{templ.Raw(label), templ.Raw(meter)}})
		for i, c := range s.Children {
			walk(c, depth+1, fmt.Sprintf("%s-%d", id, i))
		}
	}
	walk(root, 0, "0")
	return renderKitTable(table.TableParams{
		TableID: "progress-weighted", Caption: "Proven weight over total weight; a refused or deferred row weighs nothing",
		Columns: []table.Column{{Key: "scope", Label: "Scope"}, {Key: "meter", Label: "Proven"}, {Key: "rows", Label: "Rows", Align: "right"}, {Key: "share", Label: "Proven / total weight", Align: "right"}},
		Rows:    rows,
	})
}

// indentClass indents a weighted row by its depth (complete literals for
// the CSS build).
func indentClass(depth int) string {
	switch depth {
	case 0:
		return "font-semibold"
	case 1:
		return "pl-3"
	case 2:
		return "pl-6"
	default:
		return "pl-9"
	}
}

// progressBoardCards caps one lane's cards; the badge counts all of them.
const progressBoardCards = 12

// renderDerivedKanban renders the derived board (PE-5) on the kit's kanban,
// read-only: no card is draggable, and a claimed-unproven card links to its
// row page with the failing criterion as its meta line.
func renderDerivedKanban(cols []console.ProgressColumn) string {
	kcols := make([]kanban.Column, 0, len(cols))
	var cards []kanban.CardItem
	cut := 0
	for _, c := range cols {
		kcols = append(kcols, kanban.Column{ID: c.ID, Label: c.Label, Count: len(c.Cards)})
		shown := c.Cards
		if len(shown) > progressBoardCards {
			cut += len(shown) - progressBoardCards
			shown = shown[:progressBoardCards]
		}
		for _, card := range shown {
			meta := card.Criterion
			if meta == "" {
				meta = "since " + card.Since
			}
			if len(meta) > 140 {
				meta = meta[:137] + "…"
			}
			badges := []kanban.Badge{{Label: "weight " + fmt.Sprint(card.Weight), Role: "surface-sunken"}}
			if card.Criterion != "" {
				badges = append(badges, kanban.Badge{Label: "failing criterion", Role: "danger-soft"})
			}
			cards = append(cards, kanban.CardItem{
				ID: c.ID + "-" + card.Service + "-" + card.ID, StageID: c.ID, Title: card.ID, Subtitle: card.Service + " · " + shortPlanName(card.Plan),
				Badges: badges, Meta: meta, Href: planRowHref(card.Service, console.PlanSlug(card.Plan), card.ID),
			})
		}
	}
	note := ""
	if cut > 0 {
		note = `<p class="mt-2 text-xs text-ink-muted">` + fmt.Sprintf("Each lane shows its first %d cards; %s more are counted in the lane badges.", progressBoardCards, plural(cut, "row")) + `</p>`
	}
	return `<div role="region" aria-label="Rows by computed state, read-only" data-derived-board data-read-only>` + readOnlyBoard(renderKanbanBoard(kcols, cards)) + `</div>` + note
}

// renderFrontier renders `plans next --eligible-only` (PE-7): rows whose
// needs: are satisfied, each linking to its row page.
func renderFrontier(rows []progressFrontierRow) string {
	if len(rows) == 0 {
		return `<p class="text-ink-muted" data-frontier-empty>No eligible row in this selection: every open row waits on a needs: target, or none is open.</p>`
	}
	trs := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		rel := shortPlanName(r.Plan)
		link := `<a class="inline-flex min-h-11 items-center font-mono text-xs text-accent hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="` + html.EscapeString(planRowHref(r.Service, console.PlanSlug(r.Plan), r.ID)) + `">` + html.EscapeString(r.ID) + `</a>`
		trs = append(trs, table.Row{ID: "f-" + r.Service + "-" + rel + "-" + r.ID, Cells: []string{r.ID, r.Service + " " + rel, r.Title, r.State}, HTMLCells: []templ.Component{templ.Raw(link)}})
	}
	return renderKitTable(table.TableParams{
		TableID: "progress-frontier", Caption: "Rows every needs: target of which is proven or dispositioned",
		Columns: []table.Column{{Key: "id", Label: "Row"}, {Key: "plan", Label: "Plan"}, {Key: "title", Label: "Title"}, {Key: "state", Label: "State"}},
		Rows:    trs,
	})
}

func renderProgressBurnUp(b console.BurnUpModel) string {
	var out strings.Builder
	p := b.Projection
	switch p.State {
	case console.StatePresent:
		hi := p.V.High
		if hi == "" {
			hi = "no late bound (the slope's lower bound is not positive)"
		}
		out.WriteString(`<p class="text-sm text-ink" data-projection>` + html.EscapeString(fmt.Sprintf("Projected finish %s (interval %s to %s): a linear fit over %d days, %.2f weight a day (±%.2f).", p.V.Finish, p.V.Low, hi, p.V.Days, p.V.Slope, p.V.SE)) + `</p>`)
	default:
		out.WriteString(`<p class="text-sm italic text-ink-muted" data-projection-withheld>` + html.EscapeString("No projected finish — "+p.Reason) + `</p>`)
	}
	if len(b.Points) == 0 {
		return out.String()
	}
	vals := make([]float64, len(b.Points))
	for i, pt := range b.Points {
		vals[i] = float64(pt.Proven)
	}
	out.WriteString(`<div class="mt-2 w-full max-w-xl">` + viz.Sparkline(vals, viz.SparklineOptions{AriaLabel: "proven weight per day"}) + `</div>`)
	rows := make([]table.Row, 0, len(b.Points))
	for i := len(b.Points) - 1; i >= 0; i-- {
		pt := b.Points[i]
		rows = append(rows, table.Row{ID: "b-" + pt.Day, Cells: []string{pt.Day, fmtThousands(pt.Proven), fmtThousands(pt.Total)}})
	}
	out.WriteString(renderKitTable(table.TableParams{TableID: "progress-burnup", Caption: "Proven weight at each day's end, newest first",
		Columns: []table.Column{{Key: "day", Label: "Day (UTC)"}, {Key: "proven", Label: "Proven weight", Align: "right"}, {Key: "total", Label: "Total weight", Align: "right"}},
		Rows:    rows}))
	return out.String()
}

func renderRegressions(regs []console.Regression) string {
	if len(regs) == 0 {
		return `<p class="text-ink-muted" data-regressions-empty>No row once proven reads otherwise in this selection.</p>`
	}
	rows := make([]table.Row, 0, len(regs))
	for _, r := range regs {
		link := `<a class="inline-flex min-h-11 items-center font-mono text-xs text-accent hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="` + html.EscapeString(planRowHref(r.Service, console.PlanSlug(r.Plan), r.ID)) + `">` + html.EscapeString(r.ID) + `</a>`
		rows = append(rows, table.Row{ID: "r-" + r.Service + "-" + r.ID,
			Cells:     []string{r.ID, shortPlanName(r.Plan), r.ProvenAt + " @ " + orDash(r.ProvenCommit), r.ChangedAt + " @ " + orDash(r.ChangedCommit), r.State, r.Criterion},
			HTMLCells: []templ.Component{templ.Raw(link)}})
	}
	return renderKitTable(table.TableParams{TableID: "progress-regressions", Caption: "Newest change first: when the row was proven, and the first reading that changed",
		Columns: []table.Column{{Key: "id", Label: "Row"}, {Key: "plan", Label: "Plan"}, {Key: "proven", Label: "Proven at"}, {Key: "changed", Label: "First changed at"}, {Key: "state", Label: "Now"}, {Key: "crit", Label: "Criterion"}},
		Rows:    rows})
}

func renderTimeToProof(m console.TimeToProofModel) string {
	var out strings.Builder
	out.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("rows proven", m.Proven),
		valueStat("median hours to proof", m.MedianHours),
	}, stat.BandOptions{AriaLabel: "time to proof at a glance"}))
	var rows []table.Row
	for _, r := range m.Rows {
		if r.FirstProven == "" {
			continue
		}
		hours := fmtThousands(r.Hours)
		if r.LowerBound {
			hours = "≥ " + hours
		}
		rows = append(rows, table.Row{ID: "t-" + r.Service + "-" + r.ID, Cells: []string{r.ID, shortPlanName(r.Plan), r.FirstSeen, r.FirstProven, hours}})
	}
	if len(rows) > 0 {
		out.WriteString(`<div class="mt-3">` + renderKitTable(table.TableParams{TableID: "progress-ttp", Caption: "≥ marks a row that existed before the history began",
			Columns: []table.Column{{Key: "id", Label: "Row"}, {Key: "plan", Label: "Plan"}, {Key: "seen", Label: "First seen"}, {Key: "proven", Label: "First proven"}, {Key: "hours", Label: "Hours", Align: "right"}},
			Rows:    rows}) + `</div>`)
	}
	return out.String()
}

func renderClaimHonesty(m console.ClaimHonestyModel) string {
	var out strings.Builder
	if len(m.Rows) == 0 {
		out.WriteString(`<p class="text-ink-muted">No reading falls inside a recorded session window.</p>`)
	} else {
		rows := make([]table.Row, 0, len(m.Rows))
		for _, r := range m.Rows {
			rate := "not measured — " + r.Rate.Reason
			if r.Rate.Known() {
				rate = fmt.Sprintf("%.0f%%", r.Rate.V*100)
			}
			rows = append(rows, table.Row{ID: "h-" + r.Harness + "-" + r.Model, Cells: []string{r.Harness, r.Model, fmtThousands(r.Readings), fmtThousands(r.Claimed), fmtThousands(r.Unproven), rate}})
		}
		out.WriteString(renderKitTable(table.TableParams{TableID: "progress-honesty", Caption: "Claimed rows the gate did not prove, per agent identity",
			Columns: []table.Column{{Key: "harness", Label: "Harness"}, {Key: "model", Label: "Model"}, {Key: "readings", Label: "Readings", Align: "right"}, {Key: "claimed", Label: "Claimed", Align: "right"}, {Key: "unproven", Label: "Unproven", Align: "right"}, {Key: "rate", Label: "Unproven rate", Align: "right"}},
			Rows:    rows}))
	}
	out.WriteString(`<p class="mt-2 text-xs text-ink-muted" data-unattributed>` + html.EscapeString(fmt.Sprintf("%s no session window covers (a person's CLI run, a CI gate) — not attributed to anyone.", plural(m.Unattributed, "reading"))) + `</p>`)
	return out.String()
}

// renderProofHeatmap renders the heatmap (PE-11): newly proven weight by
// plan and week on the kit's heatmap with truth states (a week with no
// proof is a measured zero, never a hole), and the streak and "proved
// today" as counts.
func renderProofHeatmap(m console.HeatmapModel) string {
	var out strings.Builder
	out.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("day streak", m.Streak),
		valueStat("proved today", m.ProvedToday),
	}, stat.BandOptions{AriaLabel: "proof streak and today, as counts"}))
	if len(m.Cells) == 0 {
		out.WriteString(`<p class="mt-3 text-ink-muted" data-heatmap-empty>No row became proven inside the history for this selection.</p>`)
		return out.String()
	}
	idxP := map[string]int{}
	for i, p := range m.Plans {
		idxP[p] = i
	}
	idxW := map[string]int{}
	for i, w := range m.Weeks {
		idxW[w] = i
	}
	grid := make([][]float64, len(m.Plans))
	states := make([][]viz.HeatmapCell, len(m.Plans))
	for i := range grid {
		grid[i] = make([]float64, len(m.Weeks))
		states[i] = make([]viz.HeatmapCell, len(m.Weeks))
		for j := range states[i] {
			states[i][j] = viz.HeatmapCell{State: viz.CellZero}
		}
	}
	for _, c := range m.Cells {
		i, j := idxP[c.Plan], idxW[c.Week]
		grid[i][j] = float64(c.Weight)
		states[i][j] = viz.HeatmapCell{State: viz.CellValue}
	}
	ylabels := make([]string, len(m.Plans))
	for i, p := range m.Plans {
		ylabels[i] = strings.TrimSuffix(filepath.Base(p), ".md")
	}
	xlabels := make([]string, len(m.Weeks))
	for i, w := range m.Weeks {
		xlabels[i] = w[5:]
	}
	out.WriteString(`<div class="mt-3 overflow-x-auto" data-proof-heatmap>` + viz.Heatmap(grid, viz.HeatmapOptions{Role: "positive", XLabels: xlabels, YLabels: ylabels, ShowValues: true, States: states, Cell: 32,
		AriaLabel: "newly proven weight by plan (rows) and week starting Monday (columns)"}) + `</div>`)
	return out.String()
}

func renderGoalGauges(gs []console.GoalGauge, note string) string {
	if len(gs) == 0 {
		msg := "No scope-bet goal in the repositories in view (docs/intent/scope.md)."
		if note != "" {
			msg += " " + note
		}
		return `<p class="text-ink-muted" data-goals-empty>` + html.EscapeString(msg) + `</p>`
	}
	rows := make([]table.Row, 0, len(gs))
	for _, g := range gs {
		gauge := valuePlain(g.Proven) + " of " + fmt.Sprint(g.Total)
		meter := ""
		if g.Proven.Known() && g.Total > 0 {
			meter = `<div class="w-40 max-w-full">` + viz.MeterRow(float64(g.Proven.V), float64(g.Total), viz.MeterOptions{AriaLabel: fmt.Sprintf("%s: %d of %d measurement rows proven", g.Goal, g.Proven.V, g.Total)}) + `</div>`
		} else {
			meter = `<span class="text-xs italic text-ink-muted">` + html.EscapeString(g.Proven.Reason) + `</span>`
		}
		rows = append(rows, table.Row{ID: "g-" + g.Goal, Cells: []string{g.Goal, g.Title, "", gauge, g.Review},
			HTMLCells: []templ.Component{nil, nil, templ.Raw(meter)}})
	}
	return renderKitTable(table.TableParams{TableID: "progress-goals", Caption: "Each goal's measurement rows proven over those it names",
		Columns: []table.Column{{Key: "goal", Label: "Goal"}, {Key: "title", Label: "Title"}, {Key: "gauge", Label: "Gauge"}, {Key: "proven", Label: "Proven"}, {Key: "review", Label: "Review"}},
		Rows:    rows})
}
