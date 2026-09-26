package main

// console_learn_test.go — the Learn group (OC-46..OC-48) through the real
// serve-http handler on the fixture fleet: the pipeline's counts are the
// fleet report's and the store's, each stage links to a page the console
// serves, the project's capabilities section is its verb's output; the ask
// page's try-it is one read-only call on the agent surface that the call
// log never counts as usage and that writes no ledger; the guides render
// with a table of contents, beside the glossary and the laws.

import (
	"bytes"
	"encoding/json"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

func newLearnFixture(t *testing.T, o consoleFixtureOpts) *consoleFixture {
	t.Helper()
	resetLearnCache()
	resetAskSampleCache()
	t.Cleanup(resetLearnCache)
	t.Cleanup(resetAskSampleCache)
	return newConsoleFixtureOpts(t, o)
}

// learnModelOf decodes /learn's model.
func learnModelOf(t *testing.T, env consolePageJSONWire) learnModel {
	t.Helper()
	var m learnModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestConsoleLearn_PipelineStagesCountsAndLinks(t *testing.T) {
	f := newLearnFixture(t, consoleFixtureOpts{})
	code, _, page := f.get(t, "/learn")
	if code != http.StatusOK || strings.Count(page, "<h1") != 1 || !strings.Contains(page, ">How fathom works</h1>") {
		t.Fatalf("/learn = %d:\n%.600s", code, page)
	}
	// No project named: the most-queried one in the call log is drawn, and
	// the heading says why.
	if !strings.Contains(page, "project payment-service (the most-queried project in the call log)") {
		t.Error("the default project must be the most-queried one, named with the reason")
	}
	// The picker is a GET form over every project; no JavaScript needed.
	if !strings.Contains(page, `<form method="get" action="/learn"`) || !strings.Contains(page, `<option value="payment-service" selected>`) {
		t.Error("the project picker must be a GET form with the drawn project selected")
	}
	// The pipeline: the kit's layered graph, its table alternative behind a
	// disclosure, one column per stage.
	for _, want := range []string{`role="img"`, `data-learn-graph-table`, "The pipeline as a table", `tabindex="0" role="region"`} {
		if !strings.Contains(page, want) {
			t.Errorf("/learn missing %q", want)
		}
	}
	for i, st := range []string{"Source", "Extract", "Index", "Compile", "Serve", "Gate", "Evidence"} {
		if !strings.Contains(page, itoa(i+1)+" · "+st) {
			t.Errorf("the pipeline lacks stage %d · %s", i+1, st)
		}
	}
	// The table alternative — the keyboard and screen-reader path to the
	// drawing — names each node by its stage, in its links and in every
	// hand-off, never by a bare count.
	graphTable := page[strings.Index(page, "data-learn-graph-table"):]
	graphTable = graphTable[:strings.Index(graphTable, "</details>")]
	for _, st := range []string{"Source", "Extract", "Index", "Compile", "Serve", "Gate", "Evidence"} {
		if !regexp.MustCompile(`<th scope="row">(<a [^>]*>)?` + st + `: `).MatchString(graphTable) {
			t.Errorf("the table alternative does not name the %s node by its stage", st)
		}
	}
	if !strings.Contains(graphTable, "<td>Source: ") || !strings.Contains(graphTable, "<td>Extract: ") {
		t.Error("the hand-offs must name the stages they join")
	}
	// Seven stage panels, each with its question and, where a verb prints
	// its headline, that exact verb.
	for _, id := range console.PipelineStageIDs() {
		if !strings.Contains(page, `id="stage-`+string(id)+`"`) {
			t.Errorf("no panel for stage %s", id)
		}
	}
	for _, verb := range []string{
		"fathom report evidence --service payment-service --json",
		`staleness_report {&#34;service&#34;:&#34;payment-service&#34;}`,
		"fathom metrics traffic --service payment-service --by tool --since 720h --json",
		"fathom plans status --service payment-service",
	} {
		if !strings.Contains(page, verb) {
			t.Errorf("/learn lacks the verb %q", verb)
		}
	}
	// Every stage either links to a page the console serves — a ?tab= only
	// when the page declares that tab — or says its page is not built yet;
	// no link promises a destination that renders something else.
	reg := defaultConsoleRegistry()
	links := regexp.MustCompile(`href="([^"]+)"[^>]*data-stage-link`).FindAllStringSubmatch(page, -1)
	if nopage := strings.Count(page, "data-stage-nopage"); len(links)+nopage != 7 || len(links) < 5 {
		t.Fatalf("stage links = %d and pages not built = %d, want 7 stages with at least 5 links", len(links), nopage)
	}
	for _, l := range links {
		href := strings.ReplaceAll(l[1], "&amp;", "&")
		if _, _, ok := resolveConsoleHref(reg, []string{href}); !ok {
			t.Errorf("stage link %s resolves to no registered page (or an undeclared tab)", href)
		}
		if strings.Contains(href, "?tab=") {
			t.Errorf("stage link %s names a tab the project workspace does not serve yet", href)
		}
	}
	if !strings.Contains(page, "Open payment-service&#39;s project page") {
		t.Error("the project workspace link must name the page it opens, not a tab")
	}
	if !strings.Contains(page, "(/projects/payment-service?tab=index is planned)") || !strings.Contains(page, "(/plans is planned)") {
		t.Error("a stage with no page must name the page that is planned")
	}
	assertRoleUtilitiesOnly(t, page)

	// The JSON: the pipeline's registry counts are the fleet report's row
	// (P10); an instrument that did not answer is absent, never 0.
	env := f.pageJSON(t, "/learn")
	m := learnModelOf(t, env)
	if m.Project != "payment-service" || len(m.Pipeline.Stages) != 7 {
		t.Fatalf("model = %+v", m)
	}
	var rep struct {
		Services []struct {
			Service      string `json:"service"`
			Symbols      int    `json:"symbols"`
			CallsTotal   *int   `json:"calls_total"`
			SessionCount *int   `json:"session_count"`
		} `json:"services"`
	}
	if err := json.Unmarshal(env.Sections["fathom fleet report --json"], &rep); err != nil {
		t.Fatal(err)
	}
	stage := map[console.PipelineStageID]console.PipelineStage{}
	for _, st := range m.Pipeline.Stages {
		stage[st.ID] = st
	}
	for _, r := range rep.Services {
		if r.Service != "payment-service" {
			continue
		}
		if got := stage[console.StageExtract].Headline.Value; got.V != r.Symbols || !got.Known() {
			t.Errorf("extract symbols %+v, the fleet report %d", got, r.Symbols)
		}
		if got := stage[console.StageEvidence].Headline.Value; r.SessionCount == nil || got.V != *r.SessionCount {
			t.Errorf("evidence sessions %+v, the fleet report %v", got, r.SessionCount)
		}
		if got := stage[console.StageEvidence].Counts[0].Value; r.CallsTotal == nil || got.V != *r.CallsTotal {
			t.Errorf("evidence calls %+v, the fleet report %v", got, r.CallsTotal)
		}
	}
	// The fixture has no verify ledger: the gate stage is absent with the
	// reason — the page says "not measured", never a 0.
	if g := stage[console.StageGate].Headline.Value; g.State != console.StateAbsent || !strings.Contains(g.Reason, "no verify ledger") {
		t.Errorf("gate with no ledger = %+v", g)
	}
	if !strings.Contains(page, "not measured — no verify ledger at") {
		t.Error("the gate panel must say its instrument did not answer")
	}
	// Two of the fixture's calls name payment-service, one of them an
	// obligations call — the serve stage's traffic.
	if s := stage[console.StageServe]; s.Headline.Value.V != 1 || s.Counts[0].Value.V != 2 {
		t.Errorf("serve = %+v", s)
	}
	// A ledger row for the project turns the gate into counts: its latest
	// run per plan, summed.
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), `{"schema_version":2,"at":"2026-09-24T10:00:00Z","service":"payment-service","plan":"docs/plans/a.md","total":4,"claimed":3,"verified":1}
{"schema_version":2,"at":"2026-09-25T10:00:00Z","service":"payment-service","plan":"docs/plans/a.md","total":4,"claimed":3,"verified":3}
{"schema_version":2,"at":"2026-09-25T10:00:00Z","service":"ledger-service","plan":"docs/plans/b.md","total":1,"claimed":1,"verified":0}
`)
	planRoot := t.TempDir()
	mustWrite(t, filepath.Join(planRoot, "docs", "plans", "a.md"), "- [x] a {id: A-1, symbol: X}\n")
	g := readVerifyLedgerFor(os.Getenv("FATHOM_VERIFY_LEDGER"), "payment-service", planRoot)
	if g.runs.V != 2 || g.plans.V != 1 || g.verified.V != 3 || g.claimed.V != 3 {
		t.Errorf("gate from the ledger = %+v", g)
	}
	if g := readVerifyLedgerFor(os.Getenv("FATHOM_VERIFY_LEDGER"), "checkout-service", planRoot); g.runs.State != console.StateZero || g.verified.State != console.StateAbsent {
		t.Errorf("a readable ledger with no row for the project is a measured zero of runs: %+v", g)
	}

	// The project's capabilities section is its exact command's output.
	if m.Capabilities == "" || !strings.HasPrefix(m.Capabilities, "fathom capabilities --repo ") {
		t.Fatalf("capabilities verb = %q", m.Capabilities)
	}
	section, ok := env.Sections[m.Capabilities]
	if !ok {
		t.Fatalf("no section for %q", m.Capabilities)
	}
	args := append(strings.Fields(m.Capabilities)[1:], "--url", f.ts.URL)
	var out, errb bytes.Buffer
	if code := run(args, &out, &errb); code != 0 {
		t.Fatalf("%s: %d %s", m.Capabilities, code, errb.String())
	}
	var fromVerb, fromPage any
	_ = json.Unmarshal(out.Bytes(), &fromVerb)
	_ = json.Unmarshal(section, &fromPage)
	if !reflect.DeepEqual(fromVerb, fromPage) {
		t.Errorf("the page's capabilities section differs from `%s`", m.Capabilities)
	}

	// Another project by name; an unknown one is the shell's 404 with the
	// index remedy.
	if env := f.pageJSON(t, "/learn?svc=checkout-service"); learnModelOf(t, env).Project != "checkout-service" {
		t.Error("?svc= must draw the named project")
	}
	code, _, body := f.get(t, "/learn?svc=nope-service")
	if code != http.StatusNotFound || !strings.Contains(body, ">Unknown project</h1>") || !strings.Contains(body, "fathom init --path") {
		t.Errorf("/learn?svc=nope-service = %d", code)
	}
}

