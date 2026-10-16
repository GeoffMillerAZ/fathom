package main

// console_explain.go — the serving explainer at /explain?svc=&file= (vision
// 35 §4.3, design §3 "Serving explainer", plan OC-34): the obligations
// algorithm made visible. It draws semantic.ExplainObligations — every
// candidate claim in four columns by scope tier (this file, its package,
// ancestor directories, the repository root) in ranking order, the cap and
// the nearest-tier reservation, the served list, and each displaced rule
// with the reason it lost — beside what the agent actually saw at the edit
// (raw-fallback rows first, ADR-037) and the coverage that explains them.
// Why the agent saw what it saw.
//
// servingExplanation below is the one builder: `fathom obligations
// --explain` (obligations.go) prints what it returns, and this page's JSON
// section is the same value.

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/pkg/client"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
)

const (
	verbExplainFile = "fathom obligations --service {svc} --file {file} --layer {layer} --explain --json"
	verbExplainRoot = "fathom obligations --service {svc} --layer {layer} --explain --json"
)

func init() {
	declarePage(consolePage{
		ID: "explain", Route: "/explain", Href: "/explain", Group: "Knowledge", Order: 4, Nav: true,
		Title:    "Serving explainer",
		Question: "What governs this file, and why this rule and not that one?",
		Verbs:    []string{verbExplainFile, verbExplainRoot},
		Build:    buildExplainPage,
	})
}

// unknownKnowledgeService is a service the store neither indexes nor holds
// claims for.
type unknownKnowledgeService struct {
	service string
	known   []string
}

func (e *unknownKnowledgeService) Error() string {
	return fmt.Sprintf("service %q is not in the store (known: %s)", e.service, strings.Join(e.known, ", "))
}

// servingExplanation is the serving explainer's one builder: ExplainObligations
// over the client's store, plus the raw-fallback rows and the coverage the
// obligations tool adds around the compiled rules (the same enrichments,
// under the same conditions, renderObligationReceipt applies). It never
// calls a decision-port provider.
func servingExplanation(ctx context.Context, c *client.Client, svc, file string, layer semantic.ObligationLayerFilter, storePath string) (console.ServingExplanation, error) {
	st := c.Store()
	known, all, err := knowledgeServiceKnown(ctx, st, svc)
	if err != nil {
		return console.ServingExplanation{}, err
	}
	if !known {
		return console.ServingExplanation{}, &unknownKnowledgeService{service: svc, known: all}
	}
	ex, err := semantic.ExplainObligations(ctx, st, svc, file, 0, layer)
	if err != nil {
		return console.ServingExplanation{}, err
	}
	in := console.ServingInputs{Explanation: ex, RerankProvider: rerankProvider(), StorePath: storePath}
	if file != "" && ex.Layer != string(semantic.ObligationLayerClaims) {
		if rows, why, ok, rerr := c.RawFallbackObligations(ctx, svc, file, semantic.RawFallbackReserve); rerr == nil && ok {
			in.Raw, in.RawWhy, in.RawOK = rows, why, true
		}
	}
	in.Coverage, in.CoverageFound, in.CoverageErr = c.ObligationsLayerCoverage(ctx, svc)
	return console.ServingView(in), nil
}

// explainCache keeps recent explanations per daemon. The two enrichments
// read the project's whole corpus from disk (hundreds of milliseconds on a
// large repository), so an explanation is reused while no event has been
// published since it was built — an ingest or a compile moves the event
// bus's cursor — and for at most explainCacheTTL. Bounded.
var explainCache = struct {
	sync.Mutex
	entries map[string]explainCacheEntry
}{entries: map[string]explainCacheEntry{}}

type explainCacheEntry struct {
	cursor string
	at     time.Time
	se     console.ServingExplanation
}

const (
	explainCacheTTL = 30 * time.Second
	explainCacheMax = 32
)

