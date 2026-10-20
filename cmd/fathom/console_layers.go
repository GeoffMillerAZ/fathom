package main

// console_layers.go — the semantic-layers pages (vision 35 §4.2 "the
// strata", design §3 "Semantic layers", plan OC-27..OC-29): /layers draws
// the four strata with every layer in its band, the services × layers
// coverage matrix (the kit's viz.Heatmap with truth states — FATHOM-KIT-03:
// a cell nothing compiled is hatched "not compiled", never a low value) and
// the claim lifecycle; /layers/{layer} is one layer's compile history.
//
// Every panel renders a verb (P10): `fathom layers strata --json`,
// `fathom layers coverage --json` and `fathom layers show <layer> --json`
// print the same internal/console projections the page's JSON sections
// carry, over the daemon's own store.

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

const (
	verbLayersStrata   = "fathom layers strata --json"
	verbLayersCoverage = "fathom layers coverage --json"
	verbLayerShow      = "fathom layers show {layer} --json"
)

func init() {
	declarePage(consolePage{
		ID: "layers", Route: "/layers", Href: "/layers", Group: "Knowledge", Order: 0, Nav: true,
		Title:    "Semantic layers",
		Question: "What governed claims exist, where they came from, and how fresh they are?",
		Verbs:    []string{verbLayersStrata, verbLayersCoverage},
		Shows:    []console.MetricID{console.MetricClaims},
		Build:    buildLayersPage,
	})
	declarePage(consolePage{
		ID: "layer", Route: "/layers/{layer}", Group: "Knowledge", Order: 1, NavParent: "layers",
		Title:    "Layer",
		Question: "What did this layer's compiles cost, what did they keep and reject, and what is still uncompiled?",
		Verbs:    []string{verbLayerShow},
		Build:    buildLayerPage,
	})
}

// knowledgeSourceFor is the daemon's store as the Knowledge projections
// read it; a daemon with no store open is a named error on the shell.
func knowledgeSourceFor(cr *consoleRequest) (console.KnowledgeSource, error) {
	if cr.deps.c == nil {
		return console.KnowledgeSource{}, &consoleError{Status: http.StatusServiceUnavailable, Title: "Store unreadable",
			Reason: "this daemon has no store open, so no governed claim can be read", Remedy: "fathom serve-http --store <path>",
			RemedyHref: "/statusz", RemedyLabel: "Open the daemon status"}
	}
	return console.KnowledgeSource{Store: cr.deps.c.Store(), StorePath: cr.deps.storePath,
		EdgeTally: func(ctx context.Context, svc string) (console.EdgeTally, error) { return cachedEdgeTally(ctx, cr, svc) }}, nil
}

// edgeTallyCache keeps each service's typed-edge tally per daemon. Reading
// every typed_edges row of a large fleet costs more than /packs' render
// budget, and the rows only move when a service is ingested — which moves
// the event bus's cursor. A tally is reused while the cursor holds, for at
// most edgeTallyTTL (an out-of-band `ingest --store` publishes no event).
// Bounded.
var edgeTallyCache = struct {
	sync.Mutex
	entries map[string]edgeTallyEntry
}{entries: map[string]edgeTallyEntry{}}

type edgeTallyEntry struct {
	cursor string
	at     time.Time
	tally  console.EdgeTally
}

const (
	edgeTallyTTL = 2 * time.Minute
	edgeTallyMax = 256
)

func cachedEdgeTally(ctx context.Context, cr *consoleRequest, svc string) (console.EdgeTally, error) {
	key := fmt.Sprintf("%p\x00%s", cr.deps, svc)
	cursor := ""
	if cr.deps.bus != nil {
		cursor = cr.deps.bus.Cursor()
	}
	edgeTallyCache.Lock()
	if e, ok := edgeTallyCache.entries[key]; ok && e.cursor == cursor && !cr.now.Before(e.at) && cr.now.Sub(e.at) < edgeTallyTTL {
		edgeTallyCache.Unlock()
		return e.tally, nil
	}
	edgeTallyCache.Unlock()
	tally, err := console.TallyTypedEdges(ctx, cr.deps.c.Store(), svc)
	if err != nil {
		return nil, err
	}
	edgeTallyCache.Lock()
	if len(edgeTallyCache.entries) >= edgeTallyMax {
		clear(edgeTallyCache.entries)
	}
	edgeTallyCache.entries[key] = edgeTallyEntry{cursor: cursor, at: cr.now, tally: tally}
	edgeTallyCache.Unlock()
	return tally, nil
}

// knowledgeFreshness is the panels' freshness: they are read per request
// and follow compile events live.
const knowledgeFreshness = "read from the store for this request; follows compile events"

const storeFooter = "the daemon's store — semantic_claims, compile_runs, claim_events"

func buildLayersPage(cr *consoleRequest) (consoleOut, error) {
	src, err := knowledgeSourceFor(cr)
	if err != nil {
		return consoleOut{}, err
	}
	ctx := cr.r.Context()
	strata, cov, err := console.LayersOverview(ctx, src)
	if err != nil {
		return consoleOut{}, err
	}
	t := strata.Totals
	meta := []string{
		pluralValue(t.Layers, "layer"),
		valuePlain(t.Claims) + " current claims",
		plural(t.Projects, "project") + " indexed",
	}
	return consoleOut{
		Meta:     meta,
		Actions:  knowledgeActions([][2]string{{"Browse claims", claimsHref("", "", "")}, {"Explain a file", "/explain"}, {"Open packs", "/packs"}}),
		Metrics:  pageMetrics(cr.page, console.Metrics{console.MetricClaims: t.Claims}),
		Sections: map[string]any{verbLayersStrata: strata, verbLayersCoverage: cov},
		Body:     func() string { return renderLayersPage(strata, cov) },
	}, nil
}

// knowledgeActions renders heading actions as the kit's link-mode ghost
// buttons at 44 px.
func knowledgeActions(links [][2]string) string {
	var b strings.Builder
	for _, l := range links {
		b.WriteString(`<span class="[&>a]:min-h-11">` + form.RenderButton(form.Button{Label: l[0], Href: l[1], Variant: form.ButtonGhost}) + `</span>`)
	}
	return b.String()
}

