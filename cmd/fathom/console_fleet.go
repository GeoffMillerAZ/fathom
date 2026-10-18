package main

// console_fleet.go — the Fleet page at /fleet (vision 35 §3, plan OC-19,
// OC-20): every project in one sortable, filterable table, rolled up by
// health above it — unknown is never green — with calls per day on one
// shared scale that never draws across a day the call log did not observe.
//
// Sort, filter and page are URL parameters (console.FleetQuery): every view
// is a real route, works without JavaScript, and back returns to it. The
// page's JSON carries `fathom fleet report --json`, computed by the verb's
// own function (buildFleetReport), and the table's own projection
// (console.FleetTable) as its model.

import (
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/topology"
)

func init() {
	declarePage(consolePage{
		ID: "fleet", Route: "/fleet", Href: "/fleet", Group: "Fleet", Order: 0, Nav: true,
		Title:    "Fleet",
		Question: "How is every project doing: wired, fresh, proven, used?",
		Verbs:    []string{"fathom fleet report --json"},
		Shows: []console.MetricID{
			console.MetricServices, console.MetricSymbols, console.MetricCalls, console.MetricObservations,
			console.MetricWiredFresh, console.MetricPlansProven, console.MetricClaims, console.MetricCertLive,
		},
		Build: buildFleetPage,
	})
}

func buildFleetPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	pr := cr.projects()
	q := console.ParseFleetQuery(cr.r.URL.Query())
	tbl := console.FleetTable(pr.projects, q)
	metrics := consoleSharedMetrics(cr, m, pr)
	// SR-55 (F-57): the reconcile trigger runs ONLY on an explicit
	// ?reconcile=1 — a person following the fleet page's own "Check wiring
	// drift" button, never on every ordinary page load (which would make
	// the read-only wiring walk over every repo cost something on the
	// page's normal budget, and would blur "user-triggered" into "runs
	// whenever anyone looks at the fleet").
	reconcileTriggered := cr.r.URL.Query().Get("reconcile") == "1"
	var reconcileDiffs []wiringDiff
	if reconcileTriggered {
		roots, svcByRoot := fleetReconcileRoots(pr.projects.Rows)
		reconcileDiffs = consoleReconcileAction(roots, svcByRoot)
	}
	out := consoleOut{
		Meta:    modelMeta(m),
		Model:   tbl,
		Metrics: pageMetrics(cr.page, metrics),
		Body:    func() string { return renderFleet(tbl, metrics, m, reconcileTriggered, reconcileDiffs) },
	}
	// The verb section is computed for JSON only: the fleet report walks
	// every repository and is not on the HTML render path.
	if wantsJSON(cr.r) {
		out.Sections = map[string]any{"fathom fleet report --json": fleetReportSection(cr)}
	}
	return out, nil
}

// renderFleet is the page body: the totals, the health rollup, the
// filterable table, then the wiring-drift panel (SR-55, F-57).
func renderFleet(t console.FleetTableModel, metrics console.Metrics, m observatoryModel, reconcileTriggered bool, reconcileDiffs []wiringDiff) string {
	var b strings.Builder
	// The shared projections (OC-9): every row in the resolved log, the
	// unattributed rows named in the caption, never dropped.
	callFacts := modelCallFacts(m)
	callsTile := valueStat("tool calls (log)", metrics[console.MetricCalls])
	if caption := callLogRuleCaption(callFacts); caption != "" {
		callsTile.Sub = strings.TrimPrefix(callsTile.Sub+" · "+caption, " · ")
	}
	b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("services indexed", metrics[console.MetricServices]),
		valueStat("symbols", metrics[console.MetricSymbols]),
		callsTile,
		valueStat("open observations", metrics[console.MetricObservations]),
	}, stat.BandOptions{AriaLabel: "fleet totals"}))
	b.WriteString(`<div class="mt-3">`)
	b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("projects wired and fresh", metrics[console.MetricWiredFresh]),
		valueStat("plan rows proven", metrics[console.MetricPlansProven]),
		valueStat("governed claims current", metrics[console.MetricClaims]),
		valueStat("live certificates", metrics[console.MetricCertLive]),
	}, stat.BandOptions{AriaLabel: "fleet proof"}))
	b.WriteString(`</div>`)
	b.WriteString(`<div class="mt-4">` + renderFleetHealth(t, m) + `</div>`)
	b.WriteString(`<div class="mt-4">` + renderFleetTablePanel(t, m) + `</div>`)
	// RG-7: the realization grades over 30 days, the phase-U scoreboard.
	b.WriteString(`<div class="mt-4">` + renderGradeDistribution(m) + `</div>`)
	b.WriteString(`<div class="mt-4">` + renderFleetReconcile(reconcileTriggered, reconcileDiffs) + `</div>`)
	return b.String()
}

