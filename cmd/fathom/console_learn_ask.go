package main

// console_learn_ask.go — Ask the right way at /learn/ask (OC-47; vision 35
// §3 Learn, design §3 Learn). Question shapes mapped to the agent-surface
// tool that answers them (console.AskGuide: the stanza's SB-7 routing and
// the decision port's DP-9 route site), how often each was asked on the
// chosen project, and a try-it per shape: ?try=<shape>&q=<input> makes ONE
// call of that tool on this daemon's agent surface (/rpc/agent), the same
// call an agent makes, and projects its answer (console.AskAnswerFrom).
//
// Nothing new is exposed and nothing is mutated: every tool the guide
// teaches is a read on the agent surface already (the test holds the list
// to mcp.AgentSurfaceTools), the call carries the console's origin so the
// call log never counts it as usage, and the gate's own try-it verifies a
// row without writing the verify ledger (only `fathom verify` appends).
// The page's verb is `fathom harness matrix --json`: how each assistant
// reaches these tools, with the `mcp add` line naming this daemon.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func init() {
	declarePage(consolePage{
		ID: "learn-ask", Route: "/learn/ask", Href: "/learn/ask", Group: "Learn", Order: 10, Nav: true,
		Title:    "Ask the right way",
		Question: "Which tool answers which question — and what does it answer on this project?",
		Verbs:    []string{"fathom harness matrix --json"},
		Shows:    []console.MetricID{console.MetricAgentTools},
		Build:    buildLearnAskPage,
	})
}

// askTryTimeout bounds one try-it: the gate's first call at a new store
// watermark builds the call graph (O33), which can take seconds.
const askTryTimeout = 30 * time.Second

// askModel is /learn/ask's JSON projection.
type askModel struct {
	Project  string          `json:"project"`
	Chosen   string          `json:"chosen"`
	Projects []string        `json:"projects"`
	Guide    console.AskView `json:"guide"`
	// AskedVerb is the exact command whose output the "asked here" column
	// is: the project's traffic by tool.
	AskedVerb string `json:"asked_verb,omitempty"`
}

func buildLearnAskPage(cr *consoleRequest) (consoleOut, error) {
	m := cr.model()
	choice, err := chooseLearnProject(cr, m)
	if err != nil {
		return consoleOut{}, err
	}
	q := cr.r.URL.Query()
	var shape console.AskShape
	try := q.Get("try")
	if try != "" {
		s, ok := console.AskShapeByID(try)
		if !ok {
			var ids []string
			for _, s := range console.AskShapes() {
				ids = append(ids, s.ID)
			}
			return consoleOut{}, &consoleError{
				Status: http.StatusBadRequest, Title: "Unknown question shape",
				Reason: "No question shape is named " + try + ".",
				Remedy: "ask for one of: ?try=" + strings.Join(ids, ", ?try="), RemedyHref: "/learn/ask", RemedyLabel: "Open Ask the right way",
			}
		}
		shape = s
	}

	shared := pageMetrics(cr.page, consoleModelMetrics(m))
	facts := learnFacts{traffic: learnTraffic{Total: console.Absent[int](choice.How), Window: "the last 30 days"}}
	samples := map[string]console.AskSample{}
	if choice.Service != "" {
		facts = learnFacts{traffic: readLearnTraffic(cr, choice.Service)}
		samples = cachedAskSamples(cr, choice)
	}
	view := console.AskGuide(console.AskFacts{
		Service: choice.Service, AgentSurface: mcp.AgentSurfaceTools(), AgentTools: shared[console.MetricAgentTools],
		Calls: facts.traffic.Total, ToolCalls: facts.traffic.ByTool, Window: facts.traffic.Window, Samples: samples,
	})
	if try != "" {
		for _, s := range view.Shapes {
			if s.ID == shape.ID {
				shape = s // with this project's sample
			}
		}
		input := shape.Sample
		if q.Has("q") {
			input = q.Get("q")
		}
		ans := askTry(cr, choice, shape, input)
		view.Answer = &ans
	}

	out := consoleOut{
		Meta:    learnMeta(choice, consoleModelMetrics(m)[console.MetricServices], m),
		Actions: learnProjectPicker(cr.r.URL.Path, choice, m),
		Model: askModel{
			Project: choice.Service, Chosen: choice.How, Projects: learnProjectNames(m),
			Guide: view, AskedVerb: facts.traffic.Verb,
		},
		Metrics: shared,
		Sections: map[string]any{
			"fathom harness matrix --json": harness.AdoptionRows(cr.deps.selfURL, harnessMatrixNow()),
		},
		Body: func() string { return renderLearnAsk(choice, view, facts, cr.deps.selfURL, m) },
	}
	return out, nil
}

