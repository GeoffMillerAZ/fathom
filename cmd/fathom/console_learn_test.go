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
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

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
	// Every stage link is a page the console serves.
	reg := defaultConsoleRegistry()
	links := regexp.MustCompile(`href="([^"]+)"[^>]*data-stage-link`).FindAllStringSubmatch(page, -1)
	if len(links) != 7 {
		t.Fatalf("stage links = %d, want 7", len(links))
	}
	for _, l := range links {
		path, _, _ := strings.Cut(strings.ReplaceAll(l[1], "&amp;", "&"), "?")
		if _, _, ok := resolveConsoleHref(reg, []string{path}); !ok {
			t.Errorf("stage link %s resolves to no registered page", l[1])
		}
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
	g := readVerifyLedgerFor(os.Getenv("FATHOM_VERIFY_LEDGER"), "payment-service")
	if g.runs.V != 2 || g.plans.V != 1 || g.verified.V != 3 || g.claimed.V != 3 {
		t.Errorf("gate from the ledger = %+v", g)
	}
	if g := readVerifyLedgerFor(os.Getenv("FATHOM_VERIFY_LEDGER"), "checkout-service"); g.runs.State != console.StateZero || g.verified.State != console.StateAbsent {
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
// design's page once a wave registers it, the project workspace's tab until
// then.
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
	pages := []string{"/packs", "/projects/pay?tab=index"}
	if href, _, ok := resolveConsoleHref(reg, pages); !ok || href != "/projects/pay?tab=index" {
		t.Errorf("without /packs = %q %v", href, ok)
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
