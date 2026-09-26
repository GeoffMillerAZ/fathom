package live

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

func sampleRegion() Region {
	return Region{
		ID:          "plans",
		Sources:     []string{"plans:fathom", "verify:fathom"},
		Label:       "Plan status for fathom",
		AsOf:        "As of 12:03 UTC",
		RefreshHref: "/projects/fathom",
		Body:        template.HTML(`<h2 class="text-lg">Plans</h2><p>12 proven</p>`),
	}
}

func assertGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".html.golden")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run UPDATE_GOLDEN=1 go test to generate)", path, err)
	}
	if got != string(want) {
		t.Errorf("%s differs from golden:\n got: %s\nwant: %s", name, got, want)
	}
}

func TestRenderRegionGoldenAndDeterministic(t *testing.T) {
	first, err := RenderRegion(sampleRegion())
	if err != nil {
		t.Fatal(err)
	}
	second, _ := RenderRegion(sampleRegion())
	if first != second {
		t.Fatal("same region rendered differently")
	}
	assertGolden(t, "region", first)
	page, err := RenderPageStatus(Page{URL: "/events", Events: []string{"ingest", "verify"}, LastEventID: "42"})
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "page_status", page)
}

func parse(t *testing.T, markup string) *xhtml.Node {
	t.Helper()
	doc, err := xhtml.Parse(strings.NewReader(markup))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func attr(n *xhtml.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func find(n *xhtml.Node, match func(*xhtml.Node) bool) []*xhtml.Node {
	var out []*xhtml.Node
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && match(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return out
}

func hasAttr(key string) func(*xhtml.Node) bool {
	return func(n *xhtml.Node) bool { _, ok := attr(n, key); return ok }
}

func classes(n *xhtml.Node) map[string]bool {
	v, _ := attr(n, "class")
	out := map[string]bool{}
	for _, c := range strings.Fields(v) {
		out[c] = true
	}
	return out
}

func text(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// The updated affordance shares the as-of cell and starts invisible, so marking
// a region reserves no new space and exposes nothing before the script runs.
func TestRegionReservesTheUpdatedAffordanceWithoutClaimingIt(t *testing.T) {
	markup, err := RenderRegion(sampleRegion())
	if err != nil {
		t.Fatal(err)
	}
	doc := parse(t, markup)
	sections := find(doc, hasAttr("data-live-source"))
	if len(sections) != 1 || sections[0].Data != "section" {
		t.Fatalf("want one live section, got %d", len(sections))
	}
	s := sections[0]
	for key, want := range map[string]string{"id": "plans", "tabindex": "-1", "data-live-source": "plans:fathom verify:fathom", "data-live-label": "Plan status for fathom", "aria-label": "Plan status for fathom"} {
		if got, _ := attr(s, key); got != want {
			t.Errorf("section %s = %q, want %q", key, got, want)
		}
	}
	if !strings.Contains(markup, `<h2 class="text-lg">Plans</h2><p>12 proven</p>`) {
		t.Error("app-owned body was altered")
	}
	asof, updated := find(s, hasAttr("data-live-asof")), find(s, hasAttr("data-live-updated"))
	if len(asof) != 1 || len(updated) != 1 {
		t.Fatalf("status layers: asof=%d updated=%d", len(asof), len(updated))
	}
	for _, layer := range []*xhtml.Node{asof[0], updated[0]} {
		c := classes(layer)
		if !c["col-start-1"] || !c["row-start-1"] {
			t.Error("status layers must share one grid cell so revealing moves nothing")
		}
	}
	if !classes(updated[0])["invisible"] || classes(asof[0])["invisible"] {
		t.Error("only the updated layer starts invisible")
	}
	if _, hidden := attr(updated[0], "hidden"); hidden {
		t.Error("display:none would drop the reserved space; use visibility")
	}
	if !strings.HasPrefix(text(updated[0]), "As of 12:03 UTC · ") {
		t.Errorf("updated layer must keep the rendered freshness: %q", text(updated[0]))
	}
	links := find(updated[0], func(n *xhtml.Node) bool { return n.Data == "a" })
	if len(links) != 1 {
		t.Fatal("want one refresh link")
	}
	if href, _ := attr(links[0], "href"); href != "/projects/fathom#plans" {
		t.Errorf("refresh href = %q", href)
	}
	if _, ok := attr(links[0], "data-live-refresh"); !ok {
		t.Error("refresh link lacks its script hook")
	}
	if c := classes(links[0]); !c["py-3.5"] || !c["focus-visible:outline-focus"] || c["inline-flex"] {
		t.Error("refresh link needs an inline 44px padded target and visible focus")
	}
	if got := strings.TrimSpace(text(links[0])); got != "refresh this panel (Plan status for fathom)" {
		t.Errorf("refresh link accessible text = %q", got)
	}
	status := find(s, hasAttr("data-live-status"))
	if len(status) != 1 || !classes(status[0])["grid"] || !classes(status[0])["py-3.5"] {
		t.Fatal("status line must be one grid whose padding contains the link target")
	}
}

// Under Tailwind v4, focus:outline-none sets --tw-outline-style:none on :focus
// and focus-visible:outline-2 reads that variable, so the pair paints no ring at
// all. The region is focused after a refresh; its ring must actually render.
func TestFocusRingsAreNotCancelled(t *testing.T) {
	region, err := RenderRegion(sampleRegion())
	if err != nil {
		t.Fatal(err)
	}
	page, err := RenderPageStatus(Page{URL: "/events"})
	if err != nil {
		t.Fatal(err)
	}
	doc := parse(t, region+page)
	ringed := 0
	for _, n := range find(doc, hasAttr("class")) {
		c := classes(n)
		if !c["focus-visible:outline-2"] {
			continue
		}
		ringed++
		if c["focus:outline-none"] || c["outline-none"] || c["focus-visible:outline-none"] {
			t.Errorf("<%s> cancels its own focus-visible outline: %v", n.Data, c)
		}
		if !c["focus-visible:outline-focus"] {
			t.Errorf("<%s> focus ring lacks the focus color role", n.Data)
		}
	}
	if ringed != 3 {
		t.Errorf("want focus rings on the region, refresh link and control, got %d", ringed)
	}
	section := find(doc, func(n *xhtml.Node) bool { return n.Data == "section" })[0]
	if !classes(section)["focus-visible:outline-2"] {
		t.Error("the refocused region needs a visible focus ring")
	}
}

func TestRegionAttrsAndStatusComposeIntoAppOwnedWrappers(t *testing.T) {
	r := sampleRegion()
	r.RefreshHref = ""
	attrs, err := RegionAttrs(r)
	if err != nil {
		t.Fatal(err)
	}
	status, err := RenderStatus(r)
	if err != nil {
		t.Fatal(err)
	}
	doc := parse(t, `<article `+string(attrs)+`><h2>Plans</h2>`+status+`</article>`)
	article := find(doc, func(n *xhtml.Node) bool { return n.Data == "article" })[0]
	if got, _ := attr(article, "data-live-source"); got != "plans:fathom verify:fathom" {
		t.Errorf("wrapper source = %q", got)
	}
	if _, ok := attr(article, "aria-label"); ok {
		t.Error("RegionAttrs must leave naming to the app-owned wrapper")
	}
	link := find(article, func(n *xhtml.Node) bool { return n.Data == "a" })[0]
	if href, _ := attr(link, "href"); href != "#plans" {
		t.Errorf("current-page refresh href = %q", href)
	}
	if strings.Contains(status, "mt-3") {
		t.Error("standalone status line must not impose outer spacing")
	}
}

func TestPageStatusIsHonestWithoutScript(t *testing.T) {
	markup, err := RenderPageStatus(Page{URL: "/events"})
	if err != nil {
		t.Fatal(err)
	}
	doc := parse(t, markup)
	page := find(doc, hasAttr("data-live-page"))[0]
	if got, _ := attr(page, "data-live-url"); got != "/events" {
		t.Errorf("stream url = %q", got)
	}
	if _, ok := attr(page, "data-live-events"); ok {
		t.Error("default event list should be omitted")
	}
	if _, ok := attr(page, "data-live-last-event-id"); ok {
		t.Error("an empty cursor should be omitted")
	}
	if !classes(page)["min-h-11"] {
		t.Error("page status must reserve the control height")
	}
	conn := find(page, hasAttr("data-live-connection"))[0]
	if got := text(conn); got != "Live updates are off. Reload for newer data." {
		t.Errorf("no-script connection text = %q", got)
	}
	toggle := find(page, hasAttr("data-live-toggle"))[0]
	if toggle.Data != "button" {
		t.Error("control must be a native button")
	}
	if typ, _ := attr(toggle, "type"); typ != "button" {
		t.Error("control must not submit an enclosing form")
	}
	// The control keeps its box until the script can use it: invisible (out of
	// the tab order and accessibility tree), never display:none, so showing it
	// moves nothing.
	if _, hidden := attr(toggle, "hidden"); hidden || !classes(toggle)["invisible"] {
		t.Error("control must start invisible, keeping its space, not hidden")
	}
	for c := range classes(toggle) {
		if c == "flex" || c == "inline-flex" || c == "block" || c == "inline-block" || c == "grid" {
			t.Errorf("display utility %q would override a hidden attribute an app sets", c)
		}
	}
	ann := find(page, hasAttr("data-live-announcer"))[0]
	for key, want := range map[string]string{"role": "status", "aria-live": "polite", "aria-atomic": "true"} {
		if got, _ := attr(ann, key); got != want {
			t.Errorf("announcer %s = %q", key, got)
		}
	}
	if text(ann) != "" {
		t.Error("announcer must start empty")
	}
}

// No state change or marking may resize the page status line, at any width:
// the connection text shares one grid cell with an invisible copy of every
// message live.js can write there, the control stacks both of its labels, and
// the line wraps the control below the text from a fixed basis, so its size
// depends only on the container width.
func TestPageStatusReservesEveryStateAtAnyWidth(t *testing.T) {
	markup, err := RenderPageStatus(Page{URL: "/events"})
	if err != nil {
		t.Fatal(err)
	}
	page := find(parse(t, markup), hasAttr("data-live-page"))[0]
	if c := classes(page); !c["flex"] || !c["flex-wrap"] || !c["items-center"] {
		t.Error("the line must wrap the control below the text when narrow")
	}
	conn := find(page, hasAttr("data-live-connection"))[0]
	cell := conn.Parent
	if c := classes(cell); !c["grid"] || !c["min-w-0"] || !c["grow"] || !c["basis-40"] || !c["tabular-nums"] {
		t.Errorf("connection cell must be a growing grid with a fixed basis and tabular digits: %v", c)
	}
	if c := classes(conn); !c["col-start-1"] || !c["row-start-1"] {
		t.Error("connection text must share the reserved cell")
	}
	var sizers []string
	for c := cell.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != xhtml.ElementNode || c == conn {
			continue
		}
		cls := classes(c)
		if hidden, _ := attr(c, "aria-hidden"); hidden != "true" || !cls["invisible"] || !cls["col-start-1"] || !cls["row-start-1"] {
			t.Errorf("sizer %q must be an aria-hidden invisible layer of the same cell", text(c))
		}
		sizers = append(sizers, text(c))
	}
	if !slices.Equal(sizers, connectionMessages) {
		t.Errorf("sizers = %q, want %q", sizers, connectionMessages)
	}

	// The sizers must be exactly live.js's messages, placeholders at their widest.
	src := Script()
	block := regexp.MustCompile(`(?s)var MESSAGES = \{(.*?)\n  \};`).FindStringSubmatch(src)
	limit := regexp.MustCompile(`maxAttempts: \[1, (\d+), \d+\]`).FindStringSubmatch(src)
	if block == nil || limit == nil {
		t.Fatal("live.js MESSAGES or the maxAttempts bound not found")
	}
	widest := strings.NewReplacer("{attempt}", limit[1], "{max}", limit[1])
	want := []string{offMessage}
	for _, m := range regexp.MustCompile(`(\w+): "([^"]*)"`).FindAllStringSubmatch(block[1], -1) {
		want = append(want, widest.Replace(m[2]))
	}
	got := slices.Clone(connectionMessages)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("connectionMessages %q do not match live.js MESSAGES %q", got, want)
	}

	toggle := find(page, hasAttr("data-live-toggle"))[0]
	labels := find(toggle, hasAttr("data-live-toggle-label"))
	if len(labels) != 1 || text(labels[0]) != "Stop live updates" {
		t.Fatal("control needs one script-owned label layer")
	}
	stack := labels[0].Parent
	if !classes(stack)["grid"] {
		t.Error("labels must stack in one grid cell")
	}
	var reserved []string
	for c := stack.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == xhtml.ElementNode && c != labels[0] {
			if hidden, _ := attr(c, "aria-hidden"); hidden != "true" || !classes(c)["invisible"] {
				t.Error("the reserved label must stay out of the accessible name")
			}
			reserved = append(reserved, text(c))
		}
	}
	if !slices.Equal(reserved, []string{"Resume live updates"}) {
		t.Errorf("control must reserve its wider label, got %q", reserved)
	}
}

func TestInvalidInputFailsBeforeOutput(t *testing.T) {
	cases := map[string]func(*Region){
		"empty id":           func(r *Region) { r.ID = "" },
		"id with space":      func(r *Region) { r.ID = "a b" },
		"id with quote":      func(r *Region) { r.ID = `a"b` },
		"numeric id":         func(r *Region) { r.ID = "1plans" },
		"no sources":         func(r *Region) { r.Sources = nil },
		"wildcard source":    func(r *Region) { r.Sources = []string{"*"} },
		"source with space":  func(r *Region) { r.Sources = []string{"a b"} },
		"duplicate source":   func(r *Region) { r.Sources = []string{"a", "a"} },
		"too many sources":   func(r *Region) { r.Sources = strings.Fields(strings.Repeat("s ", MaxSources+1)) },
		"blank label":        func(r *Region) { r.Label = "  " },
		"control label":      func(r *Region) { r.Label = "a\nb" },
		"long label":         func(r *Region) { r.Label = strings.Repeat("x", MaxTextRunes+1) },
		"invalid utf8 label": func(r *Region) { r.Label = "\xff" },
		"blank as-of":        func(r *Region) { r.AsOf = "" },
		"script refresh":     func(r *Region) { r.RefreshHref = "javascript:alert(1)" },
		"protocol-relative":  func(r *Region) { r.RefreshHref = "//evil.example/x" },
		"refresh fragment":   func(r *Region) { r.RefreshHref = "/x#y" },
		"refresh port range": func(r *Region) { r.RefreshHref = "https://example.test:65536/x" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			r := sampleRegion()
			change(&r)
			if out, err := RenderRegion(r); err == nil || out != "" {
				t.Fatalf("RenderRegion accepted invalid input: %q", out)
			}
			if out, err := RenderStatus(r); err == nil || out != "" {
				t.Fatal("RenderStatus accepted invalid input")
			}
			if out, err := RegionAttrs(r); err == nil || out != "" {
				t.Fatal("RegionAttrs accepted invalid input")
			}
		})
	}
	for name, p := range map[string]Page{
		"empty url":      {},
		"script url":     {URL: "javascript:alert(1)"},
		"fragment url":   {URL: "/events#x"},
		"bad event":      {URL: "/events", Events: []string{"has space"}},
		"repeated event": {URL: "/events", Events: []string{"a", "a"}},
		"too many":       {URL: "/events", Events: strings.Fields(strings.Repeat("e ", MaxEvents+1))},
		"cursor newline": {URL: "/events", LastEventID: "4\n2"},
		"cursor NUL":     {URL: "/events", LastEventID: "4\x002"},
		"long cursor":    {URL: "/events", LastEventID: strings.Repeat("9", 257)},
		"long utf8":      {URL: "/events", LastEventID: strings.Repeat("ω", 129)},
		"padded cursor":  {URL: "/events", LastEventID: " 42"},
		"port range":     {URL: "http://127.0.0.1:99999/events"},
	} {
		t.Run("page "+name, func(t *testing.T) {
			if out, err := RenderPageStatus(p); err == nil || out != "" {
				t.Fatalf("RenderPageStatus accepted %+v", p)
			}
		})
	}
}

