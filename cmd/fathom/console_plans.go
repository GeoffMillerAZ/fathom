package main

// console_plans.go — the proof ledger's portfolio, /plans (vision 35 §4.4,
// design §3 "Proof ledger"; OC-21, SD-2, SD-3): every plan across the fleet
// with its proof bar, the claimed-vs-proven gap as the headline — for the
// fleet and per repository (?svc=) — and the claimed-vs-proven trend from
// the verify ledger, drawn only once the ledger holds 14 days of rows.
// The plan page (console_plan.go) and a row's evidence
// (console_planrow.go) share the helpers here.
//
// Every number is the verify engine's (console_plansdata.go), never the
// checkbox: a row whose verdict has not arrived reads "not yet verified".

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "plans", Route: "/plans", Href: "/plans", Group: "Proof", Order: 0, Nav: true,
		Title:    "Plans",
		Question: "What is planned, what is proven done, what remains, and how is it trending?",
		Verbs:    []string{"fathom plans status --service {svc} --json", "fathom plans history --service {svc} --json", "fathom plans history --json"},
		Shows:    []console.MetricID{console.MetricServices, console.MetricPlansUnproven, console.MetricPlansProven},
		Build:    buildPlansPortfolioPage,
	})
}

// repositories counts repositories in words ("1 repository", "7
// repositories").
func repositories(n int) string {
	if n == 1 {
		return "1 repository"
	}
	return fmtThousands(n) + " repositories"
}

// plansPageSize is how many plans one page of the portfolio table lists.
const plansPageSize = 50

// planHref is a plan's page; planRowHref one of its rows.
func planHref(service, slug string) string {
	return "/plans/" + url.PathEscape(service) + "/" + url.PathEscape(slug)
}

func planRowHref(service, slug, id string) string {
	return planHref(service, slug) + "/rows/" + url.PathEscape(id)
}

// plansPortfolioModel is /plans' JSON model.
type plansPortfolioModel struct {
	Scope string `json:"scope"`
	console.Portfolio
	Page  int `json:"page"`
	Pages int `json:"pages"`
	// PendingSections names the verbs whose sections are not in this
	// response because a plan they cover was still being verified.
	PendingSections []string `json:"pending_sections,omitempty"`
}

// plansScope resolves ?svc= against the registry.
func plansScope(cr *consoleRequest, m observatoryModel) ([]serviceRoot, string, error) {
	if m.DegradedNote != "" {
		if scope := cr.r.URL.Query().Get("svc"); scope != "" {
			return nil, "", unknownProjectError(scope, m, cr.deps.selfURL)
		}
		return nil, "", &consoleError{
			Status: http.StatusServiceUnavailable, Title: "Registry unreachable",
			Reason:      "The service registry could not be read, so no repository can be listed: " + m.DegradedNote,
			Remedy:      "fathom doctor",
			RemedyHref:  "/statusz",
			RemedyLabel: "Open the daemon status",
		}
	}
	roots := modelServiceRoots(m)
	scope := cr.r.URL.Query().Get("svc")
	if scope == "" {
		return roots, "", nil
	}
	for _, sr := range roots {
		if sr.Service == scope {
			return []serviceRoot{sr}, scope, nil
		}
	}
	return nil, "", unknownProjectError(scope, m, cr.deps.selfURL)
}

// snapshotPlans snapshots every in-scope service's plans, waiting up to
// budget for their verification.
func snapshotPlans(ctx context.Context, pc *consolePlans, roots []serviceRoot, only string, budget time.Duration) []console.ServicePlans {
	snap := func() ([]console.ServicePlans, []*planEntry) {
		var sps []console.ServicePlans
		var waits []*planEntry
		for _, sr := range roots {
			sp, w := pc.servicePlans(ctx, sr, only)
			sps = append(sps, sp)
			waits = append(waits, w...)
		}
		return sps, waits
	}
	sps, waits := snap()
	pending := false
	for _, e := range waits {
		select {
		case <-e.done:
		default:
			pending = true
		}
	}
	if pending {
		pc.wait(waits, budget)
		sps, _ = snap()
	}
	return sps
}