// cachedServingExplanation is servingExplanation behind explainCache.
func cachedServingExplanation(cr *consoleRequest, svc, file string, layer semantic.ObligationLayerFilter) (console.ServingExplanation, error) {
	key := fmt.Sprintf("%p\x00%s\x00%s\x00%s", cr.deps, svc, file, layer)
	cursor := ""
	if cr.deps.bus != nil {
		cursor = cr.deps.bus.Cursor()
	}
	explainCache.Lock()
	if e, ok := explainCache.entries[key]; ok && e.cursor == cursor && !cr.now.Before(e.at) && cr.now.Sub(e.at) < explainCacheTTL {
		explainCache.Unlock()
		return e.se, nil
	}
	explainCache.Unlock()
	se, err := servingExplanation(cr.r.Context(), cr.deps.c, svc, file, layer, cr.deps.storePath)
	if err != nil {
		return se, err
	}
	explainCache.Lock()
	if len(explainCache.entries) >= explainCacheMax {
		clear(explainCache.entries)
	}
	explainCache.entries[key] = explainCacheEntry{cursor: cursor, at: cr.now, se: se}
	explainCache.Unlock()
	return se, nil
}

// rerankProvider is the decision-port provider serving re-ranks within a
// tier with, "" when [decide] rank_within_tier is off or no provider is set.
func rerankProvider() string {
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		return ""
	}
	d := loaded.Config.Decide
	if !d.RankWithinTier || d.Provider == "" || d.Provider == "none" {
		return ""
	}
	return d.Provider
}

// explainModel is the page's own projection: the form's state.
type explainModel struct {
	Service  string   `json:"service,omitempty"`
	File     string   `json:"file,omitempty"`
	Layer    string   `json:"layer"`
	Services []string `json:"services"`
}

func buildExplainPage(cr *consoleRequest) (consoleOut, error) {
	if cr.deps.c == nil {
		_, err := knowledgeSourceFor(cr)
		return consoleOut{}, err
	}
	q := cr.r.URL.Query()
	svc, file := strings.TrimSpace(q.Get("svc")), strings.Trim(strings.TrimSpace(q.Get("file")), "/")
	layer := q.Get("layer")
	if layer == "" {
		layer = string(semantic.ObligationLayerRules)
	}
	switch semantic.ObligationLayerFilter(layer) {
	case semantic.ObligationLayerRules, semantic.ObligationLayerClaims, semantic.ObligationLayerAll:
	default:
		return consoleOut{}, &consoleError{Status: http.StatusBadRequest, Title: "Unknown layer filter",
			Reason: "layer must be rules, claims or all; got " + layer + ".", Remedy: "fathom obligations --service <project> --file <path> --explain",
			RemedyHref: "/explain", RemedyLabel: "Start over"}
	}
	_, services, err := knowledgeServiceKnown(cr.r.Context(), cr.deps.c.Store(), "")
	if err != nil {
		return consoleOut{}, err
	}
	model := explainModel{Service: svc, File: file, Layer: layer, Services: services}
	if model.Services == nil {
		model.Services = []string{}
	}
	out := consoleOut{Model: model, Verbs: []string{}}
	if svc == "" {
		out.Meta = []string{plural(len(services), "project") + " in this store"}
		out.Body = func() string { return renderExplainForm(model) + renderExplainIntro(services) }
		return out, nil
	}
	se, err := cachedServingExplanation(cr, svc, file, semantic.ObligationLayerFilter(layer))
	if err != nil {
		if uk, ok := err.(*unknownKnowledgeService); ok {
			return consoleOut{}, &consoleError{Status: http.StatusNotFound, Title: "Unknown project",
				Reason: "No indexed project is named " + uk.service + ".", Remedy: "fathom layers coverage",
				RemedyHref: "/explain", RemedyLabel: "Pick a project"}
		}
		return consoleOut{}, err
	}
	se = withRuleOutcomes(se, cr.now) // SR-61: read at request time, never cached with the explanation
	verb := strings.NewReplacer("{svc}", shellWord(svc), "{file}", shellWord(file), "{layer}", layer).Replace(verbExplainFile)
	if file == "" {
		verb = strings.NewReplacer("{svc}", shellWord(svc), "{layer}", layer).Replace(verbExplainRoot)
	}
	out.Verbs = []string{verb}
	out.Sections = map[string]any{verb: se}
	subject := file
	if subject == "" {
		subject = "the repository root"
	}
	out.Title = "Serving explainer"
	out.Meta = []string{svc, subject, "layer " + layer}
	page := 1
	if p, perr := strconv.Atoi(q.Get("page")); perr == nil && p > 1 {
		page = p
	}
	pageHref := func(p int) string {
		v := url.Values{}
		v.Set("svc", svc)
		if file != "" {
			v.Set("file", file)
		}
		v.Set("layer", layer)
		v.Set("page", strconv.Itoa(p))
		return "/explain?" + v.Encode() + "#candidates"
	}
	out.Body = func() string { return renderExplainForm(model) + renderExplanation(se, verb, page, pageHref) }
	return out, nil
}

