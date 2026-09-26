package main

// console_project_index.go — the workspace's Index tab at
// /projects/{svc}/index (vision 35 §3 "Index: the dialect packs at work",
// §4.6 "the code neighborhood"; OC-36, OC-37). It renders:
//
//   - the languages the store's symbols are written in, with the engine
//     that extracted each and the tier it reached, the typed share of the
//     call edges, the edge kinds and the inventory keys — the facts
//     `fathom report evidence --service S --json` reports, computed from
//     the same primitives (console.IndexProfile);
//   - a symbol's code neighborhood: callers and callees to depth 2 through
//     the kit's layered graph (viz.LayoutEgo, FATHOM-KIT-02), resolved
//     edges solid and heuristic ones dashed, every column bounded with the
//     rest aggregated (console.Neighborhood), and its table alternative;
//   - the Scope panel (read-only): active suppressions, a preview of what a
//     glob would hide, and the purge plan a glob resolves to — with the
//     exact commands, never a button that deletes (SP-10 stays open; see
//     projectScopePanel).
//
// The index facts and the call graph are memoized per store watermark
// (invariant I21): an unchanged index never recomputes them.

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/analysis/inventory"
	"github.com/geoffmilleraz/fathom/internal/analysis/symbolnav"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

const (
	verbReportEvidence = "fathom report evidence --service {svc} --store {store} --json"
	verbScopeList      = "fathom scope list --service {svc} --json"
)

func init() {
	declarePage(consolePage{
		ID: "project-index", Route: "/projects/{svc}/index", Group: "Fleet", Order: 11, NavParent: "fleet",
		Title:    "Project index",
		Question: "What does the index hold for this repository — languages, engines and tiers, typed edges, inventory — and who calls what?",
		Verbs:    []string{verbReportEvidence, verbScopeList},
		Shows:    []console.MetricID{console.MetricProjectSymbols},
		Build:    buildProjectIndexPage,
	})
}

// projectIndexModel is the index tab's JSON projection.
type projectIndexModel struct {
	Profile      console.IndexProfileModel `json:"profile"`
	Neighborhood console.NeighborhoodModel `json:"neighborhood"`
	Suggestions  []string                  `json:"suggestions"`
	Scope        projectScopeModel         `json:"scope"`
}

func buildProjectIndexPage(cr *consoleRequest) (consoleOut, error) {
	pc, err := loadProjectContext(cr)
	if err != nil {
		return consoleOut{}, err
	}
	out := pc.frame("Index")
	out.Metrics = pc.metrics(cr.page)
	c := cr.deps.c
	if c == nil {
		return consoleOut{}, &consoleError{Status: 503, Title: "Store unreadable", Reason: "this daemon has no store open, so " + pc.svc + "'s index cannot be read", Remedy: "fathom serve-http --store <path>"}
	}
	ctx := cr.r.Context()
	idx := projectIndexFor(ctx, c, pc)
	profile := console.IndexProfile(idx.facts)

	q := strings.TrimSpace(cr.r.URL.Query().Get("symbol"))
	nb := projectNeighborhood(pc.svc, idx.graph, q)

	storePath, storeKnown := consoleStorePath(cr.deps.selfURL)
	sc := projectScope(cr, c, pc, storePath)

	vals := map[string]string{"svc": pc.svc, "store": storePath}
	out.Verbs = []string{fillVerb(verbScopeList, vals)}
	// The languages panel's chip is the evidence verb, whose figures it
	// shows — or no chip at all when that verb is not offered: the scope
	// list prints none of the panel's languages, engines or tiers.
	languagesVerb := ""
	if storeKnown {
		languagesVerb = fillVerb(verbReportEvidence, vals)
		out.Verbs = []string{languagesVerb, fillVerb(verbScopeList, vals)}
	}
	out.Model = projectIndexModel{Profile: profile, Neighborhood: nb, Suggestions: idx.graph.suggestions, Scope: sc}
	if wantsJSON(cr.r) {
		out.Sections = map[string]any{fillVerb(verbScopeList, vals): sc.Entries}
		// The evidence row re-derives a receipt from a bare checkout: it is
		// computed for JSON only, never on the HTML render path.
		if storeKnown {
			out.Sections[fillVerb(verbReportEvidence, vals)] = map[string]any{
				"schema": reportEvidenceSchema, "store": storePath,
				"services": []evidenceRow{computeEvidenceRow(ctx, c.Store(), c, pc.svc)},
			}
		}
	}
	out.Body = func() string {
		return renderProjectTabs(pc.svc, "index") + projectBody(
			renderIndexLanguagesPanel(pc, profile, languagesVerb, storeKnown)+
				`<div class="mt-4 grid gap-4 lg:grid-cols-2">`+renderIndexEdgesPanel(pc, profile)+renderIndexInventoryPanel(pc, profile)+`</div>`+
				`<div class="mt-4 grid gap-4">`+renderNeighborhoodPanel(pc, nb, idx.graph.suggestions, q, cr.deps.selfURL)+projectScopePanel(pc, sc)+`</div>`)
	}
	return out, nil
}