// dollarStat is a kit tile for a projected dollar figure, its truth state
// kept: absent is stat.Absent, never "$0.00".
func dollarStat(label string, v console.Value[float64]) stat.Stat {
	switch v.State {
	case console.StateAbsent:
		s := stat.Absent(label, v.Reason)
		s.ValueRole = "ink-muted"
		return s
	case console.StateError:
		return stat.Stat{Label: label, Value: "unreadable", ValueRole: "danger", Sub: v.Reason}
	case console.StatePartial:
		return stat.Stat{Label: label, Value: fmt.Sprintf("≥ $%.2f", v.V), Sub: v.Reason}
	}
	return stat.Stat{Label: label, Value: fmt.Sprintf("$%.2f", v.V), Sub: v.Denominator}
}

// dollarText renders a dollar figure inline, never a $0.00 for nothing.
func dollarText(v console.Value[float64]) string {
	switch v.State {
	case console.StateAbsent:
		return `<span class="italic text-ink-muted">not measured — ` + html.EscapeString(v.Reason) + `</span>`
	case console.StateError:
		return `<span class="text-danger">unreadable — ` + html.EscapeString(v.Reason) + `</span>`
	case console.StatePartial:
		return fmt.Sprintf("≥ $%.2f", v.V)
	}
	return fmt.Sprintf("$%.2f", v.V)
}

// layerHref is a layer page.
func layerHref(layer string) string { return "/layers/" + url.PathEscape(layer) }