// fleetReconcileRoots maps the fleet's own project rows into the
// (roots, service-by-root) shape consoleReconcileAction takes — every row
// with a known local Path, mirroring cmdFleetReconcile's own "only repos
// whose path is known and stat-able" discipline (reconcile.go) minus the
// stat (a path the registry itself gave the console is trusted the same
// way the rest of this page already trusts it for every other panel).
func fleetReconcileRoots(rows []console.ProjectFacts) ([]string, map[string]string) {
	var roots []string
	svcByRoot := map[string]string{}
	for _, r := range rows {
		if r.Path == "" {
			continue
		}
		roots = append(roots, r.Path)
		svcByRoot[r.Path] = r.Service
	}
	return roots, svcByRoot
}

// renderFleetReconcile is SR-55's own panel (F-57): a "Check wiring drift"
// button when not yet triggered, or — once GET /fleet?reconcile=1 has
// run — every repo's drift, with the CLI command that fixes it (--apply
// stays CLI-only; see this file's own package doc comment for why).
func renderFleetReconcile(triggered bool, diffs []wiringDiff) string {
	// SourceKey "daemon" (console.EventDaemon's own key): wiring drift is
	// fleet-wide, not scoped to one service's index/layers/plans/sessions —
	// "daemon" is the one existing producible key that, like this panel,
	// names no particular service (events.go's own EventDaemon).
	const reconcileSourceKey = "daemon"
	if !triggered {
		return renderPanel(consolePanel{
			ID: "reconcile", Title: "Wiring drift",
			Question: "Is every project's hook/stanza/skill wiring still current with the template fathom ships?",
			Verb:     "fathom fleet reconcile",
			Source:   "a read-only walk of each project's .claude/settings.json, AGENTS.md stanza and CLAUDE-file import, and shipped skill files",
			State:    console.StatePresent, SourceKey: reconcileSourceKey,
			Body: `<p class="text-sm text-ink-muted mb-2">Not checked this page load — reconcile runs only when asked (never on a schedule).</p>` +
				form.RenderButton(form.Button{Label: "Check wiring drift", Href: "/fleet?reconcile=1", Variant: form.ButtonGhost}),
		})
	}
	var driftedRepos int
	var b strings.Builder
	for _, d := range diffs {
		if !wiringDiffHasDrift(d) {
			continue
		}
		driftedRepos++
		label := d.Repo
		if d.Service != "" {
			label = d.Service + " (" + d.Repo + ")"
		}
		fmt.Fprintf(&b, "<li><strong>%s</strong>: %s</li>", html.EscapeString(label), html.EscapeString(strings.Join(wiringDiffDriftSummary(d), "; ")))
	}
	body := `<p class="text-sm text-ink-muted mb-2">` + plural(driftedRepos, "project") + ` drifted, of ` + plural(len(diffs), "checked") + `.</p>`
	if driftedRepos > 0 {
		body += `<ul class="list-disc pl-5 text-sm space-y-1">` + b.String() + `</ul>` +
			`<p class="text-sm text-ink-muted mt-2">Fix the safe subset: <code>fathom fleet reconcile --apply</code> (dry-run shown here first; applying is a deliberate, separate CLI step).</p>`
	}
	body += `<p class="mt-2">` + form.RenderButton(form.Button{Label: "Re-check", Href: "/fleet?reconcile=1", Variant: form.ButtonGhost}) + `</p>`
	state := console.StatePresent
	if driftedRepos > 0 {
		state = console.StatePartial
	}
	return renderPanel(consolePanel{
		ID: "reconcile", Title: "Wiring drift",
		Question: "Is every project's hook/stanza/skill wiring still current with the template fathom ships?",
		Verb:     "fathom fleet reconcile",
		Source:   "a read-only walk of each project's .claude/settings.json, AGENTS.md stanza and CLAUDE-file import, and shipped skill files",
		State:    state, SourceKey: reconcileSourceKey,
		Body: body,
	})
}