// consoleStorePath is the store this daemon serves, from its own record
// (serve-http writes it once its listener is bound). ok is false when no
// record names one: the evidence verb is then not offered, rather than a
// command that would read some other store.
func consoleStorePath(selfURL string) (string, bool) {
	rec, ok := readDaemonRecordFor(selfURL)
	if !ok || rec.Store == "" {
		return "", false
	}
	return rec.Store, true
}

// ─── the index facts and the call graph, memoized per store watermark ───

type projectGraph struct {
	nodes       []core.SymbolNode
	g           *callgraph.Graph
	isTest      map[string]bool
	suggestions []string
	err         string
}

type projectIndexData struct {
	facts console.IndexFacts
	graph projectGraph
}

// projectIndexMemo keeps the six most recent projects' index facts and
// graphs (console_memo.go: a project's cold read never holds up another's
// kept one).
var projectIndexMemo = consoleMemo[projectIndexData]{max: 6}

// projectIndexFor reads (or recalls) one project's index facts and graph.
// The memo key is the store's watermark (StoreWatermark: the ingest's
// commit and time), so only a re-ingest invalidates it; "" never memoizes.
// The read runs detached from the request that started it: the requests
// waiting on the same read must not see it cancelled by that one's client
// going away.
func projectIndexFor(ctx context.Context, c *client.Client, pc projectContext) projectIndexData {
	wm, _ := c.StoreWatermark(ctx, pc.svc)
	key := pc.svc + "@" + wm + "@" + pc.root + "@" + pc.rootNote
	d, _ := projectIndexMemo.do(key, time.Now(), func() (projectIndexData, time.Duration) {
		d := computeProjectIndex(context.WithoutCancel(ctx), c, pc)
		// A read that failed is never memoized: the next view retries it (an
		// EPERM wedge on the volume clears; a memoized error would not).
		failed := d.facts.LanguagesErr != "" || d.facts.EdgesErr != "" || d.facts.InventoryErr != ""
		if wm == "" || failed {
			return d, 0
		}
		return d, -1
	})
	return d
}

func computeProjectIndex(ctx context.Context, c *client.Client, pc projectContext) projectIndexData {
	d := projectIndexData{facts: console.IndexFacts{Service: pc.svc, Source: "the daemon's store (" + pc.svc + ")"}}
	nodes, err := c.ListAllSymbols(ctx, pc.svc)
	if err != nil {
		d.facts.LanguagesErr = err.Error()
		d.graph.err = err.Error()
	}
	edges, eerr := c.TypedEdges(ctx, pc.svc)
	if eerr != nil {
		d.facts.EdgesErr = eerr.Error()
	}

	// Languages: files and symbols per language, by conserve's own naming
	// rule, joined to the tier the tier breakdown assigns and the engine
	// the capabilities tool names for the language's extensions.
	type langAgg struct {
		files map[string]bool
		syms  int
		exts  map[string]bool
	}
	langs := map[string]*langAgg{}
	var paths []string
	seenPath := map[string]bool{}
	for _, n := range nodes {
		l := conserve.LanguageOf(n.FilePath)
		if l == "" {
			continue
		}
		a := langs[l]
		if a == nil {
			a = &langAgg{files: map[string]bool{}, exts: map[string]bool{}}
			langs[l] = a
		}
		a.syms++
		a.files[n.FilePath] = true
		a.exts[strings.ToLower(filepath.Ext(n.FilePath))] = true
		if !seenPath[n.FilePath] {
			seenPath[n.FilePath] = true
			paths = append(paths, n.FilePath)
		}
	}
	tiers := map[string]string{}
	for _, t := range conserve.TiersByLanguage(ctx, conserve.Input{Service: pc.svc, Store: c.Store(), ChangedFiles: paths}) {
		tiers[t.Language] = t.Tier
	}
	engines := extractor.EngineProvenance()
	for l, a := range langs {
		eng := map[string]bool{}
		for ext := range a.exts {
			if e, ok := engines[ext]; ok {
				eng[e] = true
			} else {
				eng["built-in"] = true
			}
		}
		names := make([]string, 0, len(eng))
		for e := range eng {
			names = append(names, e)
		}
		sort.Strings(names)
		d.facts.Languages = append(d.facts.Languages, console.IndexLanguage{Language: l, Engine: strings.Join(names, ", "), Tier: tiers[l], Files: len(a.files), Symbols: a.syms})
	}

	// Edge kinds: the typed pass's rows, resolved by report evidence's rule.
	kinds := map[string]*console.IndexEdgeKind{}
	for _, e := range edges {
		k := string(e.Kind)
		if k == "" {
			k = "unlabelled"
		}
		ek := kinds[k]
		if ek == nil {
			ek = &console.IndexEdgeKind{Kind: k, Resolved: core.EdgeConfidenceOf(e) == core.EdgeConfidenceResolved}
			kinds[k] = ek
		}
		ek.Count++
	}
	for _, ek := range kinds {
		d.facts.EdgeKinds = append(d.facts.EdgeKinds, *ek)
	}

	// Inventory needs the corpus's content, readable only from a local root.
	if pc.rootNote != "" {
		d.facts.InventoryNote = "the inventory reads the corpus's files: " + pc.rootNote
	} else if sources, serr := c.Sources(ctx, pc.svc); serr != nil {
		d.facts.InventoryErr = serr.Error()
	} else {
		files := make([]inventory.File, 0, len(sources))
		for _, f := range sources {
			files = append(files, inventory.File{Path: f.Path, Content: f.Content})
		}
		d.facts.Inventory = map[string]int{}
		for _, it := range inventory.Extract(files, evidenceSurfacePatterns(pc.root)) {
			d.facts.Inventory[it.Kind]++
		}
	}

	// The call graph, typed edges included (the graph find_callers walks).
	if d.graph.err == "" {
		d.graph.nodes = nodes
		d.graph.g = callgraph.BuildBoth(nodes, edges)
		d.graph.isTest = map[string]bool{}
		type fan struct {
			key string
			n   int
		}
		var fans []fan
		for _, n := range nodes {
			key := n.Key()
			if strings.HasPrefix(n.Name, "Test") && strings.HasSuffix(n.FilePath, "_test.go") {
				d.graph.isTest[key] = true
				continue
			}
			if c := len(callgraph.CallersDetail(d.graph.g, key)); c > 0 {
				fans = append(fans, fan{key, c})
			}
		}
		sort.Slice(fans, func(i, j int) bool {
			if fans[i].n != fans[j].n {
				return fans[i].n > fans[j].n
			}
			return fans[i].key < fans[j].key
		})
		seen := map[string]bool{}
		for i := 0; i < len(fans) && len(d.graph.suggestions) < 6; i++ {
			if !seen[fans[i].key] {
				seen[fans[i].key] = true
				d.graph.suggestions = append(d.graph.suggestions, fans[i].key)
			}
		}
	}
	return d
}

