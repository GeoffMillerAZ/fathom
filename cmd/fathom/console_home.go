package main

// console_home.go — the Now page at / (vision 35 §3, plan OC-17, OC-18):
// what needs my attention, and what changed since I last looked.
//
//   - A stat band: projects wired and fresh, plan rows proven, governed
//     claims current, live certificates, gate refusals and agent sessions —
//     each a shared projection (the Fleet page shows the same four project
//     metrics, TestConsoleMetricsAgreeAcrossPages holds them equal).
//   - The attention queue (console.AttentionQueue): problems ordered by what
//     blocks proof first, each with its evidence link and the command that
//     fixes it. Nothing is applied for the reader.
//   - Since you last looked (console.SinceLastLook): pushes, deploys,
//     measurements and restarts, marked new against the previous view.
//   - The fleet pulse: the busiest projects' realization ladders and calls
//     per day on one shared scale (console.FleetTable, OC-20).
//
// The page's JSON carries `fathom fleet report --json`; and, per project,
// the two verbs the queue's proof figures answer from recorded
// instruments: `fathom plans status` (each plan's verify run at HEAD, from
// the ledger) and `fathom certify status` (certify.Gaps over the store and
// the repo's intent corpus), each in the verb's own shape and held to it
// by TestConsolePagesRenderTheirVerbs.

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func init() {
	declarePage(consolePage{
		ID: "home", Route: "/{$}", Href: "/", Group: "Now", Order: 0, Nav: true,
		Title: "Now", NavLabel: "Home",
		Question:     "What needs my attention, and what changed since I last looked?",
		Verbs:        []string{"fathom fleet report --json"},
		ProjectVerbs: []string{verbPlansStatusFor, verbCertifyStatusFor},
		Shows: []console.MetricID{
			console.MetricServices, console.MetricAgentTools, console.MetricSessions,
			console.MetricWiredFresh, console.MetricPlansProven, console.MetricClaims, console.MetricCertLive,
		},
		Build: buildHomePage,
	})
}

// The per-project verbs the Now page renders from recorded instruments.
const (
	verbPlansStatusFor   = "fathom plans status --service {svc} --json"
	verbCertifyStatusFor = "fathom certify status --service {svc} --json"
)

// nowPlanStatus is one plan as `fathom plans status --json` counts it,
// read from the plan's recorded verify run: proven is the run's verified
// rows, claimed_unproven its other claimed rows (failing and unprovable),
// unclaimed the open, deferred and skipped rows the ledger does not split.
// A run that proved an older commit says so (stale) — only a run at HEAD
// is the verb's answer today.
type nowPlanStatus struct {
	Plan            string `json:"plan"`
	Total           int    `json:"total"`
	Proven          int    `json:"proven"`
	ClaimedUnproven int    `json:"claimed_unproven"`
	Unclaimed       int    `json:"unclaimed"`
	Failing         int    `json:"failing"`
	At              string `json:"at"`
	Commit          string `json:"commit,omitempty"`
	Stale           string `json:"stale,omitempty"`
}

// nowProjectSections are the Now page's per-project verb sections, keyed
// by service.
func nowProjectSections(p console.Projects) map[string]any {
	plans := map[string][]nowPlanStatus{}
	certs := map[string]map[string]any{}
	for _, r := range p.Rows {
		for _, run := range r.Runs {
			unclaimed := run.Total - run.Claimed
			if unclaimed < 0 {
				unclaimed = 0
			}
			plans[r.Service] = append(plans[r.Service], nowPlanStatus{
				Plan: run.Plan, Total: run.Total, Proven: run.Verified, ClaimedUnproven: run.Claimed - run.Verified,
				Unclaimed: unclaimed, Failing: run.Failing, At: run.At, Commit: run.Commit, Stale: run.StaleReason,
			})
		}
		c := r.Certs
		if c.State == console.StateAbsent || (c.State == console.StateError && len(c.V.Unreadable) == 0) {
			continue // no store, or the store failed: the verb has no answer either
		}
		certs[r.Service] = certifyStatusJSON(r.Service, certify.GapSummary{Declared: c.V.Declared, Live: c.V.Live, Gaps: c.V.Gaps, Cases: c.V.Cases, JoinedCases: c.V.JoinedCases}, c.V.Unreadable)
	}
	return map[string]any{verbPlansStatusFor: plans, verbCertifyStatusFor: certs}
}