func TestHostileTextIsEscaped(t *testing.T) {
	r := sampleRegion()
	r.Label = `"><script>alert(1)</script>`
	r.AsOf = `<img src=x onerror=alert(1)>`
	markup, err := RenderRegion(r)
	if err != nil {
		t.Fatal(err)
	}
	doc := parse(t, markup)
	if len(find(doc, func(n *xhtml.Node) bool { return n.Data == "script" || n.Data == "img" })) != 0 {
		t.Fatal("hostile label or as-of text became markup")
	}
	page, err := RenderPageStatus(Page{URL: `/events?q="><script>`})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(page, "<script>") || len(find(parse(t, page), func(n *xhtml.Node) bool { return n.Data == "script" })) != 0 {
		t.Fatal("stream URL escaped its attribute")
	}
}

func TestHandlerServesRevalidatedScript(t *testing.T) {
	h := Handler()
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/assets/live.js", nil))
	if get.Code != http.StatusOK || get.Body.String() != Script() {
		t.Fatalf("GET = %d", get.Code)
	}
	for key, want := range map[string]string{"Content-Type": "application/javascript; charset=utf-8", "X-Content-Type-Options": "nosniff", "Cache-Control": "public, max-age=0, must-revalidate"} {
		if got := get.Header().Get(key); got != want {
			t.Errorf("%s = %q", key, got)
		}
	}
	head := httptest.NewRecorder()
	h.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/assets/live.js", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Errorf("HEAD = %d body=%d", head.Code, head.Body.Len())
	}
	post := httptest.NewRecorder()
	h.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/assets/live.js", nil))
	if post.Code != http.StatusMethodNotAllowed || post.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST = %d", post.Code)
	}
	cond := httptest.NewRequest(http.MethodGet, "/assets/live.js", nil)
	cond.Header.Set("If-None-Match", get.Header().Get("ETag"))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, cond)
	if rr.Code != http.StatusNotModified {
		t.Errorf("conditional GET = %d", rr.Code)
	}
}

