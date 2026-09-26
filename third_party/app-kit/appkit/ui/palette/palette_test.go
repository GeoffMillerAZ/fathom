package palette

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/overlay"
	"golang.org/x/net/html"
)

// ---- golden helpers (same contract as appkit/ui/vizframe) -------------------
// testdata/<name>.html.golden; regenerate with UPDATE_GOLDEN=1 go test ./appkit/ui/palette.

func assertGolden(t *testing.T, got, name string) {
	t.Helper()
	path := filepath.Join("testdata", name+".html.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 go test to generate)", path, err)
	}
	if string(want) != got {
		t.Errorf("output mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, want)
	}
}

// ---- fixtures -----------------------------------------------------------------

func fixture() Palette {
	return Palette{ID: "cmd", Action: "/search"}
}

func fixtureResults() Results {
	return Results{
		Query: "orbit",
		Groups: []Group{
			{Heading: "Projects", Results: []Result{
				{Label: "orbit-service", Href: "/projects/orbit-service", Detail: "Indexed 4 minutes ago", Meta: "Project"},
				{Label: "orbit-web", Href: "/projects/orbit-web"},
			}},
			{Heading: "Empty source", Results: nil},
			{Heading: "Pages", Results: []Result{
				{Label: "Orbit wiring", Href: "/wiring?service=orbit", Meta: "Page"},
			}},
		},
	}
}

func parse(t *testing.T, markup string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func findAll(n *html.Node, match func(*html.Node) bool) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func hasAttr(key string) func(*html.Node) bool {
	return func(n *html.Node) bool { _, ok := attr(n, key); return ok }
}

func tag(name string) func(*html.Node) bool {
	return func(n *html.Node) bool { return n.Data == name }
}

func classes(n *html.Node) []string {
	v, _ := attr(n, "class")
	return strings.Fields(v)
}

func hasClass(n *html.Node, want ...string) bool {
	have := map[string]bool{}
	for _, c := range classes(n) {
		have[c] = true
	}
	for _, w := range want {
		if !have[w] {
			return false
		}
	}
	return true
}

// ---- trigger ------------------------------------------------------------------

func TestRenderTriggerIsARealLinkWithTargetAndShortcut(t *testing.T) {
	got, err := RenderTrigger(fixture())
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, got, "trigger")
	doc := parse(t, got)
	links := findAll(doc, tag("a"))
	if len(links) != 1 {
		t.Fatalf("trigger must be exactly one link: %s", got)
	}
	a := links[0]
	if href, _ := attr(a, "href"); href != "/search" {
		t.Errorf("trigger must navigate to the search page without JS, href=%q", href)
	}
	if v, _ := attr(a, "data-palette-open"); v != "cmd" {
		t.Errorf("data-palette-open=%q", v)
	}
	if !hasClass(a, "min-h-11", "min-w-11", "focus-visible:outline-2", "focus-visible:outline-focus") {
		t.Errorf("trigger needs a 44px target and visible focus: %v", classes(a))
	}
	// Only palette.js implements the shortcut, so the server never advertises it:
	// without the script (or before it runs) there is no aria-keyshortcuts and no
	// visible hint. The script fills in and reveals the platform's hint.
	if v, ok := attr(a, "aria-keyshortcuts"); ok {
		t.Errorf("no-JavaScript trigger advertises a shortcut that does not exist: aria-keyshortcuts=%q", v)
	}
	kbd := findAll(doc, tag("kbd"))
	if len(kbd) != 1 || kbd[0].FirstChild != nil {
		t.Fatalf("want one empty hint placeholder for the script: %s", got)
	}
	if _, hidden := attr(kbd[0], "hidden"); !hidden {
		t.Error("the shortcut hint must start hidden; the script reveals it once the shortcut works")
	}
	if _, ok := attr(kbd[0], "data-palette-hint"); !ok {
		t.Error("the hint placeholder needs its script hook")
	}
	if v, _ := attr(kbd[0], "aria-hidden"); v != "true" {
		t.Error("the visual hint must not pollute the link's accessible name")
	}

	none := fixture()
	none.Shortcut = "none"
	plain, err := RenderTrigger(none)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "aria-keyshortcuts") || strings.Contains(plain, "<kbd") {
		t.Errorf("disabled shortcut still advertised: %s", plain)
	}

	custom := fixture()
	custom.Shortcut = "mod+shift+p"
	withShift, err := RenderTrigger(custom)
	if err != nil {
		t.Fatal(err)
	}
	if withShift != got {
		t.Errorf("the trigger carries no shortcut text; the dialog's data-palette-shortcut does: %s", withShift)
	}
}