// nowReportSchema names the Now projection's wire version.
const nowReportSchema = "fathom.console.now/v1"

// nowReport is the Now page's projection (SR-14, surface reckoning F-53,
// removed the `fathom brief --json` CLI mirror this used to also serve —
// 0 lifetime calls; the page's own JSON, via Model above, is unaffected).
type nowReport struct {
	Schema      string                  `json:"schema"`
	GeneratedAt string                  `json:"generated_at"`
	Summary     nowSummary              `json:"summary"`
	Attention   console.Attention       `json:"attention"`
	Since       console.Since           `json:"since_last_look"`
	Pulse       console.FleetTableModel `json:"pulse"`
}

// nowSummary is the stat band: every value with its truth state.
type nowSummary struct {
	Projects      console.Value[int] `json:"projects"`
	WiredFresh    console.Value[int] `json:"wired_fresh"`
	ClaimsCurrent console.Value[int] `json:"claims_current"`
	CertifyLive   console.Value[int] `json:"certify_live"`
	GateRefusals  console.Value[int] `json:"gate_refusals"`
	Sessions      console.Value[int] `json:"sessions"`
}

// nowPulseRows is how many projects the pulse shows.
const nowPulseRows = 7

// buildNowReport projects the page from one read of every source.
func buildNowReport(m observatoryModel, pr projectsRead, since console.Since, now time.Time) nowReport {
	metrics := consoleModelMetrics(m)
	pm := console.ProjectMetrics(pr.projects)
	return nowReport{
		Schema: nowReportSchema, GeneratedAt: now.UTC().Format(time.RFC3339),
		Summary: nowSummary{
			Projects:      metrics[console.MetricServices],
			WiredFresh:    pm[console.MetricWiredFresh],
			ClaimsCurrent: pm[console.MetricClaims],
			CertifyLive:   pm[console.MetricCertLive],
			GateRefusals:  pr.gateRefusals,
			Sessions:      metrics[console.MetricSessions],
		},
		Attention: console.AttentionQueue(pr.projects),
		Since:     since,
		Pulse:     console.FleetTable(pr.projects, console.FleetQuery{Sort: "calls", Dir: "desc", Limit: nowPulseRows}),
	}
}

// consoleSharedMetrics is every shared metric a model-backed page with the
// project set can show, the Proof ledger's plans headline included.
func consoleSharedMetrics(cr *consoleRequest, m observatoryModel, pr projectsRead) console.Metrics {
	all := consoleModelMetrics(m)
	for id, v := range console.ProjectMetrics(pr.projects) {
		all[id] = v
	}
	all[console.MetricPlansUnproven], all[console.MetricPlansProven] = fleetPlanMetrics(cr, m)
	return all
}

func buildHomePage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	pr := cr.projects()
	sincePage, _ := strconv.Atoi(cr.r.URL.Query().Get("since_page"))
	since := gatherSince(cr.r.Context(), sinceSources{
		baseURL: cr.deps.selfURL, callLog: modelCallFacts(m).Log.Path, viewsLog: cr.deps.views.logPath, model: m, now: cr.now, page: sincePage,
	})
	rep := buildNowReport(m, pr, since, cr.now)
	metrics := consoleSharedMetrics(cr, m, pr)
	out := consoleOut{
		Meta:    nowMeta(m, rep),
		Actions: `<span class="flex flex-wrap gap-2 [&>a]:min-h-11">` + form.RenderButton(form.Button{Label: "Download brief", Href: "/brief?format=md", Variant: form.ButtonGhost}) + form.RenderButton(form.Button{Label: "Open the fleet", Href: "/fleet", Variant: form.ButtonGhost}) + `</span>`,
		Model:   rep,
		Metrics: pageMetrics(cr.page, metrics),
		Body:    func() string { return renderNow(rep, metrics[console.MetricPlansProven], m, cr.now) },
	}
	if wantsJSON(cr.r) {
		out.Sections = nowProjectSections(pr.projects)
		out.Sections["fathom fleet report --json"] = fleetReportSection(cr)
	}
	return out, nil
}