// projectNeighborhood resolves q against the project's symbols and walks
// its neighbourhood. An ambiguous bare name lists its candidates instead of
// guessing one (symbol_context's J3 rule).
func projectNeighborhood(svc string, pg projectGraph, q string) console.NeighborhoodModel {
	in := console.NeighborhoodInput{Service: svc}
	switch {
	case pg.err != "":
		in.Unresolved = "the project's symbols could not be read: " + pg.err
	case q == "":
		in.Unresolved = "no symbol chosen — pick one below, or type a name"
	default:
		key, cands, ok := symbolnav.ResolveTiered(pg.nodes, q)
		switch {
		case len(cands) > 1:
			seen := map[string]bool{}
			for _, n := range cands {
				if k := n.Key(); !seen[k] && len(in.Candidates) < 12 {
					seen[k] = true
					in.Candidates = append(in.Candidates, k)
				}
			}
			in.Unresolved = fmt.Sprintf("%q names %d symbols — choose one:", q, len(cands))
		case !ok || key == "":
			if k := callgraph.Canonical(pg.g, q); k != "" {
				key = k
			} else {
				in.Unresolved = fmt.Sprintf("no symbol named %q in %s", q, svc)
			}
		}
		if in.Unresolved == "" {
			in.Focus = key
		}
	}
	if in.Focus != "" {
		// An edge is resolved only by the one confidence rule
		// (core.EdgeConfidenceOf): a file-resolved edge carries a kind and is
		// still heuristic — per-file import matching, no type checker — so it
		// is drawn dashed and counted heuristic, as the edge-kinds panel
		// beside it says ("never used for proof").
		edgesOf := func(infos []callgraph.EdgeInfo) []console.NeighborEdge {
			out := make([]console.NeighborEdge, 0, len(infos))
			for _, e := range infos {
				resolved := core.EdgeConfidenceOf(core.Edge{Kind: e.Kind}) == core.EdgeConfidenceResolved
				out = append(out, console.NeighborEdge{Key: e.Key, Resolved: resolved, Kind: string(e.Kind)})
			}
			return out
		}
		in.Callers = func(k string) []console.NeighborEdge { return edgesOf(callgraph.CallersDetail(pg.g, k)) }
		in.Callees = func(k string) []console.NeighborEdge { return edgesOf(callgraph.CalleesDetail(pg.g, k)) }
		in.IsTest = func(k string) bool { return pg.isTest[k] }
	}
	return console.Neighborhood(in)
}

// ─── rendering ───────────────────────────────────────────────────────────