// ---- dialog -------------------------------------------------------------------

func TestRenderDialogBuildsOnOverlayWithComboboxContract(t *testing.T) {
	got, err := RenderDialog(fixture())
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, got, "dialog")
	if !strings.HasPrefix(got, `<dialog id="cmd" `) || strings.Count(got, "<dialog") != 1 {
		t.Fatalf("dialog must be one overlay.RenderDialog frame: %.120s", got)
	}
	for _, want := range []string{"max-w-2xl", "data-overlay-light-dismiss", "data-overlay-panel", "data-overlay-dismiss", `aria-label="Close"`} {
		if !strings.Contains(got, want) {
			t.Errorf("overlay frame lost %q", want)
		}
	}
	doc := parse(t, got)

	roots := findAll(doc, hasAttr("data-palette"))
	if len(roots) != 1 {
		t.Fatalf("want one data-palette root, got %d", len(roots))
	}
	for key, want := range map[string]string{
		"data-palette":          "cmd",
		"data-palette-url":      "/search",
		"data-palette-param":    "q",
		"data-palette-shortcut": "mod+k",
		"data-palette-debounce": "150",
		"data-palette-timeout":  "8000",
		"data-palette-label":    "Search",
	} {
		if v, _ := attr(roots[0], key); v != want {
			t.Errorf("%s=%q, want %q", key, v, want)
		}
	}

	forms := findAll(doc, tag("form"))
	if len(forms) != 1 {
		t.Fatalf("want one form, got %d", len(forms))
	}
	for key, want := range map[string]string{"role": "search", "action": "/search", "method": "get"} {
		if v, _ := attr(forms[0], key); v != want {
			t.Errorf("form %s=%q, want %q (Enter without a selection must reach the GET page)", key, v, want)
		}
	}
	inputs := findAll(doc, tag("input"))
	if len(inputs) != 1 {
		t.Fatalf("want one input, got %d", len(inputs))
	}
	in := inputs[0]
	for key, want := range map[string]string{
		"id": "cmd-input", "name": "q", "type": "search", "role": "combobox",
		"aria-autocomplete": "list", "aria-expanded": "false", "aria-controls": "cmd-results", "autocomplete": "off",
	} {
		if v, _ := attr(in, key); v != want {
			t.Errorf("input %s=%q, want %q", key, v, want)
		}
	}
	if !hasClass(in, "min-h-11", "focus-visible:outline-focus") {
		t.Errorf("input needs a 44px target and focus ring: %v", classes(in))
	}
	labels := findAll(doc, func(n *html.Node) bool { v, _ := attr(n, "for"); return n.Data == "label" && v == "cmd-input" })
	if len(labels) != 1 {
		t.Error("input needs exactly one associated label")
	}

	ids := map[string]int{}
	for _, n := range findAll(doc, hasAttr("id")) {
		v, _ := attr(n, "id")
		ids[v]++
	}
	for id, n := range ids {
		if n != 1 {
			t.Errorf("duplicate id %q", id)
		}
	}
	if ids["cmd-results"] != 1 {
		t.Error("aria-controls target missing")
	}
	for _, hook := range []string{"data-palette-results", "data-palette-status", "data-palette-busy", "data-palette-page", "data-palette-error", "data-palette-input", "data-palette-form"} {
		if len(findAll(doc, hasAttr(hook))) != 1 {
			t.Errorf("want exactly one %s hook", hook)
		}
	}
	busy := findAll(doc, hasAttr("data-palette-busy"))[0]
	if _, hidden := attr(busy, "hidden"); !hidden {
		t.Error("busy indicator must start hidden")
	}
	status := findAll(doc, hasAttr("data-palette-status"))[0]
	if v, _ := attr(status, "role"); v != "status" {
		t.Error("announcements need a status live region")
	}
	page := findAll(doc, hasAttr("data-palette-page"))[0]
	if v, _ := attr(page, "href"); v != "/search" || !hasClass(page, "min-h-11") {
		t.Errorf("search-page link must be a real 44px link: %v", page.Attr)
	}
	results := findAll(doc, hasAttr("data-palette-results"))[0]
	idle := findAll(results, hasAttr("data-palette-fragment"))
	if len(idle) != 1 {
		t.Fatal("results region must start with the idle fragment")
	}
	if v, _ := attr(idle[0], "data-palette-state"); v != "idle" {
		t.Errorf("initial state %q, want idle", v)
	}
	if !strings.Contains(got, "<template data-palette-error><div data-palette-fragment data-palette-state=\"error\"") {
		t.Error("client failure template must hold an error fragment")
	}
}