// askTry makes one try-it: the input checked, then one call of the shape's
// tool on this daemon's agent surface, its answer projected.
func askTry(cr *consoleRequest, choice learnChoice, shape console.AskShape, input string) console.AskAnswer {
	if choice.Service == "" {
		return console.AskNotAsked(shape, "", input, "no project to ask about: "+choice.How)
	}
	if problem := console.AskInputProblem(shape, input); problem != "" {
		if shape.Sample == "" && shape.SampleError != "" && strings.TrimSpace(input) == "" {
			// No example because its source could not be read: an error,
			// never a refusal that reads as "nothing to ask about".
			return console.AskUnreadable(shape, choice.Service, input, problem+" — "+shape.SampleNote+": "+shape.SampleError)
		}
		if shape.Sample == "" && shape.SampleNote != "" && strings.TrimSpace(input) == "" {
			problem += " (" + shape.SampleNote + ")"
		}
		return console.AskNotAsked(shape, choice.Service, input, problem)
	}
	ctx, cancel := context.WithTimeout(cr.r.Context(), askTryTimeout)
	defer cancel()
	raw, err := askToolCall(ctx, cr.deps.selfURL, shape.Tool, console.AskArgs(shape, choice.Service, input))
	return console.AskAnswerFrom(shape, choice.Service, input, raw, err)
}

// askToolCall is one tools/call on this daemon's agent surface, carrying
// the console's origin (the call log records it and every usage reader
// leaves it out).
func askToolCall(ctx context.Context, selfURL, tool string, args map[string]any) ([]byte, error) {
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	})
	if err != nil {
		return nil, err
	}
	// Inside the daemon the call runs in process, admitted by the server's
	// gate as a console call (RT-115): no loopback request.
	if raw, ok := inProcessCall(ctx, selfURL, "agent", body); ok {
		return unwrapRPCResult(raw)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, selfURL+"/rpc/agent", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := consoleRPCClient(askTryTimeout).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the agent surface answered HTTP %d", resp.StatusCode)
	}
	return unwrapRPCResult(raw)
}

// ─── the project's example inputs ───────────────────────────────────────────

var askSampleCache struct {
	sync.Mutex
	entries map[string]map[string]console.AskSample
}

// cachedAskSamples reads a project's example inputs once per store
// watermark (they come from its indexed symbols and its plans). A read that
// failed is never kept: a cleared wedge is read again on the next request.
func cachedAskSamples(cr *consoleRequest, choice learnChoice) map[string]console.AskSample {
	key := choice.Service
	if cr.deps.c != nil {
		if wm, err := cr.deps.c.StoreWatermark(cr.r.Context(), choice.Service); err == nil {
			key += "\x00" + wm
		}
	}
	askSampleCache.Lock()
	if s, ok := askSampleCache.entries[key]; ok {
		askSampleCache.Unlock()
		return s
	}
	askSampleCache.Unlock()
	s := readAskSamples(cr, choice)
	for _, smp := range s {
		if smp.Err != "" {
			return s
		}
	}
	askSampleCache.Lock()
	if askSampleCache.entries == nil || len(askSampleCache.entries) > 64 {
		askSampleCache.entries = map[string]map[string]console.AskSample{}
	}
	askSampleCache.entries[key] = s
	askSampleCache.Unlock()
	return s
}

// resetAskSampleCache forgets every project's samples (tests).
func resetAskSampleCache() {
	askSampleCache.Lock()
	askSampleCache.entries = nil
	askSampleCache.Unlock()
}