func indexPageHref(svc, symbol, anchor string) string {
	h := projectHref(svc) + "/index"
	if symbol != "" {
		h += "?symbol=" + url.QueryEscape(symbol)
	}
	if anchor != "" {
		h += "#" + anchor
	}
	return h
}

func renderIndexLanguagesPanel(pc projectContext, p console.IndexProfileModel, verb string, storeKnown bool) string {
	var body strings.Builder
	if len(p.Languages) == 0 {
		body.WriteString(`<p class="italic text-ink-muted">` + html.EscapeString(p.LanguageSet.Reason) + `</p>`)
	} else {
		rows := make([]table.Row, 0, len(p.Languages))
		for _, l := range p.Languages {
			tier := l.Tier
			if tier == "" {
				tier = "not reported"
			}
			rows = append(rows, table.Row{ID: "lang-" + l.Language, Cells: []string{l.Language, l.Engine, tier, fmtThousands(l.Files), fmtThousands(l.Symbols)}})
		}
		body.WriteString(renderKitTable(table.TableParams{
			TableID: "project-languages",
			Caption: fmt.Sprintf("%d languages in %s's index, in name order (never ranked)", len(p.Languages), pc.svc),
			Columns: []table.Column{{Key: "language", Label: "Language"}, {Key: "engine", Label: "Engine"}, {Key: "tier", Label: "Tier"}, {Key: "files", Label: "Files", Align: "right"}, {Key: "symbols", Label: "Symbols", Align: "right"}},
			Rows:    rows,
		}))
	}
	source := "the store's symbols (their files), conserve's tier breakdown, and the capabilities tool's extraction engines (an extension it does not list is read by a built-in extractor)"
	if !storeKnown {
		source += "; the evidence verb is not offered — this daemon recorded no store path"
	}
	return renderPanel(consolePanel{
		ID: "index-languages", Title: "Dialect packs at work",
		Question: "What does each language's extractor deliver here — which engine, and which tier?",
		Verb:     verb, Source: source, Denominator: p.LanguageSet.Denominator,
		State: p.LanguageSet.State, Reason: p.LanguageSet.Reason,
		SourceKey: "index:" + pc.svc, Body: body.String(),
	})
}

func renderIndexEdgesPanel(pc projectContext, p console.IndexProfileModel) string {
	share := stat.Absent("typed share", p.TypedShare.Reason)
	switch {
	case p.TypedShare.State == console.StateError:
		share = stat.Stat{Label: "typed share", Value: "unreadable", ValueRole: "danger", Sub: p.TypedShare.Reason}
	case p.TypedShare.Known():
		share = stat.Stat{Label: "typed share", Value: fmt.Sprintf("%.1f%%", p.TypedShare.V*100), Sub: p.TypedShare.Denominator}
	}
	var body strings.Builder
	body.WriteString(stat.RenderBand([]stat.Stat{valueStat("typed call edges", p.Edges), valueStat("resolved", p.TypedEdges), share}, stat.BandOptions{AriaLabel: "typed edges"}))
	if len(p.EdgeKinds) > 0 {
		roles := map[string]string{"direct": "accent", "iface-dispatch": "info", "func-value": "positive", "file-resolved": "caution", "unresolved": "ink-faint"}
		var segs []viz.Segment
		rows := make([]table.Row, 0, len(p.EdgeKinds))
		total := 0
		for _, k := range p.EdgeKinds {
			total += k.Count
		}
		for _, k := range p.EdgeKinds {
			role := roles[k.Kind]
			if role == "" {
				role = "ink-muted"
			}
			segs = append(segs, viz.Segment{Value: float64(k.Count), Role: role, Label: k.Kind})
			resolved := "never used for proof"
			if k.Resolved {
				resolved = "names its callee"
			}
			rows = append(rows, table.Row{ID: "kind-" + k.Kind, Cells: []string{k.Kind, fmtThousands(k.Count), fmt.Sprintf("%.1f%%", float64(k.Count)*100/float64(total)), resolved}})
		}
		body.WriteString(`<div class="mt-3" data-edge-kinds>` + viz.StackBar(segs, viz.StackBarOptions{W: 560, H: 16, Fluid: true, AriaLabel: "typed edges by kind"}) + `</div>`)
		body.WriteString(`<ul class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-ink-muted" aria-hidden="true">`)
		for _, k := range p.EdgeKinds {
			body.WriteString(`<li>` + html.EscapeString(fmtThousands(k.Count)+" "+k.Kind) + `</li>`)
		}
		body.WriteString(`</ul>`)
		tbl := renderKitTable(table.TableParams{
			TableID: "project-edge-kinds", Caption: "typed call edges by kind",
			Columns: []table.Column{{Key: "kind", Label: "Kind"}, {Key: "count", Label: "Edges", Align: "right"}, {Key: "share", Label: "Share", Align: "right"}, {Key: "proof", Label: "Proof"}},
			Rows:    rows,
		})
		body.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "Edge kinds as a table", Body: template.HTML(tbl)}) + `</div>`)
	}
	return renderPanel(consolePanel{
		ID: "index-edges", Title: "Typed share and edge kinds",
		Question: "How were this repository's call edges resolved? Unresolved edges are never used for proof.",
		Verb:     "", Source: "the typed pass's edges in the daemon's store", Denominator: p.Edges.Denominator,
		State: p.Edges.State, Reason: p.Edges.Reason, SourceKey: "index:" + pc.svc, Body: body.String(),
	})
}

