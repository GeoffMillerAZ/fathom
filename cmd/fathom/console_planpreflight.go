package main

// console_planpreflight.go — the task preflight panel on the plan row page
// (WI-17, docs/design/work-intelligence.md §3 U6): the row's difficulty
// profile, the published rule table's advisory tier with the rules that
// fired, and the calibration read back from recorded outcomes. The same
// console.Preflight model `fathom plans context --preflight` prints. The
// page never asks the decision port for its Score — a page view must not
// spend or send code off-host (I14); the Score is the CLI's `--score`.
// Advisory: the panel has no control that starts, assigns or routes work.

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
)

// planRowModel is the plan row page's JSON model: the row's evidence, and
// its preflight when one could be computed.
type planRowModel struct {
	console.RowEvidenceView
	Preflight    *console.PreflightModel `json:"preflight,omitempty"`
	PreflightErr string                  `json:"preflight_error,omitempty"`
}

// buildPlanRowPreflight computes the row's preflight over the daemon's
// graph store and the same proven-change memo /work reads. The returned
// string is why no preflight could be computed, else "".
func buildPlanRowPreflight(cr *consoleRequest, sr serviceRoot, plan console.PlanFacts, row console.PlanRowFact) (*console.PreflightModel, string) {
	if cr.deps == nil || cr.deps.c == nil {
		return nil, errNoStore.Error()
	}
	ctx := cr.r.Context()
	snap, _, g, _, err := consoleBothGraph(ctx, cr.deps.c, sr.Service)
	if err != nil {
		return nil, "the call graph could not be read: " + err.Error()
	}
	work := memoWork(ctx, cr, cr.model(), sr)
	src := &preflightSources{c: cr.deps.c, graph: g, nodes: snap.Nodes, service: sr.Service, root: sr.Root}
	retries, weight, _ := console.RowProofFacts(work.history, sr.Service, plan.Rel, row.ID)
	prow := console.PreflightRow{Service: sr.Service, Plan: plan.Rel, ID: row.ID, Title: row.Title, Symbol: row.Symbol, Package: row.Package, Weight: weight, Retries: retries}
	in := src.preflightInput(ctx, prow, work.changes, work.zones)
	outcomes, _ := console.PreflightOutcomes(ctx, preflightLedgerPath(), preflightLedgerTail)
	m := console.Preflight(in, console.Value[console.ScoreReading]{}, filterOutcomes(outcomes, sr.Service))
	return &m, ""
}

// renderPreflightPanel renders the row's preflight as one panel.
func renderPreflightPanel(m *console.PreflightModel, why string, sr serviceRoot, plan console.PlanFacts, row console.PlanRowFact, live string) string {
	verb := "fathom plans context " + plan.Rel + " --row " + row.ID + " --service " + sr.Service + " --store <db> --preflight"
	p := consolePanel{
		ID: "row-preflight", SourceKey: live, Title: "Task preflight", Verb: verb,
		Question: "How hard is this row before work starts, and which model tier does the published rule table recommend?",
		Source:   "the index's call graph, obligations and certifications at the row's anchor; U2's interest rate and the nearest proven rows from the proof history joined to the session ledger",
	}
	if m == nil {
		p.State, p.Reason = console.StateAbsent, why
		p.Body = `<p class="text-ink-muted" data-preflight-absent>` + html.EscapeString("No preflight: "+why) + `</p>`
		return renderPanel(p)
	}
	rec := m.Recommendation
	switch {
	case rec.Tier == "":
		p.State, p.Reason = console.StateAbsent, rec.Note
	case !rec.Complete:
		p.State, p.Reason = console.StatePartial, fmt.Sprintf("at least %s: %d rules could not be evaluated", rec.Tier, len(rec.Unevaluated))
	}
	p.Denominator = fmt.Sprintf("%d rules in table revision %d", len(m.RuleTable), rec.TableVersion)
	p.Body = renderPreflightBody(m)
	return renderPanel(p)
}

