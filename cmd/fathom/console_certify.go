package main

// console_certify.go — /certifications (OC-24, design §3 "Certifications"):
// the targets the certifications layer considers — declared targets,
// invariant rows, incident records, ports — counted by the state of their
// best certificate (a targets × certificate-states matrix, kit heatmap with
// truth states and its table), each repository's line, and the gap list
// with the remedy for each. It renders `fathom certify status` and `certify
// gaps` per repository (certify.Gaps over this daemon's store); fathom
// asserts liveness, never adequacy (ADR-043).

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/fathom/internal/certify"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/viz"
)

func init() {
	declarePage(consolePage{
		ID: "certifications", Route: "/certifications", Href: "/certifications", Group: "Proof", Order: 3, Nav: true,
		Title:    "Certifications",
		Question: "Which invariants, ports and incidents have live test proof, and which are gaps?",
		Verbs:    []string{"fathom certify status --service {svc} --store {store} --json", "fathom certify gaps --service {svc} --store {store} --json"},
		Shows:    []console.MetricID{console.MetricServices, console.MetricCertGaps, console.MetricCertLive},
		Build:    buildCertificationsPage,
	})
}

// certGapsPageSize is how many gaps one page of the gap list shows.
const certGapsPageSize = 50

// certReport is one repository's certify.Gaps read.
type certReport struct {
	service, indexCommit string
	report               certify.GapReport
	err                  string
}

// certCache memoizes certify.Gaps per repository and index watermark and
// HEAD (the inputs a report depends on); a failed read is not memoized.
var certCache = struct {
	sync.Mutex
	m map[string]certReport
}{m: map[string]certReport{}}

// certifyReport is one repository's gaps report, memoized.
func certifyReport(ctx context.Context, pc *consolePlans, sr serviceRoot) certReport {
	c := pc.deps.c
	if c == nil {
		return certReport{service: sr.Service, err: "this console has no store"}
	}
	key := fmt.Sprintf("%p|%s|%s|%s", c, sr.Service, pc.storeWatermark(ctx, sr.Service), pc.head(sr.Root))
	certCache.Lock()
	r, ok := certCache.m[key]
	certCache.Unlock()
	if ok {
		return r
	}
	r = certReport{service: sr.Service}
	st := c.Store()
	if meta, found, err := st.GetServiceMeta(ctx, sr.Service); err == nil && found {
		r.indexCommit = meta.IndexedCommit
	}
	rep, err := certify.Gaps(ctx, st, sr.Service, sr.Root, nil)
	if err != nil {
		// A failure is shown with its reason and never memoized: the next
		// read asks again.
		r.err = err.Error()
		return r
	}
	if len(rep.Unread) > 0 {
		// A source that could not be read makes every count a lower
		// bound (I38): the repository reads as an error naming them, not
		// as fewer targets, and the next read asks again.
		r.err = console.CertUnreadErr(rep.Unread)
		return r
	}
	r.report = rep
	certCache.Lock()
	if len(certCache.m) > 512 {
		certCache.m = map[string]certReport{}
	}
	certCache.m[key] = r
	certCache.Unlock()
	return r
}

// certFacts is a report as the projection reads it.
func certFacts(r certReport) console.CertServiceFacts {
	f := console.CertServiceFacts{Service: r.service, Err: r.err, Targets: []console.CertTargetFact{}}
	if r.err != "" {
		return f
	}
	s := r.report.Summary
	f.Declared, f.Live, f.Gaps, f.JoinedCases, f.Cases = s.Declared, s.Live, s.Gaps, s.JoinedCases, s.Cases
	if s.Declared == 0 {
		f.Skip = "no manifests found — nothing declared"
	}
	for _, t := range r.report.Targets {
		tf := console.CertTargetFact{Kind: t.Kind, ID: t.ID, Gap: t.Gap, Remedy: t.Remedy, SourceFile: t.Source.File, SourceLine: t.Source.Line, Certificates: []console.CertFact{}}
		for _, c := range t.Certificates {
			tf.Certificates = append(tf.Certificates, console.CertFact{Test: c.Test, Suite: c.Suite, Strength: c.Strength, State: c.State, Tier: c.Tier})
		}
		f.Targets = append(f.Targets, tf)
	}
	return f
}

// certificationsModel is /certifications' JSON model.
type certificationsModel struct {
	console.CertMatrixView
	Page  int `json:"page"`
	Pages int `json:"pages"`
}

func buildCertificationsPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	roots, scope, err := plansScope(cr, m)
	if err != nil {
		return consoleOut{}, err
	}
	pc := plansFor(cr.deps)
	ctx := cr.r.Context()
	var facts []console.CertServiceFacts
	verbs := []string{}
	sections := map[string]any{}
	for _, sr := range roots {
		r := certifyReport(ctx, pc, sr)
		f := certFacts(r)
		// A repository that declares nothing adds nothing to the fleet
		// view; with ?svc= it is the page's whole (named) answer.
		if scope == "" && f.Err == "" && f.Declared == 0 {
			continue
		}
		facts = append(facts, f)
		status := "fathom certify status --service " + sr.Service + cr.deps.storeArg() + " --json"
		verbs = append(verbs, status)
		if r.err == "" {
			sections[status] = certifyStatusDoc(sr.Service, r.report)
		}
		if scope != "" {
			gaps := "fathom certify gaps --service " + sr.Service + cr.deps.storeArg() + " --json"
			verbs = append(verbs, gaps)
			if r.err == "" {
				sections[gaps] = certifyGapsPayloadFor(sr.Service, r.indexCommit, cr.now, r.report)
			}
		}
	}
	view := console.CertMatrix(facts, scope, func(s string) string { return "/certifications?svc=" + url.QueryEscape(s) })
	model := certificationsModel{CertMatrixView: view}
	model.Pages = (len(view.Gaps) + certGapsPageSize - 1) / certGapsPageSize
	model.Page = pageParam(cr.r, model.Pages)
	all := consoleModelMetrics(m)
	if scope == "" {
		all[console.MetricCertGaps], all[console.MetricCertLive] = view.GapCount, view.Live
	}
	metrics := pageMetrics(cr.page, all)
	meta := []string{fmt.Sprintf("%s declaring targets of %s in the registry", repositories(len(facts)), valuePlain(metrics[console.MetricServices])), "generated " + m.GeneratedAt}
	if scope != "" {
		meta = []string{scope, "generated " + m.GeneratedAt}
	}
	out := consoleOut{
		Meta: meta, Model: model, Metrics: metrics, Sections: sections, Verbs: verbs,
		Body: func() string { return renderCertifications(model, scope, roots, cr.deps.storeArg()) },
	}
	if scope == "" && len(verbs) > 0 {
		out.Chips, out.Actions = []string{}, verbsDisclosure(verbs)
	}
	return out, nil
}

// certHeatmap draws the matrix on the kit heatmap with truth states: a
// count is a value and a measured zero is drawn as zero, never as missing.
func certHeatmap(v console.CertMatrixView) string {
	if len(v.Rows) == 0 {
		return ""
	}
	rows := make([][]float64, len(v.Rows))
	states := make([][]viz.HeatmapCell, len(v.Rows))
	ylabels := make([]string, len(v.Rows))
	for i, r := range v.Rows {
		ylabels[i] = r.Kind
		rows[i] = make([]float64, len(r.Cells))
		states[i] = make([]viz.HeatmapCell, len(r.Cells))
		for j, c := range r.Cells {
			rows[i][j] = float64(c.Count)
			if c.Count == 0 {
				states[i][j] = viz.HeatmapCell{State: viz.CellZero, Reason: "no " + r.Kind + " target stands " + v.States[j]}
			} else {
				states[i][j] = viz.HeatmapCell{State: viz.CellValue}
			}
		}
	}
	return viz.Heatmap(rows, viz.HeatmapOptions{Role: "accent", Cell: 76, XLabels: v.States, YLabels: ylabels, ShowValues: true, States: states,
		AriaLabel: "certification targets by kind and by their best certificate's state"})
}

// certMatrixTable is the matrix as a table (the heatmap's alternative).
func certMatrixTable(v console.CertMatrixView) string {
	cols := []table.Column{{Key: "kind", Label: "Target kind"}}
	for _, s := range v.States {
		cols = append(cols, table.Column{Key: s, Label: s, Align: "right"})
	}
	cols = append(cols, table.Column{Key: "total", Label: "Targets", Align: "right"})
	rows := make([]table.Row, 0, len(v.Rows))
	for _, r := range v.Rows {
		cells := []string{r.Kind}
		for _, c := range r.Cells {
			cells = append(cells, fmtThousands(c.Count))
		}
		cells = append(cells, fmtThousands(r.Total))
		rows = append(rows, table.Row{ID: "kind-" + r.Kind, Cells: cells})
	}
	return renderKitTable(table.TableParams{TableID: "cert-matrix", Caption: "Targets by kind and by their best certificate's state (only live satisfies)", Columns: cols, Rows: rows})
}