func TestRenderDialogCarriesOverrides(t *testing.T) {
	p := Palette{
		ID: "find", Action: "/find", ResultsURL: "/find/fragment?scope=all", Param: "term",
		Label: "Find anything", Placeholder: "Projects, plans, verbs", Shortcut: "/",
		Debounce: 40 * time.Millisecond, Timeout: 3 * time.Second,
		IdleText: "Search projects and plans.", ErrorText: "The index is offline.",
	}
	got, err := RenderDialog(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`data-palette-url="/find/fragment?scope=all"`, `data-palette-param="term"`, `data-palette-shortcut="/"`,
		`data-palette-debounce="40"`, `data-palette-timeout="3000"`, `name="term"`, `placeholder="Projects, plans, verbs"`,
		`action="/find"`, "Find anything", "Search projects and plans.", "The index is offline.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("override missing %q", want)
		}
	}
}

// Action's query is app context: a GET form would drop it, so both forms carry it
// as hidden fields while the trigger and page link keep the full URL.
func TestActionQueryTravelsAsHiddenFields(t *testing.T) {
	p := Palette{ID: "cmd", Action: "/search?theme=dark&scope=a&scope=b"}
	dialog, err := RenderDialog(p)
	if err != nil {
		t.Fatal(err)
	}
	form, err := RenderSearchForm(p, "x")
	if err != nil {
		t.Fatal(err)
	}
	trigger, err := RenderTrigger(p)
	if err != nil {
		t.Fatal(err)
	}
	hidden := `<input type="hidden" name="scope" value="a"><input type="hidden" name="scope" value="b"><input type="hidden" name="theme" value="dark">`
	for name, got := range map[string]string{"dialog": dialog, "form": form} {
		if !strings.Contains(got, `action="/search" method="get"`) || !strings.Contains(got, hidden) {
			t.Errorf("%s form lost app context: %s", name, got)
		}
	}
	if !strings.Contains(dialog, `data-palette-action="/search?theme=dark&amp;scope=a&amp;scope=b"`) || !strings.Contains(dialog, `data-palette-url="/search?theme=dark&amp;scope=a&amp;scope=b"`) {
		t.Error("script configuration must keep the full action and default fragment URL")
	}
	if !strings.Contains(dialog, `href="/search?theme=dark&amp;scope=a&amp;scope=b" data-palette-page`) || !strings.Contains(trigger, `href="/search?theme=dark&amp;scope=a&amp;scope=b"`) {
		t.Error("links must keep the full action URL")
	}
}

// ---- search page form ----------------------------------------------------------