// A stage links to the most specific page the console has registered: the
// design's page once a wave registers it, the project workspace's tab once
// the workspace declares it, the workspace itself until then.
func TestConsoleLearn_StageLinksFollowTheRegistry(t *testing.T) {
	build := func(*consoleRequest) (consoleOut, error) { return consoleOut{}, nil }
	reg := newConsoleRegistry()
	for _, p := range []consolePage{
		{ID: "project", Route: "/projects/{svc}", Group: "Fleet", Title: "Project", Question: "?", Build: build},
		{ID: "impact", Route: "/impact", Href: "/impact", Group: "Evidence", Title: "Impact", Question: "?", Build: build},
	} {
		if err := registerPage(reg, p); err != nil {
			t.Fatal(err)
		}
	}
	// A tab resolves only when the page declares it: until the project
	// workspace serves ?tab=index, the link falls through to the page
	// itself — never a tab URL that renders the overview.
	pages := []string{"/packs", "/projects/pay?tab=index", "/projects/pay"}
	if href, _, ok := resolveConsoleHref(reg, pages); !ok || href != "/projects/pay" {
		t.Errorf("without /packs or the index tab = %q %v", href, ok)
	}
	if _, _, ok := resolveConsoleHref(reg, []string{"/projects/pay?tab=index"}); ok {
		t.Error("an undeclared tab must not resolve")
	}
	withTabs := newConsoleRegistry()
	if err := registerPage(withTabs, consolePage{ID: "project", Route: "/projects/{svc}", Group: "Fleet", Title: "Project", Question: "?", Tabs: []string{"index", "plans"}, Build: build}); err != nil {
		t.Fatal(err)
	}
	if href, _, ok := resolveConsoleHref(withTabs, pages); !ok || href != "/projects/pay?tab=index" {
		t.Errorf("with the index tab declared = %q %v", href, ok)
	}
	if err := registerPage(reg, consolePage{ID: "packs", Route: "/packs", Href: "/packs", Group: "Knowledge", Title: "Packs", Question: "?", Build: build}); err != nil {
		t.Fatal(err)
	}
	if href, title, ok := resolveConsoleHref(reg, pages); !ok || href != "/packs" || title != "Packs" {
		t.Errorf("with /packs = %q %q %v", href, title, ok)
	}
	if _, _, ok := resolveConsoleHref(reg, []string{"/plans"}); ok {
		t.Error("an unregistered page must not resolve")
	}
	if !routeMatches("/projects/{svc}", "/projects/fathom") || routeMatches("/projects/{svc}", "/projects/") || routeMatches("/{$}", "/x") {
		t.Error("routeMatches")
	}
}