// knowledgeLink is an inline 44 px link. Names break only where the text
// allows (a hyphen, a space), so a table column is never squeezed to a
// letter wide; pathLink breaks a path anywhere.
func knowledgeLink(href, text string) string {
	return `<a href="` + html.EscapeString(href) + `" class="inline-flex min-h-11 items-center rounded-sm font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(text) + `</a>`
}

// pathLink is knowledgeLink for a path or citation, which may break
// anywhere.
func pathLink(href, text string) string {
	return `<a href="` + html.EscapeString(href) + `" class="inline-flex min-h-11 items-center rounded-sm font-mono text-xs text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus [overflow-wrap:anywhere]">` + html.EscapeString(text) + `</a>`
}

// claimsWord renders a count inside a table cell: the number, or a short
// word with the reason as its title — never 0 for nothing compiled.
func claimsWord(v console.Value[int]) string {
	switch v.State {
	case console.StateAbsent:
		return `<span class="italic text-ink-muted" data-truth-state="absent" title="` + html.EscapeString(v.Reason) + `">not compiled</span>`
	case console.StateError:
		return `<span class="text-danger" data-truth-state="error" title="` + html.EscapeString(v.Reason) + `">unreadable</span>`
	}
	return valueText(v)
}

// pluralValue renders a projected count with its noun: "9 layers", a lower
// bound "≥ 8 layers", or the named absence.
func pluralValue(v console.Value[int], noun string) string {
	switch v.State {
	case console.StatePartial:
		return "≥ " + plural(v.V, noun)
	case console.StateAbsent, console.StateError:
		return noun + "s " + valuePlain(v)
	}
	return plural(v.V, noun)
}

// sourceBadge names where a pack comes from, in words, as the kit's badge
// (callout.Badge): neutral, except a pack that could not be looked up.
func sourceBadge(source string) string {
	label := map[string]string{console.PackEmbedded: "embedded", console.PackProject: "project pack", console.PackKit: "kit pack", console.PackTech: "tech pack", console.PackOrphan: "no pack", console.PackUnknown: "pack unknown"}[source]
	if label == "" {
		label = source
	}
	role := ""
	if source == console.PackUnknown {
		role = "caution"
	}
	return callout.Badge(label, role)
}

// lifecycleBar is a layer's statuses as the kit's stacked bar with the
// counts in words beside it (color is never the only channel).
func lifecycleBar(name string, s console.StatusCounts) string {
	total := s.Current() + s.Superseded + s.Refuted
	var words []string
	for _, p := range []struct {
		n     int
		label string
	}{{s.Fresh, "fresh"}, {s.Stale, "stale"}, {s.Contradicted, "contradicted"}, {s.Superseded, "superseded"}, {s.Refuted, "refuted"}} {
		if p.n > 0 {
			words = append(words, fmtThousands(p.n)+" "+p.label)
		}
	}
	if total == 0 {
		return `<p class="mt-2 text-xs italic text-ink-muted" data-lifecycle>no claims — lifecycle not measured</p>`
	}
	bar := viz.StackBar([]viz.Segment{
		{Value: float64(s.Fresh), Role: "positive", Label: "fresh"},
		{Value: float64(s.Stale), Role: "caution", Label: "stale"},
		{Value: float64(s.Contradicted), Role: "danger", Label: "contradicted"},
		{Value: float64(s.Superseded), Role: "ink-faint", Label: "superseded"},
		{Value: float64(s.Refuted), Role: "line-strong", Label: "refuted"},
	}, viz.StackBarOptions{W: 240, H: 8, Fluid: true, AriaLabel: name + " claim lifecycle: " + strings.Join(words, ", ")})
	return `<div class="mt-2" data-lifecycle><div class="max-w-60">` + bar + `</div><p class="mt-1 text-xs text-ink-muted">` + html.EscapeString(strings.Join(words, " · ")) + `</p></div>`
}

func renderLayersPage(s console.StrataView, cov console.CoverageMatrix) string {
	var b strings.Builder
	t := s.Totals
	b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("current claims", t.Claims),
		valueStat("projects with a claim", t.Services),
		valueStat("real compile runs", t.Runs),
		dollarStat("spent compiling", t.CostUSD),
		valueStat("rejected at compile", t.Rejected),
	}, stat.BandOptions{AriaLabel: "semantic-layer totals"}))

	b.WriteString(`<div class="mt-4 flex flex-col gap-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "strata", Title: "The strata", SourceKey: "layers index",
		Question:    "Where does every governed claim come from? Dialects index code; platform and concern packs compile knowledge on top.",
		Verb:        verbLayersStrata,
		Source:      storeFooter + " — and each project's .fathom/layers",
		Denominator: pluralValue(t.Layers, "layer") + " across " + plural(t.Projects, "project"),
		Freshness:   knowledgeFreshness,
		Body:        renderStrataBands(s) + renderStrataTable(s),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "coverage", Title: "Knowledge coverage", SourceKey: "layers index",
		Question:    "Which projects have governed claims in which layer? Hatched means nothing was compiled — absent, not zero.",
		Verb:        verbLayersCoverage,
		Source:      storeFooter,
		Denominator: valuePlain(cov.Covered) + " " + cov.Covered.Denominator,
		Freshness:   knowledgeFreshness,
		State:       cov.Covered.State, Reason: cov.Covered.Reason,
		Body: renderCoverage(cov),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "lifecycle", Title: "Claim lifecycle", SourceKey: "layers",
		Question:    "What happens to a claim after it is compiled?",
		Verb:        verbLayersStrata,
		Source:      "compile_runs (emitted, rejected) and claim_events (verdicts; store-wide — events carry no project)",
		Denominator: "every recorded run and verdict",
		Freshness:   knowledgeFreshness,
		Body:        renderLifecycle(s),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

// renderStrataBands draws the bands (four, plus tech when a tech pack ships a layer): a stratum label beside its
// blocks, stacking below md.
func renderStrataBands(s console.StrataView) string {
	var b strings.Builder
	b.WriteString(`<ol class="flex flex-col divide-y divide-line rounded-md border border-line" aria-label="the strata, dialect first">`)
	for _, band := range s.Bands {
		b.WriteString(`<li class="grid gap-3 p-3 md:grid-cols-[12rem_1fr]" data-stratum="` + html.EscapeString(band.Stratum) + `">`)
		b.WriteString(`<div class="min-w-0"><h3 class="text-sm font-semibold text-ink">` + html.EscapeString(band.Title) + `</h3>`)
		b.WriteString(`<p class="mt-1 text-xs text-ink-muted">` + html.EscapeString(band.Summary) + `</p></div>`)
		b.WriteString(`<div class="grid min-w-0 gap-2 sm:grid-cols-2 xl:grid-cols-3">`)
		for _, d := range band.Dialects {
			b.WriteString(`<div class="min-w-0 rounded-md border border-line bg-surface p-3" data-dialect-level="` + html.EscapeString(d.Level) + `">`)
			b.WriteString(`<p class="text-sm font-semibold text-ink">` + html.EscapeString(dialectLevelWords(d.Level)) + `</p>`)
			b.WriteString(`<p class="mt-1 text-xs text-ink [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(d.Languages, ", ")) + `</p>`)
			if len(d.Producers) > 0 {
				b.WriteString(`<p class="mt-1 text-xs text-ink-muted [overflow-wrap:anywhere]">from an imported SCIP index: ` + html.EscapeString(strings.Join(d.Producers, ", ")) + `</p>`)
			}
			b.WriteString(`</div>`)
		}
		for _, l := range band.Layers {
			b.WriteString(renderLayerBlock(l))
		}
		if len(band.Dialects) == 0 && len(band.Layers) == 0 {
			b.WriteString(`<p class="text-xs italic text-ink-muted" data-band-empty>` + html.EscapeString(band.Reason) + `</p>`)
		}
		if band.Stratum == console.StratumDialect {
			b.WriteString(`<p class="text-xs text-ink-muted sm:col-span-2 xl:col-span-3">` + knowledgeLink("/packs", "The dialect matrix: engines, typed edges and measured precision per language") + `</p>`)
		}
		b.WriteString(`</div></li>`)
	}
	b.WriteString(`</ol>`)
	if len(s.Unplaced) > 0 {
		// A layer whose stratum is not known is listed apart, never guessed
		// into a band: a project pack that failed to load, or one whose
		// holder's pack catalog could not be read.
		b.WriteString(`<div class="mt-3 rounded-md border border-line p-3" data-strata-unplaced><h3 class="text-sm font-semibold text-ink">Not placed in a stratum</h3>`)
		b.WriteString(`<p class="mt-1 text-xs text-ink-muted">Their pack could not be read, so the stratum each declares is not known.</p>`)
		b.WriteString(`<div class="mt-2 grid min-w-0 gap-2 sm:grid-cols-2 xl:grid-cols-3">`)
		for _, l := range s.Unplaced {
			b.WriteString(renderLayerBlock(l))
		}
		b.WriteString(`</div></div>`)
	}
	for _, n := range s.Notes {
		b.WriteString(`<p class="mt-2 text-xs text-caution [overflow-wrap:anywhere]" data-truth-line="partial">` + html.EscapeString(n) + `</p>`)
	}
	return b.String()
}

func dialectLevelWords(level string) string {
	switch level {
	case "native":
		return "native — fathom resolves the types"
	case "scip":
		return "scip — typed edges from an imported index"
	case "file":
		return "file — per-file resolver"
	case "syntax":
		return "syntax — call names only, no typed edges"
	}
	return level
}

// renderLayerBlock is one layer in its band.
func renderLayerBlock(l console.LayerBlock) string {
	var b strings.Builder
	b.WriteString(`<div class="min-w-0 rounded-md border border-line bg-surface p-3" data-layer-block="` + html.EscapeString(l.Name) + `" data-truth-state="` + html.EscapeString(string(l.Claims.State)) + `">`)
	b.WriteString(`<div class="flex flex-wrap items-center justify-between gap-2">` + knowledgeLink(layerHref(l.Name), l.Name) + sourceBadge(l.Source) + `</div>`)
	if l.Claims.State == console.StateAbsent && !l.Runs.Known() {
		// Nothing compiled: one line says so, instead of three absences.
		b.WriteString(`<p class="text-sm italic text-ink-muted" data-truth-line="absent">not compiled — ` + html.EscapeString(l.Claims.Reason) + `</p>`)
		if l.Refusals > 0 {
			b.WriteString(`<p class="mt-1 text-xs text-ink-muted">` + plural(l.Refusals, "refused estimate") + `</p>`)
		}
		if len(l.InstalledIn) > 0 {
			b.WriteString(`<p class="mt-1 text-xs text-ink-muted">installed in ` + html.EscapeString(strings.Join(l.InstalledIn, ", ")) + `</p>`)
		}
		if l.Note != "" {
			b.WriteString(`<p class="mt-1 text-xs text-caution [overflow-wrap:anywhere]">` + html.EscapeString(l.Note) + `</p>`)
		}
		if l.Source == console.PackUnknown {
			b.WriteString(`<p class="mt-1 text-xs text-caution [overflow-wrap:anywhere]">` + html.EscapeString(l.Origin) + `</p>`)
		}
		b.WriteString(`</div>`)
		return b.String()
	}
	b.WriteString(`<p class="text-sm text-ink">` + claimsCount(l.Claims, claimsHref(l.Name, "", "")) + ` <span class="text-ink-muted">current claims</span></p>`)
	runs := valueText(l.Runs) + " runs · " + dollarText(l.CostUSD)
	if l.Refusals > 0 {
		runs += " · " + plural(l.Refusals, "refused estimate")
	}
	b.WriteString(`<p class="mt-1 text-xs text-ink-muted">` + runs + `</p>`)
	if len(l.InstalledIn) > 0 {
		b.WriteString(`<p class="mt-1 text-xs text-ink-muted [overflow-wrap:anywhere]">installed in ` + html.EscapeString(strings.Join(l.InstalledIn, ", ")) + `</p>`)
	}
	b.WriteString(lifecycleBar(l.Name, l.Statuses))
	if l.Note != "" {
		b.WriteString(`<p class="mt-1 text-xs text-caution [overflow-wrap:anywhere]">` + html.EscapeString(l.Note) + `</p>`)
	}
	if l.Source == console.PackUnknown {
		b.WriteString(`<p class="mt-1 text-xs text-caution [overflow-wrap:anywhere]">` + html.EscapeString(l.Origin) + `</p>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// claimsCount renders a claim count linked to the claims it counts (vision
// 35 §5: every number drills down): a known count links, a named absence
// does not.
func claimsCount(v console.Value[int], href string) string {
	if !v.Known() {
		return valueText(v)
	}
	return knowledgeLink(href, valueText(v))
}

// statusCount is a lifecycle count linked to its rows; zero stays plain.
func statusCount(n int, layer, service, status string) string {
	if n == 0 {
		return "0"
	}
	return knowledgeLink(claimsHref(layer, service, status), fmtThousands(n))
}

// renderStrataTable is the strata's table alternative.
func renderStrataTable(s console.StrataView) string {
	var rows []table.Row
	row := func(stratum string, l console.LayerBlock) table.Row {
		claims := claimsWord(l.Claims)
		if l.Claims.Known() {
			claims = knowledgeLink(claimsHref(l.Name, "", ""), valueText(l.Claims))
		}
		return table.Row{
			ID:    "layer-" + l.Name,
			Cells: []string{stratum, l.Name, l.Source, "", fmtThousands(l.Statuses.Fresh), fmtThousands(l.Statuses.Stale), fmtThousands(l.Statuses.Contradicted), fmtThousands(l.Statuses.Superseded), "", ""},
			HTMLCells: []templ.Component{nil, templ.Raw(knowledgeLink(layerHref(l.Name), l.Name)), nil, templ.Raw(claims),
				templ.Raw(statusCount(l.Statuses.Fresh, l.Name, "", "fresh")), templ.Raw(statusCount(l.Statuses.Stale, l.Name, "", "stale")),
				templ.Raw(statusCount(l.Statuses.Contradicted, l.Name, "", "contradicted")), templ.Raw(statusCount(l.Statuses.Superseded, l.Name, "", "superseded")),
				templ.Raw(claimsWord(l.Runs)), templ.Raw(dollarText(l.CostUSD))},
		}
	}
	for _, band := range s.Bands {
		for _, l := range band.Layers {
			rows = append(rows, row(band.Title, l))
		}
	}
	for _, l := range s.Unplaced {
		rows = append(rows, row("not placed", l))
	}
	tbl := renderKitTable(table.TableParams{
		TableID: "strata-layers",
		Caption: "Every layer by stratum",
		Columns: []table.Column{
			{Key: "stratum", Label: "Stratum"}, {Key: "layer", Label: "Layer"}, {Key: "source", Label: "Source"},
			{Key: "claims", Label: "Current claims", Align: "right"}, {Key: "fresh", Label: "Fresh", Align: "right"},
			{Key: "stale", Label: "Stale", Align: "right"}, {Key: "contradicted", Label: "Contradicted", Align: "right"},
			{Key: "superseded", Label: "Superseded", Align: "right"}, {Key: "runs", Label: "Runs", Align: "right"},
			{Key: "cost", Label: "Spent", Align: "right"},
		},
		Rows: rows,
	})
	return `<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "The strata as a table", Body: template.HTML(tbl)}) + `</div>`
}

// heatmapLabelBudget is how wide a row label may run, in px at the kit
// heatmap's 10 px label size: the kit right-aligns each label at x=62 in a
// fixed 70 px gutter, and the SVG clips what starts left of x=0 — a label
// cut there reads as a different name ("esume_as_…"), not a shortened one.
// labelWidth's estimate runs about 5% over the system UI faces Chromium
// draws (measured: "resume_as_…" 66 px drawn, 70 estimated), so the budget
// leaves about 3 px of the 62 px before the clip edge.
const heatmapLabelBudget = 61.0

// heatmapLabel fits a project name to the heatmap's label gutter by its
// estimated rendered width, not its rune count (an "m" is three "i"s
// wide); the table alternative carries every name whole.
func heatmapLabel(s string) string {
	if labelWidth(s) <= heatmapLabelBudget {
		return s
	}
	r := []rune(s)
	for n := len(r) - 1; n > 0; n-- {
		cut := string(r[:n]) + "…"
		if labelWidth(cut) <= heatmapLabelBudget {
			return cut
		}
	}
	return "…"
}

// labelWidth estimates a string's advance width in px at 10 px in a
// proportional sans-serif (the system UI faces agree to within a few
// percent on these classes).
func labelWidth(s string) float64 {
	w := 0.0
	for _, c := range s {
		switch {
		case strings.ContainsRune("il.,:;|!'", c):
			w += 2.6
		case strings.ContainsRune("jftr() -", c):
			w += 3.6
		case c == '…':
			w += 10
		case c == 'm' || c == 'w' || c == 'M' || c == 'W':
			w += 8.8
		case c >= 'A' && c <= 'Z':
			w += 7.0
		case c == '_':
			w += 5.8
		default:
			w += 6.0
		}
	}
	return w
}

// renderCoverage is the matrix: the kit heatmap with truth states and its
// table alternative, then the layers nothing compiled.
func renderCoverage(m console.CoverageMatrix) string {
	var b strings.Builder
	if len(m.Layers) == 0 || len(m.Rows) == 0 {
		b.WriteString(chrome.RenderEmptyState(chrome.EmptyState{Heading: "No layer compiled anywhere", Body: "No project in this store has a compiled or imported claim, so there is no matrix to draw. A dry run estimates the cost first: fathom compile --layer obligations --service <project>."}))
	} else {
		rows := make([][]float64, 0, len(m.Rows))
		states := make([][]viz.HeatmapCell, 0, len(m.Rows))
		ylabels := make([]string, 0, len(m.Rows))
		for _, r := range m.Rows {
			vals := make([]float64, len(r.Cells))
			st := make([]viz.HeatmapCell, len(r.Cells))
			for i, c := range r.Cells {
				vals[i] = math.NaN()
				switch c.Claims.State {
				case console.StatePresent:
					vals[i] = float64(c.Claims.V)
					st[i] = viz.HeatmapCell{State: viz.CellValue, Reason: c.Claims.Denominator}
				case console.StateZero:
					vals[i] = 0
					st[i] = viz.HeatmapCell{State: viz.CellZero, Reason: c.Claims.Denominator}
				case console.StateError:
					st[i] = viz.HeatmapCell{State: viz.CellError, Reason: c.Claims.Reason}
				case console.StateAbsent:
					st[i] = viz.HeatmapCell{State: viz.CellMissing, Reason: c.Claims.Reason}
				default:
					st[i] = viz.HeatmapCell{State: viz.CellUnknown, Reason: c.Claims.Reason}
				}
			}
			rows = append(rows, vals)
			states = append(states, st)
			ylabels = append(ylabels, heatmapLabel(r.Service))
		}
		keys := coverageKeys(m.Layers)
		svg := viz.Heatmap(rows, viz.HeatmapOptions{
			Role: "accent", Cell: 38, XLabels: keys, YLabels: ylabels, ShowValues: true,
			States: states, AriaLabel: "current claims per project and layer; hatched cells were never compiled",
		})
		var key []string
		for i, l := range m.Layers {
			key = append(key, keys[i]+" = "+l)
		}
		b.WriteString(`<div class="grid gap-4 lg:grid-cols-[auto_minmax(0,1fr)]"><div class="min-w-0">`)
		b.WriteString(`<p class="mb-2 text-xs text-ink-muted" data-coverage-key>columns: ` + html.EscapeString(strings.Join(key, " · ")) + `</p>`)
		b.WriteString(`<div class="overflow-x-auto rounded-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" role="region" tabindex="0" aria-label="coverage matrix — scrolls sideways" data-coverage-heatmap>` + svg + `</div>`)
		b.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "The coverage matrix as a table", Body: template.HTML(renderCoverageTable(m))}) + `</div></div><div class="min-w-0">`)
	}
	if len(m.NeverCompiled) > 0 {
		b.WriteString(`<h3 class="text-sm font-semibold text-ink">Never compiled in any project</h3><ul class="mt-1 flex flex-col gap-2 text-sm" data-never-compiled>`)
		for _, a := range m.NeverCompiled {
			where := ""
			if len(a.InstalledIn) > 0 {
				where = " — installed in " + html.EscapeString(strings.Join(a.InstalledIn, ", "))
			}
			b.WriteString(`<li class="min-w-0 rounded-md border border-line bg-surface px-3 py-1"><div class="flex flex-wrap items-center gap-2">` + knowledgeLink(layerHref(a.Layer), a.Layer) + sourceBadge(a.Source) + `</div><p class="text-xs text-ink-muted">` + html.EscapeString(a.Reason) + where + `</p></li>`)
		}
		b.WriteString(`</ul>`)
	}
	b.WriteString(`<p class="mt-3 text-xs text-ink-muted">A missing layer is compiled with a dry run first: ` + `<code class="font-mono">fathom compile --layer &lt;layer&gt; --service &lt;project&gt;</code> estimates the cost and spends nothing.</p>`)
	if len(m.Layers) > 0 && len(m.Rows) > 0 {
		b.WriteString(`</div></div>`)
	}
	return b.String()
}

// coverageKeys names the heatmap's columns in at most five characters (the
// kit draws a column label under a cell without fitting it): a layer's
// first word, else its first five letters; the key line beside the chart
// spells each out.
func coverageKeys(layers []string) []string {
	out := make([]string, len(layers))
	seen := map[string]bool{}
	for i, l := range layers {
		k, _, _ := strings.Cut(l, "-")
		if utf8.RuneCountInString(k) > 5 {
			k = string([]rune(k)[:5])
		}
		if seen[k] {
			k = fmt.Sprintf("L%d", i+1)
		}
		seen[k] = true
		out[i] = k
	}
	return out
}

// renderCoverageTable is the matrix's table alternative: every cell in
// words, each linking to the layer page's row for its project.
func renderCoverageTable(m console.CoverageMatrix) string {
	cols := []table.Column{{Key: "project", Label: "Project"}}
	for _, l := range m.Layers {
		cols = append(cols, table.Column{Key: "layer-" + l, Label: l, Align: "right"})
	}
	var rows []table.Row
	for _, r := range m.Rows {
		cells := []string{r.Service}
		htmlCells := []templ.Component{templ.Raw(`<a href="` + html.EscapeString(projectHref(r.Service)) + `" class="inline-flex min-h-11 items-center rounded-sm font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(r.Service) + `</a>`)}
		for _, c := range r.Cells {
			cells = append(cells, "")
			htmlCells = append(htmlCells, templ.Raw(coverageCellHTML(c, r.Service)))
		}
		rows = append(rows, table.Row{ID: "coverage-" + r.Service, Cells: cells, HTMLCells: htmlCells})
	}
	return renderKitTable(table.TableParams{TableID: "coverage", Caption: "Current claims per project and layer", Columns: cols, Rows: rows})
}

// coverageCellHTML renders one cell in words: the count, a measured zero,
// or the absence named — a count linked to the claims it counts (a zero to
// every claim the compile left, superseded or refuted).
func coverageCellHTML(c console.CoverageCell, service string) string {
	href := claimsHref(c.Layer, service, "")
	switch c.Claims.State {
	case console.StateAbsent:
		word := "not compiled"
		if strings.HasPrefix(c.Claims.Reason, "not installed") {
			word = "not installed"
		}
		return `<span class="italic text-ink-muted" data-truth-state="absent" title="` + html.EscapeString(c.Claims.Reason) + `">` + word + `</span>`
	case console.StateError:
		return `<span class="text-danger" data-truth-state="error">unreadable — ` + html.EscapeString(c.Claims.Reason) + `</span>`
	case console.StateZero:
		return knowledgeLink(claimsHref(c.Layer, service, console.ClaimStatusAll), "0") + ` <span class="text-xs text-ink-muted">compiled, none current</span>`
	}
	return knowledgeLink(href, fmtThousands(c.Claims.V))
}

// renderLifecycle states the lifecycle counts in words.
func renderLifecycle(s console.StrataView) string {
	t := s.Totals
	items := []chrome.DescItem{
		{Term: "emitted by real compiles", DetailHTML: template.HTML(valueText(t.Emitted))},
		{Term: "rejected at compile (verification)", DetailHTML: template.HTML(valueText(t.Rejected))},
		{Term: "refused before starting (estimate over budget)", DetailHTML: template.HTML(valueText(t.Refused))},
	}
	for _, v := range []struct{ key, term string }{
		{"re-anchored", "re-anchored (followed their text when lines moved)"},
		{"contradicted", "contradicted (citation no longer resolves; served flagged)"},
		{"corroborated", "corroborated by a person"},
		{"refuted", "refuted by a person (never served again)"},
		{"superseded", "superseded by a review"},
	} {
		items = append(items, chrome.DescItem{Term: v.term, DetailHTML: template.HTML(valueText(s.Lifecycle.Events[v.key]))})
	}
	st := s.Lifecycle.Statuses
	items = append(items, chrome.DescItem{Term: "status now", Detail: fmt.Sprintf("%s fresh · %s stale · %s contradicted · %s superseded · %s refuted",
		fmtThousands(st.Fresh), fmtThousands(st.Stale), fmtThousands(st.Contradicted), fmtThousands(st.Superseded), fmtThousands(st.Refuted))})
	return chrome.RenderDescriptionList(items) +
		`<p class="mt-2 text-xs text-ink-muted">A re-anchored claim followed its cited text when lines moved and kept serving. A contradicted claim stays served with a flag until a recompile heals or confirms it; a refuted one is never served again.</p>`
}

// ─── /layers/{layer} ──────────────────────────────────────────────────────

func buildLayerPage(cr *consoleRequest) (consoleOut, error) {
	name := cr.r.PathValue("layer")
	src, err := knowledgeSourceFor(cr)
	if err != nil {
		return consoleOut{}, err
	}
	d, err := console.LayerDetail(cr.r.Context(), src, name)
	if err != nil {
		return consoleOut{}, err
	}
	if !d.Known {
		return consoleOut{}, &consoleError{Status: http.StatusNotFound, Title: "Unknown layer",
			Reason:     "No pack defines a layer named " + name + ", and the store holds no claim or compile run for it.",
			Remedy:     "fathom layers strata",
			RemedyHref: "/layers", RemedyLabel: "Open the semantic layers"}
	}
	verb := strings.ReplaceAll(verbLayerShow, "{layer}", name)
	page := 1
	if p, perr := strconv.Atoi(cr.r.URL.Query().Get("page")); perr == nil && p > 1 {
		page = p
	}
	role := d.Block.Role
	if role == "" {
		role = "unknown"
	}
	meta := []string{d.Block.Source + " pack", "role " + role}
	if d.Definition != nil {
		stratum := d.Definition.Stratum
		if stratum == "" {
			stratum = "unknown"
		}
		meta = append(meta, "stratum "+stratum)
	}
	var actions string
	if d.Block.Claims.Known() {
		actions = knowledgeActions([][2]string{{"Browse its claims", claimsHref(name, "", "")}})
	}
	return consoleOut{
		Title:    name,
		Meta:     meta,
		Actions:  actions,
		Verbs:    []string{verb},
		Sections: map[string]any{verb: d},
		Body:     func() string { return renderLayerPage(d, verb, page) },
	}, nil
}

// layerRunsPerPage pages the compile-run table (design §2: over 50 rows).
const layerRunsPerPage = 50

func renderLayerPage(d console.LayerPage, verb string, page int) string {
	var b strings.Builder
	b.WriteString(stat.RenderBand([]stat.Stat{
		valueStat("current claims", d.Block.Claims),
		valueStat("real compile runs", d.Block.Runs),
		dollarStat("spent", d.Block.CostUSD),
		valueStat("claims accepted", d.Accepted),
		valueStat("candidates rejected", d.Rejected),
	}, stat.BandOptions{AriaLabel: d.Layer + " totals"}))
	b.WriteString(`<div class="mt-4 grid gap-4 lg:grid-cols-2">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "definition", Title: "Definition", SourceKey: "layers",
		Question:  "What does this pack compile, from which documents, within which budget?",
		Verb:      verb,
		Source:    "the pack catalog each project root resolves (embedded ∪ .fathom/layers)",
		Freshness: knowledgeFreshness,
		Body:      renderLayerDefinition(d),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "rejections", Title: "Rejection reasons", SourceKey: "layers",
		Question:    "Why did compile-time verification refuse candidate claims?",
		Verb:        verb,
		Source:      "compile_runs.rejection_reasons_json",
		Denominator: valuePlain(d.Rejected) + " candidates rejected",
		Freshness:   knowledgeFreshness,
		State:       d.Rejected.State, Reason: d.Rejected.Reason,
		Body: renderRejections(d),
	}))
	b.WriteString(`</div><div class="mt-4 flex flex-col gap-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "runs", Title: "Compile runs", SourceKey: "layers",
		Question:    "When did this layer compile, for which project, at what cost, keeping and rejecting how many claims?",
		Verb:        verb,
		Source:      "compile_runs",
		Denominator: plural(len(d.Runs), "run") + " (compiles, dry runs and refused estimates)",
		Freshness:   knowledgeFreshness,
		Body:        renderLayerRuns(d, page),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "projects", Title: "Projects", SourceKey: "layers",
		Question:    "Which projects hold this layer's claims, and how fresh are they?",
		Verb:        verb,
		Source:      storeFooter,
		Denominator: plural(len(d.Projects), "project") + " with a compile, an estimate or a claim",
		Freshness:   knowledgeFreshness,
		Body:        renderLayerProjects(d),
	}))
	unc := d.UncompiledCount
	uncDenom := valuePlain(unc) + " " + unc.Denominator
	if !unc.Known() {
		uncDenom = ""
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "uncompiled", Title: "Uncompiled scopes", SourceKey: "layers",
		Question:    "What did each project's latest compile leave uncompiled, and how is it resumed?",
		Verb:        verb,
		Source:      "the latest real compile_runs row per project (uncompiled_scopes, sections)",
		Denominator: uncDenom,
		Freshness:   knowledgeFreshness,
		State:       unc.State, Reason: unc.Reason,
		Body: renderUncompiled(d),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

func renderLayerDefinition(d console.LayerPage) string {
	items := []chrome.DescItem{{Term: "source", Detail: d.Block.Origin}}
	if def := d.Definition; def != nil {
		stratum, kindRole := def.Stratum, d.Block.Kind+" · "+d.Block.Role
		if def.BrokenPackNote != "" && stratum == "" {
			stratum, kindRole = "unknown — the pack failed to load", "unknown — the pack failed to load"
		}
		items = append(items,
			chrome.DescItem{Term: "version", Detail: def.Version},
			chrome.DescItem{Term: "stratum", Detail: stratum},
			chrome.DescItem{Term: "kind · role", Detail: kindRole},
			chrome.DescItem{Term: "evidence tier", Detail: def.Tier},
			chrome.DescItem{Term: "eval gate", Detail: map[bool]string{true: "passed (eval/RESULTS.md on record)", false: "not gated — compile refuses without --unsafe-skip-eval-gate"}[d.Block.Evaled]},
			chrome.DescItem{Term: "corpus", DetailHTML: template.HTML(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(def.CorpusInclude, ", ")) + `</span>`)},
		)
		if len(def.CorpusExclude) > 0 {
			items = append(items, chrome.DescItem{Term: "excluded", DetailHTML: template.HTML(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(def.CorpusExclude, ", ")) + `</span>`)})
		}
		budget := "unbounded"
		if def.MaxTokensRun > 0 || def.MaxUSDRun > 0 {
			budget = fmt.Sprintf("%s tokens · $%.2f per run", fmtThousands(def.MaxTokensRun), def.MaxUSDRun)
		}
		items = append(items, chrome.DescItem{Term: "budget", Detail: budget})
		if len(def.ProjectPackIn) > 0 {
			items = append(items, chrome.DescItem{Term: "installed in", Detail: strings.Join(def.ProjectPackIn, ", ")})
		}
		if len(def.OverriddenIn) > 0 {
			items = append(items, chrome.DescItem{Term: "overridden in", Detail: strings.Join(def.OverriddenIn, ", ")})
		}
		if def.BrokenPackNote != "" {
			items = append(items, chrome.DescItem{Term: "broken pack", Detail: def.BrokenPackNote})
		}
	} else {
		items = append(items, chrome.DescItem{Term: "definition", Detail: "no pack defines this layer; its claims keep serving from the store until refuted or recompiled"})
	}
	items = append(items, chrome.DescItem{Term: "dry run", DetailHTML: template.HTML(renderVerbChip(d.DryRun))})
	return chrome.RenderDescriptionList(items)
}

func renderRejections(d console.LayerPage) string {
	if len(d.Rejections) == 0 {
		msg := "No rejection reasons recorded."
		if d.RejectionsNote != "" {
			msg = d.RejectionsNote + "."
		}
		return `<p class="text-sm text-ink-muted">` + html.EscapeString(msg) + `</p>`
	}
	var b strings.Builder
	b.WriteString(`<ul class="flex flex-col gap-3" data-rejections>`)
	for _, r := range d.Rejections {
		b.WriteString(`<li class="min-w-0"><p class="text-sm"><span class="font-semibold text-ink">` + html.EscapeString(r.Cause) + `</span> <span class="text-ink-muted">· ` + fmtThousands(r.Count) + `</span></p>`)
		for _, s := range r.Samples {
			b.WriteString(`<p class="mt-1 font-mono text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(s) + `</p>`)
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul>`)
	if d.RejectionsNote != "" {
		b.WriteString(`<p class="mt-2 text-xs text-ink-muted">` + html.EscapeString(d.RejectionsNote) + `</p>`)
	}
	return b.String()
}

func renderLayerRuns(d console.LayerPage, page int) string {
	if len(d.Runs) == 0 {
		return chrome.RenderEmptyState(chrome.EmptyState{Heading: "No compile run recorded", Body: "Nothing has compiled this layer in any project. " + d.DryRun + " estimates the cost without spending."})
	}
	pages := (len(d.Runs) + layerRunsPerPage - 1) / layerRunsPerPage
	if page > pages {
		page = pages
	}
	lo := (page - 1) * layerRunsPerPage
	hi := min(lo+layerRunsPerPage, len(d.Runs))
	var rows []table.Row
	for _, r := range d.Runs[lo:hi] {
		outcome := r.Kind
		switch {
		case r.Kind == "refused":
			outcome = "refused — " + r.Reason
		case r.Stopped:
			outcome = "budget stopped"
		case r.ResumedAt != "":
			outcome = "resumed at " + r.ResumedAt
		}
		sections := fmt.Sprintf("%s / %s", fmtThousands(r.Compiled), fmtThousands(r.Sections))
		cost := fmt.Sprintf("$%.2f", r.CostUSD)
		if r.Kind != "compile" {
			cost = "estimate only"
		}
		rows = append(rows, table.Row{
			ID:        "run-" + r.RunID,
			Cells:     []string{r.RanAt, "", outcome, sections, fmtThousands(r.Accepted), fmtThousands(r.Rejected), cost},
			HTMLCells: []templ.Component{nil, templ.Raw(knowledgeLink(projectHref(r.Service), r.Service))},
		})
	}
	tbl := renderKitTable(table.TableParams{
		TableID: "layer-runs",
		Caption: fmt.Sprintf("Compile runs %d–%d of %d, newest first", lo+1, hi, len(d.Runs)),
		Columns: []table.Column{
			{Key: "ran", Label: "Ran at"}, {Key: "project", Label: "Project"}, {Key: "outcome", Label: "Outcome"},
			{Key: "sections", Label: "Sections compiled", Align: "right"}, {Key: "accepted", Label: "Accepted", Align: "right"},
			{Key: "rejected", Label: "Rejected", Align: "right"}, {Key: "cost", Label: "Cost", Align: "right"},
		},
		Rows: rows,
	})
	if pages > 1 {
		// The kit's pager links are 44 px targets themselves
		// (FATHOM-F0-KIT-05); the div only spaces it.
		tbl += `<div class="mt-3">` + chrome.RenderPagination(chrome.Pagination{
			Page: page, TotalPages: pages, AriaLabel: "compile-run pages",
			HrefFor: func(p int) string { return layerHref(d.Layer) + "?page=" + strconv.Itoa(p) + "#runs" },
		}) + `</div>`
	}
	return tbl
}

func renderLayerProjects(d console.LayerPage) string {
	if len(d.Projects) == 0 {
		return `<p class="text-sm italic text-ink-muted">No project has compiled, estimated or imported this layer.</p>`
	}
	var b strings.Builder
	b.WriteString(`<ul class="flex flex-col divide-y divide-line" data-layer-projects>`)
	for _, p := range d.Projects {
		b.WriteString(`<li id="project-` + html.EscapeString(p.Service) + `" class="scroll-mt-4 py-2">`)
		b.WriteString(`<div class="flex flex-wrap items-baseline justify-between gap-2">` + knowledgeLink(projectHref(p.Service), p.Service) +
			`<span class="text-sm">` + claimsCount(p.Claims, claimsHref(d.Layer, p.Service, "")) + ` <span class="text-ink-muted">current</span></span></div>`)
		last := "never compiled"
		if p.LastRun != "" {
			last = "last compiled " + p.LastRun
		}
		b.WriteString(`<p class="flex flex-wrap items-center gap-x-1 text-xs text-ink-muted">` + html.EscapeString(plural(p.Runs, "real run")+" · "+last+" · ") +
			statusCount(p.Statuses.Fresh, d.Layer, p.Service, "fresh") + ` fresh, ` + statusCount(p.Statuses.Stale, d.Layer, p.Service, "stale") + ` stale, ` +
			statusCount(p.Statuses.Superseded, d.Layer, p.Service, "superseded") + ` superseded</p>`)
		b.WriteString(remedy(p.DryRun))
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ul>`)
	return b.String()
}

func renderUncompiled(d console.LayerPage) string {
	if len(d.Uncompiled) == 0 {
		// Nothing compiled measures nothing: the panel's truth line says
		// so, and no "all covered" sentence is read off an empty list.
		if !d.UncompiledCount.Known() {
			return ""
		}
		msg := fmt.Sprintf("The latest real compile in each of the %d projects that compiled this layer covered its whole corpus.", d.CompiledProjects)
		if d.CompiledProjects == 1 {
			msg = "The latest real compile in the one project that compiled this layer covered its whole corpus."
		}
		return `<p class="text-sm text-ink-muted">` + html.EscapeString(msg) + `</p>`
	}
	var b strings.Builder
	b.WriteString(`<ul class="flex flex-col gap-3" data-uncompiled>`)
	for _, u := range d.Uncompiled {
		state := fmt.Sprintf("%s of %s sections compiled", fmtThousands(u.Compiled), fmtThousands(u.Sections))
		if u.Stopped {
			state = "budget stopped at " + state
		}
		b.WriteString(`<li class="min-w-0">` + knowledgeLink(projectHref(u.Service), u.Service) + ` <span class="text-sm text-ink-muted">` + html.EscapeString(state+" · run "+shortCommit(u.RunID)+" · "+u.RanAt) + `</span>`)
		if len(u.Scopes) > 0 {
			b.WriteString(`<p class="mt-1 font-mono text-xs text-ink [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(u.Scopes, ", ")) + `</p>`)
		}
		b.WriteString(remedy(u.Resume) + `</li>`)
	}
	b.WriteString(`</ul>`)
	return b.String()
}