// nowMeta is the heading's meta line: the fleet's size, the last look and
// the build the page came from.
func nowMeta(m observatoryModel, rep nowReport) []string {
	meta := []string{valuePlainShort(rep.Summary.Projects, "projects")}
	if t, err := time.Parse(time.RFC3339, rep.Since.LastLook); err == nil {
		meta = append(meta, "last look "+t.UTC().Format("01-02 15:04")+" UTC")
	} else {
		meta = append(meta, "no earlier look recorded")
	}
	return append(meta, modelMeta(m)...)
}

// valuePlainShort is "18 projects", "≥ 18 projects" or "projects not measured".
func valuePlainShort(v console.Value[int], noun string) string {
	switch v.State {
	case console.StateAbsent, console.StateError:
		return noun + " not measured"
	case console.StatePartial:
		return "≥ " + fmtThousands(v.V) + " " + noun
	}
	return fmtThousands(v.V) + " " + noun
}

// renderNow is the page body.
// plansProven is the shared plans.proven metric (the brief's JSON carries
// no plans figure: the gate's verdicts live in the daemon's plan cache).
func renderNow(rep nowReport, plansProven console.Value[int], m observatoryModel, now time.Time) string {
	var b strings.Builder
	s := rep.Summary
	// The kit band stacks one tile per row below md; the phone composition
	// keeps two columns (Phone.dc.html).
	b.WriteString(`<div class="max-md:[&_dl]:grid-cols-2" data-now-band>`)
	b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("Projects wired and fresh", s.WiredFresh),
		valueStat("Plan rows proven", plansProven),
		valueStat("Governed claims current", s.ClaimsCurrent),
		valueStat("Live certificates", s.CertifyLive),
		valueStat("Gate refusals", s.GateRefusals),
		valueStat("Agent sessions", s.Sessions),
	}, stat.BandOptions{AriaLabel: "where the fleet stands"}))
	b.WriteString(`</div>`)

	b.WriteString(`<div class="mt-4 grid min-w-0 gap-4 lg:grid-cols-3">`)
	b.WriteString(`<div class="min-w-0 lg:col-span-2">` + renderAttentionPanel(rep.Attention, m) + `</div>`)
	b.WriteString(`<div class="min-w-0">` + renderSincePanel(rep.Since, now) + `</div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="mt-4">` + renderPulsePanel(rep.Pulse, m) + `</div>`)
	b.WriteString(`<div class="mt-4">` + renderMachineSurfaces(m) + `</div>`)
	return b.String()
}

// ─── the attention queue ────────────────────────────────────────────────

// nowAttentionShown is how many items the panel lists before the rest
// move behind a disclosure.
const nowAttentionShown = 10