// verifiedRollups is a service's `plans status --json` when every plan is
// verified: one rollup per plan in the verb's (sorted) order.
func verifiedRollups(sp console.ServicePlans) ([]planRollup, bool) {
	out := make([]planRollup, 0, len(sp.Plans))
	for _, p := range sp.Plans {
		if !p.Verified || p.Err != "" {
			return nil, false
		}
		out = append(out, console.RollupPlan(p.Path, p.Rows, p.DaemonStale))
	}
	return out, true
}

// plansPortfolio is the Proof ledger's portfolio over roots: every plan
// snapshotted from the background verification (waiting the portfolio's
// budget), with the verify ledger's trend. The Plans page renders it, and
// its fleet headline is the plans metrics every page shows (fleetPlanMetrics).
func plansPortfolio(cr *consoleRequest, m observatoryModel, roots []serviceRoot) (console.Portfolio, []console.ServicePlans) {
	pc := plansFor(cr.deps)
	sps := snapshotPlans(cr.r.Context(), pc, roots, "", consolePlansPortfolioWait)
	ledger := pc.ledgerRead()
	return console.PlanPortfolio(console.PortfolioInput{
		Services: sps, Registered: len(m.Services),
		Ledger: ledger.Runs, LedgerPath: ledger.Path, LedgerErr: ledger.Err,
		PlanHref: planHref, MinTrendDays: console.DefaultTrendDays,
	}), sps
}

// fleetPlanMetrics is the fleet's plans headline — plans.claimed_unproven
// and plans.proven, the portfolio over every registered repository — the
// one projection the Now, Fleet and Plans pages show (OC-9, OC-H3; the
// ledger-summed plans.verified that answered the same question is gone).
// With the registry unread no repository can be listed, so neither is
// measured.
func fleetPlanMetrics(cr *consoleRequest, m observatoryModel) (unproven, proven console.Value[int]) {
	if m.DegradedNote != "" {
		a := console.Absent[int]("no repository can be listed: " + m.DegradedNote)
		return a, a
	}
	pf, _ := plansPortfolio(cr, m, modelServiceRoots(m))
	return pf.Unproven, pf.Proven
}

func buildPlansPortfolioPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	roots, scope, err := plansScope(cr, m)
	if err != nil {
		return consoleOut{}, err
	}
	pf, sps := plansPortfolio(cr, m, roots)
	model := plansPortfolioModel{Scope: scope, Portfolio: pf}
	verbs := []string{}
	sections := map[string]any{}
	for _, sp := range sps {
		if sp.Err != "" || len(sp.Plans) == 0 {
			continue
		}
		verb := "fathom plans status --service " + sp.Service + " --json"
		verbs = append(verbs, verb)
		if rollups, ok := verifiedRollups(sp); ok {
			sections[verb] = rollups
		} else {
			model.PendingSections = append(model.PendingSections, verb)
		}
	}
	// The trend is the scope's own (vision 35 §5: filter state lives in the
	// URL), rendered from the verb that prints it: the current plans of this
	// repository, or of every registered one.
	trendVerb := plansTrendVerb(scope)
	verbs = append(verbs, trendVerb)
	sections[trendVerb] = pf.Trend
	model.Pages = (len(pf.Plans) + plansPageSize - 1) / plansPageSize
	model.Page = pageParam(cr.r, model.Pages)
	all := consoleModelMetrics(m)
	if scope == "" {
		// The fleet headline is the shared metric; a repository's own
		// gap (?svc=) is not the fleet's, so it is not reported under
		// the fleet's name.
		all[console.MetricPlansUnproven], all[console.MetricPlansProven] = pf.Unproven, pf.Proven
	}
	metrics := pageMetrics(cr.page, all)
	meta := []string{fmt.Sprintf("%s across %s", plural(len(pf.Plans), "plan"), repositories(pf.WithPlans)), "generated " + m.GeneratedAt}
	if scope != "" {
		meta[0] = plural(len(pf.Plans), "plan") + " in " + scope
	}
	out := consoleOut{
		Meta: meta, Model: model, Metrics: metrics, Sections: sections, Verbs: verbs,
		Body: func() string { return renderPlansPortfolio(model, roots, scope, metrics, sps, trendVerb) },
	}
	if scope == "" && len(verbs) > 0 {
		// One verb per repository: listed behind one disclosure rather than
		// stacked chip after chip in the heading.
		out.Chips, out.Actions = []string{}, verbsDisclosure(verbs)
	}
	return out, nil
}