// L2 contract: one classic script, no imports, no dependencies, one global.
func TestScriptIsDependencyFreeWithOneGlobal(t *testing.T) {
	src := Script()
	for _, banned := range []string{"import ", "require(", "export ", "eval(", "new Function", "innerHTML", "insertAdjacentHTML", "document.write", "EventSource"} {
		if strings.Contains(src, banned) {
			t.Errorf("live.js uses %q", banned)
		}
	}
	if n := len(regexp.MustCompile(`\bw\.[A-Z][A-Za-z]+\s*=[^=]`).FindAllString(src, -1)); n != 1 || !strings.Contains(src, "w.AppKitLive = {") {
		t.Errorf("live.js must assign exactly one global (AppKitLive), found %d assignments", n)
	}
	if !strings.HasPrefix(strings.TrimSpace(src), "//") || !strings.Contains(src, "})(window, document);") {
		t.Error("live.js must stay a self-contained classic IIFE")
	}
}

func TestBrowserStateMachine(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node unavailable: live.js state-machine checks were not run; consumers do not require Node")
	}
	cmd := exec.Command(node, "--test", "live_test.mjs")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("live.js state-machine checks: %v\n%s", err, output)
	}
}

func ExampleRenderRegion() {
	region, err := RenderRegion(Region{
		ID:      "plans",
		Sources: []string{"plans:fathom"},
		Label:   "Plan status",
		AsOf:    "As of 12:03 UTC",
		Body:    template.HTML(`<h2>Plans</h2><p>12 proven</p>`),
	})
	if err != nil {
		return // refuse the page rather than render a partial region
	}
	status, _ := RenderPageStatus(Page{URL: "/events"})
	// Serve live.js with Handler at an app-owned path and load it once per page.
	_, _ = io.WriteString(io.Discard, status+region)
}