func renderAttentionPanel(a console.Attention, m observatoryModel) string {
	var body strings.Builder
	switch a.State {
	case console.StateZero:
		body.WriteString(chrome.RenderEmptyState(chrome.EmptyState{
			Heading: "Nothing needs attention",
			Body:    "Every source was read: no gate failure, unproven claim, stale index, certificate gap, uncompiled guide or open observation.",
		}))
	case console.StateAbsent:
		// The panel's truth line names why; there is no list to show.
	default:
		body.WriteString(`<p class="mb-1 text-xs text-ink-muted" data-attention-counts>` + html.EscapeString(attentionCounts(a)) + `</p>`)
		shown := a.Items
		if len(shown) > nowAttentionShown {
			shown = shown[:nowAttentionShown]
		}
		body.WriteString(renderAttentionList(shown, 1))
		if rest := a.Items[len(shown):]; len(rest) > 0 {
			body.WriteString(`<div class="mt-2">` + chrome.RenderDisclosure(chrome.Disclosure{
				Summary: fmt.Sprintf("Show the other %s", plural(len(rest), "item")),
				Body:    template.HTML(renderAttentionList(rest, len(shown)+1)),
			}) + `</div>`)
		}
		body.WriteString(`<p class="mt-2 text-xs text-ink-muted">Nothing here is applied for you: each item names the command that fixes it. Wiring changes go through Preview → Apply → Undo on <a class="inline-flex min-h-11 items-center text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="/wiring">Wiring</a>.</p>`)
	}
	return renderPanel(consolePanel{
		ID: "attention", Title: "Needs attention",
		Question:    "What blocks proof first, and which command fixes it?",
		Source:      "the verify ledger (each plan's run at the project's HEAD, from any clone; else the registered root's newest, stale), the daemon's store (certify status, the obligations compile), the registry's freshness and each repo's .fathom/observations.md",
		Denominator: fmt.Sprintf("%s across %s", plural(len(a.Items), "item"), plural(a.Projects, "project")),
		Freshness:   "as of " + m.GeneratedAt,
		State:       a.State, Reason: a.Reason,
		SourceKey: "plans index layers",
		Body:      body.String(),
	})
}

// attentionCounts is "2 gate · 14 unproven · …" in queue order.
func attentionCounts(a console.Attention) string {
	var parts []string
	for _, k := range console.AttentionKinds {
		if n := a.ByKind[k]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, attentionKindNoun(k)))
		}
	}
	return strings.Join(parts, " · ")
}

func attentionKindNoun(k console.AttentionKind) string {
	switch k {
	case console.AttentionGate:
		return "gate failures"
	case console.AttentionUnproven:
		return "unproven"
	case console.AttentionIndex:
		return "index"
	case console.AttentionCertificates:
		return "certificate gaps"
	case console.AttentionGuides:
		return "uncompiled guides"
	case console.AttentionObservations:
		return "observations"
	}
	return string(k)
}

// attentionRole is an area's badge role; the label always says the area.
func attentionRole(k console.AttentionKind) string {
	switch k {
	case console.AttentionGate, console.AttentionUnproven:
		return "danger"
	case console.AttentionIndex, console.AttentionCertificates:
		return "caution"
	}
	return "info"
}