func TestConsoleLearnAsk_ShapesAndReadOnlyTryIt(t *testing.T) {
	f := newLearnFixture(t, consoleFixtureOpts{LogCalls: true})
	code, _, page := f.get(t, "/learn/ask")
	if code != http.StatusOK || strings.Count(page, "<h1") != 1 || !strings.Contains(page, ">Ask the right way</h1>") {
		t.Fatalf("/learn/ask = %d", code)
	}
	env := f.pageJSON(t, "/learn/ask")
	var m askModel
	if err := json.Unmarshal(env.Model, &m); err != nil {
		t.Fatal(err)
	}
	// Every tool the guide teaches is on the agent surface, and every tool
	// the decision port's route site can suggest is taught under the same
	// shape (DP-9) — the guide never teaches a tool an agent cannot call.
	onSurface := map[string]bool{}
	for _, tool := range mcp.AgentSurfaceTools() {
		onSurface[tool] = true
	}
	shapeOf := map[string]string{}
	for _, s := range m.Guide.Shapes {
		if !onSurface[s.Tool] || !s.OnAgentSurface {
			t.Errorf("shape %s teaches %s, which is not on the agent surface", s.ID, s.Tool)
		}
		shapeOf[s.Tool] = s.ID
		if !strings.Contains(page, askHrefEscaped("payment-service", s.ID)) {
			t.Errorf("shape %s has no try-it link", s.ID)
		}
	}
	for _, v := range routeVerbs {
		if v.Name == "none" {
			continue
		}
		got, ok := shapeOf[v.Name]
		if !ok {
			t.Errorf("the route site's %s is not taught", v.Name)
			continue
		}
		if want := routeShapeOf[v.Name]; want != "who-calls" && got != want {
			t.Errorf("%s is taught as %q; the route site calls it %q", v.Name, got, want)
		}
	}
	// The project's example input is its own most-called function.
	if s := m.Guide.Shapes[0]; s.Sample != "validateCharge" {
		t.Errorf("where sample = %q (%s)", s.Sample, s.SampleNote)
	}
	// Asked here: the fixture log names payment-service twice.
	for _, s := range m.Guide.Shapes {
		if s.Tool == "obligations" && s.Asked.V != 1 {
			t.Errorf("obligations asked = %+v", s.Asked)
		}
	}

	// A try-it: one call on the agent surface, projected.
	askedBefore := m.Guide.Shapes[0].Asked
	if askedBefore.V != 1 {
		t.Fatalf("the fixture log holds one find_symbol call for payment-service, asked = %+v", askedBefore)
	}
	before, _ := os.ReadFile(f.callLog)
	env = f.pageJSON(t, "/learn/ask?try=where")
	_ = json.Unmarshal(env.Model, &m)
	a := m.Guide.Answer
	if a == nil || a.State != console.StatePresent || a.Call != `find_symbol {"name":"validateCharge","service":"payment-service"}` || len(a.Rows) == 0 || a.Rows[0][0] != "validateCharge" {
		t.Fatalf("where try-it = %+v", a)
	}
	after, _ := os.ReadFile(f.callLog)
	added := strings.TrimPrefix(string(after), string(before))
	if strings.Count(added, "\n") != 1 || !strings.Contains(added, `"tool":"find_symbol"`) || !strings.Contains(added, `"origin":"`+mcp.OriginConsole+`"`) {
		t.Errorf("the try-it must be one call logged with the console's origin:\n%s", added)
	}
	// ...which no usage reader counts: the asked column does not move.
	resetLearnCache()
	env = f.pageJSON(t, "/learn/ask")
	_ = json.Unmarshal(env.Model, &m)
	if m.Guide.Shapes[0].Asked.V != askedBefore.V {
		t.Errorf("a console try-it must never count as usage: %+v, before %+v", m.Guide.Shapes[0].Asked, askedBefore)
	}
	// The HTML: the exact call as the answer panel's verb, the input form.
	_, _, tried := f.get(t, "/learn/ask?try=who-calls")
	for _, want := range []string{`id="answer"`, `data-console-copy="find_callers {&#34;service&#34;:&#34;payment-service&#34;,&#34;symbol&#34;:&#34;validateCharge&#34;}"`, `<form method="get" action="/learn/ask#answer"`, `name="q"`, "Charge"} {
		if !strings.Contains(tried, want) {
			t.Errorf("who-calls try-it missing %q", want)
		}
	}
	assertRoleUtilitiesOnly(t, tried)

	// The gate's try-it verifies a row and writes no ledger: only
	// `fathom verify` appends one.
	ledger := os.Getenv("FATHOM_VERIFY_LEDGER")
	_ = os.Remove(ledger)
	env = f.pageJSON(t, "/learn/ask?try=is-it-done&q="+urlQuery("- [x] charge {symbol: validateCharge}"))
	_ = json.Unmarshal(env.Model, &m)
	if a := m.Guide.Answer; a == nil || a.State != console.StatePresent || a.Rows[0][2] != "fail" {
		t.Errorf("is-it-done try-it = %+v", a)
	}
	if _, err := os.Stat(ledger); !os.IsNotExist(err) {
		t.Errorf("a try-it must not write the verify ledger (stat: %v)", err)
	}
	// An empty input is refused before any call; an unknown shape is a
	// 400 on the shell.
	before, _ = os.ReadFile(f.callLog)
	env = f.pageJSON(t, "/learn/ask?try=where&q=")
	_ = json.Unmarshal(env.Model, &m)
	if a := m.Guide.Answer; a == nil || a.State != console.StateAbsent || !strings.Contains(a.Reason, "type a symbol name") {
		t.Errorf("empty input = %+v", a)
	}
	if after, _ := os.ReadFile(f.callLog); len(after) != len(before) {
		t.Error("a refused input must make no call")
	}
	if code, _, body := f.get(t, "/learn/ask?try=bogus"); code != http.StatusBadRequest || !strings.Contains(body, ">Unknown question shape</h1>") {
		t.Errorf("unknown shape = %d", code)
	}
}