func renderIndexInventoryPanel(pc projectContext, p console.IndexProfileModel) string {
	var body strings.Builder
	if p.Inventory.Known() {
		if len(p.Keys) == 0 {
			body.WriteString(`<p class="text-ink-muted">0 inventory items — no api, env, route or json surface was found in the corpus.</p>`)
		} else {
			rows := make([]table.Row, 0, len(p.Keys))
			for _, k := range p.Keys {
				rows = append(rows, table.Row{ID: "inv-" + k.Kind, Cells: []string{k.Kind, fmtThousands(k.Count)}})
			}
			body.WriteString(renderKitTable(table.TableParams{
				TableID: "project-inventory", Caption: fmt.Sprintf("%s inventory items by kind", fmtThousands(p.Inventory.V)),
				Columns: []table.Column{{Key: "kind", Label: "Inventory key"}, {Key: "count", Label: "Items", Align: "right"}},
				Rows:    rows,
			}))
		}
	}
	return renderPanel(consolePanel{
		ID: "index-inventory", Title: "Inventory keys",
		Question: "Which externally visible surfaces does the corpus declare — APIs, env vars, routes, JSON contracts?",
		Source:   p.Inventory.Source, Denominator: p.Inventory.Denominator,
		State: p.Inventory.State, Reason: p.Inventory.Reason, SourceKey: "index:" + pc.svc, Body: body.String(),
	})
}

// renderNeighborhoodPanel is the code neighborhood: the symbol form, the
// kit's layered graph in its own scroll region, and its table alternative.
func renderNeighborhoodPanel(pc projectContext, nb console.NeighborhoodModel, suggestions []string, q, selfURL string) string {
	var body strings.Builder
	body.WriteString(`<form method="get" action="` + html.EscapeString(projectHref(pc.svc)+"/index") + `#neighborhood" class="mb-3 flex flex-wrap items-end gap-2" role="search" aria-label="Choose a symbol">`)
	body.WriteString(`<div class="min-w-0 flex-1 [&_input]:min-h-11">` + form.RenderField(form.Field{Label: "Symbol", For: "neighborhood-symbol", Help: "a bare name, or a qualified key when the name is ambiguous",
		Control: template.HTML(form.RenderInput(form.Input{ID: "neighborhood-symbol", Name: "symbol", Value: q, Placeholder: "e.g. Search or internal/console/console.Search", Autocomplete: "off", HasHelp: true}))}) + `</div>`)
	body.WriteString(`<div class="[&>button]:min-h-11">` + form.RenderButton(form.Button{Label: "Show neighborhood", Variant: form.ButtonPrimary}) + `</div></form>`)

	symbolLinks := func(keys []string) string {
		var l strings.Builder
		l.WriteString(`<ul class="mt-1 flex flex-wrap gap-x-3">`)
		for _, s := range keys {
			label := s
			if i := strings.Index(label, "::"); i >= 0 {
				label = label[i+2:]
			}
			l.WriteString(`<li class="min-w-0"><a class="inline-flex min-h-11 items-center font-mono text-xs text-accent [overflow-wrap:anywhere] hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="` + html.EscapeString(indexPageHref(pc.svc, s, "neighborhood")) + `">` + html.EscapeString(label) + `</a></li>`)
		}
		l.WriteString(`</ul>`)
		return l.String()
	}
	if nb.Focus == "" {
		body.WriteString(`<p class="mb-2 text-ink-muted" data-neighborhood-unresolved>` + html.EscapeString(nb.Unresolved) + `</p>`)
		if len(nb.Candidates) > 0 {
			body.WriteString(symbolLinks(nb.Candidates))
		} else if len(suggestions) > 0 {
			body.WriteString(`<p class="text-xs text-ink-muted">The most-called symbols here:</p>` + symbolLinks(suggestions))
		}
	} else {
		svg, tbl, err := layoutNeighborhood(pc.svc, nb)
		if err != nil {
			body.WriteString(`<p class="text-danger">the neighbourhood could not be laid out: ` + html.EscapeString(err.Error()) + `</p>`)
		} else {
			body.WriteString(`<p class="mb-2 text-xs text-ink-muted" data-neighborhood-counts>` + html.EscapeString(fmt.Sprintf("%d callers (%d resolved, %d heuristic) · %d callees (%d resolved, %d heuristic) · depth %d, at most %d per column",
				nb.CallersResolved+nb.CallersHeuristic, nb.CallersResolved, nb.CallersHeuristic, nb.CalleesResolved+nb.CalleesHeuristic, nb.CalleesResolved, nb.CalleesHeuristic, nb.Depth, nb.Bound)) + `</p>`)
			body.WriteString(`<div class="overflow-x-auto rounded-md border border-line bg-surface p-2" role="region" tabindex="0" aria-label="call neighborhood graph — scrolls sideways" data-neighborhood-graph>` + svg + `</div>`)
			for _, n := range nb.Notes {
				body.WriteString(`<p class="mt-1 text-xs text-ink-muted">` + html.EscapeString(n) + `</p>`)
			}
			body.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "The neighborhood as a table", Body: template.HTML(`<div class="overflow-x-auto [&_table]:w-full [&_table]:text-sm [&_caption]:py-2 [&_caption]:text-left [&_caption]:text-ink-muted [&_th]:px-2 [&_th]:py-1 [&_th]:text-left [&_th]:font-medium [&_th]:text-ink-muted [&_td]:border-t [&_td]:border-line [&_td]:px-2 [&_td]:py-1 [&_a]:inline-flex [&_a]:min-h-11 [&_a]:items-center [&_a]:text-accent [&_a:focus-visible]:outline-2 [&_a:focus-visible]:outline-focus">` + tbl + `</div>`)}) + `</div>`)
		}
	}
	// The neighbourhood is served by no CLI verb; its instrument is this
	// page's own JSON (statusz's precedent), and the agent surface's
	// symbol_context / find_callers walk the same graph.
	verb := ""
	if nb.Focus != "" {
		verb = "curl -s " + shellWord(selfURL+indexPageHref(pc.svc, nb.Focus, "")+"&format=json")
	}
	return renderPanel(consolePanel{
		ID: "neighborhood", Title: "Code neighborhood", Verb: verb,
		Question: "Who calls this symbol, and what does it call? Solid edges are type-checked; dashed ones are syntax-only; a column past its bound is aggregated and says how many.",
		Source:   "the call graph find_callers and symbol_context walk (typed edges included), memoized per store watermark",
		Denominator: func() string {
			if nb.Focus == "" {
				return ""
			}
			return fmt.Sprintf("%d nodes drawn, %d aggregated", len(nb.Nodes), nb.AggregatedNodes)
		}(),
		SourceKey: "index:" + pc.svc, Body: body.String(),
	})
}

