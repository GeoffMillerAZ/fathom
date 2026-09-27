package main

// console_refactor.go — /refactors (OC-26, vision 35 §4.7, design §3
// "Refactor"): for one repository and a base ref (?svc=&since=), the
// conservation planes of `fathom conserve --since` as verdict chips — a
// plane that did not run reads "not run", never "conserved" — and the
// reading set, the changed functions only a human can vouch for, with the
// lines of each partial item no test ran shown as a code excerpt
// (code.RenderExcerpt).
//
// conserve takes seconds on a large repository (5.7 s on this one), so the
// page computes it in the background, keyed by the range's resolved
// commits and the index watermark, and waits a bounded time; a report still
// being made is said so, with every plane "not run" until it arrives.

import (
	"context"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/code"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/conserve"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

func init() {
	declarePage(consolePage{
		ID: "refactors", Route: "/refactors", Href: "/refactors", Group: "Proof", Order: 4, Nav: true,
		Title:    "Refactors",
		Question: "What did a change keep, what did it lose, and what is left for a human to read?",
		Verbs:    []string{"fathom conserve --since {since} --service {svc} --store {store} --json"},
		Build:    buildRefactorsPage,
	})
}

// consoleRefactorWait is how long a request waits for a conservation
// report before rendering "still being computed".
var consoleRefactorWait = 10 * time.Second

// defaultRefactorSince is the base a repository's refactor view starts at.
const defaultRefactorSince = "HEAD~1"

// refactorRefRe is the shape a base ref may take: a git revision, never an
// option (a leading "-") and never a shell word.
var refactorRefRe = regexp.MustCompile(`^[A-Za-z0-9._/~^@{}+-]{1,100}$`)

// The console's bounds on conserve (RT-118, design §14): each git read
// the page makes answers within consoleGitTimeout or is killed and named;
// a computation has consoleConserveDeadline; and a range changing more
// than consoleConserveMaxFiles files (`[console] conserve_max_files`) is
// refused by name — the CLI verb still runs it — never computed here.
var (
	consoleGitTimeout       = 10 * time.Second
	consoleConserveDeadline = 2 * time.Minute
	consoleConserveMaxFiles = defaultConserveMaxFiles
)

// defaultConserveMaxFiles is `[console] conserve_max_files`' default.
const defaultConserveMaxFiles = 500

// conserveCacheKeep bounds the finished reports kept (oldest dropped
// first); a report still being computed is never dropped.
const conserveCacheKeep = 16

// conserveEntry is one range's cached report.
type conserveEntry struct {
	key      string
	done     chan struct{}
	finished bool
	report   conserve.Report
	err      string
	at       time.Time
	job      *console.Job
}

var conserveCache = struct {
	sync.Mutex
	m map[string]*conserveEntry
}{m: map[string]*conserveEntry{}}

// resetConserveCache forgets every report (tests).
func resetConserveCache() {
	conserveCache.Lock()
	conserveCache.m = map[string]*conserveEntry{}
	conserveCache.Unlock()
}

// consoleGit runs one git read for a page under consoleGitTimeout; a git
// that does not answer in time is killed and named.
func consoleGit(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), consoleGitTimeout)
	defer cancel()
	out, err := gitro.Output(ctx, root, args...)
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("git %s in %s did not answer within %s and was stopped", args[0], root, consoleGitTimeout)
	}
	return strings.TrimSpace(string(out)), err
}

