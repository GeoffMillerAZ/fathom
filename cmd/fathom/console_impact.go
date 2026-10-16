package main

// console_impact.go — the Impact page at /impact (vision 35 §3 Evidence,
// design §3 "Impact"; OC-41, OC-42). The four questions in a fixed order,
// Q1..Q4. Each counterfactual arm is a range (the interquartile span of its
// per-turn token units, the median marked) beside the sessions it rests on
// (law 12). Every tool's outcome says "not instrumented" where no
// instrument exists — never a colored zero. And what fathom caught — verify
// refusals, conserve losses, integrity catches, guard refusals — is one
// filterable timeline with its rows as the table alternative.
//
// The page renders `fathom metrics impact --json` and `fathom metrics
// catches --json` (metrics_evidence.go), built by the same adapters over the
// observatory model's one read of the ledger and the call log (OC-9).

import (
	"fmt"
	"html"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

func init() {
	declarePage(consolePage{
		ID: "impact", Route: "/impact", Href: "/impact", Group: "Evidence", Order: 0, Nav: true,
		Title:    "Impact",
		Question: "Has fathom helped, measured honestly, with ranges and arm counts?",
		Verbs:    []string{"fathom metrics impact --json", "fathom metrics catches --json"},
		Shows:    []console.MetricID{console.MetricSessions, console.MetricObservations, console.MetricAgentTools},
		Build:    buildImpactPage,
	})
}

// impactPageModel is the Impact page's JSON projection.
type impactPageModel struct {
	Impact  console.ImpactModel        `json:"impact"`
	Catches console.CatchTimelineModel `json:"catches"`
}

// catchDotLimit is how many catches the timeline draws as dots; past it
// each lane is a density strip.
const catchDotLimit = 150

// impactReads caches the page's own reads of the verify ledger and the
// hooks log for the model's TTL: both are megabytes, and every catch they
// hold is already in the rendered timeline.
var impactReads struct {
	sync.Mutex
	verifyAt  time.Time
	verifyKey string
	verify    []verifyLedgerRow
	guardAt   time.Time
	guardKey  string
	guard     catchSources
}

func impactVerifyRows(now time.Time) []verifyLedgerRow {
	impactReads.Lock()
	defer impactReads.Unlock()
	path := verifyLedgerPath()
	if impactReads.verifyKey != path || now.Sub(impactReads.verifyAt) >= obsCacheTTL || now.Before(impactReads.verifyAt) {
		impactReads.verify, impactReads.verifyAt, impactReads.verifyKey = readVerifyLedger(), now, path
	}
	return impactReads.verify
}

// resetImpactReads drops the cached reads (tests that grow a ledger).
func resetImpactReads() {
	impactReads.Lock()
	impactReads.verifyKey, impactReads.guardKey = "", ""
	impactReads.Unlock()
}

// impactCatchSources is freshCatchSources over the model's review rows,
// with the ledger and hooks reads reused within the TTL.
func impactCatchSources(m observatoryModel, facts console.CallLogFacts, labelKey func(string) string, now time.Time) catchSources {
	key := m.GeneratedAt + "|" + verifyLedgerPath() + "|" + beatLogPath()
	impactReads.Lock()
	if impactReads.guardKey != key || now.Sub(impactReads.guardAt) >= obsCacheTTL || now.Before(impactReads.guardAt) {
		impactReads.Unlock()
		src := freshCatchSources(m.reviews, facts, labelKey, m.registry, m.ledgerRoots())
		impactReads.Lock()
		impactReads.guard, impactReads.guardKey, impactReads.guardAt = src, key, now
	}
	src := impactReads.guard
	impactReads.Unlock()
	return src
}

// catchWindows are the timeline's window chips, in days (0 = all time).
var catchWindows = []int{0, 90, 30, 7}

func buildImpactPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	q := cr.r.URL.Query()
	facts := modelCallFacts(m)
	verify, verifyIgnored := verifyLedgerRegisteredRows(impactVerifyRows(cr.now), m.ledgerRoots()) // PE-24
	im := console.ImpactView(impactInputsFrom(impactSources{
		imp: m.Impact, misses: m.Misses, classes: m.toolClass, callFacts: facts,
		ledgerPath: sessionLedgerPath(), ledgerErr: m.ledgerErr, verify: verify, verifyIgnored: verifyIgnored,
	}))
	days, _ := strconv.Atoi(q.Get("days"))
	if days < 0 {
		days = 0
	}
	kinds := q["kind"]
	service := strings.TrimSpace(q.Get("svc"))
	roots := map[string]string{}
	for _, s := range m.Services {
		roots[s.Service] = s.Path
	}
	catchIn := catchInputsFrom(impactCatchSources(m, facts, sessionLabelKey(roots), cr.now))
	if _, registered := roots[service]; service != "" && !registered && !catchIn.NamesService(service) {
		return consoleOut{}, unknownCatchProjectError(service, catchIn.ExcludesNamedService(service), cr)
	}
	cm := console.CatchTimeline(catchIn, catchFilterFrom(kinds, service, days, cr.now), evPage(q), catchPageSize)
	catchVerb := catchCommand(cm.Filter, days, cm.Page)
	metrics := consoleModelMetrics(m)
	return consoleOut{
		Meta:    modelMeta(m),
		Model:   impactPageModel{Impact: im, Catches: cm},
		Metrics: pageMetrics(cr.page, metrics),
		Sections: map[string]any{
			"fathom metrics impact --json": withCallLog(im, facts.Log),
			catchVerb:                      withCallLog(cm, facts.Log),
		},
		Body: func() string { return renderImpactPage(cr, m, im, cm, catchVerb, days, metrics) },
	}, nil
}