// layoutNeighborhood draws the model with the kit's ego layout.
func layoutNeighborhood(svc string, nb console.NeighborhoodModel) (string, string, error) {
	nodes := make([]viz.GraphNode, 0, len(nb.Nodes))
	for _, n := range nb.Nodes {
		gn := viz.GraphNode{ID: n.ID, Label: n.Label, Group: n.Group}
		switch {
		case n.Side == "focus":
			gn.Kind = viz.GraphNodeFocus
		case n.Aggregate:
			gn.Kind = viz.GraphNodeAggregate
		default:
			gn.Href = indexPageHref(svc, n.ID, "neighborhood")
		}
		nodes = append(nodes, gn)
	}
	edges := make([]viz.GraphEdge, 0, len(nb.Links))
	for _, l := range nb.Links {
		e := viz.GraphEdge{From: l.From, To: l.To, Emphasis: l.TestReach}
		if !l.Resolved {
			e.Style = viz.GraphEdgeDashed
		}
		edges = append(edges, e)
	}
	label := nb.Focus
	if i := strings.Index(label, "::"); i >= 0 {
		label = label[i+2:]
	}
	layout, err := viz.LayoutEgo(nb.Focus, nodes, edges, viz.EgoOptions{
		Depth: nb.Depth,
		GraphOptions: viz.GraphOptions{
			AriaLabel: "call neighborhood of " + label, MaxPerRank: max(nb.Bound, 2),
			SolidLabel: "Resolved (type-checked) call", DashedLabel: "Heuristic (syntax-only) call", EmphasisLabel: "Called from a test",
		},
	})
	if err != nil {
		return "", "", err
	}
	return layout.SVG(), layout.Table(), nil
}

// ─── the Scope panel (read-only; SP-10 stays open) ───────────────────────

// projectScopeModel is the Scope panel's projection: the suppressions in
// force for the project, and — when a glob was asked about — what it would
// hide and what a purge of it would delete, per location.
type projectScopeModel struct {
	SuppressionsPath string                `json:"suppressions_path"`
	Entries          []scopeListEntry      `json:"suppressions"`
	EntriesErr       string                `json:"suppressions_error,omitempty"`
	Glob             string                `json:"glob,omitempty"`
	Files            []string              `json:"files,omitempty"`
	FileCount        int                   `json:"file_count,omitempty"`
	Counts           []store.LocationCount `json:"counts,omitempty"`
	PreviewErr       string                `json:"preview_error,omitempty"`
	StorePath        string                `json:"store,omitempty"`
}