// refactorReport returns the range's report, computing it in the
// console's job pool (RT-118) and waiting up to budget; ok is false while
// it is pending. The range is one flight per key (its resolved commits
// and the index watermark); a newer key for the same service and base
// drops a queued older one.
func refactorReport(pc *consolePlans, sr serviceRoot, since string, budget time.Duration) (*conserveEntry, bool) {
	base, gerr := consoleGit(sr.Root, "rev-parse", "--verify", "--quiet", since+"^{commit}")
	if gerr != nil && strings.Contains(gerr.Error(), "did not answer") {
		return &conserveEntry{finished: true, err: gerr.Error(), at: time.Now()}, true
	}
	ctx := context.Background()
	key := strings.Join([]string{fmt.Sprintf("%p", pc.deps), sr.Service, sr.Root, since, base, pc.head(sr.Root), pc.storeWatermark(ctx, sr.Service)}, "|")
	conserveCache.Lock()
	e := conserveCache.m[key]
	if e == nil || (e.finished && e.err != "" && time.Since(e.at) >= consolePlansRetry) || conserveDropped(e) {
		// A failed report stands, with its reason, for consolePlansRetry;
		// then the next read computes it again.
		e = &conserveEntry{key: key, done: make(chan struct{})}
		ne := e
		job, err := pc.deps.jobPool().SubmitJob("conserve:"+sr.Service+"|"+since, key, func() { computeRefactorReport(pc, sr, since, ne) })
		if err != nil {
			conserveCache.Unlock()
			return &conserveEntry{finished: true, err: err.Error(), at: time.Now()}, true
		}
		e.job = job
		conserveCache.m[key] = e
		trimConserveCacheLocked()
	}
	conserveCache.Unlock()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case <-e.done:
		return e, true
	case <-timer.C:
		return e, false
	}
}

// conserveDropped reports whether the pool dropped e's job unrun (a newer
// key for the same range superseded it while queued).
func conserveDropped(e *conserveEntry) bool {
	if e == nil || e.job == nil || e.finished {
		return false
	}
	select {
	case <-e.job.Done():
		return e.job.Dropped()
	default:
		return false
	}
}

// trimConserveCacheLocked drops dropped flights and the oldest finished
// reports past conserveCacheKeep; a report being computed stays.
func trimConserveCacheLocked() {
	var finished []*conserveEntry
	for k, e := range conserveCache.m {
		switch {
		case conserveDropped(e):
			delete(conserveCache.m, k)
		case e.finished:
			finished = append(finished, e)
		}
	}
	if len(finished) <= conserveCacheKeep {
		return
	}
	sort.Slice(finished, func(i, j int) bool {
		if !finished[i].at.Equal(finished[j].at) {
			return finished[i].at.Before(finished[j].at)
		}
		return finished[i].key < finished[j].key
	})
	for _, e := range finished[:len(finished)-conserveCacheKeep] {
		delete(conserveCache.m, e.key)
	}
}

func computeRefactorReport(pc *consolePlans, sr serviceRoot, since string, e *conserveEntry) {
	var rep conserve.Report
	var err error
	if pc.deps.c == nil {
		err = errNoStore
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), consoleConserveDeadline)
		defer cancel()
		// RT-61 (§3.3): conserve is an in-process console call, admitted
		// and metered as console-inprocess.
		err = inProcessToolCall(ctx, "console:conserve", tenancy.ClassTree, func(ctx context.Context) error {
			var cerr error
			rep, cerr = conserveReportCapped(ctx, pc.deps.c.Store(), sr.Root, since, sr.Service, discoverPlanPaths(sr.Root), "", conserve.DefaultResidueMax, consoleConserveMaxFiles)
			return cerr
		})
	}
	conserveCache.Lock()
	if err != nil {
		e.err = err.Error()
	} else {
		e.report = rep
	}
	e.finished, e.at = true, time.Now()
	close(e.done)
	conserveCache.Unlock()
}