func TestRenderSearchFormIsTheNoScriptPath(t *testing.T) {
	got, err := RenderSearchForm(fixture(), `orbit "service"`)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, got, "search-form")
	doc := parse(t, got)
	form := findAll(doc, tag("form"))
	if len(form) != 1 {
		t.Fatal("want one form")
	}
	if v, _ := attr(form[0], "method"); v != "get" {
		t.Error("search page form must be GET")
	}
	in := findAll(doc, tag("input"))[0]
	if v, _ := attr(in, "value"); v != `orbit "service"` {
		t.Errorf("query not prefilled: %q", v)
	}
	button := findAll(doc, tag("button"))
	if len(button) != 1 || !hasClass(button[0], "min-h-11") {
		t.Error("submit button must be a 44px target")
	}
	if len(findAll(doc, func(n *html.Node) bool { v, _ := attr(n, "for"); return n.Data == "label" && v == "cmd-page-input" })) != 1 {
		t.Error("page input needs a visible associated label")
	}
}

// ---- results fragment -----------------------------------------------------------

func TestRenderResultsStates(t *testing.T) {
	results, err := RenderResults(fixtureResults())
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, results, "results")
	doc := parse(t, results)
	frag := findAll(doc, hasAttr("data-palette-fragment"))[0]
	for key, want := range map[string]string{
		"data-palette-state": "results", "data-palette-query": "orbit", "data-palette-count": "3",
		"data-palette-announce": "3 results for “orbit”",
	} {
		if v, _ := attr(frag, key); v != want {
			t.Errorf("%s=%q, want %q", key, v, want)
		}
	}
	if n := len(findAll(doc, hasAttr("data-palette-group"))); n != 2 {
		t.Errorf("empty groups must be omitted: %d groups", n)
	}
	if n := len(findAll(doc, tag("h3"))); n != 2 {
		t.Errorf("default heading level is h3: %d", n)
	}
	options := findAll(doc, hasAttr("data-palette-option"))
	if len(options) != 3 {
		t.Fatalf("want 3 options, got %d", len(options))
	}
	for _, o := range options {
		if o.Data != "a" {
			t.Errorf("options must be real links, got <%s>", o.Data)
		}
		if !hasClass(o, "min-h-11", "aria-selected:bg-accent-soft", "aria-selected:outline-2", "aria-selected:outline-focus", "focus-visible:outline-2") {
			t.Errorf("option needs 44px, non-color-only selection and focus styling: %v", classes(o))
		}
		if _, ok := attr(o, "role"); ok {
			t.Error("the fragment stays plain links; palette.js adds option roles")
		}
	}
	if len(findAll(doc, hasAttr("data-palette-list"))) != 1 {
		t.Error("want one listbox candidate")
	}

	page, err := RenderResults(Results{Query: "orbit", HeadingLevel: 2, Groups: fixtureResults().Groups})
	if err != nil || strings.Count(page, "<h2 ") != 2 || strings.Contains(page, "<h3") {
		t.Errorf("page heading level not honored: %v", err)
	}

	one, _ := RenderResults(Results{Query: "x", Groups: []Group{{Heading: "Pages", Results: []Result{{Label: "X", Href: "/x"}}}}})
	if !strings.Contains(one, `data-palette-announce="1 result for “x”"`) {
		t.Errorf("singular summary: %s", one)
	}

	for _, tc := range []struct {
		name  string
		in    Results
		state string
		wants []string
	}{
		{"idle blank query", Results{Query: "  "}, "idle", []string{DefaultIdleText, `data-palette-announce=""`}},
		{"idle custom", Results{Message: "Search projects."}, "idle", []string{"Search projects."}},
		{"empty", Results{Query: "zzz"}, "empty", []string{"No results for “zzz”", "Check the spelling", `data-palette-count="0"`, `data-palette-announce="No results for “zzz”"`}},
		{"empty groups only", Results{Query: "zzz", Groups: []Group{{Heading: "Projects"}}}, "empty", []string{"No results for “zzz”"}},
		// A source was not searched, so zero results is not a definitive absence.
		{"empty with notice", Results{Query: "zzz", Notice: "Symbols are unavailable."}, "partial", []string{"No results for “zzz”", "Partial results: ", "Symbols are unavailable.", `data-palette-count="0"`, `data-palette-announce="No results for “zzz” (partial)"`, "Matches may exist in sources that were not searched."}},
		{"partial", Results{Query: "orbit", Groups: fixtureResults().Groups, Notice: "The symbol index is stale."}, "partial", []string{"Partial results: ", "(partial)", "border-caution"}},
		{"error", Results{Query: "orbit", Error: "The index is locked."}, "error", []string{"Search failed", "The index is locked.", "border-danger", `data-palette-announce="Search failed. The index is locked."`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenderResults(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, `data-palette-state="`+tc.state+`"`) {
				t.Errorf("state: %s", got)
			}
			for _, want := range tc.wants {
				if !strings.Contains(got, want) {
					t.Errorf("missing %q in %s", want, got)
				}
			}
			if tc.state != "partial" && strings.Contains(got, "data-palette-option") {
				t.Error("non-result states must not render options")
			}
			if strings.Contains(got, "Partial results: ") && strings.Contains(got, "Check the spelling") {
				t.Error("spelling advice claims a definitive absence while a source was not searched")
			}
		})
	}
}