// wiringDiffDriftSummary names each kind of drift a repo carries, in a
// short human list — never the full wiringDiff dump, which cmdFleetReconcile's
// own stdout already renders in full for whoever runs the CLI command
// this panel names.
func wiringDiffDriftSummary(d wiringDiff) []string {
	var out []string
	if n := len(d.Missing); n > 0 {
		out = append(out, plural(n, "missing hook entry"))
	}
	if n := len(d.StaleCmds); n > 0 {
		out = append(out, plural(n, "stale absolute path"))
	}
	if n := len(d.MissingSkills); n > 0 {
		out = append(out, plural(n, "missing skill file"))
	}
	if n := len(d.StaleSkills); n > 0 {
		out = append(out, plural(n, "stale skill file"))
	}
	if n := len(d.BlockedSkills); n > 0 {
		out = append(out, plural(n, "blocked skill file")+" (hand-edited; not auto-applied)")
	}
	if n := len(d.StaleStanzas); n > 0 {
		out = append(out, plural(n, "stale stanza"))
	}
	if n := len(d.DriftedStanzas); n > 0 {
		out = append(out, plural(n, "hand-edited stanza")+" (blocked; not auto-applied)")
	}
	if d.SettingsNote != "" {
		out = append(out, d.SettingsNote)
	}
	if n := len(d.WiringDrift); n > 0 {
		out = append(out, plural(n, "MCP/skill entry")+" drifted")
	}
	if n := len(d.WiringUnreadable); n > 0 {
		out = append(out, plural(n, "unreadable wiring file"))
	}
	return out
}

// ─── the health rollup ──────────────────────────────────────────────────

//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var healthStatus = map[console.Health]topology.Status{
	console.HealthFailed: topology.Failed, console.HealthDegraded: topology.Degraded,
	console.HealthUnknown: topology.Unknown, console.HealthHealthy: topology.Healthy,
}

func renderFleetHealth(t console.FleetTableModel, m observatoryModel) string {
	var nodes []topology.Node
	for _, g := range t.Rollup {
		node := topology.Node{
			Name: healthLabel(g.Health), Status: healthStatus[g.Health], Metric: plural(g.Count, "project"),
			Href: fleetHref(console.FleetQuery{Health: string(g.Health)}.Normalize()),
		}
		for _, svc := range g.Services {
			node.Children = append(node.Children, topology.Node{Name: svc, Status: healthStatus[g.Health], Metric: g.Why[svc], Href: projectHref(svc)})
		}
		nodes = append(nodes, node)
	}
	// The kit tree's links are text-sized; the wrapper gives every link and
	// summary the 44 px target the console holds itself to (returned as
	// FATHOM-B-KIT-02, docs/app-kit.md).
	body := `<div class="[&_a]:inline-flex [&_a]:min-h-11 [&_a]:items-center [&_a]:focus-visible:outline-2 [&_a]:focus-visible:outline-offset-2 [&_a]:focus-visible:outline-focus [&_summary]:min-h-11 [&_summary]:focus-visible:outline-2 [&_summary]:focus-visible:outline-focus [&_[role=treeitem]]:min-w-0 [&_.ml-auto]:min-w-0 [&_.ml-auto]:[overflow-wrap:anywhere] [&_.ml-auto]:text-right" data-fleet-health>` +
		topology.Render(nodes, topology.Options{AriaLabel: "projects by health", Collapsed: true}) + `</div>` +
		`<p class="mt-2 text-xs text-ink-muted">Open a state to see its projects and why each is there; its name filters the table below. Failed: a claimed plan row fails the gate at HEAD, or the typed call-edge pass failed. Degraded: a stale index, missing wiring, unproven claims at HEAD or an unreadable observations file. Unknown: freshness or the gate state at HEAD cannot be determined — a plan never verified, or verified only at an older commit — never shown as healthy.</p>`
	state, reason := console.StatePresent, ""
	if m.DegradedNote != "" {
		state, reason = console.StatePartial, "registry read incompletely: "+m.DegradedNote
	} else if m.RegistryStale != "" {
		state, reason = console.StateStale, m.RegistryStale
	}
	return renderPanel(consolePanel{
		ID: "health", Title: "Health",
		Question:    "Which projects are failed, degraded, unknown or healthy — and why?",
		Verb:        "fathom fleet report --json",
		Source:      "the registry's freshness and wiring, the verify ledger, the typed-edge pass on /statusz and each repo's observations file",
		Denominator: plural(t.Total, "project"),
		Freshness:   "as of " + m.GeneratedAt,
		State:       state, Reason: reason,
		SourceKey: "index plans",
		Body:      body,
	})
}