// projectScope reads the suppressions and, for ?scope=<glob>, resolves the
// glob against the daemon's store exactly as `fathom scope preview` and
// `fathom purge --dry-run` do (store.ResolvePurge: every location's own
// path column, past the read filter).
func projectScope(cr *consoleRequest, c *client.Client, pc projectContext, storePath string) projectScopeModel {
	m := projectScopeModel{SuppressionsPath: scope.ResolvePath(), Entries: []scopeListEntry{}, StorePath: storePath}
	if m.SuppressionsPath == "" {
		m.EntriesErr = "no suppressions file resolves (set FATHOM_SUPPRESSIONS or $HOME)"
	} else if sups, err := scope.LoadSuppressions(m.SuppressionsPath); err != nil {
		m.EntriesErr = err.Error()
	} else {
		m.Entries = scopeListEntries(sups, cr.now, pc.svc, false)
	}
	glob := strings.TrimSpace(cr.r.URL.Query().Get("scope"))
	if glob == "" {
		return m
	}
	m.Glob = glob
	if strings.HasPrefix(glob, "!") {
		m.PreviewErr = "a re-include pattern (!…) hides and deletes nothing; scope and purge take exclusions only"
		return m
	}
	sq, ok := c.Store().(*store.SQLiteStore)
	if !ok {
		m.PreviewErr = fmt.Sprintf("the preview needs the SQLite backend (this daemon serves %T)", c.Store())
		return m
	}
	plan, err := sq.ResolvePurge(cr.r.Context(), pc.svc, filter.New([]string{glob}))
	if err != nil {
		m.PreviewErr = err.Error()
		return m
	}
	m.FileCount = len(plan.Paths)
	m.Files = plan.Paths
	if len(m.Files) > 50 {
		m.Files = m.Files[:50]
	}
	m.Counts = plan.Counts
	return m
}

// projectScopePanel is the Scope panel (docs/design/scope-and-purge.md §4)
// in its read-only form: what is hidden now, what a glob would hide, and
// what a purge of it would delete, per location — then the exact commands.
// It carries no button that mutates: suppress, unsuppress and purge stay
// CLI verbs until the daemon has a purge path that also flushes its own
// live caches (SP-10 stays open; see docs/plans/ambient-fathom.md).
func projectScopePanel(pc projectContext, sc projectScopeModel) string {
	var b strings.Builder
	b.WriteString(`<h3 class="mb-1 text-sm font-semibold text-ink">Hidden from answers now</h3>`)
	switch {
	case sc.EntriesErr != "":
		b.WriteString(`<p class="font-medium text-danger">could not be read — ` + html.EscapeString(sc.EntriesErr) + `</p>`)
	case len(sc.Entries) == 0:
		b.WriteString(`<p class="text-ink-muted" data-scope-none>No active suppression covers ` + html.EscapeString(pc.svc) + ` — nothing is hidden from its answers on this machine.</p>`)
	default:
		rows := make([]table.Row, 0, len(sc.Entries))
		for _, e := range sc.Entries {
			svc := e.Service
			if svc == "" {
				svc = "fleet-wide"
			}
			until := "until removed"
			if e.ExpiresAt != "" {
				until = "until " + e.ExpiresAt
			}
			rows = append(rows, table.Row{ID: "sup-" + e.ID, Cells: []string{e.Pattern, svc, e.Reason, until, "fathom scope unsuppress " + e.ID}})
		}
		b.WriteString(renderKitTable(table.TableParams{
			TableID: "project-suppressions", Caption: fmt.Sprintf("%d active suppressions over %s", len(sc.Entries), pc.svc),
			Columns: []table.Column{{Key: "pattern", Label: "Pattern"}, {Key: "scope", Label: "Scope"}, {Key: "reason", Label: "Reason"}, {Key: "until", Label: "Expires"}, {Key: "undo", Label: "Reverse with"}},
			Rows:    rows,
		}))
	}

	b.WriteString(`<h3 class="mb-1 mt-4 text-sm font-semibold text-ink">Preview a glob</h3>`)
	b.WriteString(`<form method="get" action="` + html.EscapeString(projectHref(pc.svc)+"/index") + `#scope" class="mb-2 flex flex-wrap items-end gap-2" aria-label="Preview what a glob would hide">`)
	b.WriteString(`<div class="min-w-0 flex-1 [&_input]:min-h-11">` + form.RenderField(form.Field{Label: "Path glob", For: "scope-glob", Help: "gitignore syntax, matched against every location's own path column — nothing is hidden or deleted by previewing",
		Control: template.HTML(form.RenderInput(form.Input{ID: "scope-glob", Name: "scope", Value: sc.Glob, Placeholder: "vendor/**", Autocomplete: "off", HasHelp: true}))}) + `</div>`)
	b.WriteString(`<div class="[&>button]:min-h-11">` + form.RenderButton(form.Button{Label: "Preview", Variant: form.ButtonGhost}) + `</div></form>`)

	if sc.Glob != "" {
		switch {
		case sc.PreviewErr != "":
			b.WriteString(`<p class="font-medium text-danger" data-scope-preview-error>preview failed — ` + html.EscapeString(sc.PreviewErr) + `</p>`)
		default:
			symbols := 0
			for _, c := range sc.Counts {
				if c.Location == string(store.LocObservations) {
					symbols = c.Rows
				}
			}
			b.WriteString(`<p class="text-ink" data-scope-preview>` + html.EscapeString(fmt.Sprintf("%q matches %s and %s in %s.", sc.Glob, plural(sc.FileCount, "file"), plural(symbols, "symbol"), pc.svc)) + `</p>`)
			if sc.FileCount > 0 {
				var files strings.Builder
				files.WriteString(`<ul class="font-mono text-xs text-ink">`)
				for _, f := range sc.Files {
					files.WriteString(`<li class="[overflow-wrap:anywhere]">` + pathHTML(f) + `</li>`)
				}
				if sc.FileCount > len(sc.Files) {
					files.WriteString(`<li class="text-ink-muted">… and ` + fmtThousands(sc.FileCount-len(sc.Files)) + ` more</li>`)
				}
				files.WriteString(`</ul>`)
				b.WriteString(`<div class="mt-2">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: fmt.Sprintf("The %s it resolves to", plural(sc.FileCount, "file")), Body: template.HTML(files.String())}) + `</div>`)
				b.WriteString(`<p class="mt-2 text-ink-muted">Hide them from answers (reversible, instant):</p>`)
				b.WriteString(`<div class="mt-1 flex flex-wrap gap-2">` + renderVerbChip("fathom scope suppress "+shellWord(sc.Glob)+" --service "+shellWord(pc.svc)+" --reason '<why>' --until 7d") + `</div>`)
				b.WriteString(`<div class="mt-4">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "Danger zone — permanently remove indexed content", Body: template.HTML(renderPurgePlan(pc, sc))}) + `</div>`)
			}
		}
	}
	return renderPanel(consolePanel{
		ID: "scope", Title: "Scope",
		Question:  "What is hidden from this repository's answers, and what would a glob hide — or a purge delete?",
		Verb:      "fathom scope list --service " + shellWord(pc.svc) + " --json",
		Source:    "the suppressions file " + sc.SuppressionsPath + "; previews read the daemon's store (store.ResolvePurge)",
		SourceKey: "index:" + pc.svc, Body: b.String(),
	})
}