func TestRenderResultsLongLabelsWrap(t *testing.T) {
	long := strings.Repeat("internal/connector/extract/", 8) + "ReallyLongSymbolName"
	got, err := RenderResults(Results{Query: long, Groups: []Group{{Heading: "Symbols", Results: []Result{{Label: long, Href: "/s", Detail: long}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(got, "[overflow-wrap:anywhere]") < 2 || !strings.Contains(got, "min-w-0 flex-1") {
		t.Errorf("long identifiers must wrap instead of overflowing the dialog: %s", got)
	}
}

// ---- refusals -------------------------------------------------------------------

func TestInvalidConfigurationIsRefusedBeforeOutput(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Palette)
	}{
		{"missing id", func(p *Palette) { p.ID = "" }},
		{"id with space", func(p *Palette) { p.ID = "a b" }},
		{"id starting with digit", func(p *Palette) { p.ID = "1cmd" }},
		{"id too long", func(p *Palette) { p.ID = "a" + strings.Repeat("b", 64) }},
		{"missing action", func(p *Palette) { p.Action = "" }},
		{"absolute action", func(p *Palette) { p.Action = "https://example.test/search" }},
		{"protocol-relative action", func(p *Palette) { p.Action = "//evil.test/search" }},
		{"script action", func(p *Palette) { p.Action = "javascript:alert(1)" }},
		{"action query sets the search param", func(p *Palette) { p.Action = "/search?q=preset" }},
		{"action query sets a custom search param", func(p *Palette) { p.Action = "/search?term=x"; p.Param = "term" }},
		{"action without path", func(p *Palette) { p.Action = "?scope=all" }},
		{"malformed action query", func(p *Palette) { p.Action = "/search?scope=%zz" }},
		{"action fragment", func(p *Palette) { p.Action = "/search#top" }},
		{"action whitespace", func(p *Palette) { p.Action = " /search" }},
		{"cross-origin results", func(p *Palette) { p.ResultsURL = "https://example.test/fragment" }},
		{"results fragment id", func(p *Palette) { p.ResultsURL = "/fragment#x" }},
		{"bad param", func(p *Palette) { p.Param = "q=1" }},
		{"blank label", func(p *Palette) { p.Label = "   " }},
		{"control in placeholder", func(p *Palette) { p.Placeholder = "a\x00b" }},
		{"bad shortcut key", func(p *Palette) { p.Shortcut = "mod+enter" }},
		{"unknown modifier", func(p *Palette) { p.Shortcut = "hyper+k" }},
		{"repeated modifier", func(p *Palette) { p.Shortcut = "shift+shift+k" }},
		{"mod with ctrl", func(p *Palette) { p.Shortcut = "mod+ctrl+k" }},
		{"uppercase shortcut", func(p *Palette) { p.Shortcut = "Mod+K" }},
		{"negative debounce", func(p *Palette) { p.Debounce = -time.Millisecond }},
		{"huge debounce", func(p *Palette) { p.Debounce = 3 * time.Second }},
		{"tiny timeout", func(p *Palette) { p.Timeout = 10 * time.Millisecond }},
		{"huge timeout", func(p *Palette) { p.Timeout = 2 * time.Minute }},
		{"blank error text", func(p *Palette) { p.ErrorText = "\t" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fixture()
			tc.edit(&p)
			for name, render := range map[string]func() (string, error){
				"trigger": func() (string, error) { return RenderTrigger(p) },
				"dialog":  func() (string, error) { return RenderDialog(p) },
				"form":    func() (string, error) { return RenderSearchForm(p, "q") },
			} {
				got, err := render()
				if err == nil || got != "" {
					t.Errorf("%s accepted invalid config (err=%v, %d bytes)", name, err, len(got))
				}
			}
		})
	}
}

func TestInvalidResultsAreRefusedBeforeOutput(t *testing.T) {
	ok := Result{Label: "A", Href: "/a"}
	many := make([]Result, MaxResults+1)
	for i := range many {
		many[i] = ok
	}
	for _, tc := range []struct {
		name string
		in   Results
	}{
		{"blank heading", Results{Query: "a", Groups: []Group{{Heading: " ", Results: []Result{ok}}}}},
		{"blank label", Results{Query: "a", Groups: []Group{{Heading: "G", Results: []Result{{Label: "", Href: "/a"}}}}}},
		{"empty href", Results{Query: "a", Groups: []Group{{Heading: "G", Results: []Result{{Label: "A"}}}}}},
		{"script href", Results{Query: "a", Groups: []Group{{Heading: "G", Results: []Result{{Label: "A", Href: "javascript:alert(1)"}}}}}},
		{"data href", Results{Query: "a", Groups: []Group{{Heading: "G", Results: []Result{{Label: "A", Href: "data:text/html,x"}}}}}},
		{"protocol-relative href", Results{Query: "a", Groups: []Group{{Heading: "G", Results: []Result{{Label: "A", Href: "//evil.test"}}}}}},
		{"error with results", Results{Query: "a", Error: "boom", Groups: []Group{{Heading: "G", Results: []Result{ok}}}}},
		{"blank notice", Results{Query: "a", Notice: " "}},
		{"heading level 1", Results{Query: "a", HeadingLevel: 1}},
		{"heading level 7", Results{Query: "a", HeadingLevel: 7}},
		{"too many groups", Results{Query: "a", Groups: make([]Group, MaxGroups+1)}},
		{"too many results", Results{Query: "a", Groups: []Group{{Heading: "G", Results: many}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := RenderResults(tc.in); err == nil || got != "" {
				t.Errorf("accepted invalid results (err=%v): %s", err, got)
			}
		})
	}
	if _, err := RenderResults(Results{Query: "a", Groups: []Group{{Heading: "G", Results: []Result{{Label: "Docs", Href: "https://docs.example.test/x"}}}}}); err != nil {
		t.Errorf("absolute HTTPS destinations are allowed: %v", err)
	}
}

// ---- safety and tokens -------------------------------------------------------------

var (
	hexColor    = regexp.MustCompile(`(?i)#[0-9a-f]{3,8}\b`)
	darkVariant = regexp.MustCompile(`\bdark:`)
)

func TestEscapingAndRoleUtilitiesOnly(t *testing.T) {
	xss := `"><script>alert(1)</script><img src=x onerror=alert(2)>`
	p := Palette{ID: "cmd", Action: "/search", Label: xss, Placeholder: xss, IdleText: xss, ErrorText: xss}
	var all strings.Builder
	for _, render := range []func() (string, error){
		func() (string, error) { return RenderTrigger(p) },
		func() (string, error) { return RenderDialog(p) },
		func() (string, error) { return RenderSearchForm(p, xss) },
		func() (string, error) {
			return RenderResults(Results{Query: xss, Notice: xss, Groups: []Group{{Heading: xss, Results: []Result{{Label: xss, Href: "/a?b=" + "%22", Detail: xss, Meta: xss}}}}})
		},
		func() (string, error) { return RenderResults(Results{Query: xss}) },
		func() (string, error) { return RenderResults(Results{Query: xss, Error: xss}) },
	} {
		got, err := render()
		if err != nil {
			t.Fatal(err)
		}
		all.WriteString(got)
	}
	out := all.String()
	if strings.Contains(out, "<script>") || strings.Contains(out, "<img") || strings.Contains(out, "onerror=alert") && !strings.Contains(out, "&lt;img src=x onerror=alert(2)&gt;") {
		t.Errorf("unescaped hostile text:\n%s", out)
	}
	if hexColor.MatchString(out) || strings.Contains(out, "style=") {
		t.Error("colors must come from role utilities, not literals or inline styles")
	}
	if darkVariant.MatchString(out) {
		t.Error("role utilities already theme both schemes; no dark: variants")
	}
}

func TestDeterministic(t *testing.T) {
	f := func() string {
		d, _ := RenderDialog(fixture())
		r, _ := RenderResults(fixtureResults())
		tr, _ := RenderTrigger(fixture())
		return d + r + tr
	}
	if f() != f() {
		t.Error("rendering is non-deterministic")
	}
}

// The dialog frame is overlay.RenderDialog's; if its markup changes, the palette
// inherits the change rather than drifting from a private copy.
func TestDialogFrameIsOverlayOutput(t *testing.T) {
	got, err := RenderDialog(fixture())
	if err != nil {
		t.Fatal(err)
	}
	frame := overlay.RenderDialog(overlay.Dialog{ID: "cmd", Title: "Search", Size: "lg", LightDismiss: true, Body: "BODY", Footer: "FOOTER"})
	head, _, _ := strings.Cut(frame, "BODY")
	if !strings.HasPrefix(got, head) {
		t.Errorf("palette dialog does not start with overlay.RenderDialog's frame")
	}
}

// ---- request helpers and script --------------------------------------------------------

func TestWantsFragment(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/search?q=a", nil)
	if WantsFragment(r) {
		t.Error("plain navigation must get the full page")
	}
	r.Header.Set(FragmentHeader, "1")
	if !WantsFragment(r) {
		t.Error("palette.js requests must get the fragment")
	}
}

func TestScriptContractAndHandler(t *testing.T) {
	js := Script()
	for _, want := range []string{
		"window.AppKitPalette", "w.AppKitPalette = {", "X-AppKit-Palette", "aria-activedescendant", "AbortController",
		"data-palette-open", "data-palette-option", "data-palette-fragment", "showModal", `"ArrowDown"`, `"ArrowUp"`, `"Enter"`, `"Escape"`,
	} {
		if !strings.Contains(js, want) {
			t.Errorf("palette.js missing %q", want)
		}
	}
	if strings.Contains(js, "import ") || strings.Contains(js, "require(") || strings.Contains(js, "class=\"") {
		t.Error("palette.js must stay a dependency-free classic script with no markup classes")
	}

	h := Handler()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/palette.js", nil))
	if rec.Code != 200 || rec.Body.String() != js || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/javascript") || rec.Header().Get("ETag") == "" {
		t.Errorf("handler: %d %q", rec.Code, rec.Header())
	}
	cond := httptest.NewRequest(http.MethodGet, "/palette.js", nil)
	cond.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, cond)
	if rec2.Code != http.StatusNotModified {
		t.Errorf("conditional GET = %d", rec2.Code)
	}
	rec3 := httptest.NewRecorder()
	h.ServeHTTP(rec3, httptest.NewRequest(http.MethodPost, "/palette.js", nil))
	if rec3.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d", rec3.Code)
	}
}
