package main

// console_planrow.go — one plan row's evidence, /plans/{svc}/{plan}/rows/{id}
// (OC-23): the contract as authored, the gate's verdict criterion by
// criterion (`fathom verify --row`), the call path the reach claim stands on
// or the line a lexical oracle matched, and the row's history. Its
// ?format=fragment is the body of the plan page's drawer.

import (
	"context"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/code"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/fathom/internal/analysis/callgraph"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func init() {
	declarePage(consolePage{
		ID: "plan-row", Route: "/plans/{svc}/{plan}/rows/{id}", Group: "Proof", Order: 2, NavParent: "plans",
		Title:    "Plan row",
		Question: "What evidence does the gate use for this row, and what has happened to it?",
		Verbs:    []string{"fathom verify --service {svc} --plan {plan} --row {id} --json"},
		Formats:  []string{"fragment"},
		Build:    buildPlanRowPage,
	})
}

func buildPlanRowPage(cr *consoleRequest) (consoleOut, error) {
	sr, plan, err := resolvePlanRequest(cr, consolePlansPlanWait)
	if err != nil {
		return consoleOut{}, err
	}
	id := cr.r.PathValue("id")
	pc := plansFor(cr.deps)
	var row console.PlanRowFact
	found := false
	for _, r := range plan.Rows {
		if r.ID == id {
			row, found = r, true
		}
	}
	if !found {
		return consoleOut{}, &consoleError{
			Status: http.StatusNotFound, Title: "Unknown row",
			Reason: "No row " + id + " in " + plan.Rel + ".",
			Remedy: "fathom plans next --service " + sr.Service + " --plan " + plan.Rel, RemedyHref: planHref(sr.Service, plan.Slug()), RemedyLabel: "Open the plan",
		}
	}
	in := console.RowEvidenceInput{Service: sr.Service, Plan: plan, RowID: id, MaxTests: 20}
	contract := console.ContractOf(row)
	if (contract.Kind == console.ContractReach || contract.Kind == console.ContractCertified) && row.Symbol != "" {
		in.Reach = rowReach(cr.r.Context(), pc, sr.Service, row)
	}
	if contract.Kind == console.ContractLexical {
		m := rowOracle(sr.Root, row)
		in.Oracle = &m
	}
	revs, _, note := pc.history(sr.Root, plan.Rel)
	in.History, in.HistoryNote = rowRevisions(sr.Service, id, revs), note
	view := console.RowEvidence(in)

	verb := "fathom verify --service " + sr.Service + " --plan " + plan.Rel + " --row " + id + " --json"
	sections := map[string]any{}
	if e, ok := pc.entry(plan.Path); ok && e.err == "" && plan.Verified {
		if narrowed, ok := narrowReportsToRow([]verify.Report{e.result.Report}, id); ok {
			sections[verb] = verifyJSONDoc(narrowed, verifyGateFailures(narrowed, false, false))
		}
	}
	pre, preWhy := buildPlanRowPreflight(cr, sr, plan, row)
	body := func() string {
		return renderRowEvidence(view, verb, "plans:"+sr.Service) + `<div class="mt-4">` + renderPreflightPanel(pre, preWhy, sr, plan, row, "plans:"+sr.Service) + `</div>`
	}
	title := id + " · " + row.Title
	return consoleOut{
		Title: title,
		Crumbs: []chrome.Crumb{
			{Label: "Plans", Href: "/plans"},
			{Label: sr.Service, Href: "/plans?svc=" + url.QueryEscape(sr.Service)},
			{Label: plan.Title, Href: planHref(sr.Service, plan.Slug())},
			// The row's own crumb is its id: the heading carries the title,
			// and a sentence-long last crumb wrapped under the trail.
			{Label: id},
		},
		Meta:  []string{fmt.Sprintf("%s:%d", plan.Rel, row.Line), stateBadge[view.State].Label, view.Contract.Kind},
		Model: planRowModel{RowEvidenceView: view, Preflight: pre, PreflightErr: preWhy}, Sections: sections, Verbs: []string{verb},
		Body: body,
		// The drawer's fragment is a person opening this row's evidence
		// over the plan page: a view of this page (UX-M4 reads it).
		RawIsView: true,
		Raw: func(w http.ResponseWriter) bool {
			if cr.r.URL.Query().Get("format") != "fragment" {
				return false
			}
			// The drawer's fragment: the same panels, not live regions (the
			// plan page around it follows the plan's source already).
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(renderRowEvidence(view, verb, "")))
			return true
		},
	}, nil
}