func askHrefEscaped(svc, shape string) string {
	return strings.ReplaceAll(askHref(svc, shape), "&", "&amp;")
}

func urlQuery(s string) string {
	r := strings.NewReplacer(" ", "+", "[", "%5B", "]", "%5D", "{", "%7B", "}", "%7D", ":", "%3A")
	return r.Replace(s)
}

func TestConsoleLearnGuides_TableOfContentsGlossaryAndLaws(t *testing.T) {
	f := newLearnFixture(t, consoleFixtureOpts{})
	code, _, index := f.get(t, "/learn/guides")
	if code != http.StatusOK || !strings.Contains(index, ">Guides and glossary</h1>") {
		t.Fatalf("/learn/guides = %d", code)
	}
	for _, want := range []string{`href="/learn/guides/fleet-health"`, `href="/learn/guides/glossary#term-1"`, `href="/learn/guides/laws#law-15"`, `id="glossary"`, `id="laws"`} {
		if !strings.Contains(index, want) {
			t.Errorf("/learn/guides missing %q", want)
		}
	}
	var gm guidesModel
	if err := json.Unmarshal(f.pageJSON(t, "/learn/guides").Model, &gm); err != nil || len(gm.Laws) != 15 || len(gm.Glossary) < 20 || len(gm.Problems) != 0 {
		t.Fatalf("guides model = %d laws, %d terms, problems %v (%v)", len(gm.Laws), len(gm.Glossary), gm.Problems, err)
	}

	code, _, page := f.get(t, "/learn/guides/getting-started")
	if code != http.StatusOK || strings.Count(page, "<h1") != 1 || !strings.Contains(page, ">Getting started: wire fathom into Claude Code</h1>") {
		t.Fatalf("guide = %d", code)
	}
	// The table of contents: the kit's rail from xl up, its disclosure
	// below, both over the same outline; every entry's anchor is a heading
	// id in the prose.
	rail := page[strings.Index(page, "data-guide-rail"):]
	if !strings.Contains(rail, `aria-label="Table of contents"`) || !strings.Contains(page, "data-guide-toc-panel") || !strings.Contains(page, `class="doc-prose`) {
		t.Error("a guide renders its prose with the kit's rail and panel")
	}
	var guide guideModel
	if err := json.Unmarshal(f.pageJSON(t, "/learn/guides/getting-started").Model, &guide); err != nil || len(guide.Headings) < 5 || guide.Markdown == "" {
		t.Fatalf("guide model: %d headings (%v)", len(guide.Headings), err)
	}
	for _, h := range guide.Headings {
		if h.ID == "" || !strings.Contains(page, `id="`+h.ID+`"`) || !strings.Contains(rail, `href="#`+h.ID+`"`) {
			t.Errorf("outline entry %+v has no heading anchor or rail link", h)
		}
	}
	// Links: another guide is its page; a repository file with no page is
	// shown unresolved with its path, never as a link that 404s.
	if !strings.Contains(page, `<a href="/learn/guides/coding-agents">`) {
		t.Error("a link to another guide must become its page")
	}
	if !strings.Contains(page, `doc-link--unresolved`) || !strings.Contains(page, "(in the repository: docs/mcp-tools.md") {
		t.Error("a link to a file with no page must render unresolved, naming its path")
	}
	if strings.Contains(page, `<a href="">`) || strings.Contains(page, `href="../`) {
		t.Error("no guide link may point nowhere or up the repository tree")
	}

	// The laws and the glossary are guides too, with stable anchors.
	_, _, laws := f.get(t, "/learn/guides/laws")
	for n := 1; n <= 15; n++ {
		if !strings.Contains(laws, `id="law-`+itoa(n)+`"`) {
			t.Errorf("the laws page lacks #law-%d", n)
		}
	}
	if _, _, glossary := f.get(t, "/learn/guides/glossary"); !strings.Contains(glossary, `id="term-1"`) || !strings.Contains(glossary, ">observation") {
		t.Error("the glossary page must anchor each term")
	}
	if code, _, body := f.get(t, "/learn/guides/nope"); code != http.StatusNotFound || !strings.Contains(body, ">Unknown guide</h1>") {
		t.Errorf("unknown guide = %d", code)
	}
	assertRoleUtilitiesOnly(t, page)

	// Link placement, one by one.
	lib := loadGuideLibrary(fstest.MapFS{
		"use-cases/README.md":  {Data: []byte("| [a.md](a.md) | A |\n")},
		"use-cases/a.md":       {Data: []byte("# A\n")},
		"intent/domain.md":     {Data: []byte("## Vocabulary\n\n- **x** — y. Source: z.\n")},
		"intent/principles.md": {Data: []byte("**P1. One.** Text. Source: law 1.\n")},
	})
	for dest, want := range map[string][2]string{
		"a.md#setup":                   {"/learn/guides/a#setup", ""},
		"README.md":                    {"/learn/guides", ""},
		"../intent/principles.md":      {"/learn/guides/laws", ""},
		"../mcp-tools.md":              {"", "docs/mcp-tools.md"},
		"https://example.com/x":        {"https://example.com/x", ""},
		"#local":                       {"#local", ""},
		"javascript:alert(1)":          {"", "javascript:alert(1)"},
		"../../cmd/fathom/main.go#L10": {"", "cmd/fathom/main.go#L10"},
	} {
		href, unresolved := placeGuideLink(dest, lib)
		if href != want[0] || unresolved != want[1] {
			t.Errorf("placeGuideLink(%q) = %q, %q; want %q, %q", dest, href, unresolved, want[0], want[1])
		}
	}
	if len(lib.entries) != 3 || lib.entries[1].Slug != "glossary" || lib.entries[2].Slug != "laws" {
		t.Errorf("library = %+v", lib.entries)
	}
	if bad := loadGuideLibrary(fstest.MapFS{}); len(bad.problems) != 3 {
		t.Errorf("missing documents must be named, got %v", bad.problems)
	}
}