func healthLabel(h console.Health) string {
	switch h {
	case console.HealthFailed:
		return "Failed"
	case console.HealthDegraded:
		return "Degraded"
	case console.HealthUnknown:
		return "Unknown"
	}
	return "Healthy"
}

// ─── the table ──────────────────────────────────────────────────────────

func renderFleetTablePanel(t console.FleetTableModel, m observatoryModel) string {
	var body strings.Builder
	body.WriteString(renderFleetFilters(t))
	shown := "Showing " + fleetShownRange(t)
	if t.Query.Filtered() {
		var f []string
		if t.Query.Health != "" {
			f = append(f, "health "+t.Query.Health)
		}
		if t.Query.Q != "" {
			f = append(f, "name or path containing “"+t.Query.Q+"”")
		}
		if t.Query.Lang != "" {
			f = append(f, "language "+t.Query.Lang)
		}
		shown += " — " + strings.Join(f, ", ")
	}
	sortLabel := t.Query.Sort
	if s, ok := fleetSortByKey(t.Query.Sort); ok {
		sortLabel = s.Label
	}
	shown += fmt.Sprintf(" · sorted by %s, %s", strings.ToLower(sortLabel), map[string]string{"asc": "ascending", "desc": "descending"}[t.Query.Dir])
	body.WriteString(`<p class="my-2 text-xs text-ink-muted" data-fleet-shown>` + html.EscapeString(shown) + `</p>`)
	if len(t.Rows) == 0 {
		body.WriteString(chrome.RenderEmptyState(chrome.EmptyState{Heading: "No project matches", Body: "Clear the filters to see every project."}))
	} else {
		// One rendering: the kit table from xl up, the same rows as cards
		// below it (cardTableXL), never two copies of every row.
		body.WriteString(`<div class="` + cardTableXL + ` xl:[&_td]:px-2 xl:[&_th]:px-2" data-fleet-table>` + fleetKitTable(t) + `</div>`)
	}
	if t.Pages > 1 {
		q := t.Query
		body.WriteString(`<div class="mt-3">` + chrome.RenderPagination(chrome.Pagination{
			Page: q.Page, TotalPages: t.Pages, AriaLabel: "fleet pages",
			HrefFor: func(n int) string { p := q; p.Page = n; return fleetHref(p) },
		}) + `</div>`)
	}
	body.WriteString(`<p class="mt-2 text-xs text-ink-muted">` + html.EscapeString(pulseLegend(t)) + `</p>`)
	if len(t.Rows) > 0 {
		body.WriteString(`<div class="mt-2">` + chrome.RenderDisclosure(chrome.Disclosure{
			Summary: "Calls per day as a table",
			Body:    template.HTML(trafficTable("fleet-days", "calls per day for the projects above; \"no rows\" is a day the call log holds no row at all", t.Days, t.Rows)),
		}) + `</div>`)
	}
	state, reason := console.StatePresent, ""
	switch {
	case m.DegradedNote != "":
		state, reason = console.StatePartial, "registry read incompletely: "+m.DegradedNote
	case t.CallLogNote != "":
		state, reason = console.StatePartial, "calls not measured: "+t.CallLogNote
	case m.RegistryStale != "":
		state, reason = console.StateStale, m.RegistryStale
	}
	return renderPanel(consolePanel{
		ID: "projects", Title: "Projects",
		Question:    "How is each project doing — wired, fresh, proven, used — sorted and filtered your way?",
		Verb:        "fathom fleet report --json",
		Source:      "the fleet report's registry, freshness, wiring and observations; the realization ladder and calls per day from the observatory model's call-log fold; plans from the verify ledger; claims and certify status from the daemon's store",
		Denominator: fleetTableDenominator(t, m),
		Freshness:   "as of " + m.GeneratedAt,
		State:       state, Reason: reason,
		SourceKey: "index layers sessions plans",
		Body:      body.String(),
	})
}