// renderExplainForm is the picker: a GET form, so every explanation is a
// URL.
func renderExplainForm(m explainModel) string {
	opts := make([]form.Option, 0, len(m.Services))
	for _, s := range m.Services {
		opts = append(opts, form.Option{Value: s, Label: s})
	}
	project := form.RenderField(form.Field{Label: "Project", For: "explain-svc", Control: template.HTML(form.RenderSelect(form.Select{
		ID: "explain-svc", Name: "svc", Value: m.Service, Placeholder: "Choose a project", Options: opts, Required: true,
	}))})
	file := form.RenderField(form.Field{Label: "File", For: "explain-file", Help: "Repository-relative; empty asks what governs the repository root.", Control: template.HTML(form.RenderInput(form.Input{
		ID: "explain-file", Name: "file", Value: m.File, Placeholder: "internal/conserve/conserve.go", HasHelp: true, Autocomplete: "off",
	}))})
	layer := form.RenderField(form.Field{Label: "Claims", For: "explain-layer", Control: template.HTML(form.RenderSelect(form.Select{
		ID: "explain-layer", Name: "layer", Value: m.Layer, Options: []form.Option{
			{Value: "rules", Label: "rules — what the edit hook serves"},
			{Value: "claims", Label: "claims — plan-row bindings"},
			{Value: "all", Label: "all — the union"},
		},
	}))})
	return `<form method="get" action="/explain" class="mb-4 grid items-start gap-3 rounded-lg border border-line bg-surface-raised p-4 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_minmax(0,1fr)_auto] [&_select]:min-h-11 [&_input]:min-h-11" aria-label="Explain a file" data-explain-form>` +
		project + file + layer +
		`<div class="md:pt-6 [&>button]:min-h-11 [&>button]:w-full">` + form.RenderButton(form.Button{Label: "Explain"}) + `</div></form>`
}

// renderExplainIntro is the page before a project is chosen.
func renderExplainIntro(services []string) string {
	body := "Choose a project and a file. The explainer runs the obligations algorithm for that file and shows every candidate rule by scope tier, the six-rule cap, what was served, and why each other rule lost."
	var action template.HTML
	if len(services) == 0 {
		body = "This store holds no indexed project, so there is nothing to explain yet."
	} else {
		action = template.HTML(`<span class="[&>a]:min-h-11">` + form.RenderButton(form.Button{Label: "Explain what governs " + services[0] + "'s root", Href: explainHref(services[0], ""), Variant: form.ButtonGhost}) + `</span>`)
	}
	return chrome.RenderEmptyState(chrome.EmptyState{Heading: "Why an agent editing a file sees the rules it sees", Body: body, Action: action})
}

// scopeTierTitle names a scope-tier column.
func scopeTierTitle(tier string) string {
	switch tier {
	case semantic.ScopeTierExactFile:
		return "This file"
	case semantic.ScopeTierPackage:
		return "Package directory"
	case semantic.ScopeTierParentDir:
		return "Ancestor directories"
	case semantic.ScopeTierRoot:
		return "Repository root"
	}
	return tier
}

// explainColumnShown bounds each column's list: every served rule, then
// this many displaced ones; the table below lists the rest.
const explainColumnShown = 5

// explainTablePerPage pages the candidates table (design §2: tables over
// 50 rows are paged), so every candidate is on some page of it.
const explainTablePerPage = 50