// An unreadable call log is the external-volume wedge exactly: stat works,
// the read fails. The Learn pages then say the traffic could not be read —
// an error naming what failed — never a measured 0 of rules served, tools
// answered or asks per tool; and the pages recover when the log does.
func TestConsoleLearn_UnreadableCallLogIsAnErrorNeverZero(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a chmod 000 file")
	}
	f := newLearnFixture(t, consoleFixtureOpts{})
	if err := os.Chmod(f.callLog, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(f.callLog, 0o644) })
	if _, err := os.Stat(f.callLog); err != nil {
		t.Fatalf("the wedge's tell is a working stat: %v", err)
	}

	m := learnModelOf(t, f.pageJSON(t, "/learn?svc=payment-service"))
	for _, st := range m.Pipeline.Stages {
		if st.ID != console.StageServe {
			continue
		}
		for _, c := range append([]console.PipelineCount{st.Headline}, st.Counts...) {
			if c.Value.State != console.StateError || !strings.Contains(c.Value.Reason, "call log unreadable") {
				t.Errorf("serve %q over an unreadable log = %+v, want an error naming the log", c.Label, c.Value)
			}
		}
	}
	_, _, page := f.get(t, "/learn?svc=payment-service")
	serve := page[strings.Index(page, `id="stage-serve"`):]
	serve = serve[:strings.Index(serve, `id="stage-gate"`)]
	if !strings.Contains(serve, "unreadable — call log unreadable") || strings.Contains(serve, `<dd class="text-ink">0</dd>`) {
		t.Errorf("the serve panel must say its log is unreadable:\n%s", serve)
	}

	var ask askModel
	if err := json.Unmarshal(f.pageJSON(t, "/learn/ask?svc=payment-service").Model, &ask); err != nil {
		t.Fatal(err)
	}
	for _, s := range ask.Guide.Shapes {
		if s.Asked.State != console.StateError {
			t.Errorf("%s asked over an unreadable log = %+v, want an error", s.ID, s.Asked)
		}
	}
	if ask.Guide.NeverAsked.Known() {
		t.Errorf("never asked over an unreadable log = %+v, want no number", ask.Guide.NeverAsked)
	}
	_, _, askPage := f.get(t, "/learn/ask?svc=payment-service")
	if strings.Contains(askPage, "of them never called") {
		t.Error("the coverage line must not count never-called tools from a log it could not read")
	}
	if !strings.Contains(askPage, `data-truth-state="error"`) {
		t.Error("the routing panel must carry the log's error state")
	}

	// The wedge clears: the counts come back without a restart.
	if err := os.Chmod(f.callLog, 0o644); err != nil {
		t.Fatal(err)
	}
	m = learnModelOf(t, f.pageJSON(t, "/learn?svc=payment-service"))
	for _, st := range m.Pipeline.Stages {
		if st.ID == console.StageServe && (st.Headline.Value.State != console.StatePresent || st.Headline.Value.V != 1) {
			t.Errorf("serve after the log is readable again = %+v", st.Headline.Value)
		}
	}
}