// refactorInput is a report as the projection reads it.
func refactorInput(sr serviceRoot, since string, e *conserveEntry, ok bool) console.RefactorInput {
	in := console.RefactorInput{Service: sr.Service, Since: since, ExcerptContext: 2}
	for _, p := range conserve.Planes() {
		in.Registered = append(in.Registered, p.Name())
	}
	switch {
	case !ok:
		in.Pending = true
		return in
	case e.err != "":
		in.Err = e.err
		return in
	}
	rep := e.report
	in.Ran, in.Base, in.Head = true, rep.Base, rep.Head
	for _, pl := range rep.Planes {
		cp := console.ConservePlane{Plane: pl.Plane, Verdict: pl.Verdict, Missing: pl.Missing, Findings: []console.ConserveFinding{}}
		for _, f := range pl.Findings {
			cp.Findings = append(cp.Findings, console.ConserveFinding{Plane: f.Plane, Kind: f.Kind, Key: f.Key, Detail: f.Detail, Declared: f.Declared, Remedy: f.Remedy, File: f.File, Line: f.Line, Informational: f.Informational()})
		}
		in.Planes = append(in.Planes, cp)
	}
	if r := rep.Residue; r != nil {
		res := &console.Residue{
			Changed: r.Changed, Attested: r.Attested, Unattested: r.Unattested, Unreached: r.Unreached,
			Partial: r.Partial, Unexecuted: r.Unexecuted, UnanalyzedFunctions: r.UnanalyzedFunctions, ClassedByExecution: r.ClassedByExecution,
			Truncated: r.Truncated, Unanalyzed: r.Unanalyzed, Missing: r.Missing, CoverageUnusable: r.CoverageUnusable,
		}
		in.Sources = map[string]string{}
		files := verify.DirFiles(sr.Root)
		for _, it := range r.Items {
			res.Items = append(res.Items, console.ResidueItem{
				Symbol: it.Symbol, File: it.File, Line: it.Line, Change: it.Change, Class: it.Class,
				Tests: it.Tests, Callers: it.Callers, Evidence: it.Evidence, UnexecutedLines: it.UnexecutedLines,
			})
			if it.Class == "partial" {
				if _, seen := in.Sources[it.File]; !seen {
					if raw, err := files.ReadRepoFile(it.File); err == nil {
						in.Sources[it.File] = string(raw)
					}
				}
			}
		}
		in.Residue = res
	}
	return in
}

// refactorsModel is /refactors' JSON model.
type refactorsModel struct {
	console.RefactorPage
	Repositories []string `json:"repositories"`
}

func buildRefactorsPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	roots, scope, err := plansScope(cr, m)
	if err != nil {
		return consoleOut{}, err
	}
	var names []string
	for _, sr := range modelServiceRoots(m) {
		names = append(names, sr.Service)
	}
	if scope == "" {
		model := refactorsModel{Repositories: names, RefactorPage: console.RefactorPage{State: console.StateAbsent, Reason: "choose a repository and a base ref", Planes: []console.PlaneChip{}, Findings: []console.ConserveFinding{}}}
		return consoleOut{
			Meta: []string{repositories(len(names)) + " in the registry"}, Model: model, Verbs: []string{},
			Body: func() string { return renderRefactorChooser(names, "", defaultRefactorSince) },
		}, nil
	}
	sr := roots[0]
	since := cr.r.URL.Query().Get("since")
	if since == "" {
		since = defaultRefactorSince
	}
	if !refactorRefRe.MatchString(since) || strings.HasPrefix(since, "-") {
		return consoleOut{}, &consoleError{
			Status: http.StatusBadRequest, Title: "Unusable base ref",
			Reason: "since=" + since + " is not a git revision this page will pass to git (letters, digits and . _ / ~ ^ @ { } + -, not starting with -).",
			Remedy: "fathom conserve --since HEAD~1 --service " + sr.Service + cr.deps.storeArg(), RemedyHref: "/refactors?svc=" + url.QueryEscape(sr.Service), RemedyLabel: "Compare with the last commit",
		}
	}
	e, ok := refactorReport(plansFor(cr.deps), sr, since, consoleRefactorWait)
	view := console.RefactorView(refactorInput(sr, since, e, ok))
	verb := "fathom conserve --since " + since + " --service " + sr.Service + cr.deps.storeArg() + " --json"
	sections := map[string]any{}
	if ok && e.err == "" {
		sections[verb] = e.report
	}
	meta := []string{sr.Service, "since " + since}
	if view.Base != "" {
		meta = append(meta, view.Base+" .. "+view.Head)
	}
	return consoleOut{
		Title:  "Refactor · " + sr.Service + " since " + since,
		Crumbs: []chrome.Crumb{{Label: "Refactors", Href: "/refactors"}},
		Meta:   meta, Model: refactorsModel{RefactorPage: view, Repositories: names},
		Sections: sections, Verbs: []string{verb},
		Body: func() string { return renderRefactorPage(view, names, sr, since, verb) },
	}, nil
}