func renderExplanation(se console.ServingExplanation, verb string, page int, pageHref func(int) string) string {
	var b strings.Builder
	served := len(se.Served.Rules)
	subject := se.File
	if subject == "" {
		subject = "the repository root"
	}
	summary := fmt.Sprintf("%s served · %s displaced · %s eligible", fmtThousands(served), fmtThousands(se.Displaced), fmtThousands(se.Eligible))
	if se.Deduped > 0 {
		summary += " · " + plural(se.Deduped, "duplicate statement") + " served once"
	}
	b.WriteString(`<div class="flex flex-col gap-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "candidates", Title: "Candidates by scope", SourceKey: "layers:" + se.Service + " index:" + se.Service,
		Question: "Nearer scope wins: rules for this file, then its package, then ancestor directories, then the root — fresh before stale, human-authored guides before plan bindings.",
		Verb:     verb,
		Source:   "semantic.ExplainObligations over the daemon's store — the serving engine itself, run with a trace",
		Denominator: fmt.Sprintf("%s scope-matched claims for %s; cap %d, %d reserved for the nearest tier",
			fmtThousands(se.Eligible), subject, se.Cap, se.ReservedSlots),
		Freshness: knowledgeFreshness,
		Body:      renderExplainColumns(se) + `<p class="mt-3 text-sm text-ink" data-explain-summary>` + html.EscapeString(summary) + `</p>` + renderExplainRanking(se) + renderExplainTable(se, page, pageHref),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "served", Title: "What the agent saw at the edit", SourceKey: "layers:" + se.Service,
		Question:    "The exact list the obligations tool injects for this file, in order, each citing the lines it restates.",
		Verb:        strings.Replace(verb, " --explain", "", 1),
		Source:      "the served list, plus the raw-fallback rows the tool prepends for an uncompiled governing guide (ADR-037)",
		Denominator: plural(len(se.AgentSees), "rule"),
		Freshness:   knowledgeFreshness,
		Body:        renderAgentSees(se),
	}))
	b.WriteString(renderPanel(consolePanel{
		ID: "coverage", Title: "Why raw rows appear", SourceKey: "layers:" + se.Service,
		Question:    "How completely does the obligations layer cover this project's governing guides?",
		Verb:        strings.Replace(strings.Replace(verb, " --explain", "", 1), "--file "+shellWord(se.File)+" ", "", 1),
		Source:      se.Coverage.Source,
		Denominator: se.Coverage.Denominator,
		State:       se.Coverage.State, Reason: se.Coverage.Reason,
		Freshness: knowledgeFreshness,
		Body:      renderExplainCoverage(se),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

// renderExplainColumns draws the four scope columns in ranking order.
func renderExplainColumns(se console.ServingExplanation) string {
	var b strings.Builder
	cols := "md:grid-cols-2 xl:grid-cols-4"
	if len(se.Tiers) == 1 {
		cols = ""
	}
	b.WriteString(`<ol class="grid items-start gap-3 ` + cols + `" aria-label="candidates by scope tier, nearest first">`)
	for _, t := range se.Tiers {
		b.WriteString(`<li class="flex min-w-0 flex-col rounded-md border border-line bg-surface p-3" data-scope-tier="` + html.EscapeString(t.Tier) + `">`)
		b.WriteString(`<h3 class="text-sm font-semibold text-ink">` + html.EscapeString(scopeTierTitle(t.Tier)) + `</h3>`)
		counts := fmt.Sprintf("%d served · %d displaced", t.Served, t.Displaced)
		if t.Duplicates > 0 {
			counts += fmt.Sprintf(" · %d duplicate", t.Duplicates)
		}
		b.WriteString(`<p class="text-xs text-ink-muted">` + counts + `</p>`)
		if t.Candidates == 0 {
			b.WriteString(`<p class="mt-2 text-xs italic text-ink-muted">no candidate rules at this scope</p></li>`)
			continue
		}
		b.WriteString(`<ul class="mt-2 flex flex-col gap-2">`)
		shownDisplaced, hidden := 0, 0
		for _, c := range se.Candidates {
			if c.ScopeTier != t.Tier {
				continue
			}
			if !c.Served {
				if shownDisplaced >= explainColumnShown {
					hidden++
					continue
				}
				shownDisplaced++
			}
			b.WriteString(renderCandidate(c))
		}
		b.WriteString(`</ul>`)
		if hidden > 0 {
			b.WriteString(`<p class="mt-2 text-xs text-ink-muted">+` + fmtThousands(hidden) + ` more displaced — every candidate is in the paged table below</p>`)
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol>`)
	return b.String()
}