// The gate stage counts the project's own current plans — the plans
// `fathom plans status --service S` reads from the recorded root — once
// each, at each plan's latest run, whichever checkout ran it: a plan named
// by a relative path, by its absolute path, or from a worktree or the
// pre-push gate's clone is one plan. A scratch plan under /tmp and a
// deleted plan are not this project's plans, and are said to be left out,
// never counted as gated.
func TestReadVerifyLedgerFor_CountsTheProjectsCurrentPlansOnce(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"docs/plans/a.md", "docs/plans/b.md", "docs/plans/README.md", ".claude/worktrees/w/docs/plans/a.md"} {
		mustWrite(t, filepath.Join(root, p), "- [x] row {id: R-1, symbol: X}\n")
	}
	scratch := filepath.Join(t.TempDir(), "demo-plan.md")
	mustWrite(t, scratch, "- [x] scratch\n")
	row := func(at, repoRoot, plan string, claimed, verified int) string {
		raw, _ := json.Marshal(map[string]any{"schema_version": 2, "at": at, "repo_root": repoRoot, "service": "pay", "plan": plan, "claimed": claimed, "verified": verified})
		return string(raw) + "\n"
	}
	ledger := filepath.Join(t.TempDir(), "verify-ledger.jsonl")
	mustWrite(t, ledger,
		row("2026-09-20T10:00:00Z", root, "docs/plans/a.md", 3, 1)+ // a, relative
			row("2026-09-21T10:00:00Z", "", filepath.Join(root, "docs/plans/a.md"), 3, 3)+ // a, absolute: the latest
			row("2026-09-22T10:00:00Z", root, "docs/plans/b.md", 2, 1)+ // b
			row("2026-09-23T10:00:00Z", root, scratch, 5, 0)+ // scratch plan
			row("2026-09-23T11:00:00Z", root, "docs/plans/gone.md", 7, 0)+ // deleted plan
			row("2026-09-24T10:00:00Z", filepath.Join(root, ".claude/worktrees/w"), "docs/plans/a.md", 9, 0)+ // a worktree's copy
			`{"schema_version":2,"at":"2026-09-24T10:00:00Z","service":"other","plan":"docs/plans/a.md","claimed":1,"verified":0}`+"\n")

	g := readVerifyLedgerFor(ledger, "pay", root)
	if g.runs.V != 4 || g.plans.V != 2 || g.verified.V != 1 || g.claimed.V != 11 {
		t.Errorf("gate = runs %+v plans %+v verified %+v claimed %+v; want 4 runs over 2 plans, 1 of 11 claimed rows verified (a's latest run is the worktree's)", g.runs, g.plans, g.verified, g.claimed)
	}
	if !strings.Contains(g.runs.Denominator, "of 6 rows for pay") || !strings.Contains(g.runs.Denominator, "2 name no current plan") || !strings.Contains(g.plans.Denominator, "of the 2 plans") {
		t.Errorf("the left-out rows and the current plans must be named: %q / %q", g.runs.Denominator, g.plans.Denominator)
	}
	// A symlinked path to the same checkout is the same plan.
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	if got := ledgerPlanPath(verifyLedgerRow{Plan: filepath.Join(link, "docs/plans/b.md")}, root); got != "docs/plans/b.md" {
		t.Errorf("a plan through a symlinked root = %q", got)
	}
	// No recorded root: the ledger cannot be matched to the project's
	// plans, so nothing is counted — absent with the reason.
	if g := readVerifyLedgerFor(ledger, "pay", ""); g.runs.State != console.StateAbsent || !strings.Contains(g.runs.Reason, "no corpus root") {
		t.Errorf("no root = %+v", g.runs)
	}
	// A project whose plans directory cannot be read is an error, never
	// "the gate never ran".
	if os.Geteuid() != 0 {
		plans := filepath.Join(root, "docs", "plans")
		if err := os.Chmod(plans, 0o000); err != nil {
			t.Fatal(err)
		}
		g := readVerifyLedgerFor(ledger, "pay", root)
		_ = os.Chmod(plans, 0o755)
		if g.runs.State != console.StateError || !strings.Contains(g.runs.Reason, "permission denied") {
			t.Errorf("an unreadable plans directory = %+v", g.runs)
		}
		// A cached count is not an answer for a source unreadable now.
		resetLearnCache()
		t.Cleanup(resetLearnCache)
		if g := cachedVerifyLedgerFor(ledger, "pay", root); g.runs.V != 4 {
			t.Fatalf("cached gate = %+v", g.runs)
		}
		for _, locked := range []string{ledger, plans} {
			_ = os.Chmod(locked, 0o000)
			g := cachedVerifyLedgerFor(ledger, "pay", root)
			_ = os.Chmod(locked, 0o755)
			if g.runs.State != console.StateError {
				t.Errorf("%s unreadable after a cached read = %+v, want an error", locked, g.runs)
			}
		}
	}
}