// renderRefactorChooser is the repository and base-ref form (GET, so the
// choice lives in the URL).
func renderRefactorChooser(names []string, current, since string) string {
	opts := make([]form.Option, 0, len(names))
	for _, n := range names {
		opts = append(opts, form.Option{Value: n, Label: n})
	}
	sel := form.RenderField(form.Field{Label: "Repository", For: "refactor-svc", Control: template.HTML(form.RenderSelect(form.Select{ID: "refactor-svc", Name: "svc", Value: current, Placeholder: "Choose a repository", Options: opts, Required: true}))})
	ref := form.RenderField(form.Field{Label: "Base ref", For: "refactor-since", Help: "A git revision: HEAD~1 is the last commit; a tag or a SHA works too.", Control: template.HTML(form.RenderInput(form.Input{ID: "refactor-since", Name: "since", Value: since, Required: true, HasHelp: true}))})
	btn := form.RenderButton(form.Button{Label: "Compare", Variant: form.ButtonPrimary})
	// The kit's select, input and button are 36–38 px tall: the form lifts
	// them to 44 px targets (design §2; FATHOM-C-KIT-05).
	return `<form method="get" action="/refactors" class="mb-4 grid max-w-3xl gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-start [&_select]:min-h-11 [&_input]:min-h-11 [&_button]:min-h-11" data-refactor-form>` + sel + ref + `<div class="sm:pt-7">` + btn + `</div></form>`
}