func renderCertifications(m certificationsModel, scope string, roots []serviceRoot, storeArg string) string {
	v := m.CertMatrixView
	var b strings.Builder
	b.WriteString(renderCertScopeNav(v, scope))
	b.WriteString(`<div class="mb-4">` + stat.RenderBand([]stat.Stat{
		valueStat("targets considered", v.Declared),
		valueStat("with a live certificate", v.Live),
		valueStat("gaps", v.GapCount),
		{Label: "repositories declaring", Value: fmtThousands(len(v.Services)), Sub: "in this scope"},
	}, stat.BandOptions{AriaLabel: "certifications at a glance"}) + `</div>`)

	verb := "fathom certify status --service <svc>" + storeArg + " --json"
	gapsVerb := "fathom certify gaps --service <svc>" + storeArg + " --json"
	if scope != "" {
		verb = "fathom certify status --service " + scope + storeArg + " --json"
		gapsVerb = "fathom certify gaps --service " + scope + storeArg + " --json"
	}
	matrix := `<p class="italic text-ink-muted">no certification targets in this scope</p>`
	if len(v.Rows) > 0 {
		// From md up the heatmap with its table behind a disclosure; below
		// md, where six state columns do not fit, the table itself.
		matrix = `<div class="hidden md:block"><div class="overflow-x-auto" role="region" aria-label="targets by certificate state" tabindex="0">` + certHeatmap(v) + `</div>` +
			`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: "The matrix as a table", Body: template.HTML(certMatrixTable(v))}) + `</div></div>` +
			`<div class="md:hidden">` + certMatrixTable(v) + `</div>`
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "cert-matrix", Title: "Targets by certificate state", SourceKey: "index",
		Question:    "For each kind of target, how many stand on a live certificate, and how many on something weaker or on nothing?",
		Verb:        gapsVerb,
		Source:      "the certifications layer's compiled certificates and the intent corpus (certify.Gaps over this daemon's store)",
		Denominator: valuePlain(v.Declared) + " targets",
		Freshness:   "as rendered; recomputed when the index or HEAD moves",
		State:       v.GapCount.State,
		Reason:      v.GapCount.Reason,
		Body:        matrix,
	}))

	if scope == "" {
		b.WriteString(`<div class="mt-4">`)
		b.WriteString(renderPanel(consolePanel{
			ID: "cert-services", Title: "Per repository", SourceKey: "index",
			Question:    "Which repositories declare targets, and how many are live?",
			Verb:        verb,
			Source:      "certify status per repository",
			Denominator: repositories(len(v.Services)) + " declaring targets",
			Body:        renderCertServices(v),
		}))
		b.WriteString(`</div>`)
	}

	b.WriteString(`<div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "cert-gaps", Title: "Gaps", SourceKey: "index",
		Question:    "Which targets have no live certificate, and what closes each one?",
		Verb:        gapsVerb,
		Source:      "certify gaps — each remedy names the marker or manifest field to add",
		Denominator: plural(len(v.Gaps), "gap"),
		Body:        renderCertGaps(m, scope),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

func renderCertScopeNav(v console.CertMatrixView, scope string) string {
	var b strings.Builder
	b.WriteString(`<nav aria-label="Filter certifications by repository" class="mb-4 flex flex-wrap gap-2">`)
	link := func(href, label string, current bool) {
		cls := "inline-flex min-h-11 items-center rounded-full border border-line px-3 text-sm text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
		cur := ""
		if current {
			cls = "inline-flex min-h-11 items-center rounded-full border border-accent bg-accent-soft px-3 text-sm font-medium text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
			cur = ` aria-current="page"`
		}
		b.WriteString(`<a href="` + html.EscapeString(href) + `" class="` + cls + `"` + cur + `>` + html.EscapeString(label) + `</a>`)
	}
	link("/certifications", "All repositories", scope == "")
	for _, s := range v.Services {
		link(s.Href, s.Service, s.Service == scope)
	}
	b.WriteString(`</nav>`)
	return b.String()
}

func renderCertServices(v console.CertMatrixView) string {
	if len(v.Services) == 0 {
		return chrome.RenderEmptyState(chrome.EmptyState{Heading: "No repository declares a target", Body: "A target is declared by a `// certifies:` marker, a tests/*.cue manifest, an invariant row or an incident record; certify gaps lists what each one needs."})
	}
	rows := make([]table.Row, 0, len(v.Services))
	for _, s := range v.Services {
		link := `<a href="` + html.EscapeString(s.Href) + `" class="inline-flex min-h-11 items-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">` + html.EscapeString(s.Service) + `</a>`
		join := valueCell(s.JoinRate)
		if s.JoinRate.Known() {
			join += "%"
		}
		rows = append(rows, table.Row{
			ID:        "cert-svc-" + s.Service,
			Cells:     []string{s.Service},
			HTMLCells: []templ.Component{templ.Raw(link), templ.Raw(valueCell(s.Declared)), templ.Raw(valueCell(s.Live)), templ.Raw(valueCell(s.Gaps)), templ.Raw(join)},
		})
	}
	return renderKitTable(table.TableParams{
		TableID: "cert-services", Caption: "Each repository's targets — open one for its exact certify status and certify gaps",
		Columns: []table.Column{{Key: "repository", Label: "Repository"}, {Key: "declared", Label: "Targets", Align: "right"}, {Key: "live", Label: "Live", Align: "right"}, {Key: "gaps", Label: "Gaps", Align: "right"}, {Key: "join", Label: "Cases joined to a symbol", Align: "right"}},
		Rows:    rows,
	}) + `<p class="mt-2 text-xs text-ink-muted">"not measured" joins mean the repository has recorded no test cases (task tests:attest, or fathom tests ingest); a repository that declares nothing is absent, never zero.</p>`
}

func renderCertGaps(m certificationsModel, scope string) string {
	v := m.CertMatrixView
	if len(v.Gaps) == 0 {
		if v.Declared.Known() {
			return `<p class="text-ink">no gaps: every target in this scope stands on a live certificate</p>`
		}
		return `<p class="italic text-ink-muted">no targets, so no gaps to list</p>`
	}
	lo := (m.Page - 1) * certGapsPageSize
	hi := lo + certGapsPageSize
	if hi > len(v.Gaps) {
		hi = len(v.Gaps)
	}
	rows := make([]table.Row, 0, hi-lo)
	var list strings.Builder
	for _, g := range v.Gaps[lo:hi] {
		remedy := `<code class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(g.Remedy) + `</code>`
		src := `<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(orDash(g.Source)) + `</span>`
		rows = append(rows, table.Row{
			ID:        "gap-" + g.Service + "-" + g.Kind + "-" + g.ID,
			Cells:     []string{g.Kind + " " + g.ID, g.Service, g.State},
			HTMLCells: []templ.Component{nil, nil, nil, templ.Raw(src), templ.Raw(remedy)},
		})
		// The same gap as a stacked entry for narrow screens.
		list.WriteString(`<li class="min-w-0 rounded-lg border border-line bg-surface-raised px-3 py-2"><p class="font-medium text-ink">` + html.EscapeString(g.Kind+" "+g.ID) + ` <span class="font-normal text-ink-muted">· ` + html.EscapeString(g.Service) + `</span></p>` +
			chrome.RenderDescriptionList([]chrome.DescItem{
				{Term: "best certificate", Detail: g.State},
				{Term: "declared at", DetailHTML: template.HTML(src)},
				{Term: "remedy", DetailHTML: template.HTML(remedy)},
			}) + `</li>`)
	}
	pager := consolePager(chrome.Pagination{
		Page: m.Page, TotalPages: m.Pages, AriaLabel: "Gap pages",
		HrefFor: func(page int) string {
			q := url.Values{}
			if scope != "" {
				q.Set("svc", scope)
			}
			q.Set("page", fmt.Sprint(page))
			return "/certifications?" + q.Encode() + "#cert-gaps"
		},
	})
	caption := fmt.Sprintf("Gaps %d–%d of %d, by repository, kind and id", lo+1, hi, len(v.Gaps))
	// From lg up the kit table; below it, where the remedy column squeezed
	// every row tall, the same gaps stack one entry each (as /statusz does).
	return `<div class="hidden lg:block" data-gaps-table>` + renderKitTable(table.TableParams{
		TableID: "cert-gaps", Caption: caption,
		Columns: []table.Column{{Key: "target", Label: "Target"}, {Key: "repository", Label: "Repository"}, {Key: "state", Label: "Best certificate"}, {Key: "source", Label: "Declared at"}, {Key: "remedy", Label: "Remedy"}},
		Rows:    rows,
	}) + `</div><div class="lg:hidden" data-gaps-list><p class="mb-2 text-xs text-ink-muted">` + html.EscapeString(caption) + `</p><ul class="flex flex-col gap-2" aria-label="gaps">` + list.String() + `</ul></div>` + pager
}