// renderAttentionList is an ordered list of items starting at rank start.
func renderAttentionList(items []console.AttentionItem, start int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<ol class="divide-y divide-line" start="%d" aria-label="attention items">`, start)
	for _, it := range items {
		fmt.Fprintf(&b, `<li class="grid grid-cols-1 gap-x-3 gap-y-2 py-3 sm:grid-cols-[6.5rem_minmax(0,1fr)]" data-attention-item data-kind="%s">`, html.EscapeString(string(it.Kind)))
		fmt.Fprintf(&b, `<div class="flex items-center gap-2 sm:items-start sm:pt-0.5"><span class="w-6 shrink-0 text-xs tabular-nums text-ink-muted" aria-hidden="true">%d</span>%s</div>`, it.Rank, calloutBadge(it.Area, attentionRole(it.Kind)))
		b.WriteString(`<div class="min-w-0"><p class="font-semibold text-ink [overflow-wrap:anywhere]">` + html.EscapeString(it.Title) + `</p>`)
		b.WriteString(`<p class="mt-0.5 text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(it.Detail) + `</p></div>`)
		// The remedy gets the content column's full width, beside its evidence.
		b.WriteString(`<div class="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 sm:col-start-2">` + remedyHTML(it.Remedy))
		b.WriteString(`<a class="inline-flex min-h-11 items-center text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="` + html.EscapeString(it.Evidence.Href) + `">Evidence: ` + html.EscapeString(it.Evidence.Label) + ` →</a></div>`)
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol>`)
	return b.String()
}

// remedyHTML is a remedy: a command as a copyable chip, anything else as
// a sentence.
func remedyHTML(r string) string {
	if r == "" {
		return ""
	}
	if strings.HasPrefix(r, "fathom ") || strings.HasPrefix(r, "cd ") {
		return renderVerbChip(r)
	}
	return `<p class="text-xs text-ink [overflow-wrap:anywhere]">` + html.EscapeString(r) + `</p>`
}

// calloutBadge is the kit badge with its area label.
func calloutBadge(label, role string) string {
	return `<span data-area>` + kitBadge(label, role) + `</span>`
}

// ─── since you last looked ──────────────────────────────────────────────

// nowSinceShown is how many changes the panel lists before a disclosure.
const nowSinceShown = 8

func renderSincePanel(s console.Since, now time.Time) string {
	var body strings.Builder
	look := "No earlier look is recorded."
	if s.LastLook != "" {
		if t, err := time.Parse(time.RFC3339, s.LastLook); err == nil {
			look = "Last look " + t.UTC().Format("01-02 15:04") + " UTC (" + agoText(t, now) + ")."
		}
	}
	fmt.Fprintf(&body, `<p class="mb-1 text-xs text-ink-muted" data-last-look>%s %s new since.</p>`, html.EscapeString(look), html.EscapeString(plural(s.New, "change")))
	switch {
	case s.State == console.StateZero:
		body.WriteString(chrome.RenderEmptyState(chrome.EmptyState{Heading: "Nothing recorded since " + dayOnly(s.From), Body: "No push, deploy, measurement or restart is in the ledgers this page reads."}))
	case len(s.Items) > 0:
		first := (s.Page-1)*s.PerPage + 1
		if s.Pages > 1 {
			fmt.Fprintf(&body, `<p class="mb-1 text-xs text-ink-muted" data-since-shown>Changes %d–%d of %s, newest first.</p>`, first, first+len(s.Items)-1, fmtThousands(s.Total))
		}
		shown := s.Items
		if len(shown) > nowSinceShown {
			shown = shown[:nowSinceShown]
		}
		body.WriteString(renderChangeList(shown))
		if rest := s.Items[len(shown):]; len(rest) > 0 {
			body.WriteString(`<div class="mt-2">` + chrome.RenderDisclosure(chrome.Disclosure{
				Summary: fmt.Sprintf("Show %s earlier on this page", plural(len(rest), "change")),
				Body:    template.HTML(renderChangeList(rest)),
			}) + `</div>`)
		}
		if s.Pages > 1 {
			// Every change in the window is on some page: the list is paged,
			// never cut (design §2).
			body.WriteString(`<div class="mt-3 [&_a]:min-h-11 [&_span]:min-h-11" data-since-pages>` + chrome.RenderPagination(chrome.Pagination{
				Page: s.Page, TotalPages: s.Pages, AriaLabel: "changes pages",
				HrefFor: func(n int) string {
					if n <= 1 {
						return "/#since"
					}
					return "/?since_page=" + strconv.Itoa(n) + "#since"
				},
			}) + `</div>`)
		}
	}
	var read []string
	for _, src := range s.Sources {
		if src.State == console.StatePresent {
			read = append(read, src.Name)
		}
	}
	denom := fmt.Sprintf("%s since %s", plural(s.Total, "change"), dayOnly(s.From))
	if s.Pages > 1 {
		denom += fmt.Sprintf(" · page %d of %d", s.Page, s.Pages)
	}
	return renderPanel(consolePanel{
		ID: "since", Title: "Since you last looked",
		Question:    "What changed: pushes, deploys, measurements and restarts?",
		Source:      "read: " + strings.Join(read, ", ") + " — each change is a ledger row, never inferred",
		Denominator: denom,
		Freshness:   "as rendered",
		State:       s.State, Reason: s.Reason,
		SourceKey: "daemon index",
		Body:      body.String(),
	})
}

func renderChangeList(items []console.Change) string {
	var b strings.Builder
	b.WriteString(`<ul class="divide-y divide-line" aria-label="changes, newest first">`)
	for _, c := range items {
		t, _ := time.Parse(time.RFC3339, c.At)
		fmt.Fprintf(&b, `<li class="grid grid-cols-[5.5rem_minmax(0,1fr)] gap-3 py-2" data-change="%s">`, html.EscapeString(string(c.Kind)))
		fmt.Fprintf(&b, `<time class="pt-0.5 font-mono text-xs tabular-nums text-ink-muted" datetime="%s">%s</time>`, html.EscapeString(c.At), html.EscapeString(t.UTC().Format("01-02 15:04")))
		b.WriteString(`<div class="min-w-0"><p class="flex flex-wrap items-center gap-2 text-sm font-semibold text-ink">` + html.EscapeString(c.Title))
		if c.New {
			b.WriteString(kitBadge("new", "accent"))
		}
		b.WriteString(`</p><p class="text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(c.Detail) + `</p></div></li>`)
	}
	b.WriteString(`</ul>`)
	return b.String()
}