// A stage panel's verb chip is a CLI command that prints the stage's
// headline (ADR-045 decision 2; the stageVerb contract): run it and the
// headline is in its output. A stage whose headline no CLI verb prints
// carries no chip — the footer names its instrument — and an agent-surface
// call is shown as a call, never as "the command".
func TestConsoleLearn_StageChipsPrintTheirHeadlines(t *testing.T) {
	f := newLearnFixture(t, consoleFixtureOpts{})
	m := learnModelOf(t, f.pageJSON(t, "/learn?svc=payment-service"))
	_, _, page := f.get(t, "/learn?svc=payment-service")
	chipOf := func(id console.PipelineStageID) string {
		t.Helper()
		m := regexp.MustCompile(`id="stage-` + string(id) + `"[^>]*?(?: data-console-verb="([^"]*)")?>`).FindStringSubmatch(page)
		if m == nil {
			t.Fatalf("no panel for stage %s", id)
		}
		return html.UnescapeString(m[1])
	}
	headline := map[console.PipelineStageID]console.Value[int]{}
	for _, st := range m.Pipeline.Stages {
		headline[st.ID] = st.Headline.Value
		chip := chipOf(st.ID)
		if chip != "" && !strings.HasPrefix(chip, "fathom ") {
			t.Errorf("stage %s's chip %q is not a CLI command", st.ID, chip)
		}
	}
	for _, id := range []console.PipelineStageID{console.StageSource, console.StageCompile, console.StageGate} {
		if chip := chipOf(id); chip != "" {
			t.Errorf("no CLI verb prints stage %s's headline, yet its chip names %q", id, chip)
		}
	}
	// The agent-surface call that carries compile's counts is offered as a
	// call, labelled as one.
	if !strings.Contains(page, `aria-label="Copy the call staleness_report {&#34;service&#34;:&#34;payment-service&#34;}"`) {
		t.Error("compile's staleness_report call must be offered as an agent-surface call")
	}

	runJSON := func(args ...string) map[string]any {
		t.Helper()
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 {
			t.Fatalf("%v: %d %s", args, code, errb.String())
		}
		var v map[string]any
		if err := json.Unmarshal(out.Bytes(), &v); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return v
	}
	row := func(v map[string]any, key string) map[string]any {
		for _, r := range v[key].([]any) {
			if r.(map[string]any)["service"] == "payment-service" {
				return r.(map[string]any)
			}
		}
		t.Fatalf("no payment-service row under %s", key)
		return nil
	}
	fleet := row(runJSON(append(strings.Fields(chipOf(console.StageExtract))[1:], "--url", f.ts.URL)...), "services")
	if got := headline[console.StageExtract]; !got.Known() || float64(got.V) != fleet["symbols"] {
		t.Errorf("extract headline %+v, `%s` symbols %v", got, chipOf(console.StageExtract), fleet["symbols"])
	}
	if chipOf(console.StageEvidence) != chipOf(console.StageExtract) {
		t.Fatalf("evidence chip = %q", chipOf(console.StageEvidence))
	}
	if got := headline[console.StageEvidence]; !got.Known() || float64(got.V) != fleet["session_count"] {
		t.Errorf("evidence headline %+v, the fleet report's session_count %v", got, fleet["session_count"])
	}
	evidence := row(runJSON(append(strings.Fields(chipOf(console.StageIndex))[1:], "--store", consoleFixtureStorePath(t))...), "services")
	if got := headline[console.StageIndex]; got.Known() && float64(got.V) != evidence["edges"] {
		t.Errorf("index headline %+v, `%s` edges %v", got, chipOf(console.StageIndex), evidence["edges"])
	}
}

// The Learn facts are cached on their inputs' watermarks, never on the
// model's 30-second clock (design §1): a model rebuild with nothing else
// changed re-reads neither the store, the call log, the verify ledger nor
// the capability probe; a call appended to the log is read — and only it.
func TestConsoleLearn_FactsReusedAcrossModelBuilds(t *testing.T) {
	f := newLearnFixture(t, consoleFixtureOpts{})
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), `{"schema_version":2,"at":"2026-09-24T10:00:00Z","service":"payment-service","plan":"docs/plans/a.md","claimed":1,"verified":1}`+"\n")
	first := learnModelOf(t, f.pageJSON(t, "/learn?svc=payment-service"))
	entries := func() [3]int {
		learnStoreCache.Lock()
		learnGateCache.Lock()
		learnCapsCache.Lock()
		defer learnStoreCache.Unlock()
		defer learnGateCache.Unlock()
		defer learnCapsCache.Unlock()
		return [3]int{len(learnStoreCache.entries), len(learnGateCache.entries), len(learnCapsCache.entries)}
	}
	cached, offset := entries(), callLogTail.offset
	if cached != [3]int{1, 1, 1} {
		t.Fatalf("the store, gate and switch reads must each be cached once: %v", cached)
	}

	// A new model build (the 30 s horizon passing) with no input changed.
	f.now = f.now.Add(time.Minute)
	resetObsCache()
	second := learnModelOf(t, f.pageJSON(t, "/learn?svc=payment-service"))
	if got := entries(); got != cached {
		t.Errorf("a model rebuild re-read the Learn facts: cache entries %v → %v", cached, got)
	}
	if callLogTail.offset != offset {
		t.Errorf("a model rebuild re-parsed the call log: offset %d → %d", offset, callLogTail.offset)
	}
	if !reflect.DeepEqual(first.Pipeline.Stages[3].Headline, second.Pipeline.Stages[3].Headline) {
		t.Errorf("compile headline moved with nothing changed: %+v → %+v", first.Pipeline.Stages[3].Headline, second.Pipeline.Stages[3].Headline)
	}

	// A call appended to the log is read, and only the appended bytes.
	fh, err := os.OpenFile(f.callLog, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fh.WriteString(`{"ts":"2026-09-25T11:00:00Z","tool":"obligations","service":"payment-service","surface":"agent","latency_ms":2}` + "\n")
	fh.Close()
	third := learnModelOf(t, f.pageJSON(t, "/learn?svc=payment-service"))
	for _, st := range third.Pipeline.Stages {
		if st.ID == console.StageServe && st.Headline.Value.V != 2 {
			t.Errorf("rules served after one more obligations call = %+v, want 2", st.Headline.Value)
		}
	}
	if callLogTail.offset <= offset {
		t.Error("the appended call must be read")
	}
}