// readAskSamples picks, deterministically: the callable symbol the most
// call sites name (who-calls then has something to show), its file, its
// name as a literal pattern, and a claimed plan row with a contract — from
// the plan the gate ran on most recently for the project, else the first
// plan by name.
func readAskSamples(cr *consoleRequest, choice learnChoice) map[string]console.AskSample {
	out := map[string]console.AskSample{}
	svc := choice.Service
	if cr.deps.c != nil {
		ctx, cancel := context.WithTimeout(cr.r.Context(), 10*time.Second)
		//fathom:tenancy-allow sources the Learn page's try-it runs one symbol listing for a chosen service under a 10 s request deadline; moving it onto the shared snapshot is follow-up work
		nodes, err := cr.deps.c.ListAllSymbols(ctx, svc)
		cancel()
		switch {
		case err != nil:
			failed := console.AskSample{Note: "the project's symbols could not be read", Err: err.Error()}
			out["symbol"], out["pattern"], out["file"] = failed, failed, failed
		default:
			if name, file, n := mostCalledSymbol(nodes); name != "" {
				why := fmt.Sprintf("the function the most call sites in %s name (%s)", svc, plural(n, "call site"))
				out["symbol"] = console.AskSample{Value: name, Note: why}
				out["pattern"] = console.AskSample{Value: regexp.QuoteMeta(name), Note: "that function's name, as a literal"}
				out["file"] = console.AskSample{Value: file, Note: "the file that function is defined in"}
			}
		}
	}
	row, note, err := samplePlanRow(choice)
	out["row"] = console.AskSample{Value: row, Note: note}
	if err != nil {
		out["row"] = console.AskSample{Note: note, Err: err.Error()}
	}
	return out
}

// mostCalledSymbol is the project's callable (a function or method, not a
// test) that the most call sites name. A call list names a callee as
// "pkg.Name" from another package or "Name" from its own (or keys it fully,
// "service::pkg.Name"), so a call counts
// only when it resolves to one of the project's own (package, name) pairs —
// "filepath.Join" is the standard library's, never a project's Join. Ties
// go to the name first in order, so the sample is stable.
func mostCalledSymbol(nodes []core.SymbolNode) (name, file string, calls int) {
	type pkgName struct{ pkg, name string }
	callable := map[pkgName]core.SymbolNode{}
	for _, n := range nodes {
		kind := strings.ToLower(string(n.Kind))
		if !strings.Contains(kind, "func") && !strings.Contains(kind, "method") {
			continue
		}
		if strings.HasPrefix(n.Name, "Test") || strings.HasSuffix(n.FilePath, "_test.go") || len(n.Name) < 3 {
			continue
		}
		if _, dup := callable[pkgName{n.Package, n.Name}]; !dup {
			callable[pkgName{n.Package, n.Name}] = n
		}
	}
	inbound := map[pkgName]int{}
	for _, n := range nodes {
		for _, c := range n.Calls {
			if _, key, ok := strings.Cut(c, "::"); ok {
				c = key // a fully keyed callee: service::pkg.Name
			}
			k := pkgName{n.Package, c}
			if i := strings.LastIndex(c, "."); i >= 0 {
				k = pkgName{c[:i], c[i+1:]}
			}
			if _, ok := callable[k]; ok {
				inbound[k]++
			}
		}
	}
	best, bestN := pkgName{}, -1
	for k, n := range callable {
		c := inbound[k]
		if c > bestN || (c == bestN && (n.Name < best.name || n.Name == best.name && k.pkg < best.pkg)) {
			best, bestN = k, c
		}
	}
	if bestN < 0 {
		return "", "", 0
	}
	return best.name, callable[best].FilePath, bestN
}

var claimedRowRE = regexp.MustCompile(`^\s*- \[x\] .+\{.*\b(symbol|oracle):.*\}\s*$`)

