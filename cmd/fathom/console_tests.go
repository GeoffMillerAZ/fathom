package main

// console_tests.go — /tests (TQ-8, docs/plans/test-intelligence.md; design
// doc docs/design/test-intelligence.md §3 T1, T2): one repository's
// risk-weighted coverage (the untested, dangerous symbols, each factor
// cited — the list test_impact rank:"risk" serves) and its test portfolio
// (each test's class, purpose line and redundancy, with the critical
// targets no live certificate covers). Both are computed per repository
// from facts stored at ingest (centrality, churn) and the index, so the
// page asks for a repository first rather than walking the fleet.
// fathom asserts reach and liveness, never adequacy.

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/url"
	"strings"
	"sync"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/certify"
)

func init() {
	declarePage(consolePage{
		ID: "tests", Route: "/tests", Href: "/tests", Group: "Proof", Order: 5, Nav: true,
		Title:    "Tests",
		Question: "Which untested code is most dangerous, and what does each test certify?",
		Build:    buildTestPortfolioPage,
	})
}

// testPortfolioModel is /tests' JSON model.
type testPortfolioModel struct {
	Service      string              `json:"service,omitempty"`
	Repositories []string            `json:"repositories"`
	Risk         *certify.RiskReport `json:"risk,omitempty"`
	Portfolio    *certify.Portfolio  `json:"portfolio,omitempty"`
	Err          string              `json:"error,omitempty"`
}

// testPortfolioRead is one repository's risk rank and portfolio.
type testPortfolioRead struct {
	risk      certify.RiskReport
	portfolio certify.Portfolio
	err       string
}

// testPortfolioCache memoizes a repository's read per store watermark and
// HEAD (the inputs it depends on); a failed read is not memoized.
var testPortfolioCache = struct {
	sync.Mutex
	m map[string]testPortfolioRead
}{m: map[string]testPortfolioRead{}}

func readTestPortfolio(ctx context.Context, pc *consolePlans, sr serviceRoot) testPortfolioRead {
	c := pc.deps.c
	if c == nil {
		return testPortfolioRead{err: "this console has no store"}
	}
	key := fmt.Sprintf("%p|%s|%s|%s", c, sr.Service, pc.storeWatermark(ctx, sr.Service), pc.head(sr.Root))
	testPortfolioCache.Lock()
	r, ok := testPortfolioCache.m[key]
	testPortfolioCache.Unlock()
	if ok {
		return r
	}
	st := c.Store()
	risk, err := certify.RiskWeightedCoverage(ctx, st, sr.Service, sr.Root, certify.RiskOptions{})
	if err != nil {
		return testPortfolioRead{err: err.Error()}
	}
	p, err := certify.ClassifyTests(ctx, st, sr.Service, sr.Root)
	if err != nil {
		return testPortfolioRead{err: err.Error()}
	}
	r = testPortfolioRead{risk: risk, portfolio: p}
	testPortfolioCache.Lock()
	if len(testPortfolioCache.m) > 128 {
		testPortfolioCache.m = map[string]testPortfolioRead{}
	}
	testPortfolioCache.m[key] = r
	testPortfolioCache.Unlock()
	return r
}

func buildTestPortfolioPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	roots, scope, err := plansScope(cr, m)
	if err != nil {
		return consoleOut{}, err
	}
	model := testPortfolioModel{Service: scope, Repositories: []string{}}
	for _, sr := range modelServiceRoots(m) {
		model.Repositories = append(model.Repositories, sr.Service)
	}
	meta := []string{"choose a repository", "generated " + m.GeneratedAt}
	if scope != "" {
		meta = []string{scope, "generated " + m.GeneratedAt}
		r := readTestPortfolio(cr.r.Context(), plansFor(cr.deps), roots[0])
		if r.err != "" {
			model.Err = r.err
		} else {
			model.Risk, model.Portfolio = &r.risk, &r.portfolio
		}
	}
	return consoleOut{
		Meta: meta, Model: model,
		Body: func() string { return renderTestPortfolio(model) },
	}, nil
}