// unknownCatchProjectError is the shell's 404 for a project filter that
// names nothing: no registered project and no instrument's row (design §2:
// an unknown service renders on the shell with the reason and the remedy),
// never a timeline of lanes that each read "not instrumented".
func unknownCatchProjectError(service string, excluded bool, cr *consoleRequest) *consoleError {
	reason := "No indexed project is named " + service + ", and no verify run, review run or guard beat names it — there is nothing to show for it."
	if excluded {
		// OC-H2: rows name it, but the registry does not list it.
		reason = "No indexed project is named " + service + ". Rows name it, but the daemon's registry does not list it, so they are not the fleet's: every lane leaves them out, and the unfiltered timeline names them."
	}
	return &consoleError{
		Status: http.StatusNotFound, Title: "Unknown project",
		Reason:      reason,
		Remedy:      "clear the project filter to see every project's catches",
		RemedyHref:  evHref(cr.r.URL.Path, cr.r.URL.Query(), "svc", ""),
		RemedyLabel: "Show every project",
	}
}

// catchCommand is the exact `metrics catches` command the timeline shows.
func catchCommand(f console.CatchFilter, days, page int) string {
	var flags []string
	for _, k := range f.Kinds {
		flags = append(flags, "--kind", string(k))
	}
	if f.Service != "" {
		flags = append(flags, "--service", f.Service)
	}
	if days > 0 {
		flags = append(flags, "--days", strconv.Itoa(days))
	}
	if page > 1 {
		flags = append(flags, "--page", strconv.Itoa(page))
	}
	return evCommand("fathom metrics catches", flags...)
}