// samplePlanRow finds a claimed plan row with a contract in the project's
// current plans (docs/plans under the recorded root, read-only): the plan
// the gate ran on most recently for the project first, whichever checkout
// ran it, then the rest by name — never a scratch or deleted plan the
// ledger names. A source that could not be read (the verify ledger, the
// plans directory, a plan) is returned as the error when no row was found,
// and the note says the plans could not be read — never that none was
// found.
func samplePlanRow(choice learnChoice) (row, note string, err error) {
	root := choice.Row.Path
	if root == "" {
		return "", "no corpus root is recorded for " + choice.Service, nil
	}
	var failures []error
	dir := filepath.Join(root, "docs", "plans")
	current, derr := currentPlanPaths(root) // a glob would swallow an unreadable directory
	if derr != nil {
		failures = append(failures, derr)
	}
	var plans []string
	seen := map[string]bool{}
	rows, lerr := verifyLedgerRowsAt(verifyLedgerPath())
	switch {
	case lerr == nil:
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].At > rows[j].At })
		for _, r := range rows {
			if r.Service != choice.Service {
				continue
			}
			if rel := ledgerPlanPath(r, root); current[rel] && !seen[rel] {
				seen[rel] = true
				plans = append(plans, rel)
			}
		}
	case !errors.Is(lerr, os.ErrNotExist):
		failures = append(failures, fmt.Errorf("the verify ledger: %w", lerr))
	}
	var names []string
	for rel := range current {
		names = append(names, rel)
	}
	sort.Strings(names)
	for _, rel := range names {
		if !seen[rel] {
			seen[rel] = true
			plans = append(plans, rel)
		}
	}
	for _, rel := range plans {
		found, ferr := firstClaimedRow(filepath.Join(root, filepath.FromSlash(rel)))
		if ferr != nil {
			if !errors.Is(ferr, os.ErrNotExist) {
				failures = append(failures, ferr)
			}
			continue
		}
		if found != "" {
			return found, "the first claimed row with a contract in " + rel, nil
		}
	}
	if len(failures) > 0 {
		return "", "the project's plans could not be read", errors.Join(failures...)
	}
	return "", "no claimed plan row with a contract was found under " + dir, nil
}

// firstClaimedRow is a plan's first claimed row with a contract ("" when it
// has none), or why the plan could not be read.
func firstClaimedRow(path string) (string, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		if line := sc.Text(); claimedRowRE.MatchString(line) && len(line) <= 2000 {
			return strings.TrimSpace(line), nil
		}
	}
	return "", sc.Err()
}