// renderCandidate is one candidate card: rank, rule, source and outcome in
// words (served in the positive role, displaced with its reason).
func renderCandidate(c semantic.ExplainCandidate) string {
	outcome := fmt.Sprintf(`<span class="font-medium text-positive">served #%d</span>`, c.Position)
	if c.Reserved {
		outcome += ` <span class="text-ink-muted">· reserved slot</span>`
	}
	border := "border-positive"
	if c.Displaced != nil {
		outcome = `<span class="font-medium text-ink-muted">` + html.EscapeString(c.Displaced.Reason) + `</span> <span class="text-ink-muted">— ` + html.EscapeString(c.Displaced.Why) + `</span>`
		border = "border-line"
	}
	status := ""
	if c.Status != "fresh" {
		status = ` <span class="text-caution">` + html.EscapeString(c.Status) + `</span>`
	}
	return `<li class="min-w-0 border-l-2 ` + border + ` pl-2" data-candidate="` + html.EscapeString(c.ClaimID) + `" data-outcome="` + candidateOutcome(c) + `">` +
		`<p class="text-sm text-ink [overflow-wrap:anywhere]"><span class="text-xs text-ink-muted">#` + fmtThousands(c.Rank) + `</span> ` + html.EscapeString(c.Rule) + status + `</p>` +
		`<p class="text-xs text-ink-muted">` + pathLink(claimHref(c.ClaimID, ""), c.Src) + ` · ` + html.EscapeString(c.Layer) + `</p>` +
		`<p class="text-xs">` + outcome + `</p></li>`
}

func candidateOutcome(c semantic.ExplainCandidate) string {
	if c.Served {
		return "served"
	}
	if c.Displaced != nil {
		return c.Displaced.Reason
	}
	return "displaced"
}

// renderExplainRanking states the ranking and the re-rank in words.
func renderExplainRanking(se console.ServingExplanation) string {
	words := map[string]string{
		semantic.RankKeyFreshness: "fresh first", semantic.RankKeyRole: "guides before plan bindings",
		semantic.RankKeyScopeTier: "nearer scope", semantic.RankKeyRelevance: "relevance to the file",
		semantic.RankKeySpecificity: "narrower glob", semantic.RankKeyText: "rule text",
	}
	var keys []string
	for _, k := range se.Ranking {
		keys = append(keys, words[k])
	}
	return `<p class="mt-1 text-xs text-ink-muted" data-explain-ranking>ranked by: ` + html.EscapeString(strings.Join(keys, " → ")) + ` · re-rank: ` + html.EscapeString(se.Rerank) + `</p>`
}