// agoText is "3 h ago", "12 min ago", "2 d ago".
func agoText(t, now time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d d ago", int(d.Hours()/24))
}

// ─── the fleet pulse ────────────────────────────────────────────────────

func renderPulsePanel(p console.FleetTableModel, m observatoryModel) string {
	var body strings.Builder
	if len(p.Rows) == 0 {
		body.WriteString(chrome.RenderEmptyState(chrome.EmptyState{Heading: "No projects to show", Body: "The registry listed no project."}))
	} else {
		rows := make([]table.Row, 0, len(p.Rows))
		for _, r := range p.Rows {
			rows = append(rows, table.Row{
				ID:    "pulse-" + r.Service,
				Cells: []string{r.Service, "", "", "", ""},
				HTMLCells: []templ.Component{templ.Raw(projectLink(r.Service)), templ.Raw(labelled("Symbols", "lg", shortValue(r.Symbols, "—"))),
					templ.Raw(labelled("Realization", "lg", rungPips(r.Ladder, r.LadderMet, r.LadderOf))), templ.Raw(labelled("Calls per day", "lg", sharedSparkline(r.Series, r.Service))),
					templ.Raw(labelled("Calls, 14 days", "lg", shortValue(r.Calls, "not measured")))},
			})
		}
		tbl := renderKitTable(table.TableParams{
			TableID: "now-pulse", Caption: "the busiest projects: realization and calls per day on one shared scale",
			Columns: []table.Column{{Key: "project", Label: "Project"}, {Key: "symbols", Label: "Symbols", Align: "right"}, {Key: "realization", Label: "Realization"}, {Key: "traffic", Label: "Calls per day, 14 days"}, {Key: "calls", Label: "Calls", Align: "right"}},
			Rows:    rows,
		})
		// One rendering: the table from lg up, cards below it.
		body.WriteString(`<div class="` + cardTableLG + `" data-pulse-table>` + tbl + `</div>`)
		body.WriteString(`<p class="mt-2 text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(pulseLegend(p)) +
			` · <a class="inline-flex min-h-11 items-center text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="/fleet?sort=calls&amp;dir=desc">Open the fleet →</a></p>`)
		body.WriteString(chrome.RenderDisclosure(chrome.Disclosure{
			Summary: "Calls per day as a table",
			Body:    template.HTML(trafficTable("now-pulse-days", "calls per day for the projects above; \"no rows\" is a day the call log holds no row at all", p.Days, p.Rows)),
		}))
	}
	state, reason := console.StatePresent, ""
	if p.CallLogNote != "" {
		state, reason = console.StateAbsent, p.CallLogNote
	}
	return renderPanel(consolePanel{
		ID: "pulse", Title: "Fleet pulse",
		Question:    "How far is each busy project realizing fathom — wired, fresh, queried, preventing, gating — and how busy is it?",
		Verb:        "fathom fleet report --json",
		Source:      "the daemon's registry, the realization ladder and the call log's calls per day (the observatory model's one fold)",
		Denominator: fmt.Sprintf("%d of %s shown, by 14-day calls", len(p.Rows), plural(p.Total, "project")),
		Freshness:   "as of " + m.GeneratedAt,
		State:       state, Reason: reason,
		SourceKey: "index sessions",
		Body:      body.String(),
	})
}

// pulseLegend says how to read the pulse.
func pulseLegend(p console.FleetTableModel) string {
	l := fmt.Sprintf("One scale for every row, 0–%s calls a day, so a quiet project looks quiet", fmtThousands(p.ScaleMax))
	if n := len(p.AbsentDays); n > 0 {
		l += fmt.Sprintf(" · a dashed box is a day the call log holds no row (%s)", plural(n, "day"))
	}
	return l + " · ✓ met, ✕ not met, – not applicable, ? unknown"
}

// projectLink is a project's name as its workspace link, a 44 px target.
func projectLink(service string) string {
	return `<a href="` + html.EscapeString(projectHref(service)) + `" class="inline-flex min-h-11 items-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus [overflow-wrap:anywhere]">` + html.EscapeString(service) + `</a>`
}

// ─── machine surfaces ───────────────────────────────────────────────────

// renderMachineSurfaces keeps every machine surface one step from the
// front door (OB-11), and points at where the four questions now live.
func renderMachineSurfaces(m observatoryModel) string {
	var mach strings.Builder
	mach.WriteString(kv("MCP over HTTP", `<code class="font-mono text-xs">/rpc/agent</code> · <code class="font-mono text-xs">/rpc/companion</code> · <code class="font-mono text-xs">/rpc</code>`))
	mach.WriteString(kv("services indexed", valueText(console.ServiceCount(fleetFactsFromModel(m)))))
	mach.WriteString(kv("context tax", contextTaxValue(m.AgentSurfaceTools, m.AgentSchemaBytes)))
	mach.WriteString(kv("daemon", `<a class="inline-flex min-h-11 items-center text-accent" href="/statusz">/statusz</a> · <a class="inline-flex min-h-11 items-center text-accent" href="/metrics">/metrics</a> · <a class="inline-flex min-h-11 items-center text-accent" href="/healthz">/healthz</a>`))
	mach.WriteString(kv("the four questions", `<a class="inline-flex min-h-11 items-center text-accent" href="/impact">Impact — Q1 to Q4, with arm ranges →</a> · <a class="inline-flex min-h-11 items-center text-accent" href="/sessions">the week →</a>`))
	return chrome.RenderDisclosure(chrome.Disclosure{Summary: "Machine surfaces and the four questions", Body: template.HTML(mach.String())})
}

// fleetReportSection is `fathom fleet report --json` computed by the verb's
// own function for a page's JSON (it walks every repository, so it is never
// on the HTML path).
func fleetReportSection(cr *consoleRequest) any {
	rep, err := buildFleetReport(consoleRPCClient(5*time.Second), cr.deps.selfURL, fleetReportNow())
	if err != nil {
		return map[string]string{"error": err.Error()}
	}
	return rep
}

// pageMetrics keeps only the shared metrics the page declares it shows.
func pageMetrics(p *consolePage, all console.Metrics) console.Metrics {
	if len(p.Shows) == 0 {
		return nil
	}
	out := console.Metrics{}
	for _, id := range p.Shows {
		if v, ok := all[id]; ok {
			out[id] = v
		}
	}
	return out
}