// verifyLedgerRowsAt reads the verify ledger through the shared cache.
func verifyLedgerRowsAt(path string) ([]verifyLedgerRow, error) {
	if path == "" {
		return nil, os.ErrNotExist
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	rows, err := verifyLedgerRows(path, info)
	return append([]verifyLedgerRow(nil), rows...), err
}

// ─── rendering ──────────────────────────────────────────────────────────────

func askHref(svc, shape string) string {
	v := url.Values{}
	if svc != "" {
		v.Set("svc", svc)
	}
	v.Set("try", shape)
	return "/learn/ask?" + v.Encode() + "#answer"
}

func renderLearnAsk(choice learnChoice, view console.AskView, facts learnFacts, selfURL string, m observatoryModel) string {
	var b strings.Builder
	if view.Answer != nil {
		b.WriteString(`<div class="mb-4">` + renderAskAnswer(choice, view, *view.Answer, m) + `</div>`)
	}
	b.WriteString(renderAskShapes(choice, view, facts, m))
	b.WriteString(`<div class="mt-4">` + renderAskHarnesses(selfURL) + `</div>`)
	return b.String()
}

// renderAskShapes is the routing table: each shape, its tool, what the
// tool answers, how often it was asked here, and its try-it link.
func renderAskShapes(choice learnChoice, view console.AskView, facts learnFacts, m observatoryModel) string {
	rows := make([]table.Row, 0, len(view.Shapes))
	for _, s := range view.Shapes {
		tool := `<code class="font-mono text-xs text-ink">` + html.EscapeString(s.Tool) + `</code>`
		if !s.OnAgentSurface {
			tool += ` <span class="text-xs text-caution">not on the agent surface</span>`
		}
		try := `<span class="italic text-ink-muted">pick a project first</span>`
		if choice.Service != "" {
			try = `<a href="` + html.EscapeString(askHref(choice.Service, s.ID)) + `" class="inline-flex min-h-11 items-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" data-ask-try="` + html.EscapeString(s.ID) + `">Try it on ` + html.EscapeString(choice.Service) + ` →</a>`
		}
		rows = append(rows, table.Row{
			ID:    "shape-" + s.ID,
			Cells: []string{s.Question, s.Tool, s.Gets, "", ""},
			HTMLCells: []templ.Component{
				templ.Raw(`<span class="font-medium text-ink">` + html.EscapeString(s.Question) + `</span>`),
				templ.Raw(tool),
				templ.Raw(`<span class="text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(s.Gets) + `</span>`),
				templ.Raw(valueText(s.Asked)),
				templ.Raw(try),
			},
		})
	}
	tbl := renderKitTable(table.TableParams{
		TableID: "learn-ask-shapes",
		Caption: "Route by the shape of the question, not by habit: each tool answers one shape",
		Columns: []table.Column{{Key: "question", Label: "Question"}, {Key: "tool", Label: "Tool"}, {Key: "gets", Label: "What you get"}, {Key: "asked", Label: "Asked here (30 days)", Align: "right"}, {Key: "try", Label: "Try it"}},
		Rows:    rows,
	})
	// The same shapes stacked for a phone, where five columns squeeze each
	// cell to a word per line (the statusz precedent).
	var list strings.Builder
	for _, s := range view.Shapes {
		items := []chrome.DescItem{
			{Term: "tool", DetailHTML: template.HTML(`<code class="font-mono text-xs">` + html.EscapeString(s.Tool) + `</code>`)},
			{Term: "you get", Detail: s.Gets},
			{Term: "asked here", DetailHTML: template.HTML(valueText(s.Asked))},
		}
		list.WriteString(`<li class="min-w-0 rounded-lg border border-line bg-surface-raised px-3 py-2"><p class="font-medium text-ink">` + html.EscapeString(s.Question) + `</p>` +
			`<div class="[&_dl]:grid-cols-1 [&_dd]:[overflow-wrap:anywhere]">` + chrome.RenderDescriptionList(items) + `</div>`)
		if choice.Service != "" {
			list.WriteString(`<a href="` + html.EscapeString(askHref(choice.Service, s.ID)) + `" class="inline-flex min-h-11 items-center font-medium text-accent underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Try it on ` + html.EscapeString(choice.Service) + ` →</a>`)
		}
		list.WriteString(`</li>`)
	}
	never := valueText(view.NeverAsked) + ` of them never called for ` + html.EscapeString(orDash(choice.Service)) + ` in ` + html.EscapeString(view.Window)
	if !view.NeverAsked.Known() {
		// No count from a log that was not read: say so, never a number.
		never = `how many of them were called for ` + html.EscapeString(orDash(choice.Service)) + ` is ` + valueText(view.NeverAsked)
	}
	footnote := `<p class="mt-3 text-ink-muted" data-ask-coverage>This guide teaches ` + plural(view.ToolsTaught, "tool") +
		` of the ` + valueText(view.AgentTools) + ` on the agent surface every connected session sees; ` + never + `.</p>`
	state, reason := console.StatePresent, ""
	switch asked := view.Shapes[0].Asked; asked.State {
	case console.StateError:
		state, reason = console.StateError, "how often each tool was asked here could not be read — "+asked.Reason
	case console.StateAbsent, "":
		state, reason = console.StateAbsent, "how often each tool was asked here is not measured — "+asked.Reason
	}
	return renderPanel(consolePanel{
		ID: "shapes", Title: "Route by the shape of the question",
		Question:    "Which tool answers which question, and which of them does this project actually use?",
		Verb:        facts.traffic.Verb,
		Source:      "the routing fathom's operating stanza gives every agent (SB-7) and the decision port's route site (DP-9); asked: " + orDash(facts.traffic.Total.Source),
		Denominator: plural(len(view.Shapes), "question shape") + " over " + plural(view.ToolsTaught, "tool"),
		Freshness:   "as of " + m.GeneratedAt,
		State:       state, Reason: reason,
		SourceKey: liveKeyIfValid("sessions:" + choice.Service),
		Body: `<div class="hidden lg:block" data-ask-table>` + tbl + `</div>` +
			`<ul class="flex flex-col gap-3 lg:hidden" aria-label="Question shapes" data-ask-list>` + list.String() + `</ul>` + footnote,
	})
}

// renderAskAnswer is one try-it: the exact call as the panel's verb, a form
// to ask again with another input, and the tool's answer as rows.
func renderAskAnswer(choice learnChoice, view console.AskView, a console.AskAnswer, m observatoryModel) string {
	var shape console.AskShape
	for _, s := range view.Shapes {
		if s.ID == a.Shape {
			shape = s
		}
	}
	var body strings.Builder
	if shape.Arg != "" {
		body.WriteString(`<form method="get" action="/learn/ask#answer" class="mb-4 flex min-w-0 flex-wrap items-end gap-2 [&_button]:min-h-11 [&_input]:min-h-11" data-ask-form>`)
		body.WriteString(`<input type="hidden" name="svc" value="` + html.EscapeString(choice.Service) + `"><input type="hidden" name="try" value="` + html.EscapeString(shape.ID) + `">`)
		help := shape.SampleNote
		switch {
		case shape.Sample != "":
			help = "Example from " + choice.Service + ": " + shape.SampleNote
		case shape.SampleError != "":
			help = shape.SampleNote + ": " + shape.SampleError
		}
		field := form.RenderField(form.Field{
			Label: shape.ArgLabel, For: "ask-input", Help: help,
			Control: template.HTML(form.RenderInput(form.Input{ID: "ask-input", Name: "q", Value: a.Input, Placeholder: shape.Sample, HasHelp: help != ""})),
		})
		body.WriteString(`<div class="min-w-0 flex-1 basis-64">` + field + `</div>`)
		body.WriteString(form.RenderButton(form.Button{Label: "Ask " + shape.Tool, Variant: form.ButtonPrimary, Type: "submit"}))
		body.WriteString(`</form>`)
	}
	if a.Summary != "" {
		body.WriteString(`<p class="mb-3 font-medium text-ink [overflow-wrap:anywhere]" data-ask-summary>` + html.EscapeString(a.Summary) + `</p>`)
	}
	if len(a.Rows) > 0 {
		cols := make([]table.Column, 0, len(a.Columns))
		for i, c := range a.Columns {
			cols = append(cols, table.Column{Key: fmt.Sprintf("c%d", i), Label: c})
		}
		rows := make([]table.Row, 0, len(a.Rows))
		for i, r := range a.Rows {
			cells := make([]templ.Component, len(r))
			for j, v := range r {
				cells[j] = templ.Raw(askCell(v))
			}
			rows = append(rows, table.Row{ID: fmt.Sprintf("answer-%d", i), Cells: r, HTMLCells: cells})
		}
		caption := fmt.Sprintf("%s's answer on %s", shape.Tool, choice.Service)
		switch {
		case a.State == console.StatePartial:
			caption += " — " + askShownWords(a)
		case a.Total > len(a.Rows):
			caption += fmt.Sprintf(" — showing %d of %s", len(a.Rows), fmtThousands(a.Total))
		}
		body.WriteString(renderKitTable(table.TableParams{TableID: "learn-ask-answer", Caption: caption, Columns: cols, Rows: rows}))
	}
	if a.HowToGetMore != "" {
		body.WriteString(`<p class="mt-3 text-ink-muted [overflow-wrap:anywhere]" data-ask-more>To get the rest, ` + html.EscapeString(shape.Tool) + ` says: ` + html.EscapeString(a.HowToGetMore) + `</p>`)
	}
	title := "Answer"
	if shape.Question != "" {
		title = "Answer: " + shape.Question
	}
	denom := ""
	switch a.State {
	case console.StatePresent, console.StateZero:
		denom = plural(a.Total, "row") + " in the answer"
	case console.StatePartial:
		denom = askTotalWords(a) + " rows in the tool's answer, " + fmtThousands(a.Listed) + " of them listed"
	}
	return renderPanel(consolePanel{
		ID: "answer", SourceKey: liveKeyIfValid("index:" + choice.Service), Title: title,
		Question:    "What does " + shape.Tool + " answer on " + orDash(choice.Service) + " — exactly what an agent would get?",
		Verb:        a.Call,
		Source:      "one read-only call of " + shape.Tool + " on this daemon's agent surface (/rpc/agent), logged with the console's origin and never counted as usage",
		Denominator: denom,
		Freshness:   "live — asked for this request",
		State:       a.State, Reason: a.Reason,
		Body: body.String(),
	})
}

// askCell is one answer cell. A short single word ("resolved", "GoFunc",
// "payment.go:26") never breaks inside itself — breaking anywhere would let
// the table squeeze its column to a few letters a line; a long path or a
// line of code may break anywhere, so the table still fits a phone.
func askCell(v string) string {
	if utf8.RuneCountInString(v) <= 16 && !strings.ContainsAny(v, " \t") {
		return `<span class="whitespace-nowrap">` + html.EscapeString(v) + `</span>`
	}
	return `<span class="[overflow-wrap:anywhere]">` + html.EscapeString(v) + `</span>`
}

// askTotalWords is a cut answer's total as the tool stated it: "277", or
// "at least 3" when the tool stopped early without a count.
func askTotalWords(a console.AskAnswer) string {
	if a.TotalLowerBound {
		return "at least " + fmtThousands(a.Total)
	}
	return fmtThousands(a.Total)
}

// askShownWords is a cut answer's caption: the rows shown, the rows the
// tool listed, and the total it reported — three different numbers, each
// named.
func askShownWords(a console.AskAnswer) string {
	words := fmt.Sprintf("showing %d of the %s it listed", len(a.Rows), fmtThousands(a.Listed))
	if a.Total > a.Listed {
		return words + "; it found " + askTotalWords(a)
	}
	return words + "; there may be more"
}

// countWord is "1 harness" / "6 harnesses" for a noun plural can't form.
func countWord(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmtThousands(n) + " " + many
}

// renderAskHarnesses is `fathom harness matrix --json` rendered: how each
// assistant reaches these tools, each `mcp add` line naming this daemon.
func renderAskHarnesses(selfURL string) string {
	rows := harness.AdoptionRows(selfURL, harnessMatrixNow())
	var list strings.Builder
	for _, r := range rows {
		items := []chrome.DescItem{
			{Term: "register the tools", DetailHTML: template.HTML(renderVerbChip(r.MCPAdd))},
			{Term: "instructions", Detail: strings.Join(r.InstructionFile, ", ")},
			{Term: "MCP config", Detail: strings.Join(r.MCPConfig, ", ")},
			{Term: "skills", Detail: strings.Join(r.SkillsPath, ", ")},
			{Term: "verified against", Detail: r.VerifiedAgainst},
		}
		list.WriteString(`<li class="min-w-0 rounded-lg border border-line bg-surface px-3 py-2">` +
			`<p class="font-medium text-ink">` + html.EscapeString(r.Display) + ` <span class="font-mono text-xs text-ink-muted">tier ` + html.EscapeString(r.Tier) + `</span></p>` +
			`<div class="[&_dl]:grid-cols-1 sm:[&_dl]:grid-cols-[max-content_1fr] [&_dd]:min-w-0 [&_dd]:[overflow-wrap:anywhere]">` + chrome.RenderDescriptionList(items) + `</div></li>`)
	}
	return renderPanel(consolePanel{
		ID: "harnesses", SourceKey: "daemon", Title: "Reach these tools from your assistant",
		Question:    "How does each coding assistant connect to these tools, and how well is that path verified?",
		Verb:        "fathom harness matrix --json",
		Source:      "the harness registry (internal/harness), with this daemon's address in each mcp add line",
		Denominator: countWord(len(rows), "harness", "harnesses"),
		Freshness:   "the registry built into this binary",
		Body:        `<ul class="grid grid-cols-1 gap-3 xl:grid-cols-2" aria-label="Assistants">` + list.String() + `</ul>`,
	})
}