// renderExplainTable is every candidate as a table (the columns' table
// alternative), paged 50 at a time so every candidate is on a page.
func renderExplainTable(se console.ServingExplanation, page int, pageHref func(int) string) string {
	if len(se.Candidates) == 0 {
		return `<p class="mt-3 text-sm text-ink-muted">No compiled claim's scope reaches ` + html.EscapeString(se.File) + ` — nothing competed for the cap.</p>`
	}
	pages := (len(se.Candidates) + explainTablePerPage - 1) / explainTablePerPage
	page = min(max(page, 1), pages)
	lo := (page - 1) * explainTablePerPage
	hi := min(lo+explainTablePerPage, len(se.Candidates))
	var rows []table.Row
	for _, c := range se.Candidates[lo:hi] {
		outcome := fmt.Sprintf("served #%d", c.Position)
		if c.Displaced != nil {
			outcome = c.Displaced.Reason + " — " + c.Displaced.Why
		}
		rows = append(rows, table.Row{
			ID:    "cand-" + c.ClaimID,
			Cells: []string{fmtThousands(c.Rank), scopeTierTitle(c.ScopeTier), c.Rule, c.Layer, c.Status, outcome},
			Muted: !c.Served,
		})
	}
	caption := fmt.Sprintf("All %d candidates in ranking order", len(se.Candidates))
	if pages > 1 {
		caption = fmt.Sprintf("Candidates %d–%d of %d in ranking order", lo+1, hi, len(se.Candidates))
	}
	tbl := renderKitTable(table.TableParams{
		TableID: "explain-candidates", Caption: caption,
		Columns: []table.Column{{Key: "rank", Label: "Rank", Align: "right"}, {Key: "tier", Label: "Scope"}, {Key: "rule", Label: "Rule"}, {Key: "layer", Label: "Layer"}, {Key: "status", Label: "Status"}, {Key: "outcome", Label: "Outcome"}},
		Rows:    rows,
	})
	if pages > 1 {
		// The kit's pager links are 44 px targets themselves
		// (FATHOM-F0-KIT-05); the div only spaces it.
		tbl += `<div class="mt-3">` + chrome.RenderPagination(chrome.Pagination{
			Page: page, TotalPages: pages, AriaLabel: "candidate pages", HrefFor: pageHref,
		}) + `</div>`
	}
	// A later page is opened for reading: the disclosure starts open there.
	return `<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "Every candidate as a table", Body: template.HTML(tbl), Open: page > 1}) + `</div>`
}

// renderAgentSees is the injected list, raw rows flagged.
func renderAgentSees(se console.ServingExplanation) string {
	if len(se.AgentSees) == 0 {
		return `<p class="text-sm text-ink-muted" data-truth-state="zero">Nothing is served for this file: no compiled rule's scope reaches it and no uncompiled guide governs it.</p>`
	}
	// The served compiled rules map to candidates by position.
	byPos := map[int]semantic.ExplainCandidate{}
	for _, c := range se.Candidates {
		if c.Served {
			byPos[c.Position] = c
		}
	}
	var rows []table.Row
	var stacked strings.Builder
	raw := len(se.RawFallback)
	for i, r := range se.AgentSees {
		// The agent reads the raw rows first, then the compiled rules; a
		// compiled rule keeps its served number (#1…) — the one its
		// candidate card shows — and a raw row is numbered apart, so no
		// rule carries two numbers on one page.
		scope := "nearest uncompiled guide"
		ruleHTML := html.EscapeString(r.Rule)
		num := fmt.Sprintf("raw %d", i+1)
		if i >= raw {
			num = "#" + fmtThousands(i-raw+1)
			if c, ok := byPos[i-raw+1]; ok {
				scope = scopeTierTitle(c.ScopeTier)
				ruleHTML = knowledgeLink(claimHref(c.ClaimID, se.Service), r.Rule)
			}
		}
		if r.Why != "" {
			ruleHTML += `<span class="block text-xs text-caution">` + html.EscapeString(r.Why) + `</span>`
		}
		if r.Status != "" {
			ruleHTML += `<span class="block text-xs text-caution">served flagged ` + html.EscapeString(r.Status) + `</span>`
		}
		if o := ruleOutcomeHTML(se.Outcomes, r.ClaimID); o != "" {
			ruleHTML += o
		}
		rows = append(rows, table.Row{
			ID:        fmt.Sprintf("seen-%d", i+1),
			Cells:     []string{num, r.Tier, r.Rule, r.Src, scope},
			HTMLCells: []templ.Component{nil, nil, templ.Raw(ruleHTML), templ.Raw(`<span class="font-mono text-xs">` + html.EscapeString(r.Src) + `</span>`)},
		})
		stacked.WriteString(`<li class="min-w-0 rounded-lg border border-line bg-surface-raised p-3"><p class="text-xs text-ink-muted">` + html.EscapeString(num) + ` · ` + html.EscapeString(r.Tier) + ` · ` + html.EscapeString(scope) + `</p>` +
			`<p class="mt-1 text-sm text-ink [overflow-wrap:anywhere]">` + ruleHTML + `</p>` +
			`<p class="mt-1 font-mono text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(r.Src) + `</p></li>`)
	}
	// From lg up the list is the kit table; below lg, where five columns
	// squeeze a rule to a word per line, the same rows stack (as /statusz's
	// services do). One is displayed at a width.
	caption := "Injected at the edit, in order"
	if raw > 0 {
		caption = "Injected at the edit, in order: the raw rows first, then the compiled rules by their served number"
	}
	tbl := `<div class="hidden lg:block" data-explain-served-table>` + renderKitTable(table.TableParams{
		TableID: "explain-served", Caption: caption,
		Columns: []table.Column{{Key: "n", Label: "#", Align: "right"}, {Key: "tier", Label: "Tier"}, {Key: "rule", Label: "Rule"}, {Key: "src", Label: "Source"}, {Key: "scope", Label: "Scope"}},
		Rows:    rows,
	}) + `</div><ol class="flex flex-col gap-3 lg:hidden" aria-label="injected at the edit, in order" data-explain-served-list>` + stacked.String() + `</ol>`
	if se.RawWhy != "" {
		tbl += `<p class="mt-2 text-xs text-ink-muted">` + plural(raw, "raw row") + ` served first because ` + html.EscapeString(se.RawWhy) + `.</p>`
	}
	// The hook's record under each rule is read from a log on this machine;
	// when there is none (or it cannot be read) say so once, never a zero.
	if !se.Outcomes.Known() {
		tbl += `<p class="mt-2 text-xs text-ink-muted" data-truth-state="` + string(se.Outcomes.State) + `" data-rule-outcomes-note>Hook record: ` + html.EscapeString(se.Outcomes.Reason) + `</p>`
	}
	return tbl
}

// ruleOutcomeHTML is one served rule's hook record under its statement
// (SR-61): "3 served · 1 top changed · 1 still served over the last 30 days", with
// GL-11's class as a proposal when one applies. "" when the record is not
// known or the rule has no claim id.
func ruleOutcomeHTML(v console.Value[console.RuleOutcomeSet], claimID string) string {
	if !v.Known() || claimID == "" {
		return ""
	}
	c, ok := v.V.ByClaim(claimID)
	if !ok {
		return ""
	}
	line := fmt.Sprintf("%d served · %d top changed · %d still served over the last %d days", c.Served, c.TopChanged, c.StillServed, v.V.WindowDays)
	if c.Class != "" {
		line += " — " + strings.ReplaceAll(c.Class, "_", " ") + " (a proposal: " + c.Remedy + ")"
	}
	return `<span class="block text-xs text-ink-muted" data-rule-outcomes>` + html.EscapeString(line) + `</span>`
}

// renderExplainCoverage names the layer's coverage and the resume command.
func renderExplainCoverage(se console.ServingExplanation) string {
	var b strings.Builder
	if cov := se.Coverage; cov.Known() {
		items := []chrome.DescItem{
			{Term: "sections compiled", Detail: fmt.Sprintf("%s of %s", fmtThousands(cov.V.SectionsCompiled), fmtThousands(cov.V.SectionsTotal))},
			{Term: "budget stopped", Detail: map[bool]string{true: "yes — the last compile ran out of budget", false: "no"}[cov.V.BudgetStopped]},
		}
		if cov.V.CorpusGrew > 0 {
			items = append(items, chrome.DescItem{Term: "corpus grew", Detail: plural(cov.V.CorpusGrew, "section") + " no compile has seen"})
		}
		if len(cov.V.UncompiledScopes) > 0 {
			items = append(items, chrome.DescItem{Term: "uncompiled", DetailHTML: template.HTML(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(cov.V.UncompiledScopes, ", ")) + `</span>`)})
		}
		b.WriteString(chrome.RenderDescriptionList(items))
	}
	if se.CoverageNote != "" {
		b.WriteString(`<p class="mt-2 text-sm text-caution">` + html.EscapeString(se.CoverageNote) + `</p>`)
		b.WriteString(remedy(se.Resume))
	}
	b.WriteString(`<p class="mt-2 text-xs text-ink-muted">Raw rows are served so a governing guide is never silently missing when the compiled layer did not reach it (ADR-037). ` +
		knowledgeLink(layerHref(semantic.LayerObligations)+"#uncompiled", "The obligations layer's uncompiled scopes") + `</p>`)
	return b.String()
}

// explainHref is an explanation's URL.
func explainHref(svc, file string) string {
	v := url.Values{}
	v.Set("svc", svc)
	if file != "" {
		v.Set("file", file)
	}
	return "/explain?" + v.Encode()
}