func renderPreflightBody(m *console.PreflightModel) string {
	var b strings.Builder
	pr, rec := m.Profile, m.Recommendation
	mono := func(s string) template.HTML {
		return template.HTML(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(s) + `</span>`)
	}
	measured := func(known bool, reason, s string) string {
		if known {
			return s
		}
		return "not measured — " + reason
	}
	tier := "none recommended"
	switch {
	case rec.Tier != "" && rec.Complete:
		tier = string(rec.Tier)
	case rec.Tier != "":
		tier = "at least " + string(rec.Tier)
	}
	items := []chrome.DescItem{
		{Term: "recommended tier", DetailHTML: template.HTML(`<span class="inline-flex items-center rounded-full border border-line bg-surface-sunken px-2 py-0.5 text-xs font-medium text-ink" data-preflight-tier>` + html.EscapeString(tier) + `</span>`)},
		{Term: "blast radius", Detail: measured(pr.Blast.Known(), pr.Blast.Reason, fmt.Sprintf("%d transitive callers, %d entrypoints%s", pr.Blast.V.Callers, pr.Blast.V.Entrypoints, lowerBound(pr.Blast.State)))},
		{Term: "governing rules", Detail: measured(pr.Rules.Known(), pr.Rules.Reason, fmt.Sprintf("%d (%d error, %d warn, %d info)", len(pr.Rules.V), pr.RuleLevels.Error, pr.RuleLevels.Warn, pr.RuleLevels.Info))},
		{Term: "certificates to re-prove", Detail: measured(pr.Certs.Known(), pr.Certs.Reason, fmt.Sprint(len(pr.Certs.V)))},
	}
	for _, z := range pr.ZoneInterest {
		items = append(items, chrome.DescItem{Term: "zone " + z.Zone, DetailHTML: mono(measured(z.Rate.Known(), z.Rate.Reason, fmt.Sprintf("interest rate %.2fx over %d proven changes", z.Rate.V, z.N)))})
	}
	items = append(items, chrome.DescItem{Term: "this row", Detail: fmt.Sprintf("%d claimed-unproven readings, plan weight %d", pr.Row.Retries, pr.Row.Weight)})
	if pr.Similar.Known() {
		s := pr.Similar.V
		items = append(items, chrome.DescItem{
			Term:   fmt.Sprintf("%d nearest proven rows", len(s.Rows)),
			Detail: fmt.Sprintf("median %.0f tokens, %.0f turns, %.0f s; %.0f%% proven at the first attempt%s", s.MedianTokens, s.MedianTurns, s.MedianWallSeconds, s.FirstAttemptRate*100, lowerBound(pr.Similar.State)),
		})
	} else {
		items = append(items, chrome.DescItem{Term: "nearest proven rows", Detail: "not measured — " + pr.Similar.Reason})
	}
	b.WriteString(chrome.RenderDescriptionList(items))

	if len(rec.Fired) > 0 {
		b.WriteString(`<h3 class="mt-3 text-sm font-semibold text-ink">Rules that fired</h3><ul class="mt-1 flex flex-col gap-1 text-sm" data-preflight-fired>`)
		for _, f := range rec.Fired {
			b.WriteString(`<li class="[overflow-wrap:anywhere]"><span class="font-mono text-xs">` + html.EscapeString(f.ID) + `</span> ` + html.EscapeString(f.When) + ` → <span class="font-medium">` + html.EscapeString(string(f.Floor)) + `</span> <span class="text-ink-muted">(` + html.EscapeString(f.Evidence) + `)</span></li>`)
		}
		b.WriteString(`</ul>`)
	}
	if len(rec.Unevaluated) > 0 {
		b.WriteString(`<h3 class="mt-3 text-sm font-semibold text-ink">Rules not evaluated</h3><ul class="mt-1 flex flex-col gap-1 text-xs text-ink-muted" data-preflight-unevaluated>`)
		for _, u := range rec.Unevaluated {
			b.WriteString(`<li class="[overflow-wrap:anywhere]">` + html.EscapeString(u) + `</li>`)
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`<p class="mt-3 text-sm text-ink-muted" data-preflight-score>` + html.EscapeString("Decision port Score: not asked here — a page view never spends or sends code off-host. Ask it with `fathom plans context … --preflight --score`, with [decide] configured.") + `</p>`)
	b.WriteString(`<p class="mt-2 text-sm text-ink-muted" data-preflight-calibration>` + html.EscapeString(m.Calibration.Note) + `</p>`)
	for _, r := range m.Calibration.RuleTable {
		b.WriteString(`<p class="text-xs text-ink-muted">` + html.EscapeString(fmt.Sprintf("Table tier %s: %d of %d rows proven at the first attempt.", r.Bucket, r.Hits, r.N)) + `</p>`)
	}
	b.WriteString(`<p class="mt-2 text-xs text-ink-muted" data-preflight-advisory>` + html.EscapeString(console.PreflightAdvisory) + `</p>`)
	return b.String()
}