// renderTestPortfolio renders /tests: the repository chooser, then (with a
// repository) the at-a-glance band, the risk rank, the portfolio and the
// critical gaps.
func renderTestPortfolio(m testPortfolioModel) string {
	var b strings.Builder
	b.WriteString(renderTestsScopeNav(m))
	if m.Service == "" {
		b.WriteString(chrome.RenderEmptyState(chrome.EmptyState{
			Heading: "Choose a repository",
			Body:    "The risk rank and the test portfolio are computed per repository from its index; pick one above.",
		}))
		return b.String()
	}
	if m.Err != "" {
		b.WriteString(chrome.RenderEmptyState(chrome.EmptyState{Heading: "The portfolio could not be read", Body: m.Err}))
		return b.String()
	}
	p, risk := m.Portfolio, m.Risk
	b.WriteString(`<div class="mb-4">` + stat.RenderBand([]stat.Stat{
		{Label: "tests", Value: fmtThousands(len(p.Tests))},
		{Label: "certifying", Value: fmtThousands(p.Counts[certify.ClassCertifying]), Sub: "carry a live certificate"},
		{Label: "orphans", Value: fmtThousands(p.Counts[certify.ClassOrphan]), Sub: "reach nothing governed or central"},
		{Label: "redundant", Value: fmtThousands(p.Redundant), Sub: "same reach and targets as another"},
		{Label: "critical gaps", Value: fmtThousands(len(p.Gaps)), Sub: "top-decile risk, no live certificate"},
	}, stat.BandOptions{AriaLabel: "test portfolio at a glance"}) + `</div>`)

	riskState := ""
	if risk.Watermark == "" {
		riskState = "centrality has not been stored for this repository yet; re-ingest it"
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "tests-risk", Title: "Untested and dangerous", SourceKey: "index",
		Question:    "Which symbols carry the most risk: uncovered, central, churning, governed and named by incidents?",
		Source:      `centrality and churn stored at ingest, governing rules and incident records (the list test_impact rank:"risk" serves)`,
		Denominator: fmt.Sprintf("top %d of %s", len(risk.Rows), plural(risk.Considered, "symbol")),
		Freshness:   "as rendered; recomputed when the index or HEAD moves",
		Body:        renderRiskRows(risk) + riskNote(riskState),
	}))

	b.WriteString(`<div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "tests-portfolio", Title: "Test portfolio", SourceKey: "index",
		Question:    "What does each test certify, and which tests certify nothing, reach nothing or duplicate another?",
		Source:      "the certifications layer's certificates and each test's static reach over the call graph",
		Denominator: plural(len(p.Tests), "test"),
		Freshness:   "as rendered; recomputed when the index or HEAD moves",
		Body:        renderPortfolioRows(p),
	}))
	b.WriteString(`</div><div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "tests-gaps", Title: "Critical targets without a certificate", SourceKey: "index",
		Question:    "Which top-decile risk symbols does no live certificate cover?",
		Source:      "the risk rank's top decile against live certificates' scope",
		Denominator: plural(len(p.Gaps), "gap"),
		Body:        renderPortfolioGaps(p),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

func riskNote(reason string) string {
	if reason == "" {
		return `<p class="mt-2 text-xs text-ink-muted">No coverage profile is read here: every uncovered share is 1 and says so. Reach is static, never a passing run.</p>`
	}
	return `<p class="mt-2 text-sm text-caution">` + html.EscapeString(reason) + `</p>`
}

func renderTestsScopeNav(m testPortfolioModel) string {
	var b strings.Builder
	b.WriteString(`<nav aria-label="Choose a repository" class="mb-4 flex flex-wrap gap-2">`)
	for _, s := range m.Repositories {
		cls := "inline-flex min-h-11 items-center rounded-full border border-line px-3 text-sm text-ink hover:bg-surface-sunken focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
		cur := ""
		if s == m.Service {
			cls = "inline-flex min-h-11 items-center rounded-full border border-accent bg-accent-soft px-3 text-sm font-medium text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
			cur = ` aria-current="page"`
		}
		b.WriteString(`<a href="/tests?svc=` + html.EscapeString(url.QueryEscape(s)) + `" class="` + cls + `"` + cur + `>` + html.EscapeString(s) + `</a>`)
	}
	b.WriteString(`</nav>`)
	return b.String()
}

func testsMono(s string) string {
	return `<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(s) + `</span>`
}

func renderRiskRows(r *certify.RiskReport) string {
	if len(r.Rows) == 0 {
		return `<p class="italic text-ink-muted">no ranked symbols in this repository</p>`
	}
	rows := make([]table.Row, 0, len(r.Rows))
	for _, row := range r.Rows {
		items := make([]chrome.DescItem, 0, len(row.Factors))
		for _, f := range row.Factors {
			items = append(items, chrome.DescItem{Term: fmt.Sprintf("%s %.2f", f.Name, f.Value), Detail: f.Cite})
		}
		factors := chrome.RenderDisclosure(chrome.Disclosure{Summary: "factors", Body: template.HTML(chrome.RenderDescriptionList(items))})
		rows = append(rows, table.Row{
			ID:        "risk-" + row.Symbol,
			Cells:     []string{"", "", fmt.Sprintf("%.3g", row.Risk)},
			HTMLCells: []templ.Component{templ.Raw(testsMono(row.Symbol)), templ.Raw(testsMono(row.File)), nil, templ.Raw(factors)},
		})
	}
	return renderKitTable(table.TableParams{
		TableID: "tests-risk", Caption: "Symbols by risk, highest first — open factors for each value and its citation",
		Columns: []table.Column{{Key: "symbol", Label: "Symbol"}, {Key: "file", Label: "File"}, {Key: "risk", Label: "Risk", Align: "right"}, {Key: "factors", Label: "Factors"}},
		Rows:    rows,
	})
}

func renderPortfolioRows(p *certify.Portfolio) string {
	if len(p.Tests) == 0 {
		return `<p class="italic text-ink-muted">no Test* functions in this repository's test files</p>`
	}
	rows := make([]table.Row, 0, len(p.Tests))
	for _, t := range p.Tests {
		redundant := "—"
		if len(t.RedundantWith) > 0 {
			redundant = strings.Join(t.RedundantWith, ", ")
		}
		rows = append(rows, table.Row{
			ID:        "test-" + t.Test,
			Cells:     []string{"", string(t.Class), t.Purpose, ""},
			HTMLCells: []templ.Component{templ.Raw(testsMono(t.Test)), nil, nil, templ.Raw(testsMono(redundant))},
		})
	}
	return renderKitTable(table.TableParams{
		TableID: "tests-portfolio", Caption: "Every test, its class and its purpose line",
		Columns: []table.Column{{Key: "test", Label: "Test"}, {Key: "class", Label: "Class"}, {Key: "purpose", Label: "Purpose"}, {Key: "redundant", Label: "Redundant with"}},
		Rows:    rows,
	}) + fmt.Sprintf(`<p class="mt-2 text-xs text-ink-muted">orphan: reaches nothing governed and nothing at or above the 20th-percentile centrality (%.2f)</p>`, p.CentralityP20)
}

func renderPortfolioGaps(p *certify.Portfolio) string {
	if len(p.Gaps) == 0 {
		return `<p class="text-ink">no gaps: every top-decile risk symbol sits in a file a live certificate covers, or nothing is ranked</p>`
	}
	rows := make([]table.Row, 0, len(p.Gaps))
	for _, g := range p.Gaps {
		rows = append(rows, table.Row{
			ID:        "gap-" + g.Symbol,
			Cells:     []string{"", "", fmt.Sprintf("%.3g", g.Risk)},
			HTMLCells: []templ.Component{templ.Raw(testsMono(g.Symbol)), templ.Raw(testsMono(g.File)), nil},
		})
	}
	return renderKitTable(table.TableParams{
		TableID: "tests-gaps", Caption: "Critical targets without a live certificate",
		Columns: []table.Column{{Key: "symbol", Label: "Symbol"}, {Key: "file", Label: "File"}, {Key: "risk", Label: "Risk", Align: "right"}},
		Rows:    rows,
	})
}