// plansTrendVerb is the verb that prints a scope's claimed-vs-proven trend.
func plansTrendVerb(scope string) string {
	if scope != "" {
		return "fathom plans history --service " + scope + " --json"
	}
	return "fathom plans history --json"
}

// pageParam reads ?page=, clamped to [1, pages].
func pageParam(r *http.Request, pages int) int {
	n, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || n < 1 {
		n = 1
	}
	if pages > 0 && n > pages {
		n = pages
	}
	return n
}

// plansLiveKey is the live-freshness source for a scope's plan panels.
func plansLiveKey(scope string) string {
	if scope != "" {
		return "plans:" + scope
	}
	return "plans"
}

func renderPlansPortfolio(m plansPortfolioModel, roots []serviceRoot, scope string, metrics console.Metrics, sps []console.ServicePlans, trendVerb string) string {
	pf := m.Portfolio
	var b strings.Builder
	b.WriteString(renderPlansScopeNav(sps, scope))
	services := metrics[console.MetricServices]
	repoStat := stat.Stat{Label: "repositories with plans", Value: fmtThousands(pf.WithPlans), Sub: "of " + valuePlain(services) + " in the registry"}
	b.WriteString(`<div class="mb-4">` + stat.RenderBand([]stat.Stat{
		valueStat("claimed, not proven by the gate", pf.Unproven),
		valueStat("proven by the gate", pf.Proven),
		{Label: "plans", Value: fmtThousands(len(pf.Plans)), Sub: fmtThousands(pf.Bar.Total) + " verified rows"},
		repoStat,
	}, stat.BandOptions{AriaLabel: "plans at a glance"}) + `</div>`)

	barBody := proofBarHTML(pf.Bar, "every verified plan's rows by the state the gate computed")
	if pf.Bar.Total == 0 {
		barBody = `<p class="italic text-ink-muted">no plan has been verified yet — the bar is drawn from the gate's verdicts, never the checkboxes</p>`
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "proof-bar", Title: "Proof bar", SourceKey: plansLiveKey(scope),
		Question:    "Of every row the plans claim or hold open, how many does the gate actually prove?",
		Verb:        firstVerbOr(scope, "fathom plans status --service <svc> --json"),
		Source:      "the verify engine at each repository's current index (verify_plan through this daemon, the working tree's lexical oracles)",
		Denominator: plural(pf.Bar.Total, "verified row"),
		Freshness:   "as rendered; a plan is re-verified when its file, its index or its HEAD moves",
		State:       pf.Unproven.State,
		Reason:      pf.Unproven.Reason,
		Body:        barBody,
	}))

	b.WriteString(`<div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "plans-table", Title: "Plans", SourceKey: plansLiveKey(scope),
		Question:    "Which plans are proven, which carry claims the gate refutes, and which are still open?",
		Verb:        firstVerbOr(scope, "fathom plans status --service <svc> --json"),
		Source:      "each repository's docs/plans/*.md (README excluded), verified at the current index",
		Denominator: plural(len(pf.Plans), "plan"),
		Freshness:   "as rendered",
		Body:        renderPlansTable(m) + renderPlansPager(m, scope),
	}))
	b.WriteString(`</div>`)

	if scope == "" && pf.WithPlans > 0 {
		b.WriteString(`<div class="mt-4">`)
		b.WriteString(renderPanel(consolePanel{
			ID: "per-repository", Title: "Per repository", SourceKey: "plans",
			Question:    "Where is the claimed-versus-proven gap, repository by repository?",
			Verb:        "fathom plans status --service <svc> --json",
			Source:      "the same verdicts, grouped by repository",
			Denominator: repositories(pf.WithPlans) + " with plans",
			Body:        renderPlansPerRepository(pf),
		}))
		b.WriteString(`</div>`)
	}

	b.WriteString(`<div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "trend", Title: "Claimed against proven over time", SourceKey: plansLiveKey(scope),
		Question:    "Is the gap between what is claimed and what is proven closing?",
		Verb:        trendVerb,
		Source:      pf.Trend.Source + " — " + pf.Trend.Scope + "; each plan's last gate run carried forward per day",
		Denominator: fmt.Sprintf("%d days of ledger rows (the trend needs %d)", pf.Trend.Days, pf.Trend.Need),
		State:       pf.Trend.State,
		Reason:      pf.Trend.Reason,
		Body:        renderTrend(pf.Trend, "claimed against proven, per day", "claimed against proven per day"),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

// verbsDisclosure lists a fleet page's per-repository verbs as copyable
// chips behind one disclosure in the heading.
func verbsDisclosure(verbs []string) string {
	var b strings.Builder
	b.WriteString(`<div class="flex flex-col gap-2">`)
	for _, v := range verbs {
		b.WriteString(renderVerbChip(v))
	}
	b.WriteString(`</div>`)
	return `<div class="min-w-0 max-w-full" data-verbs-disclosure>` + chrome.RenderDisclosure(chrome.Disclosure{
		Summary: verbsSummary(len(verbs)),
		Body:    template.HTML(b.String()),
	}) + `</div>`
}

func verbsSummary(n int) string {
	if n == 1 {
		return "The verb this page renders"
	}
	return fmt.Sprintf("The %d verbs this page renders", n)
}

// firstVerbOr is the portfolio panels' verb: the scope's exact command, or
// the command shape across the fleet.
func firstVerbOr(scope, shape string) string {
	if scope != "" {
		return "fathom plans status --service " + scope + " --json"
	}
	return shape
}

// renderPlansScopeNav is the repository filter: every repository with
// plans, as links (the filter lives in the URL).
func renderPlansScopeNav(sps []console.ServicePlans, scope string) string {
	var b strings.Builder
	b.WriteString(`<nav aria-label="Filter plans by repository" class="mb-4 flex flex-wrap gap-2" data-plans-scope>`)
	link := func(href, label string, current bool) {
		cls := "inline-flex min-h-11 items-center rounded-full border border-line px-3 text-sm text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
		cur := ""
		if current {
			cls = "inline-flex min-h-11 items-center rounded-full border border-accent bg-accent-soft px-3 text-sm font-medium text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
			cur = ` aria-current="page"`
		}
		b.WriteString(`<a href="` + html.EscapeString(href) + `" class="` + cls + `"` + cur + `>` + html.EscapeString(label) + `</a>`)
	}
	link("/plans", "All repositories", scope == "")
	for _, sp := range sps {
		if len(sp.Plans) == 0 && sp.Err == "" {
			continue
		}
		link("/plans?svc="+url.QueryEscape(sp.Service), sp.Service, sp.Service == scope)
	}
	b.WriteString(`</nav>`)
	return b.String()
}

// roleSwatch are the legend swatches' fill utilities, one per bar role
// (complete literals, so the CSS build sees each one).
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var roleSwatch = map[string]string{
	"positive": "bg-positive", "info": "bg-info", "danger": "bg-danger", "caution": "bg-caution",
	"caution-soft": "bg-caution-soft", "ink-faint": "bg-ink-faint", "accent": "bg-accent",
	"line-strong": "bg-line-strong", "info-soft": "bg-info-soft", "ink-muted": "bg-ink-muted",
	"line": "bg-line", "surface-sunken": "bg-surface-sunken",
}

// proofBarHTML is a proof bar: the kit's stacked bar and a legend that
// names every segment with its count, so color is never the only channel.
func proofBarHTML(bar console.ProofBar, aria string) string {
	if bar.Total == 0 {
		return ""
	}
	segs := make([]viz.Segment, 0, len(bar.Segments))
	for _, s := range bar.Segments {
		segs = append(segs, viz.Segment{Value: float64(s.Count), Role: s.Role, Label: fmt.Sprintf("%s: %d", s.Label, s.Count)})
	}
	return `<div class="max-w-3xl" data-proof-bar>` + viz.StackBar(segs, viz.StackBarOptions{W: 600, H: 14, Fluid: true, AriaLabel: aria}) + `</div>` + barLegend(bar)
}

// barLegend lists a bar's segments with their counts.
func barLegend(bar console.ProofBar) string {
	var b strings.Builder
	b.WriteString(`<ul class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-ink-muted" data-bar-legend>`)
	for _, s := range bar.Segments {
		b.WriteString(`<li class="inline-flex items-center gap-1.5"><span aria-hidden="true" class="inline-block size-2.5 rounded-sm border border-line ` + roleSwatch[s.Role] + `"></span><span class="font-medium tabular-nums text-ink">` + fmtThousands(s.Count) + `</span> ` + html.EscapeString(s.Label) + `</li>`)
	}
	b.WriteString(`</ul>`)
	return b.String()
}

// valueCell is a projected value in a table cell: the number, "≥ n" for a
// lower bound, or the absence in words ("not measured", "unreadable") with
// its reason as the cell's title and in screen-reader text — the panel's
// own truth line or note says it in full, once, instead of in every row.
func valueCell(v console.Value[int]) string {
	switch v.State {
	case console.StateAbsent:
		r := html.EscapeString(v.Reason)
		return `<span class="italic text-ink-muted" title="` + r + `">not measured<span class="sr-only"> — ` + r + `</span></span>`
	case console.StateError:
		r := html.EscapeString(v.Reason)
		return `<span class="text-danger" title="` + r + `">unreadable<span class="sr-only"> — ` + r + `</span></span>`
	case console.StatePartial:
		return "≥ " + fmtThousands(v.V)
	}
	return fmtThousands(v.V)
}

// miniBar is a small stacked bar for a table cell, labelled by its counts.
func miniBar(bar console.ProofBar, aria string) string {
	if bar.Total == 0 {
		return `<span class="text-ink-muted">no rows</span>`
	}
	segs := make([]viz.Segment, 0, len(bar.Segments))
	for _, s := range bar.Segments {
		segs = append(segs, viz.Segment{Value: float64(s.Count), Role: s.Role, Label: fmt.Sprintf("%s: %d", s.Label, s.Count)})
	}
	return `<div class="w-40 max-w-full">` + viz.StackBar(segs, viz.StackBarOptions{W: 160, H: 10, Fluid: true, AriaLabel: aria}) + `</div>`
}

// countOrPending renders a rollup count, or why there is none yet.
func countOrPending(p console.PortfolioPlan, n func(*console.PlanRollup) int) string {
	switch {
	case p.Rollup != nil:
		return fmtThousands(n(p.Rollup))
	case p.State == console.StateError:
		return `<span class="text-danger">unverified</span>`
	}
	return `<span class="italic text-ink-muted">verifying</span>`
}

func renderPlansTable(m plansPortfolioModel) string {
	pf := m.Portfolio
	if len(pf.Plans) == 0 {
		return chrome.RenderEmptyState(chrome.EmptyState{Heading: "No plans", Body: "No registered repository in this scope has a docs/plans/*.md. A plan is a checklist the gate verifies: fathom plans status reads it once it exists."})
	}
	lo := (m.Page - 1) * plansPageSize
	hi := lo + plansPageSize
	if hi > len(pf.Plans) {
		hi = len(pf.Plans)
	}
	rows := make([]table.Row, 0, hi-lo)
	for _, p := range pf.Plans[lo:hi] {
		link := `<a href="` + html.EscapeString(p.Href) + `" class="inline-flex min-h-11 flex-col justify-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(p.Title) +
			`<span class="font-mono text-xs font-normal text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(p.Rel) + `</span></a>`
		state := ""
		if p.State != console.StatePresent {
			state = `<span class="mt-1 block text-xs italic text-ink-muted">` + html.EscapeString(p.Reason) + `</span>`
			if p.State == console.StateError {
				state = `<span class="mt-1 block text-xs text-danger">` + html.EscapeString(p.Reason) + `</span>`
			}
		}
		rows = append(rows, table.Row{
			ID:    "plan-" + p.Service + "-" + console.PlanSlug(p.Plan),
			Cells: []string{p.Title, p.Service, "", "", "", "", ""},
			HTMLCells: []templ.Component{
				templ.Raw(link), nil,
				templ.Raw(miniBar(p.Bar, p.Title+" by proof state") + state),
				templ.Raw(countOrPending(p, func(r *console.PlanRollup) int { return r.Proven })),
				templ.Raw(countOrPending(p, func(r *console.PlanRollup) int { return r.ClaimedUnproven })),
				templ.Raw(countOrPending(p, func(r *console.PlanRollup) int { return r.Open })),
				templ.Raw(countOrPending(p, func(r *console.PlanRollup) int { return r.DeferredWithTrigger + r.SkippedOther })),
			},
		})
	}
	return renderKitTable(table.TableParams{
		TableID: "plans",
		Caption: fmt.Sprintf("Plans %d–%d of %d, by repository and file", lo+1, hi, len(pf.Plans)),
		Columns: []table.Column{
			{Key: "plan", Label: "Plan"}, {Key: "repository", Label: "Repository"}, {Key: "shape", Label: "Proof bar"},
			{Key: "proven", Label: "Proven, (op) included", Align: "right"}, {Key: "unproven", Label: "Claimed, not proven", Align: "right"},
			{Key: "open", Label: "Open", Align: "right"}, {Key: "set-aside", Label: "Deferred or skipped", Align: "right"},
		},
		Rows: rows,
	})
}

func renderPlansPager(m plansPortfolioModel, scope string) string {
	return consolePager(chrome.Pagination{
		Page: m.Page, TotalPages: m.Pages, AriaLabel: "Plans pages",
		HrefFor: func(page int) string {
			q := url.Values{}
			if scope != "" {
				q.Set("svc", scope)
			}
			q.Set("page", strconv.Itoa(page))
			return "/plans?" + q.Encode() + "#plans-table"
		},
	})
}

func renderPlansPerRepository(pf console.Portfolio) string {
	bars := map[string]map[string]int{}
	for _, p := range pf.Plans {
		if bars[p.Service] == nil {
			bars[p.Service] = map[string]int{}
		}
		for _, s := range p.Bar.Segments {
			bars[p.Service][s.Key] += s.Count
		}
	}
	rows := make([]table.Row, 0, len(pf.Services))
	for _, s := range pf.Services {
		link := `<a href="/plans?svc=` + html.EscapeString(url.QueryEscape(s.Service)) + `" class="inline-flex min-h-11 items-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(s.Service) + `</a>`
		bar := `<span class="text-danger">` + html.EscapeString(s.Err) + `</span>`
		if s.Err == "" {
			bar = miniBar(barFromCounts(bars[s.Service]), s.Service+" by proof state")
		}
		rows = append(rows, table.Row{
			ID:    "repo-" + s.Service,
			Cells: []string{s.Service, fmtThousands(s.Plans), fmtThousands(s.Rows), "", "", ""},
			HTMLCells: []templ.Component{templ.Raw(link), nil, nil, templ.Raw(bar),
				templ.Raw(valueCell(s.Unproven)), templ.Raw(valueCell(s.Proven))},
		})
	}
	return renderKitTable(table.TableParams{
		TableID: "plans-per-repository",
		Caption: "Each repository's plans — open one for its exact plans status",
		Columns: []table.Column{
			{Key: "repository", Label: "Repository"}, {Key: "plans", Label: "Plans", Align: "right"}, {Key: "rows", Label: "Rows", Align: "right"},
			{Key: "shape", Label: "Proof bar"}, {Key: "unproven", Label: "Claimed, not proven", Align: "right"}, {Key: "proven", Label: "Proven, (op) included", Align: "right"},
		},
		Rows: rows,
	})
}

// barFromCounts rebuilds a bar from per-state counts (the segment order
// and roles are the projection's own).
func barFromCounts(counts map[string]int) console.ProofBar {
	return console.ProofBarFromCounts(counts)
}

// trendRowsShown is how many days of a trend the page lists.
const trendRowsShown = 14

// renderTrend draws a ledger trend as one stacked bar per day on one scale
// (proven, then claimed but not proven), with the counts in the same table
// row — the chart is its own table alternative. Absent is stat.Absent with
// the reason (SD-3), never an empty chart.
func renderTrend(t console.Trend, label, caption string) string {
	if t.State != console.StatePresent {
		return `<div class="max-w-sm">` + stat.Render(stat.Absent(label, t.Reason)) + `</div>`
	}
	points := t.Points
	omitted := 0
	if len(points) > trendRowsShown {
		omitted = len(points) - trendRowsShown
		points = points[len(points)-trendRowsShown:]
	}
	maxClaimed := 0
	for _, p := range points {
		if p.Claimed > maxClaimed {
			maxClaimed = p.Claimed
		}
	}
	rows := make([]table.Row, 0, len(points))
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		unproven := p.Claimed - p.Proven
		if unproven < 0 {
			unproven = 0
		}
		segs := []viz.Segment{
			{Value: float64(p.Proven), Role: "positive", Label: fmt.Sprintf("proven: %d", p.Proven)},
			{Value: float64(unproven), Role: "danger", Label: fmt.Sprintf("claimed, not proven: %d", unproven)},
			{Value: float64(maxClaimed - p.Claimed), Role: "surface-sunken", Label: "below the period's peak"},
		}
		bar := `<div class="w-48 max-w-full">` + viz.StackBar(segs, viz.StackBarOptions{W: 192, H: 10, Fluid: true, AriaLabel: fmt.Sprintf("%s: %d proven of %d claimed", p.Day, p.Proven, p.Claimed)}) + `</div>`
		rows = append(rows, table.Row{
			ID:        "day-" + p.Day,
			Cells:     []string{p.Day, "", fmtThousands(p.Proven), fmtThousands(p.Claimed), fmtThousands(p.Failing), fmtThousands(p.Plans)},
			HTMLCells: []templ.Component{nil, templ.Raw(bar)},
		})
	}
	note := ""
	if omitted > 0 {
		note = `<p class="mt-2 text-xs text-ink-muted">the ` + plural(omitted, "earlier day") + ` of the trend are in the page's JSON (?format=json), not listed here</p>`
	}
	return renderKitTable(table.TableParams{
		TableID: "trend",
		Caption: caption + ", newest first — bars share one scale (the period's most claimed rows)",
		Columns: []table.Column{
			{Key: "day", Label: "Day"}, {Key: "bar", Label: "Proven, then claimed but not proven"},
			{Key: "proven", Label: "Proven", Align: "right"}, {Key: "claimed", Label: "Claimed", Align: "right"},
			{Key: "failing", Label: "Failing", Align: "right"}, {Key: "plans", Label: "Plans", Align: "right"},
		},
		Rows: rows,
	}) + note
}