// rowReach is the call graph's answer for a row's symbol: every test that
// reaches it and the call path from the row's reachable_from test (or the
// first reaching test) — on the verify engine's own graph.
func rowReach(ctx context.Context, pc *consolePlans, service string, row console.PlanRowFact) *console.ReachFacts {
	f := &console.ReachFacts{Symbol: row.Symbol, Tests: []console.ReachTest{}}
	g, err := pc.graph(ctx, service)
	if err != nil {
		f.Err = err.Error()
		return f
	}
	query := row.Symbol
	if pc.deps.c != nil {
		if nodes, err := pc.deps.c.FindSymbol(ctx, service, row.Symbol); err == nil {
			keys := map[string]bool{}
			for _, n := range nodes {
				if n.Kind == core.KindDocChunk {
					continue
				}
				if row.Package != "" && !strings.HasPrefix(n.FilePath, strings.TrimSuffix(row.Package, "/")+"/") {
					continue
				}
				keys[n.Key()] = true
			}
			if len(keys) == 1 {
				for k := range keys {
					query = k
				}
			}
		}
	}
	for _, d := range callgraph.TestImpactDetail(g, query) {
		f.Tests = append(f.Tests, console.ReachTest{Test: d.Key, ViaDispatch: d.ViaDispatch, Implementers: d.Implementers})
	}
	sort.SliceStable(f.Tests, func(i, j int) bool { return f.Tests[i].Test < f.Tests[j].Test })
	f.From = row.ReachableFrom
	if f.From == "" && len(f.Tests) > 0 {
		f.From = f.Tests[0].Test
	}
	if f.From != "" {
		f.Path = callgraph.CallPath(g, f.From, query)
	}
	return f
}

// rowOracle evaluates a lexical row against the service's working tree.
func rowOracle(root string, row console.PlanRowFact) console.OracleMatch {
	raw, err := verify.DirFiles(root).ReadRepoFile(row.OraclePath)
	if err != nil {
		// A file that is not there is the gate's path-absent — a measured
		// nothing — not a file that could not be read.
		return console.OracleMatch{Path: row.OraclePath, Pattern: row.OraclePattern, Err: err.Error(), Missing: errors.Is(err, fs.ErrNotExist)}
	}
	return console.MatchOracle(row.OraclePath, row.OraclePattern, string(raw), 2)
}

// trimServiceKey shows a canonical key without its service prefix.
func trimServiceKey(k string) string {
	if _, rest, ok := strings.Cut(k, "::"); ok {
		return rest
	}
	return k
}