// fleetTableDenominator is the table's range, and the call-log rows no
// project's calls count because their service is not in the registry
// (OC-H2), named.
func fleetTableDenominator(t console.FleetTableModel, m observatoryModel) string {
	d := fleetShownRange(t)
	if p := modelCallFacts(m).Unregistered.Phrase("call-log row"); p != "" {
		d += " · calls: " + p
	}
	return d
}

// fleetShownRange names the rows on this page against every match and the
// fleet: "18 of 18 projects", or, once the table pages, "51–100 of 120
// matching, of 200 projects".
func fleetShownRange(t console.FleetTableModel) string {
	if t.Pages <= 1 {
		return fmt.Sprintf("%d of %s", t.Matched, plural(t.Total, "project"))
	}
	s := fmt.Sprintf("%d–%d of %d", t.ShownFrom, t.ShownTo, t.Matched)
	if t.Matched != t.Total {
		return s + " matching, of " + plural(t.Total, "project")
	}
	return s + " projects"
}

// renderFleetFilters is the table's GET form: health, name and language,
// with the sort carried along.
func renderFleetFilters(t console.FleetTableModel) string {
	q := t.Query
	healthOpts := []form.Option{{Value: "", Label: "Every state"}}
	for _, h := range console.HealthOrder {
		healthOpts = append(healthOpts, form.Option{Value: string(h), Label: healthLabel(h)})
	}
	langOpts := []form.Option{{Value: "", Label: "Every language"}}
	for _, l := range t.Languages {
		langOpts = append(langOpts, form.Option{Value: l, Label: l})
	}
	var b strings.Builder
	b.WriteString(`<form method="get" action="/fleet" role="search" aria-label="Filter and sort projects" class="flex flex-wrap items-end gap-3 [&_input]:min-h-11 [&_select]:min-h-11 [&_button]:min-h-11" data-fleet-filters>`)
	b.WriteString(`<div class="w-full sm:w-44">` + form.RenderField(form.Field{Label: "Health", For: "fleet-health", Control: template.HTML(form.RenderSelect(form.Select{ID: "fleet-health", Name: "health", Value: q.Health, Options: healthOpts}))}) + `</div>`)
	b.WriteString(`<div class="w-full sm:w-56">` + form.RenderField(form.Field{Label: "Project name or path", For: "fleet-q", Control: template.HTML(form.RenderInput(form.Input{ID: "fleet-q", Name: "q", Type: "search", Value: q.Q, Autocomplete: "off"}))}) + `</div>`)
	b.WriteString(`<div class="w-full sm:w-44">` + form.RenderField(form.Field{Label: "Language", For: "fleet-lang", Control: template.HTML(form.RenderSelect(form.Select{ID: "fleet-lang", Name: "lang", Value: q.Lang, Options: langOpts}))}) + `</div>`)
	// Sorting from the form as well as the headers: below xl the table
	// reads as cards and its header row is hidden.
	sortOpts := make([]form.Option, 0, len(console.FleetSorts))
	for _, so := range console.FleetSorts {
		sortOpts = append(sortOpts, form.Option{Value: so.Key, Label: so.Label})
	}
	b.WriteString(`<div class="w-full sm:w-48">` + form.RenderField(form.Field{Label: "Sort by", For: "fleet-sort", Control: template.HTML(form.RenderSelect(form.Select{ID: "fleet-sort", Name: "sort", Value: q.Sort, Options: sortOpts}))}) + `</div>`)
	b.WriteString(`<div class="w-full sm:w-36">` + form.RenderField(form.Field{Label: "Order", For: "fleet-dir", Control: template.HTML(form.RenderSelect(form.Select{ID: "fleet-dir", Name: "dir", Value: q.Dir, Options: []form.Option{{Value: "desc", Label: "Descending"}, {Value: "asc", Label: "Ascending"}}}))}) + `</div>`)
	b.WriteString(`<div class="flex flex-wrap items-center gap-2">` + form.RenderButton(form.Button{Label: "Apply", Variant: form.ButtonPrimary}))
	if q.Filtered() {
		clear := console.FleetQuery{Sort: q.Sort, Dir: q.Dir}.Normalize()
		b.WriteString(`<span class="[&>a]:min-h-11">` + form.RenderButton(form.Button{Label: "Clear filters", Href: fleetHref(clear), Variant: form.ButtonGhost}) + `</span>`)
	}
	b.WriteString(`</div></form>`)
	return b.String()
}