// The is-it-done example comes from the project's plans. Plans the daemon
// cannot read (the external-volume wedge: a directory it cannot list, a
// plan it cannot open) are an error naming what failed — never "no claimed
// plan row was found" — and a cleared wedge is read again at once.
func TestSamplePlanRow_UnreadablePlansAreAnErrorNotNone(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a chmod 000 file")
	}
	t.Setenv("FATHOM_VERIFY_LEDGER", filepath.Join(t.TempDir(), "none.jsonl"))
	root := t.TempDir()
	plan := filepath.Join(root, "docs", "plans", "a.md")
	mustWrite(t, plan, "# A\n\n- [x] charge {symbol: validateCharge}\n")
	choice := learnChoice{Service: "pay", Row: obsServiceRow{}}
	choice.Row.Path = root

	if row, note, err := samplePlanRow(choice); row != "- [x] charge {symbol: validateCharge}" || err != nil || !strings.Contains(note, "docs/plans/a.md") {
		t.Fatalf("readable plans = %q %q %v", row, note, err)
	}
	for _, locked := range []string{filepath.Dir(plan), plan} {
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}
		row, note, err := samplePlanRow(choice)
		_ = os.Chmod(locked, 0o755)
		if row != "" || err == nil || !strings.Contains(note, "could not be read") || strings.Contains(note, "no claimed plan row") {
			t.Errorf("%s unreadable = %q %q %v; want an error, never none found", locked, row, note, err)
		}
	}
	// A project with no plans at all is the honest none.
	empty := learnChoice{Service: "pay"}
	empty.Row.Path = t.TempDir()
	if row, note, err := samplePlanRow(empty); row != "" || err != nil || !strings.Contains(note, "no claimed plan row") {
		t.Errorf("no plans = %q %q %v", row, note, err)
	}

	// The guide carries the failure as the sample's error; a try-it with
	// no input is then an error, not a refusal that reads as "none".
	view := console.AskGuide(console.AskFacts{Service: "pay", Samples: map[string]console.AskSample{
		"row": {Note: "the plans could not be read", Err: "open docs/plans: permission denied"},
	}})
	for _, s := range view.Shapes {
		if s.ID == "is-it-done" && s.SampleError != "open docs/plans: permission denied" {
			t.Errorf("is-it-done sample error = %q", s.SampleError)
		}
	}
}

// A guide's outline rail sticks: the kit shell's main is a scroll
// container that never scrolls, so a page that positions something sticky
// frees it (data-console-sticky, the body's :has() classes, present in the
// built CSS); the rail is one bounded sticky block with its related links,
// and every entry is a 44 px target.
func TestConsoleShell_StickyPagesFreeMain(t *testing.T) {
	if !strings.Contains(consoleStickyMainClasses, "#"+consoleContentID+"]") {
		t.Fatalf("the sticky classes must name the shell's main (#%s): %s", consoleContentID, consoleStickyMainClasses)
	}
	css := string(obsAppCSS)
	for _, rule := range []string{":has([data-console-sticky]) #observatory-main", "overflow-x: clip", "overflow-y: visible"} {
		if !strings.Contains(strings.ReplaceAll(css, "\\", ""), rule) {
			t.Errorf("the built CSS lacks %q — run task ui:css", rule)
		}
	}
	f := newLearnFixture(t, consoleFixtureOpts{})
	_, _, page := f.get(t, "/learn/guides/getting-started")
	if !strings.Contains(page, `<body class="min-h-screen bg-surface font-sans text-sm text-ink `+consoleStickyMainClasses+`">`) || !strings.Contains(page, "data-guide data-console-sticky") {
		t.Error("a guide must mark itself sticky and the shell must free its main")
	}
	rail := page[strings.Index(page, "data-guide-rail>"):]
	rail = rail[:strings.Index(rail, "</aside>")]
	if !strings.Contains(rail, `data-guide-rail-sticky`) || !strings.Contains(rail, "[&_.sticky]:static") || !strings.Contains(rail, "[&_nav_a]:min-h-11") {
		t.Error("the rail must be one bounded sticky block whose links are 44 px targets")
	}
	if strings.Index(rail, "data-guide-related") > strings.Index(rail, `aria-label="Table of contents"`) {
		t.Error("the related links lead the rail")
	}
	// Other pages keep the shell as it is.
	if _, _, learn := f.get(t, "/learn"); strings.Contains(strings.ReplaceAll(learn, consoleStickyMainClasses, ""), "data-console-sticky") {
		t.Error("only a page that positions something sticky frees the shell's main")
	}
}

// No word breaks inside itself where the layout squeezes it: a verb chip's
// Copy button never shrinks or wraps (a long `mcp add` line in a dd that
// breaks anywhere squeezed it to "Cop / y"), and a short single-word cell
// of a try-it answer ("resolved") stays whole while long paths still break.
func TestConsoleLearnAsk_NoMidWordBreaks(t *testing.T) {
	chip := renderVerbChip("claude mcp add --transport http fathom http://127.0.0.1:7575/rpc/agent")
	if !strings.Contains(chip, "shrink-0") || !strings.Contains(chip, "whitespace-nowrap") || !strings.Contains(chip, `aria-label="Copy the line claude mcp add`) {
		t.Errorf("the copy button must neither shrink nor wrap, and name what it copies: %s", chip)
	}
	if got := askCell("resolved"); !strings.Contains(got, "whitespace-nowrap") || strings.Contains(got, "anywhere") {
		t.Errorf("a short word cell = %s", got)
	}
	if got := askCell("internal/indexbench/freshness.go:107 FreshnessProbe"); !strings.Contains(got, "[overflow-wrap:anywhere]") {
		t.Errorf("a long path cell must still break anywhere: %s", got)
	}
	f := newLearnFixture(t, consoleFixtureOpts{})
	_, _, tried := f.get(t, "/learn/ask?try=who-calls")
	if !strings.Contains(tried, `<span class="whitespace-nowrap">heuristic</span>`) {
		t.Error("the answer's edge column must keep its word whole")
	}
}