func renderRefactorPage(v console.RefactorPage, names []string, sr serviceRoot, since, verb string) string {
	var b strings.Builder
	b.WriteString(renderRefactorChooser(names, sr.Service, since))

	var chips strings.Builder
	chips.WriteString(`<ul class="grid gap-2 sm:grid-cols-2 lg:grid-cols-4" data-plane-chips>`)
	for _, c := range v.Planes {
		detail := c.Reason
		if detail == "" && c.Verdict == "conserved" {
			detail = "checked, nothing lost"
		}
		chips.WriteString(`<li class="min-w-0 rounded-md border border-line px-3 py-2 ` + planeSurface[c.Role] + `" data-plane="` + html.EscapeString(c.Plane) + `" data-verdict="` + html.EscapeString(c.Verdict) + `">` +
			`<p class="text-sm font-semibold text-ink">` + html.EscapeString(c.Plane) + `</p>` +
			`<p class="text-sm font-medium ` + planeInk[c.Role] + `">` + html.EscapeString(c.Verdict) + `</p>`)
		if detail != "" {
			chips.WriteString(`<p class="text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(detail) + `</p>`)
		}
		chips.WriteString(`</li>`)
	}
	chips.WriteString(`</ul>`)
	if !strings.Contains(verb, "--json") || v.State != console.StatePresent {
		chips.WriteString(`<p class="mt-3 text-sm text-ink-muted">Run <code class="font-mono text-xs">` + html.EscapeString(strings.TrimSuffix(verb, " --json")) + `</code> from the repository to fill these in.</p>`)
	}
	b.WriteString(renderPanel(consolePanel{
		ID: "planes", Title: "Conservation planes", SourceKey: "index:" + sr.Service,
		Question:    "What did this change have to keep, and did it keep it? A plane that did not run is shown as not run, never as conserved.",
		Verb:        verb,
		Source:      "fathom conserve over " + since + "..the working tree, every plane this build registers",
		Denominator: plural(len(v.Planes), "plane") + " (ADR-044's set and every registered one)",
		Freshness:   "as rendered; recomputed when the range, HEAD or the index moves",
		State:       v.State,
		Reason:      v.Reason,
		Body:        chips.String(),
	}))

	if len(v.Findings) > 0 {
		rows := make([]table.Row, 0, len(v.Findings))
		for i, f := range v.Findings {
			where := f.File
			if f.Line > 0 {
				where += fmt.Sprintf(":%d", f.Line)
			}
			rows = append(rows, table.Row{
				ID:        fmt.Sprintf("finding-%d", i),
				Cells:     []string{f.Plane, f.Kind, f.Key, f.Declared, where, f.Remedy},
				HTMLCells: []templ.Component{nil, nil, templ.Raw(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(f.Key) + `</span>`)},
			})
		}
		b.WriteString(`<div class="mt-4">`)
		b.WriteString(renderPanel(consolePanel{
			ID: "findings", Title: "Findings", Verb: verb,
			Question:    "What did the planes find, and is each one excused by a declaration made before the change?",
			Source:      "the planes' findings; only a declaration present at the base (prior) excuses a loss",
			Denominator: plural(len(v.Findings), "finding"),
			Body: renderKitTable(table.TableParams{TableID: "findings", Caption: "Every finding, by plane, kind and key",
				Columns: []table.Column{{Key: "plane", Label: "Plane"}, {Key: "kind", Label: "Kind"}, {Key: "key", Label: "Key"}, {Key: "declared", Label: "Declared"}, {Key: "where", Label: "Where"}, {Key: "remedy", Label: "Remedy"}},
				Rows:    rows}),
		}))
		b.WriteString(`</div>`)
	}

	b.WriteString(`<div class="mt-4">`)
	b.WriteString(renderPanel(consolePanel{
		ID: "reading-set", Title: "The reading set", SourceKey: "index:" + sr.Service,
		Question:    "Which changed functions need a human to read them? Everything else ran under a passing run bound to this commit.",
		Verb:        verb,
		Source:      "conserve's residue: changed functions classed by execution evidence, else static reach",
		Denominator: valuePlain(v.Residue.Changed) + " changed functions",
		State:       v.Residue.State,
		Reason:      v.Residue.Reason,
		Body:        renderReadingSet(v.Residue),
	}))
	b.WriteString(`</div>`)
	return b.String()
}

// planeSurface and planeInk color a plane chip by its verdict role
// (complete literals for the CSS build); the verdict is always also words.
var planeSurface = map[string]string{
	"positive": "bg-positive-soft", "danger": "bg-danger-soft", "caution": "bg-caution-soft", "ink-muted": "bg-surface-sunken",
}

var planeInk = map[string]string{
	"positive": "text-positive", "danger": "text-danger", "caution": "text-caution", "ink-muted": "text-ink-muted",
}

func renderReadingSet(r console.ResidueView) string {
	var b strings.Builder
	switch {
	case r.State == console.StatePresent:
		b.WriteString(stat.RenderBand([]stat.Stat{
			valueStat("changed functions", r.Changed),
			valueStat("attested — a passing run vouches for it", r.Attested),
			valueStat("unattested — reached, no attesting run", r.Unattested),
			valueStat("partial — some changed lines never ran", r.Partial),
			valueStat("unexecuted — never ran", r.Unexecuted),
			valueStat("unreached by any test", r.Unreached),
		}, stat.BandOptions{AriaLabel: "the reading set's classes"}))
	case r.Changed.Known():
		// The classes are not measured (the panel says why, once); the
		// changed functions still are.
		b.WriteString(`<div class="max-w-sm">` + stat.Render(valueStat("changed functions", r.Changed)) + `</div>`)
	}
	if r.Coverage != "" {
		b.WriteString(`<p class="mt-2 text-sm text-caution">coverage unusable: ` + html.EscapeString(r.Coverage) + ` — classes fall back to static reach</p>`)
	}
	if len(r.Items) > 0 {
		rows := make([]table.Row, 0, len(r.Items))
		for i, it := range r.Items {
			where := fmt.Sprintf("%s:%d", it.File, it.Line)
			tests := "no reaching test"
			if len(it.Tests) > 0 {
				tests = strings.Join(it.Tests, ", ")
			}
			rows = append(rows, table.Row{
				ID:    fmt.Sprintf("item-%d", i),
				Cells: []string{it.Class, it.Symbol, where, it.Evidence, tests},
				HTMLCells: []templ.Component{nil,
					templ.Raw(`<span class="font-mono text-xs font-medium [overflow-wrap:anywhere]">` + html.EscapeString(it.Symbol) + `</span>`),
					templ.Raw(`<span class="font-mono text-xs [overflow-wrap:anywhere]">` + html.EscapeString(where) + `</span>`)},
			})
		}
		caption := fmt.Sprintf("%s in the reading set", plural(len(r.Items), "function"))
		if r.Truncated > 0 {
			caption += fmt.Sprintf("; %d more cut by the report's item bound", r.Truncated)
		}
		b.WriteString(`<div class="mt-3">` + renderKitTable(table.TableParams{TableID: "reading-set", Caption: caption,
			Columns: []table.Column{{Key: "class", Label: "Class"}, {Key: "function", Label: "Function"}, {Key: "where", Label: "Where"}, {Key: "evidence", Label: "Evidence"}, {Key: "tests", Label: "Reaching tests"}},
			Rows:    rows}) + `</div>`)
		for i, it := range r.Items {
			if it.Class != "partial" {
				continue
			}
			b.WriteString(`<section class="mt-4" aria-label="` + html.EscapeString(it.Symbol) + ` — changed lines that never ran">`)
			b.WriteString(`<h3 class="text-sm font-semibold text-ink"><span class="font-mono">` + html.EscapeString(it.Symbol) + `</span> — changed lines that never ran</h3>`)
			if it.Excerpt == nil {
				b.WriteString(`<p class="mt-1 text-sm italic text-ink-muted">` + html.EscapeString(it.ExcerptReason) + `</p></section>`)
				continue
			}
			lines := make([]code.Line, 0, len(it.Excerpt))
			for _, l := range it.Excerpt {
				st := code.LineNormal
				if l.Unexecuted {
					st = code.LineNotExecuted
				}
				lines = append(lines, code.Line{Number: l.Number, Text: l.Text, State: st})
			}
			excerpt, err := code.RenderExcerpt(code.Excerpt{
				Label:        fmt.Sprintf("%s, lines %d to %d", it.File, lines[0].Number, lines[len(lines)-1].Number),
				Caption:      it.File,
				AnchorPrefix: fmt.Sprintf("item%d-", i),
				Lines:        lines,
				Labels:       code.StateLabels{NotExecuted: "Changed, never ran"},
			})
			if err != nil {
				excerpt = `<p class="text-danger">` + html.EscapeString(err.Error()) + `</p>`
			}
			b.WriteString(`<div class="mt-2">` + excerpt + `</div>`)
			b.WriteString(`<p class="mt-1 text-xs text-ink-muted">unexecuted changed lines: ` + html.EscapeString(strings.Join(it.UnexecutedLines, ", ")) + `</p></section>`)
		}
	}
	if len(r.Unanalyzed) > 0 {
		b.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{
			Summary: plural(len(r.Unanalyzed), "changed file") + " the index does not analyze (docs, config, generated)",
			Body:    template.HTML(`<ul class="font-mono text-xs [overflow-wrap:anywhere]">` + listItems(r.Unanalyzed) + `</ul>`),
		}) + `</div>`)
	}
	return b.String()
}

func listItems(items []string) string {
	var b strings.Builder
	for _, it := range items {
		b.WriteString(`<li>` + html.EscapeString(filepath.ToSlash(it)) + `</li>`)
	}
	return b.String()
}