// fleetHref is the fleet page at a query.
func fleetHref(q console.FleetQuery) string {
	v := q.Values()
	if len(v) == 0 {
		return "/fleet"
	}
	return "/fleet?" + v.Encode()
}

func fleetSortByKey(key string) (console.FleetSort, bool) {
	for _, s := range console.FleetSorts {
		if s.Key == key {
			return s, true
		}
	}
	return console.FleetSort{}, false
}

// sortHeader is a sortable column's header: a link to the table sorted by
// it (the kit's sortable header is a script-driven button; the console's
// sort is a real route), naming the current direction in words.
func sortHeader(t console.FleetTableModel, key string) templ.Component {
	s, _ := fleetSortByKey(key)
	q := t.Query
	dir, arrow, state := "asc", "↕", ""
	if s.Desc {
		dir = "desc"
	}
	if q.Sort == key {
		if q.Dir == "asc" {
			dir, arrow, state = "desc", "↑", ", sorted ascending; activate to sort descending"
		} else {
			dir, arrow, state = "asc", "↓", ", sorted descending; activate to sort ascending"
		}
	}
	next := console.FleetQuery{Sort: key, Dir: dir, Health: q.Health, Q: q.Q, Lang: q.Lang}.Normalize()
	cls := "text-ink-muted"
	if q.Sort == key {
		cls = "text-ink"
	}
	return templ.Raw(`<a href="` + html.EscapeString(fleetHref(next)) + `" class="inline-flex min-h-11 items-center gap-1 text-left text-xs font-semibold uppercase tracking-wide hover:text-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus ` + cls + `" data-sort="` + html.EscapeString(key) + `">` +
		`<span>` + html.EscapeString(s.Label) + `&nbsp;<span aria-hidden="true" class="text-accent">` + arrow + `</span></span><span class="sr-only">` + html.EscapeString(state) + `</span></a>`)
}

// fleetCells is one row's cells, shared by the table and the cards.
type fleetCells struct {
	project, symbols, languages, index, realization, plans, claims, certs, calls, observations string
}