// renderRowEvidence is the row page's body and the drawer's fragment; live
// names the source its panels follow ("" for the fragment).
func renderRowEvidence(v console.RowEvidenceView, verb, live string) string {
	var b strings.Builder
	b.WriteString(`<div class="flex flex-col gap-4" data-row-evidence data-row-id="` + html.EscapeString(v.Row.ID) + `">`)

	items := []chrome.DescItem{{Term: "kind", Detail: v.Contract.Kind}}
	for _, f := range v.Contract.Fields {
		items = append(items, chrome.DescItem{Term: f.Name, DetailHTML: template.HTML(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(f.Value) + `</span>`)})
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "row-contract", SourceKey: live, Title: "Contract",
		Question: "What does the gate check for this row?",
		Source:   fmt.Sprintf("%s, line %d", v.Rel, v.Row.Line),
		Body:     chrome.RenderDescriptionList(items),
	}))

	var crit strings.Builder
	if v.Verdict.Reason != "" {
		crit.WriteString(`<p class="mb-2 text-ink-muted">` + html.EscapeString(v.Verdict.Reason) + `</p>`)
	}
	badge := stateBadge[v.State]
	crit.WriteString(`<p class="mb-2"><span class="inline-flex items-center rounded-full border border-line px-2 py-0.5 text-xs font-medium text-ink ` + badgeSurface[badge.Role] + `">` + html.EscapeString(badge.Label) + `</span>`)
	if v.Verdict.Label != "" {
		crit.WriteString(` <span class="font-mono text-xs text-ink-muted">` + html.EscapeString(v.Verdict.Label) + `</span>`)
	}
	crit.WriteString(`</p>`)
	if len(v.Verdict.Criteria) > 0 {
		crit.WriteString(`<ul class="flex flex-col gap-1">`)
		for _, c := range v.Verdict.Criteria {
			role := "text-ink-muted"
			switch c.Status {
			case "pass":
				role = "text-positive"
			case "fail":
				role = "text-danger"
			case "unknown":
				role = "text-caution"
			}
			crit.WriteString(`<li class="[overflow-wrap:anywhere]"><span class="font-mono text-xs ` + role + `">` + html.EscapeString(c.Status) + `</span> <span class="font-medium">` + html.EscapeString(c.Name) + `</span> — ` + html.EscapeString(c.Detail) + `</li>`)
		}
		crit.WriteString(`</ul>`)
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "row-verdict", SourceKey: live, Title: "Verdict", Verb: verb,
		Question: "What did the gate conclude, criterion by criterion?",
		Source:   "the verify engine at the current index (verify_plan), the working tree's lexical oracles",
		Body:     crit.String(),
	}))

	if v.Reach.State != console.StateAbsent || v.Contract.Kind == console.ContractReach {
		b.WriteString(renderPanel(consolePanel{
			ID: "row-reach", SourceKey: live, Title: "Reach path",
			Question:    "Which test reaches the claimed symbol, and through what calls?",
			Source:      "the index's call graph (syntax edges plus the typed pass — the graph the gate walks), callgraph.TestImpactDetail and CallPath",
			State:       v.Reach.State,
			Reason:      v.Reach.Reason,
			Denominator: plural(v.Reach.Total, "reaching test"),
			Body:        renderReach(v.Reach),
		}))
	}
	if v.Contract.Kind == console.ContractLexical {
		b.WriteString(renderPanel(consolePanel{
			ID: "row-oracle", SourceKey: live, Title: "Oracle match",
			Question: "Where in the working tree does the row's pattern match?",
			Source:   "the working tree at the repository's root (the gate's lexical oracle reads the same file)",
			State:    v.Oracle.State,
			Reason:   v.Oracle.Reason,
			Body:     renderOracle(v.Oracle, v.Row.ID),
		}))
	}

	var hist strings.Builder
	if v.History.State == console.StatePresent {
		items := []chrome.DescItem{
			{Term: "added", Detail: orDash(v.History.Added)},
			{Term: "first claimed", Detail: orDash(v.History.FirstClaimed)},
			{Term: "reopened", Detail: plural(v.History.Reopened, "time")},
		}
		hist.WriteString(chrome.RenderDescriptionList(items))
		if len(v.History.Changes) > 0 {
			hist.WriteString(`<ol class="mt-2 flex flex-col gap-1 text-sm">`)
			for _, c := range v.History.Changes {
				hist.WriteString(`<li><span class="font-mono text-xs">` + html.EscapeString(c.Commit) + `</span> <span class="text-ink-muted">` + html.EscapeString(c.At) + `</span> ` + html.EscapeString(c.From) + ` → ` + html.EscapeString(c.To) + `</li>`)
			}
			hist.WriteString(`</ol>`)
		}
	}
	hist.WriteString(`<div class="mt-3 max-w-sm">` + stat.Render(valueStat("refuted by the gate", v.History.Refutations)) + `</div>`)
	b.WriteString(renderPanel(consolePanel{
		ID: "row-history", SourceKey: live, Title: "History",
		Question: "When was this row added and claimed, and has it been reopened?",
		Source:   "the plan file's git history",
		State:    v.History.State,
		Reason:   v.History.Reason,
		Body:     hist.String(),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

// badgeSurface are the soft surfaces a state badge sits on (complete
// literals for the CSS build).
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var badgeSurface = map[string]string{
	"positive-soft": "bg-positive-soft", "info-soft": "bg-info-soft", "danger-soft": "bg-danger-soft",
	"caution-soft": "bg-caution-soft", "accent-soft": "bg-accent-soft", "surface-sunken": "bg-surface-sunken",
}

func renderReach(r console.ReachView) string {
	var b strings.Builder
	if len(r.Path) > 0 {
		b.WriteString(`<ol class="flex flex-wrap items-center gap-x-2 gap-y-1 font-mono text-xs" aria-label="call path from the test to the symbol" data-reach-path>`)
		for i, step := range r.Path {
			if i > 0 {
				b.WriteString(`<li aria-hidden="true" class="text-ink-muted">→</li>`)
			}
			b.WriteString(`<li class="rounded-md border border-line bg-surface-sunken px-2 py-1 [overflow-wrap:anywhere]">` + html.EscapeString(trimServiceKey(step)) + `</li>`)
		}
		b.WriteString(`</ol><p class="mt-1 text-xs text-ink-muted">` + html.EscapeString(plural(r.Hops, "hop")+" from "+trimServiceKey(r.From)) + `</p>`)
	}
	if len(r.Tests) > 0 {
		b.WriteString(`<h3 class="mt-3 text-sm font-semibold text-ink">Tests that reach ` + html.EscapeString(r.Symbol) + `</h3><ul class="mt-1 flex flex-col gap-0.5 font-mono text-xs">`)
		for _, t := range r.Tests {
			note := ""
			if t.ViaDispatch {
				note = fmt.Sprintf(` <span class="font-sans text-caution">via interface dispatch (%d implementers)</span>`, t.Implementers)
			}
			b.WriteString(`<li class="[overflow-wrap:anywhere]">` + html.EscapeString(trimServiceKey(t.Test)) + note + `</li>`)
		}
		b.WriteString(`</ul>`)
		if r.Omitted > 0 {
			b.WriteString(`<p class="mt-1 text-xs text-ink-muted">and ` + plural(r.Omitted, "more test") + `</p>`)
		}
	}
	return b.String()
}

func renderOracle(o console.OracleView, id string) string {
	where := `<p class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(o.Path) + ` · pattern ` + html.EscapeString(o.Pattern) + `</p>`
	// The count and the row's bound, in words: "matches nothing" beside a
	// proven verdict is the row's "exactly 0", never a contradiction.
	bound := ""
	if o.BoundMet != nil && o.State != console.StateError {
		role := "text-positive"
		if !*o.BoundMet {
			role = "text-danger"
		}
		bound = `<p class="mb-2 text-sm [overflow-wrap:anywhere] ` + role + `" data-oracle-bound>` + html.EscapeString(o.Reason) + `</p>`
	}
	if o.State != console.StatePresent {
		return bound + where
	}
	lines := make([]code.Line, 0, len(o.Lines))
	for i, t := range o.Lines {
		n := o.FirstLine + i
		st := code.LineNormal
		if n == o.Line {
			st = code.LineHighlighted
		}
		lines = append(lines, code.Line{Number: n, Text: t, State: st})
	}
	excerpt, err := code.RenderExcerpt(code.Excerpt{
		Label:        fmt.Sprintf("%s, lines %d to %d", o.Path, o.FirstLine, o.FirstLine+len(o.Lines)-1),
		Caption:      o.Path,
		AnchorPrefix: "oracle-" + anchorSafe(id) + "-",
		Lines:        lines,
		Labels:       code.StateLabels{Highlighted: "First match"},
	})
	if err != nil {
		return `<p class="text-danger">` + html.EscapeString(err.Error()) + `</p>`
	}
	return bound + `<p class="mb-2 text-sm">pattern <code class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(o.Pattern) + `</code> matches ` + plural(o.Count, "time") + `; first at line ` + strconv.Itoa(o.Line) + `</p>` + excerpt
}

// anchorSafe keeps the characters an excerpt anchor prefix allows.
func anchorSafe(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}