// renderPurgePlan is the purge's dry run for the previewed glob: the rows
// it would delete per location, the ingest-time exclusion it would write,
// and the exact command — shown only after the file list, as the design
// requires (§4), and never run from the page.
func renderPurgePlan(pc projectContext, sc projectScopeModel) string {
	var b strings.Builder
	b.WriteString(`<p class="text-ink">This deletes data. It cannot be undone; suppression above is the reversible option.</p>`)
	rows := make([]table.Row, 0, len(sc.Counts))
	for _, c := range sc.Counts {
		rows = append(rows, table.Row{ID: "purge-" + c.Location, Cells: []string{c.Location, fmtThousands(c.Rows)}})
	}
	b.WriteString(renderKitTable(table.TableParams{
		TableID: "project-purge-plan", Caption: "rows a purge of " + sc.Glob + " would delete, per location (its dry run)",
		Columns: []table.Column{{Key: "location", Label: "Location"}, {Key: "rows", Label: "Rows", Align: "right"}},
		Rows:    rows,
	}))
	b.WriteString(`<p class="mt-2 text-ink-muted">A purge also writes a service-scoped ingest exclusion (~/.fathom/ignore.d/` + html.EscapeString(pc.svc) + `) so a refresh cannot re-index what it deleted, then verifies every location and the raw store bytes, and fails loudly on a survivor.</p>`)
	storeArg := sc.StorePath
	if storeArg == "" {
		storeArg = "<store>"
	}
	b.WriteString(`<p class="mt-2 text-ink-muted">Run it from a terminal, with the reason and the service typed out:</p>`)
	b.WriteString(`<div class="mt-1 flex flex-wrap gap-2" data-purge-command>` + renderVerbChip("fathom purge "+shellWord(sc.Glob)+" --service "+shellWord(pc.svc)+" --store "+shellWord(storeArg)+" --reason '<why>' --yes") + `</div>`)
	b.WriteString(`<p class="mt-2 text-xs text-ink-muted" data-purge-not-here>The console does not run purge: the daemon has no purge path that also flushes its own live caches (the result cache, the call-graph memo, vectors), so a page-driven purge would verify the store while this process kept serving answers built from the deleted rows.</p>`)
	return b.String()
}