func fleetRowCells(r console.FleetRow) fleetCells {
	c := fleetCells{
		// From xl up the reason sits in the badge's title and the rollup
		// above (the table has no room for a sentence per row); a screen
		// reader hears it everywhere, and the cards below xl show it.
		project: `<div class="min-w-[8rem]">` + projectLink(r.Service) + `<div class="flex flex-wrap items-center gap-2" title="` + html.EscapeString(r.HealthWhy) + `">` + healthBadge(r.Health) +
			`<span class="text-xs text-ink-muted [overflow-wrap:anywhere] xl:sr-only">` + html.EscapeString(r.HealthWhy) + `</span></div></div>`,
		symbols:      shortValue(r.Symbols, "not reported"),
		realization:  rungPips(r.Ladder, r.LadderMet, r.LadderOf),
		claims:       shortValue(r.ClaimsCurrent, "none"),
		certs:        shortValue(r.CertsLive, "none declared"),
		observations: shortValue(r.Observations, "no file"),
	}
	switch n := len(r.Languages); {
	case n == 0:
		c.languages = `<span class="italic text-ink-muted">not reported</span>`
	case n <= 3:
		c.languages = html.EscapeString(strings.Join(r.Languages, ", "))
	default:
		c.languages = `<span title="` + html.EscapeString(strings.Join(r.Languages, ", ")) + `">` + html.EscapeString(strings.Join(r.Languages[:2], ", ")) + ` <span class="text-ink-muted">+` + strconv.Itoa(n-2) + `<span class="sr-only"> more: ` + html.EscapeString(strings.Join(r.Languages[2:], ", ")) + `</span></span></span>`
	}
	switch r.Freshness {
	case "fresh":
		c.index = `<span class="text-positive">fresh</span>`
	case "stale":
		short := shortFreshness(r.FreshnessNote)
		if short == "" {
			short = "stale"
		}
		c.index = `<span class="text-caution" title="` + html.EscapeString(r.FreshnessNote) + `">` + html.EscapeString(short) + `</span>`
	default:
		c.index = `<span class="italic text-ink-muted" title="` + html.EscapeString(r.FreshnessNote) + `">unknown</span>`
	}
	if r.ClaimsTotal > 0 && r.ClaimsCurrent.Known() {
		c.claims += ` <span class="text-xs text-ink-muted">of ` + fmtThousands(r.ClaimsTotal) + `</span>`
	}
	if r.CertsDeclared > 0 && r.CertsLive.Known() {
		c.certs += ` <span class="text-xs text-ink-muted">of ` + fmtThousands(r.CertsDeclared) + `</span>`
	}
	short := "no plans"
	if r.PlansVerified.State == console.StateAbsent && strings.Contains(r.PlansVerified.Reason, "no verify run") {
		short = "no verify run"
	}
	c.plans = shortValue(r.PlansVerified, short)
	if p := r.Plans; p != nil {
		c.plans += ` <span class="text-xs text-ink-muted">of ` + fmtThousands(p.Claimed) + `</span>`
		var extra []string
		if p.Failing > 0 {
			extra = append(extra, `<span class="text-danger">`+fmtThousands(p.Failing)+` failing</span>`)
		}
		if p.Unproven > 0 {
			extra = append(extra, `<span class="text-caution">`+fmtThousands(p.Unproven)+` unproven</span>`)
		}
		if p.Stale > 0 {
			// A stale run's findings are history: named as such, never red.
			then := ""
			if n := p.StaleFailing + p.StaleUnproven; n > 0 {
				then = fmt.Sprintf(" (%s not proven then)", fmtThousands(n))
			}
			extra = append(extra, `<span class="text-caution">`+plural(p.Stale, "plan")+` verified before HEAD`+then+`</span>`)
		}
		if len(p.Unrun) > 0 {
			extra = append(extra, plural(len(p.Unrun), "plan")+` never verified`)
		}
		if p.Unclaimed > 0 {
			extra = append(extra, fmtThousands(p.Unclaimed)+` open or skipped`)
		}
		if len(extra) > 0 {
			c.plans += `<div class="text-xs text-ink-muted">` + strings.Join(extra, " · ") + `</div>`
		}
	}
	c.calls = `<span class="inline-flex flex-col items-end gap-0.5 xl:items-start">` + sharedSparkline(r.Series, r.Service) + `<span class="text-xs">` + shortValue(r.Calls, "not measured") + `</span></span>`
	return c
}

func fleetKitTable(t console.FleetTableModel) string {
	keys := []string{"service", "symbols", "", "freshness", "realization", "plans", "claims", "certs", "calls", "observations"}
	labels := []string{"Project", "Symbols", "Languages", "Index", "Realization", "Plans proven", "Claims current", "Certificates", "Calls, 14 days", "Observations"}
	cols := make([]table.Column, len(keys))
	for i, k := range keys {
		cols[i] = table.Column{Key: nonEmptyKey(k, "languages"), Label: labels[i]}
		if k != "" {
			cols[i].HeaderHTML = sortHeader(t, k)
		}
		switch k {
		case "symbols", "observations":
			cols[i].Align = "right"
		}
	}
	rows := make([]table.Row, 0, len(t.Rows))
	for _, r := range t.Rows {
		c := fleetRowCells(r)
		cells := []string{c.project, c.symbols, c.languages, c.index, c.realization, c.plans, c.claims, c.certs, c.calls, c.observations}
		rich := make([]templ.Component, len(cells))
		for i, s := range cells {
			if i > 0 {
				s = labelled(labels[i], "xl", s)
			}
			rich[i] = templ.Raw(s)
		}
		rows = append(rows, table.Row{ID: "fleet-" + r.Service, Cells: make([]string, len(cells)), HTMLCells: rich})
	}
	return renderKitTable(table.TableParams{TableID: "fleet-projects", Caption: "every project; sort by a column's name, or with Sort by above", Columns: cols, Rows: rows})
}

func nonEmptyKey(k, fallback string) string {
	if k == "" {
		return fallback
	}
	return k
}