func renderImpactPage(cr *consoleRequest, m observatoryModel, im console.ImpactModel, cm console.CatchTimelineModel, catchVerb string, days int, metrics console.Metrics) string {
	asOf := "as of the model read at " + m.GeneratedAt
	const verb = "fathom metrics impact --json"
	var b strings.Builder
	b.WriteString(`<div class="grid gap-4">`)

	// Q1 — the arms as ranges.
	q1 := im.Questions[0]
	var q1b strings.Builder
	q1b.WriteString(`<p class="mb-3 max-w-prose text-ink" data-impact-answer>` + html.EscapeString(q1.Answer) + `</p>`)
	q1b.WriteString(renderArmRanges(im))
	q1b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("distinct sessions (all time)", metrics[console.MetricSessions]),
		valueStat("OFF-arm rows excluded", im.Excluded),
		figureStat(q1.Figures, "exp.raw_per_question", "raw exploration, per question"),
		figureStat(q1.Figures, "exp.indexed_per_question", "indexed discovery, per question"),
	}, stat.BandOptions{AriaLabel: "Q1 figures"}))
	note := strings.TrimSpace(im.ArmNote)
	if note != "" && !strings.HasSuffix(note, ".") {
		note += "."
	}
	q1b.WriteString(`<p class="mt-2 max-w-prose text-xs text-ink-muted">` + html.EscapeString(strings.TrimSpace(note+" The exploration figures are observational — the arms carry causality (law 12).")) + `</p>`)
	b.WriteString(renderPanel(consolePanel{
		ID: "q1", Title: "Q1 · " + q1.Question, SourceKey: "sessions",
		Question: "What would the agent have spent without fathom, as a range over the counterfactual arms?",
		Verb:     verb, Source: "the session ledger (" + sessionLedgerPath() + "), last row per session",
		Denominator: "sessions admitted to each arm (" + console.ArmUnit + ")", Freshness: asOf,
		State: armsState(im), Reason: armsReason(im), Body: q1b.String(),
	}))

	// Q2 — what fathom prevented, then the catches timeline.
	q2 := im.Questions[1]
	b.WriteString(renderPanel(consolePanel{
		ID: "q2", Title: "Q2 · " + q2.Question, SourceKey: "sessions plans",
		Question: "How often did a gate or a rule stop a mistake before it landed?",
		Verb:     verb, Source: "the verify ledger, the session ledger and the call log",
		Denominator: "each figure names its own", Freshness: asOf,
		Body: `<p class="mb-3 max-w-prose text-ink" data-impact-answer>` + html.EscapeString(q2.Answer) + `</p>` + figureBand(q2.Figures, "Q2 figures"),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "catches", Title: "What fathom caught", SourceKey: "plans sessions",
		Question:    "Which verify refusals, conserve losses, integrity catches and guard refusals happened, and when?",
		Verb:        catchVerb,
		Source:      "the verify ledger, `fathom review` rows in the call log, and the hooks log's guard beats",
		Denominator: catchCount(cm.Total) + " in the window",
		Freshness:   "read at most 30 s ago — the verify ledger and hooks log reads are reused for 30 s, the review rows are the model's read",
		Body:        renderCatchTimeline(cr, cm, days),
	}))

	// Q3 — per-tool outcomes.
	q3 := im.Questions[2]
	var q3b strings.Builder
	q3b.WriteString(`<p class="mb-3 max-w-prose text-ink" data-impact-answer>` + html.EscapeString(q3.Answer) + `</p>`)
	q3b.WriteString(stat.RenderBand([]stat.Stat{
		figureStat(q3.Figures, "tools.misses", "misses"),
		figureStat(q3.Figures, "tools.uninstrumented", "tools with no outcome instrument"),
		valueStat("open observations", metrics[console.MetricObservations]),
		valueStat("agent-surface tools", metrics[console.MetricAgentTools]),
	}, stat.BandOptions{AriaLabel: "Q3 figures"}))
	q3b.WriteString(`<div class="mt-3">` + renderToolOutcomes(im) + `</div>`)
	b.WriteString(renderPanel(consolePanel{
		ID: "q3", Title: "Q3 · " + q3.Question, SourceKey: "sessions",
		Question: "Which tools miss, and which cannot tell — every tool labelled by who calls it?",
		Verb:     verb, Source: "the call log's outcome fields (IQ-1) and each row's caller (origin, surface)",
		Denominator: plural(len(im.Tools), "called tool") + "; console reads excluded", Freshness: asOf,
		Body: q3b.String(),
	}))

	// Q4 — compounding.
	q4 := im.Questions[3]
	b.WriteString(renderPanel(consolePanel{
		ID: "q4", Title: "Q4 · " + q4.Question, SourceKey: "sessions",
		Question: "Does an agent reach its first edit sooner as the index and rules grow?",
		Verb:     verb, Source: "the session ledger (first_edit_turn) and the call log",
		Denominator: "each figure names its own", Freshness: asOf,
		Body: `<p class="mb-3 max-w-prose text-ink" data-impact-answer>` + html.EscapeString(q4.Answer) + `</p>` + figureBand(q4.Figures, "Q4 figures"),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

// figureStat is one question figure as a kit tile (absent never 0).
func figureStat(figs []console.ImpactFigure, id, label string) stat.Stat {
	for _, f := range figs {
		if f.ID == id {
			s := valueStat(label, f.Value)
			if f.Unit != "" && f.Value.Known() {
				s.Value += " " + f.Unit
				if strings.HasPrefix(f.Unit, "turns") || strings.HasPrefix(f.Unit, "tokens/") {
					s.Value = fmtThousands(f.Value.V)
					s.Sub = strings.TrimSpace(f.Unit + " · " + s.Sub)
				}
			}
			return s
		}
	}
	return stat.Absent(label, "not computed")
}

// figureBand renders every figure as a tile.
func figureBand(figs []console.ImpactFigure, aria string) string {
	items := make([]stat.Stat, 0, len(figs))
	for _, f := range figs {
		items = append(items, figureStat(figs, f.ID, f.Label))
	}
	return stat.RenderBand(items, stat.BandOptions{AriaLabel: aria})
}

// armsState is the Q1 panel's headline truth state: absent when no arm has
// an admitted session.
func armsState(im console.ImpactModel) console.State {
	for _, a := range im.Arms {
		if a.Median.Known() {
			return console.StatePresent
		}
	}
	if len(im.Arms) > 0 && im.Arms[0].Median.State == console.StateError {
		return console.StateError
	}
	return console.StateAbsent
}

func armsReason(im console.ImpactModel) string {
	if armsState(im) == console.StatePresent {
		return ""
	}
	return im.Arms[0].Median.Reason
}

// renderArmRanges draws each arm that has admitted sessions as a range bar
// (p25..p75, median marked) with its session count in the label, names each
// arm that has none, and gives the table alternative.
func renderArmRanges(im console.ImpactModel) string {
	var items, narrow []viz.RangeItem
	var absent []string
	for _, a := range im.Arms {
		if !a.Median.Known() {
			absent = append(absent, `<li data-truth-state="`+string(a.Median.State)+`"><span class="font-medium text-ink">`+html.EscapeString(a.Label)+`</span> — <span class="italic text-ink-muted">not measured: `+html.EscapeString(a.Median.Reason)+`</span></li>`)
			continue
		}
		label := strings.ToUpper(a.Arm) + " · " + plural(a.Units.V, "session")
		items = append(items, viz.RangeItem{Label: label, Low: float64(a.Low.V), High: float64(a.High.V), Point: float64(a.Median.V), HasPoint: true, Role: armRole(a.Arm)})
		narrow = append(narrow, viz.RangeItem{Label: strings.ToUpper(a.Arm) + " · " + fmtThousands(a.Units.V), Low: float64(a.Low.V), High: float64(a.High.V), Point: float64(a.Median.V), HasPoint: true, Role: armRole(a.Arm)})
	}
	var b strings.Builder
	if len(items) > 0 {
		aria := "counterfactual arms: interquartile range of " + console.ArmUnit + ", median marked"
		wide := viz.RangeBars(items, viz.RangeBarsOptions{W: 560, LabelW: 150, AriaLabel: aria, Fluid: true})
		small := viz.RangeBars(narrow, viz.RangeBarsOptions{W: 320, LabelW: 96, AriaLabel: aria, Fluid: true})
		b.WriteString(`<div class="hidden max-w-[35rem] md:block" data-chart-width="wide">` + wide + `</div>`)
		b.WriteString(`<div class="max-w-80 md:hidden" data-chart-width="narrow">` + small + `</div>`)
		b.WriteString(`<p class="mt-1 text-xs text-ink-muted">bar: p25 – p75 of ` + html.EscapeString(console.ArmUnit) + `; tick: the median</p>`)
	}
	if len(absent) > 0 {
		b.WriteString(`<ul class="mt-2 space-y-1 text-sm" data-arm-absent>` + strings.Join(absent, "") + `</ul>`)
	}
	rows := make([]table.Row, 0, len(im.Arms))
	for _, a := range im.Arms {
		rows = append(rows, table.Row{ID: "arm-" + a.Arm, Cells: []string{a.Label}, HTMLCells: []templComponent{nil,
			evRaw(evCell(a.Units, "")), evRaw(evCell(a.Low, "not measured")), evRaw(evCell(a.Median, "not measured")), evRaw(evCell(a.High, "not measured"))}})
	}
	tbl := renderKitTable(table.TableParams{TableID: "impact-arms", Caption: "counterfactual arms (" + console.ArmUnit + ")",
		Columns: []table.Column{{Key: "arm", Label: "Arm"}, {Key: "units", Label: "Sessions", Align: "right"}, {Key: "p25", Label: "p25", Align: "right"}, {Key: "median", Label: "Median", Align: "right"}, {Key: "p75", Label: "p75", Align: "right"}},
		Rows:    rows})
	b.WriteString(evChart("", "", "", "the arms as a table", tbl))
	return b.String()
}

func armRole(arm string) string {
	if arm == "off" {
		return "caution"
	}
	return "accent"
}

// renderToolOutcomes is the per-tool outcome table: a tool with no outcome
// instrument says so in words in every outcome column.
func renderToolOutcomes(im console.ImpactModel) string {
	const shown = 15
	rows := make([]table.Row, 0, shown)
	for i, o := range im.Tools {
		if i == shown {
			break
		}
		rows = append(rows, table.Row{ID: "tool-" + o.Tool, Cells: []string{o.Tool, string(o.Class)}, HTMLCells: []templComponent{
			evRaw(`<code class="font-mono text-xs">` + html.EscapeString(o.Tool) + `</code>`), nil,
			evRaw(`<span class="tabular-nums">` + fmtThousands(o.Calls) + `</span>`),
			evRaw(evCell(o.Instrumented, "")),
			evRaw(evCell(o.Hits, "not instrumented")), evRaw(evCell(o.Misses, "not instrumented")), evRaw(evCell(o.DidYouMean, "not instrumented")),
		}})
	}
	caption := fmt.Sprintf("per-tool outcomes — the %d most-called of %s", min(shown, len(im.Tools)), plural(len(im.Tools), "tool"))
	tbl := renderKitTable(table.TableParams{TableID: "impact-tools", Caption: caption,
		Columns: []table.Column{{Key: "tool", Label: "Tool"}, {Key: "class", Label: "Caller"}, {Key: "calls", Label: "Calls", Align: "right"},
			{Key: "instrumented", Label: "Outcome recorded", Align: "right"}, {Key: "hits", Label: "Hits", Align: "right"},
			{Key: "misses", Label: "Misses", Align: "right"}, {Key: "dym", Label: "Did you mean", Align: "right"}},
		Rows: rows})
	// Below md the outcome columns — the point of the table — would sit
	// off-screen in a sideways scroll: each tool reads whole instead.
	var stack []evStackItem
	for i, o := range im.Tools {
		if i == shown {
			break
		}
		stack = append(stack, evStackItem{
			Heading: `<code class="font-mono text-xs">` + html.EscapeString(o.Tool) + `</code> <span class="text-xs font-normal text-ink-muted">· ` + html.EscapeString(string(o.Class)) + ` · ` + plural(o.Calls, "call") + `</span>`,
			Items: []chrome.DescItem{
				{Term: "outcome recorded", DetailHTML: template.HTML(evCell(o.Instrumented, ""))},
				{Term: "hits · misses", DetailHTML: template.HTML(evCell(o.Hits, "not instrumented") + " · " + evCell(o.Misses, "not instrumented"))},
				{Term: "did you mean", DetailHTML: template.HTML(evCell(o.DidYouMean, "not instrumented"))},
			},
		})
	}
	return `<p class="mb-2 text-xs text-ink-muted md:hidden">` + html.EscapeString(caption) + `</p>` + evTableOrStack("impact-tools", "per-tool outcomes", tbl, stack) + `<p class="mt-2 text-xs text-ink-muted">"not instrumented": none of the tool's calls records an outcome (rows before IQ-1, or a tool with no result count) — there is no number, so none is drawn. Caller: the class making most of the tool's calls (agent, edit hook, gate, refresh, plumbing, unattributed).</p>`
}

// renderCatchTimeline is the catches panel's body: kind and window chips,
// the timeline (one lane per kind), each lane's count or its absence, and
// the rows as the table alternative, paged.
func renderCatchTimeline(cr *consoleRequest, cm console.CatchTimelineModel, days int) string {
	q := cr.r.URL.Query()
	path := cr.r.URL.Path
	var b strings.Builder
	chips := []evChip{{Label: "all kinds", Href: evHref(path, q, "kind", ""), Selected: len(cm.Filter.Kinds) == 0}}
	for _, k := range cm.Kinds {
		count := valueChipCount(k.Count)
		chips = append(chips, evChip{Label: k.Label, Href: evHref(path, q, "kind", string(k.Kind)), Selected: len(cm.Filter.Kinds) > 0 && k.Selected, Count: count})
	}
	b.WriteString(evChips("Kind", chips))
	var win []evChip
	for _, d := range catchWindows {
		label := "all time"
		if d > 0 {
			label = strconv.Itoa(d) + " days"
		}
		v := ""
		if d > 0 {
			v = strconv.Itoa(d)
		}
		win = append(win, evChip{Label: label, Href: evHref(path, q, "days", v), Selected: d == days})
	}
	b.WriteString(evChips("Window", win))
	if cm.Filter.Service != "" {
		b.WriteString(evChips("Project", []evChip{{Label: cm.Filter.Service, Href: evHref(path, q, "svc", ""), Selected: true}, {Label: "every project", Href: evHref(path, q, "svc", "")}}))
	}

	// One lane per kind that has points; the lanes' counts (or absences)
	// are listed below, so an empty kind is never an unexplained blank.
	if len(cm.Window) > 0 {
		events := make([]viz.Event, 0, len(cm.Window))
		for _, p := range cm.Window {
			events = append(events, viz.Event{At: p.At, Label: console.CatchKindLabel(p.Kind), Role: catchRole(p.Kind), Lane: console.CatchKindLabel(p.Kind)})
		}
		opts := viz.TimelineOptions{W: 900, Fluid: true, AriaLabel: "catches over time, one lane per kind"}
		if len(events) > catchDotLimit {
			// Hundreds of dots on one lane read as a bar and weigh a page;
			// past the limit each lane is a density strip — catches per
			// time bin — and the table below lists every catch.
			opts.View, opts.DensityBuckets = viz.ViewDensity, 48
			opts.AriaLabel = "catches over time: catches per time bin, one row per kind"
		}
		if !cm.Filter.From.IsZero() {
			opts.Start = cm.Filter.From
			opts.End = cr.now
		}
		tl := viz.Timeline(events, opts)
		b.WriteString(`<div class="overflow-x-auto" role="region" tabindex="0" aria-label="catches timeline — scrolls sideways on a narrow screen"><div class="min-w-[900px] max-w-[900px] md:min-w-0">` + tl + `</div></div>`)
	}
	var lanes strings.Builder
	lanes.WriteString(`<ul class="mt-3 grid gap-1 text-sm sm:grid-cols-2" aria-label="catches per kind" data-catch-lanes>`)
	for _, k := range cm.Kinds {
		lanes.WriteString(`<li class="min-w-0 [overflow-wrap:anywhere]"><span class="font-medium text-ink">` + html.EscapeString(k.Label) + `</span>: ` + valueText(k.Count))
		if k.Count.Known() && k.Count.Denominator != "" {
			lanes.WriteString(` <span class="text-xs text-ink-muted">(` + html.EscapeString(k.Count.Denominator) + `)</span>`)
		}
		lanes.WriteString(`</li>`)
	}
	lanes.WriteString(`</ul>`)
	b.WriteString(lanes.String())

	if cm.Total == 0 {
		b.WriteString(`<p class="mt-3 italic text-ink-muted" data-catches-empty>no catch in this window and filter — each kind above says whether its instrument recorded anything to count from</p>`)
		return b.String()
	}
	rows := make([]table.Row, 0, len(cm.Catches))
	var list strings.Builder
	for i, c := range cm.Catches {
		svcHTML := html.EscapeString(c.Service)
		if c.Service != "" {
			svcHTML = `<a href="` + html.EscapeString(evHref(path, q, "svc", c.Service)) + `" class="inline-flex min-h-11 items-center text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(c.Service) + `</a>`
		}
		when := c.At.UTC().Format("2006-01-02 15:04")
		rows = append(rows, table.Row{ID: "catch-" + strconv.Itoa(i), HTMLCells: []templComponent{
			evRaw(`<span class="whitespace-nowrap tabular-nums">` + when + `</span>`), evPlainCell(console.CatchKindLabel(c.Kind)), evRaw(svcHTML),
			evRaw(`<span class="[overflow-wrap:anywhere]">` + html.EscapeString(c.Subject) + `</span>`), evPlainCell(c.Detail)}})
		// The same row stacked for a phone.
		list.WriteString(`<li class="min-w-0 border-b border-line py-2"><p class="text-xs text-ink-muted"><span class="tabular-nums">` + when + `</span> · <span class="font-medium text-ink">` + html.EscapeString(console.CatchKindLabel(c.Kind)) + `</span></p>` +
			`<p class="[overflow-wrap:anywhere]">` + html.EscapeString(c.Subject) + ` — ` + html.EscapeString(c.Detail) + `</p>`)
		if c.Service != "" {
			list.WriteString(`<p class="text-sm">` + svcHTML + `</p>`)
		}
		list.WriteString(`</li>`)
	}
	caption := fmt.Sprintf("catches, newest first — %d of %s", len(cm.Catches), fmtThousands(cm.Total))
	b.WriteString(`<div class="mt-3 hidden md:block" data-catches-table>` + renderKitTable(table.TableParams{TableID: "impact-catches",
		Caption: caption,
		Columns: []table.Column{{Key: "at", Label: "When (UTC)"}, {Key: "kind", Label: "Kind"}, {Key: "service", Label: "Project"}, {Key: "subject", Label: "Subject"}, {Key: "detail", Label: "Detail"}},
		Rows:    rows}) + `</div>`)
	b.WriteString(`<div class="mt-3 md:hidden"><p class="text-xs text-ink-muted">` + html.EscapeString(caption) + `</p><ul aria-label="catches" data-catches-list>` + list.String() + `</ul></div>`)
	b.WriteString(evPagination(path, q, cm.Page, cm.Pages, "catches pages"))
	if len(cm.Window) < cm.Total {
		b.WriteString(`<p class="mt-1 text-xs text-ink-muted">the timeline draws the newest ` + fmtThousands(len(cm.Window)) + ` of ` + fmtThousands(cm.Total) + ` catches; the table pages through all of them</p>`)
	}
	return b.String()
}

func catchRole(k console.CatchKind) string {
	switch k {
	case console.CatchVerifyRefusal:
		return "danger"
	case console.CatchConserveLoss:
		return "caution"
	case console.CatchIntegrity:
		return "info"
	}
	return "accent"
}

// valueChipCount is a chip's count: the number, "≥ n", or "—" for an
// absence (the lane list below names why).
func valueChipCount(v console.Value[int]) string {
	switch v.State {
	case console.StateAbsent, console.StateError:
		return "—"
	case console.StatePartial:
		return "≥ " + fmtThousands(v.V)
	}
	return fmtThousands(v.V)
}

// catchCount is "1 catch" / "2 catches".
func catchCount(n int) string {
	if n == 1 {
		return "1 catch"
	}
	return fmtThousands(n) + " catches"
}
