package main

// console_test.go — the Observatory console's foundation (vision 35, plan
// docs/plans/observatory-console.md section A), exercised through the real
// serve-http handler on the fixture fleet (console_fixture_test.go).

import (
	"bytes"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/internal/console"
)

func stubPage(id, route string) consolePage {
	return consolePage{
		ID: id, Route: route, Href: route, Group: "Fleet", Title: "Stub " + id, Question: "stub?", Nav: true,
		Build: func(*consoleRequest) (consoleOut, error) { return consoleOut{}, nil },
	}
}

// TestConsoleRegistry_RoutesNavAndSearchAgree pins OC-3: one table drives
// routing, navigation and search. registerPage refuses a malformed page;
// every registered page answers at its route on the real handler; every
// navigable page appears once, under its group, in the fixed group order;
// every page with a destination is found by search at that destination;
// and every verb a page names is a registered CLI verb.
func TestConsoleRegistry_RoutesNavAndSearchAgree(t *testing.T) {
	// registerPage's refusals, on a fresh registry.
	reg := newConsoleRegistry()
	if err := registerPage(reg, stubPage("a", "/a")); err != nil {
		t.Fatalf("valid page refused: %v", err)
	}
	bad := map[string]consolePage{
		"duplicate id":    stubPage("a", "/other"),
		"duplicate route": stubPage("b", "/a"),
		"unknown group":   func() consolePage { p := stubPage("c", "/c"); p.Group = "Misc"; return p }(),
		"no question":     func() consolePage { p := stubPage("d", "/d"); p.Question = ""; return p }(),
		"nav without href": func() consolePage {
			p := stubPage("e", "/e/{x}")
			p.Href = ""
			return p
		}(),
		"verb not a command": func() consolePage { p := stubPage("f", "/f"); p.Verbs = []string{"fleet report"}; return p }(),
		"no build":           func() consolePage { p := stubPage("g", "/g"); p.Build = nil; return p }(),
		"relative route":     stubPage("h", "h"),
	}
	for name, p := range bad {
		if err := registerPage(reg, p); err == nil {
			t.Errorf("%s: registerPage accepted it", name)
		}
	}

	f := newConsoleFixture(t)
	real := defaultConsoleRegistry()
	if len(real.pages) < 9 {
		t.Fatalf("registry holds %d pages; want at least the nine migrated ones plus search", len(real.pages))
	}

	// Routing: every page answers at its route.
	for _, p := range real.pages {
		code, _, body := f.get(t, samplePath(p))
		if code != http.StatusOK {
			t.Errorf("page %s at %s = %d: %.300s", p.ID, samplePath(p), code, body)
			continue
		}
		if !strings.Contains(body, `<main id="observatory-main"`) {
			t.Errorf("page %s did not render on the console shell", p.ID)
		}
	}
	for _, want := range []string{"/", "/fleet", "/projects/{svc}", "/impact", "/sessions", "/sessions/{id}", "/brief", "/wiring", "/statusz", "/search",
		"/plans", "/plans/{svc}/{plan}", "/plans/{svc}/{plan}/rows/{id}", "/certifications", "/refactors"} {
		found := false
		for _, p := range real.pages {
			if p.Route == want || (want == "/" && p.Route == "/{$}") {
				found = true
			}
		}
		if !found {
			t.Errorf("no page registered at %s", want)
		}
	}

	// Navigation: groups in the fixed order, each navigable page once under
	// its own group, nothing in the sidebar that is not registered.
	groups := real.navGroups()
	last := -1
	seen := map[string]string{}
	for _, g := range groups {
		idx := consoleGroupIndex(g.Heading)
		if idx <= last {
			t.Errorf("nav group %q out of the fixed order", g.Heading)
		}
		last = idx
		for _, item := range g.Items {
			seen[item.Href] = g.Heading
		}
	}
	for _, p := range real.pages {
		if p.Nav {
			if seen[p.Href] != p.Group {
				t.Errorf("navigable page %s: in nav group %q, want %q", p.ID, seen[p.Href], p.Group)
			}
			delete(seen, p.Href)
		}
	}
	if len(seen) != 0 {
		t.Errorf("sidebar lists destinations no page registers: %v", seen)
	}
	nav := consoleNav(real, "/fleet")
	for _, g := range groups {
		if !strings.Contains(nav, `aria-label="`+g.Heading+`"`) {
			t.Errorf("rendered nav lacks group %q", g.Heading)
		}
	}

	// Search: every page with a destination is found at it.
	ix := console.SearchIndex{Docs: real.searchDocs()}
	for _, p := range real.pages {
		if p.Href == "" {
			continue
		}
		res := console.Search(ix, p.Title, 50)
		found := false
		for _, g := range res.Groups {
			for _, r := range g.Results {
				if g.Group == console.GroupPages && r.Href == p.Href {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("search for %q does not reach %s", p.Title, p.Href)
		}
	}

	// Verbs are real CLI verbs.
	registered := map[string]bool{}
	for _, v := range verbs() {
		registered[v.Name] = true
	}
	for _, p := range real.pages {
		for _, v := range p.Verbs {
			name := strings.Fields(v)[1]
			if !registered[name] {
				t.Errorf("page %s names verb %q, which the CLI does not dispatch", p.ID, v)
			}
		}
	}
}

// TestConsoleShell_GroupsBreadcrumbsThemeSkipLink pins OC-4: the shell's
// skip link is the first focusable element; the sidebar carries the grouped
// navigation; the trail, question, meta, verb chips and actions head the
// content; the theme is restored before the stylesheet with the
// fathom-theme key and offered as a control; one h1; no palette colors.
func TestConsoleShell_GroupsBreadcrumbsThemeSkipLink(t *testing.T) {
	reg := defaultConsoleRegistry()
	got := renderConsoleShell(reg, consoleShell{
		PageID: "fleet", Title: "Fleet",
		Crumbs:   []chrome.Crumb{{Label: "Observatory", Href: "/"}, {Label: "Fleet"}},
		Question: "How is every project doing?", Meta: []string{"3 projects", "generated now"},
		Verbs:   []string{"fathom fleet report --json"},
		Actions: `<a href="/brief">Brief</a>`, Body: `<section id="fixture">body</section>`,
	})

	firstLink := strings.Index(got, "<a ")
	aside := strings.Index(got, "<aside")
	if firstLink < 0 || aside < 0 || firstLink > aside || !strings.HasPrefix(got[firstLink:], `<a href="#observatory-main"`) {
		t.Error("the skip link must be the first link, before the sidebar")
	}
	if !strings.Contains(got, `<main id="observatory-main" tabindex="-1"`) {
		t.Error("the skip link's target must be the focusable main landmark")
	}
	if strings.Count(got, "<h1") != 1 || !strings.Contains(got, ">Fleet</h1>") {
		t.Error("the page must have exactly one h1, its title")
	}
	// Grouped navigation in the fixed order, current page marked.
	lastIdx := -1
	for _, g := range consoleGroups {
		i := strings.Index(got, `aria-label="`+g+`"`)
		if i < 0 {
			continue
		}
		if i < lastIdx {
			t.Errorf("group %s rendered out of order", g)
		}
		lastIdx = i
	}
	for _, g := range []string{"Now", "Fleet", "Evidence", "System"} {
		if !strings.Contains(got, `aria-label="`+g+`"`) {
			t.Errorf("nav lacks group %s", g)
		}
	}
	if !regexp.MustCompile(`<a href="/fleet"[^>]*aria-current="page"`).MatchString(got) {
		t.Error("the current page must be marked in the sidebar")
	}
	// Breadcrumbs, question, meta (the kit heading's list), verb chip, actions.
	for _, want := range []string{`aria-label="Breadcrumb"`, `<a href="/" class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-sm hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Observatory</a>`, `aria-current="page">Fleet</li>`,
		"How is every project doing?", `<li class="min-w-0 [overflow-wrap:anywhere]">3 projects</li><li class="min-w-0 [overflow-wrap:anywhere]">generated now</li>`, `data-verb-chip`, `fathom fleet report --json`,
		`data-console-copy="fathom fleet report --json"`, `<a href="/brief">Brief</a>`, `data-console-heading`} {
		if !strings.Contains(got, want) {
			t.Errorf("shell missing %q", want)
		}
	}
	// Theme: bootstrap before the stylesheet, the fathom-theme key, a control.
	boot := strings.Index(got, `window.AppKitAppearance.init({"storageKey":"fathom-theme"})`)
	css := strings.Index(got, `<link rel="stylesheet" href="/assets/app.css">`)
	if boot < 0 || css < 0 || boot > css {
		t.Error("the appearance bootstrap must run before the stylesheet, with storage key fathom-theme")
	}
	if !strings.Contains(got, `data-appkit-appearance data-appearance-storage-key="fathom-theme"`) {
		t.Error("the shell must offer the appearance control")
	}
	// Search: the top bar's palette trigger is a real link to the search
	// page (the no-JS path), and the page carries the palette's dialog with
	// its GET form (OC-12).
	if !strings.Contains(got, `<a href="/search" data-palette-open="console-palette"`) {
		t.Error("the top bar must carry the palette trigger, a link to /search")
	}
	if !strings.Contains(got, `<form role="search" action="/search" method="get" data-palette-form>`) || !strings.Contains(got, `name="q"`) {
		t.Error("the page must carry the palette dialog's search form")
	}
	assertRoleUtilitiesOnly(t, got)
}

// assertRoleUtilitiesOnly fails on literal palette colors or dark: color
// variants in rendered classes (UI doctrine rule 2).
func assertRoleUtilitiesOnly(t *testing.T, doc string) {
	t.Helper()
	for _, class := range paletteColorClasses(doc) {
		t.Errorf("class uses a palette color or dark: variant: %q", class)
	}
}

var (
	paletteColorRe = regexp.MustCompile(`\b(?:bg|text|border|ring|fill|stroke)-(?:red|blue|green|yellow|gray|slate|zinc|neutral|stone|orange|amber|lime|emerald|teal|cyan|sky|indigo|violet|purple|fuchsia|pink|rose|white|black)(?:-\d+)?\b`)
	classAttrRe    = regexp.MustCompile(`class="([^"]*)"`)
)

// paletteColorClasses returns every rendered class attribute that uses a
// literal palette color or a dark: variant.
func paletteColorClasses(doc string) []string {
	var out []string
	for _, m := range classAttrRe.FindAllStringSubmatch(doc, -1) {
		// No kit exception: the overlay backdrop is the scrim role since
		// mirror 9fa58e6 (FATHOM-S0-KIT-02).
		class := m[1]
		if paletteColorRe.MatchString(class) || strings.Contains(class, "dark:") {
			out = append(out, m[1])
		}
	}
	return out
}

// TestConsoleErrors_RenderOnShellWithRemedy pins OC-5: 404, 400, an
// unknown project and an unreadable store each render on the shell with
// the reason and the remedy; a JSON request gets the same fields as JSON.
func TestConsoleErrors_RenderOnShellWithRemedy(t *testing.T) {
	f := newConsoleFixture(t)
	cases := []struct {
		path, title, remedy string
		status              int
	}{
		{"/no-such-page", "Not found", "search the console", http.StatusNotFound},
		{"/projects/nope-service", "Unknown project", "fathom init --path <repo> --service nope-service --url " + f.ts.URL, http.StatusNotFound},
		{"/projects/payment", "Unknown project", "Did you mean: payment-service?", http.StatusNotFound},
		{"/brief?format=pdf", "Unsupported format", "?format=json, ?format=md", http.StatusBadRequest},
	}
	for _, tc := range cases {
		code, hdr, body := f.get(t, tc.path)
		if code != tc.status {
			t.Errorf("%s = %d, want %d", tc.path, code, tc.status)
		}
		if !strings.Contains(hdr.Get("Content-Type"), "text/html") {
			t.Errorf("%s: an error page is HTML on the shell, got %q", tc.path, hdr.Get("Content-Type"))
		}
		for _, want := range []string{`<main id="observatory-main"`, `aria-label="Primary"`, ">" + tc.title + "</h1>", htmlEscapeTest(tc.remedy)} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: error page missing %q", tc.path, want)
			}
		}
	}
	// The unknown-project remedy must index through THIS daemon: a shell
	// `fathom ingest` writes <repo>/workspace/db/fathom.sqlite, a store the
	// daemon never reads, so the project would stay unknown (and against
	// the shared store it is the forbidden shell ingest, INC-3).
	if _, _, body := f.get(t, "/projects/nope-service"); strings.Contains(body, "fathom ingest") {
		t.Error("the unknown-project remedy must not be a shell ingest into a store the daemon never reads")
	}
	// JSON asks get JSON with the same reason and remedy.
	code, hdr, body := f.get(t, "/projects/nope-service?format=json")
	var e map[string]any
	if code != http.StatusNotFound || !strings.Contains(hdr.Get("Content-Type"), "application/json") || json.Unmarshal([]byte(body), &e) != nil || e["remedy"] == "" || e["reason"] == "" {
		t.Errorf("JSON error = %d %q %s", code, hdr.Get("Content-Type"), body)
	}
	// Unreadable store: the registry cannot answer, so a project cannot be
	// looked up — 503 on the shell with a remedy, never a bare 404.
	h := newUnreachableConsoleHandler(t)
	rec := httptest.NewRecorder()
	resetObsCache()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/projects/payment-service", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "Store unreadable") || !strings.Contains(rec.Body.String(), "fathom doctor") {
		t.Errorf("unreadable registry = %d:\n%.400s", rec.Code, rec.Body.String())
	}
	resetObsCache()
	// A method a read-only page does not serve is a 405 on the shell.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/fleet", nil))
	if rec.Code != http.StatusMethodNotAllowed || !strings.Contains(rec.Body.String(), `<main id="observatory-main"`) {
		t.Errorf("POST /fleet = %d", rec.Code)
	}
}

func htmlEscapeTest(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;").Replace(s)
}

// TestStatusz_OnShellUnknownTypedEdgesVisible pins OC-6: /statusz renders
// on the shell. Every typed-edge state names its own cause: a pass this
// process ran shows its packages; a pass replayed from the store says it
// predates this daemon; a service with no Go module says the pass does not
// apply (enrichTypedEdges never records one for it) and is not counted
// unknown; a Go service with no recorded pass anywhere says "unknown" in
// words (the survey: 15 of 18 rendered nothing); a failed pass names its
// reason; and ?format=json is the v3 report with exactly its v3 keys.
func TestStatusz_OnShellUnknownTypedEdgesVisible(t *testing.T) {
	report := StatuszReport{
		SchemaVersion: statuszSchemaVersion, GeneratedAt: "2026-09-25T12:00:00Z", UptimeSec: 22700,
		ChannelsNote: "FATHOM_CHANNELS is not \"off\"",
		Services: []StatuszServiceRow{
			{Service: "alpha", IndexedCommit: "abc123", TypedEdges: &StatuszTypedEdges{Packages: 12}},
			{Service: "beta", IndexedCommit: "def456", TypedEdgesAbsence: statuszTypedAbsence{Reason: "no typed-edge pass is recorded for this service — not by this daemon, not in the store"}},
			{Service: "gamma", IndexedCommit: "0f0f0f", TypedEdges: &StatuszTypedEdges{Failed: true, Reason: "go/packages: no go.mod"}},
			{Service: "delta", IndexedCommit: "1a1a1a", TypedEdgesAbsence: statuszTypedAbsence{NotApplicable: true, Reason: "does not apply — no go.mod at the service root, so the typed Go pass never runs"}},
			{Service: "eps", IndexedCommit: "2b2b2b", TypedEdges: &StatuszTypedEdges{Packages: 4, Persisted: true, RecordedAt: "2026-09-24T08:00:00Z"}},
		},
	}
	body := renderStatusz(report, "http://127.0.0.1:7581")
	// The rows render twice — the table from lg up, a stacked list below
	// it — so each count reads the table alone.
	tableOnly := func(page string) string {
		i, j := strings.Index(page, "data-statusz-table"), strings.Index(page, "data-statusz-list")
		if i < 0 || j < i {
			t.Fatalf("statusz must render its table and its stacked list")
		}
		return page[i:j]
	}
	if !strings.Contains(body, "12 packages") {
		t.Error("a pass that ran must show its package count")
	}
	if n := strings.Count(tableOnly(body), "unknown — no typed-edge pass is recorded for this service"); n != 1 {
		t.Errorf("the one Go service with no recorded pass must read unknown with its cause, found %d", n)
	}
	if strings.Contains(body, "not ingested by this daemon since it started") {
		t.Error("the page must not claim a cause it never checked (the store fallback and no-Go services make it false)")
	}
	if !strings.Contains(body, "does not apply — no go.mod at the service root") {
		t.Error("a service with no Go module must say the pass does not apply")
	}
	if !strings.Contains(body, "by an earlier daemon process, replayed from the store") {
		t.Error("a store-replayed pass must say it predates this daemon")
	}
	if !strings.Contains(body, "FAILED — go/packages: no go.mod") {
		t.Error("a failed pass must name its reason")
	}
	if !strings.Contains(body, `data-truth-state="partial"`) || !strings.Contains(body, "unknown for 1 of 5 services") {
		t.Error("the services panel must count only the genuinely unknown typed-edge states")
	}
	// The classifier itself: go.mod present → unknown with its cause;
	// absent → does not apply; unreadable → unknown naming the read error.
	root := t.TempDir()
	if a := statuszTypedEdgesAbsence(root); !a.NotApplicable {
		t.Errorf("no go.mod must be not-applicable, got %+v", a)
	}
	mustWrite(t, filepath.Join(root, "go.mod"), "module x\n")
	if a := statuszTypedEdgesAbsence(root); a.NotApplicable || !strings.Contains(a.Reason, "no typed-edge pass is recorded") {
		t.Errorf("a Go module with no pass must be unknown, got %+v", a)
	}
	if a := statuszTypedEdgesAbsence(""); a.NotApplicable || a.Reason == "" {
		t.Errorf("no recorded root must be unknown with a reason, got %+v", a)
	}
	if !strings.Contains(body, "6h 18m") {
		t.Error("uptime must be human-readable")
	}
	if !strings.Contains(body, `data-console-copy="curl -s http://127.0.0.1:7581/statusz?format=json"`) {
		t.Error("the statusz verb must name this daemon's own endpoint")
	}
	if !strings.Contains(body, `href="/projects/alpha"`) {
		t.Error("each service row must drill down to its project")
	}
	assertRoleUtilitiesOnly(t, body)

	f := newConsoleFixture(t)
	code, _, page := f.get(t, "/statusz")
	if code != http.StatusOK {
		t.Fatalf("GET /statusz = %d", code)
	}
	for _, want := range []string{`<main id="observatory-main"`, `aria-label="Primary"`, ">Daemon status</h1>", `aria-label="Breadcrumb"`, "payment-service", "Typed edges"} {
		if !strings.Contains(page, want) {
			t.Errorf("statusz page missing %q", want)
		}
	}
	// The fixture fleet's services hold Go source but no go.mod: the pass
	// cannot apply to any of them, so none is unknown and the panel is not
	// partial (the live fleet showed 6 no-module repos pinned "unknown").
	if n := strings.Count(tableOnly(page), "does not apply — no go.mod"); n != len(consoleFixtureServices) {
		t.Errorf("every no-module fixture service must read does-not-apply, found %d", n)
	}
	if strings.Contains(page, "the typed-edge pass is unknown for") {
		t.Error("services the pass cannot apply to must not mark the panel partial")
	}
	if strings.Contains(page, "<style>") {
		t.Error("statusz must not carry its own inline stylesheet any more")
	}
	code, hdr, raw := f.get(t, "/statusz?format=json")
	if code != http.StatusOK || !strings.Contains(hdr.Get("Content-Type"), "application/json") {
		t.Fatalf("statusz json = %d %q", code, hdr.Get("Content-Type"))
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"binary_commit", "binary_dirty", "channels_note", "generated_at", "schema_version", "services", "uptime_seconds"}
	if _, ok := m["built_at"]; ok {
		want = append(want, "built_at")
		sort.Strings(want)
	}
	if !reflect.DeepEqual(keys, want) || m["schema_version"] != float64(3) {
		t.Errorf("statusz JSON keys = %v (schema %v), want the v3 set %v", keys, m["schema_version"], want)
	}
}

// TestLegacyObservatoryRoutes_RedirectAndKeepJSON pins OC-7: every old
// /observatory* HTML route answers 308 to its new home (the query that is
// not in the new path survives); ?format=json on an old route is the old
// JSON, unchanged; the wiring mutation endpoints are not redirected.
func TestLegacyObservatoryRoutes_RedirectAndKeepJSON(t *testing.T) {
	f := newConsoleFixture(t)
	redirects := map[string]string{
		"/observatory": "/fleet",
		"/observatory/service?id=payment-service": "/projects/payment-service",
		"/observatory/service":                    "/fleet",
		"/observatory/impact":                     "/impact",
		"/observatory/story":                      "/sessions",
		"/observatory/session?id=s-1":             "/sessions/s-1",
		"/observatory/session":                    "/sessions",
		"/observatory/brief":                      "/brief",
		"/observatory/brief?format=md":            "/brief?format=md",
		"/observatory/wiring/":                    "/wiring",
	}
	for from, to := range redirects {
		code, hdr, _ := f.get(t, from)
		if code != http.StatusPermanentRedirect || hdr.Get("Location") != to {
			t.Errorf("GET %s = %d → %q, want 308 → %q", from, code, hdr.Get("Location"), to)
			continue
		}
		// The new home answers.
		if c2, _, _ := f.get(t, to); c2 != http.StatusOK {
			t.Errorf("redirect target %s = %d", to, c2)
		}
	}

	// ?format=json keeps observatoryModel v1, key for key.
	code, _, body := f.get(t, "/observatory?format=json")
	var legacy map[string]json.RawMessage
	if code != http.StatusOK || json.Unmarshal([]byte(body), &legacy) != nil {
		t.Fatalf("legacy json = %d", code)
	}
	if string(legacy["schema_version"]) != "1" {
		t.Errorf("legacy schema_version = %s, want 1", legacy["schema_version"])
	}
	wantKeys := jsonFieldNames(reflect.TypeOf(observatoryModel{}))
	for k := range legacy {
		if !wantKeys[k] {
			t.Errorf("legacy JSON grew a key %q — observatoryModel v1 must stay unchanged", k)
		}
	}
	// "narrate" left this list with SR-14 (surface reckoning F-53): the
	// narrate feature (CLI verb, story rows, the observatory banner) was
	// removed for 0 lifetime calls, so observatoryModel no longer carries
	// a Narrate field to serialize — a legitimate v1 shrink, not drift.
	for _, k := range []string{"schema_version", "fathom_version", "generated_at", "services", "impact", "cost", "arm"} {
		if _, ok := legacy[k]; !ok {
			t.Errorf("legacy JSON lost %q", k)
		}
	}
	code, _, body = f.get(t, "/observatory/service?id=payment-service&format=json")
	var row map[string]any
	if code != http.StatusOK || json.Unmarshal([]byte(body), &row) != nil || row["service"] != "payment-service" {
		t.Errorf("legacy service json = %d %.200s", code, body)
	}
	for path, key := range map[string]string{"/observatory/impact?format=json": "verify_catches_note", "/observatory/story?format=json": "ledger_rows", "/observatory/wiring/?format=json": "services"} {
		code, _, body = f.get(t, path)
		if code != http.StatusOK || !strings.Contains(body, `"`+key+`"`) {
			t.Errorf("legacy %s = %d, want its old JSON with %q", path, code, key)
		}
	}
	if code, _, _ = f.get(t, "/observatory/session?format=json"); code != http.StatusBadRequest {
		t.Errorf("legacy session json without id = %d, want its old 400", code)
	}

	// The mutation endpoints stay where the gate guards them: never
	// redirected, still refused without a same-origin loopback request.
	req, _ := http.NewRequest(http.MethodPost, f.ts.URL+"/observatory/wiring/apply?service=payment-service", nil)
	req.Header.Set("Origin", "http://evil.example")
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin apply = %d, want the gate's 403 (not a redirect)", resp.StatusCode)
	}

	// The wrapper itself: HTML redirects, JSON and POST pass through.
	legacyHit := 0
	h := legacyObservatoryRedirect(func(r *http.Request) string { return "/new" }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { legacyHit++ }))
	for _, tc := range []struct {
		method, target, accept string
		redirect               bool
	}{
		{http.MethodGet, "/old", "", true},
		{http.MethodHead, "/old", "", true},
		{http.MethodGet, "/old?format=json", "", false},
		{http.MethodGet, "/old", "application/json", false},
		{http.MethodPost, "/old", "", false},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.target, nil)
		if tc.accept != "" {
			req.Header.Set("Accept", tc.accept)
		}
		h.ServeHTTP(rec, req)
		if got := rec.Code == http.StatusPermanentRedirect; got != tc.redirect {
			t.Errorf("%s %s (Accept %q): redirected = %v", tc.method, tc.target, tc.accept, got)
		}
	}
	if legacyHit != 3 {
		t.Errorf("legacy handler served %d requests, want 3", legacyHit)
	}
}

func jsonFieldNames(t reflect.Type) map[string]bool {
	out := map[string]bool{}
	for i := 0; i < t.NumField(); i++ {
		fld := t.Field(i)
		tag := strings.Split(fld.Tag.Get("json"), ",")[0]
		if fld.Anonymous && tag == "" {
			for k := range jsonFieldNames(fld.Type) {
				out[k] = true
			}
			continue
		}
		if tag != "" && tag != "-" {
			out[tag] = true
		}
	}
	return out
}

// TestConsolePagesRenderTheirVerbs pins OC-8: every page answers JSON in
// the console envelope naming its verbs; for each verb a page declares,
// the page JSON's section equals that verb's --json output on the fixture
// fleet, byte-for-value. Pages not yet rebuilt declare no verbs. A page
// whose registered verbs name parameters ("{svc}", "{plan}", "{layer}",
// "{root}", …) renders them filled from the request, each an instance of
// one of its registry's verbs, and is held to them at every sample URL
// (samplePath's and consoleExtraSamplePaths'); each runs as its chip reads
// (consoleVerbArgs).
func TestConsolePagesRenderTheirVerbs(t *testing.T) {
	f := newConsoleFixture(t)
	pinConsoleVerbClocks(t, f)
	withVerbs := 0
	// parity runs one section's exact command as its chip reads (a quoted
	// value stays one argument, as a shell pasting the chip would keep it;
	// runVerbJSON) and compares.
	parity := func(where, verb string, section json.RawMessage) {
		t.Helper()
		var fromPage any
		if err := json.Unmarshal(section, &fromPage); err != nil {
			t.Fatal(err)
		}
		fromVerb := runVerbJSON(t, f, verb)
		dropVolatileVerbFields(verb, fromVerb, fromPage)
		if !reflect.DeepEqual(fromVerb, fromPage) {
			a, _ := json.MarshalIndent(fromVerb, "", " ")
			b, _ := json.MarshalIndent(fromPage, "", " ")
			t.Errorf("page %s disagrees with `%s`:\nverb: %s\npage: %s", where, verb, a, b)
		}
	}
	for _, p := range defaultConsoleRegistry().pages {
		if p.ID == "statusz" {
			continue // ?format=json is the v3 report (OC-6), not the envelope
		}
		for _, path := range append([]string{samplePath(p)}, consoleExtraSamplePaths[p.ID]...) {
			env := f.pageJSON(t, path)
			if env.Schema != consolePageJSONSchema || env.Page != p.ID || env.Group != p.Group || env.Question != p.Question {
				t.Errorf("page %s: envelope = %+v", path, env)
			}
			if !verbsFillTemplates(env.Verbs, p.Verbs) {
				t.Errorf("page %s: JSON verbs %v are not the registry's %v", path, env.Verbs, p.Verbs)
			}
			for _, verb := range env.Verbs {
				withVerbs++
				section, ok := env.Sections[verb]
				if !ok {
					t.Errorf("page %s: no section for its verb %q", path, verb)
					continue
				}
				parity(path, verb, section)
			}
		}
	}
	if withVerbs == 0 {
		t.Error("no page declares a verb; the parity framework proved nothing")
	}

	// A filtered page's sections are keyed by the exact (quoted) command
	// its chips show, and each equals that command's output.
	mustWrite(t, os.Getenv("FATHOM_VERIFY_LEDGER"), `{"schema_version":2,"at":"2026-09-24T10:00:00Z","service":"payment-service","plan":"docs/plans/p.md","claimed":3,"failing":1,"origin":"cli"}`+"\n")
	resetImpactReads()
	t.Cleanup(resetImpactReads)
	for _, path := range []string{
		"/impact?svc=payment-service&kind=verify-refusal&days=7",
		"/sessions?q=s-1%20x&arm=on&sort=turns&dir=asc",
		"/sessions/s-1?channel=edit-hook",
		"/decide?site=route",
	} {
		env := f.pageJSON(t, path)
		if len(env.Sections) == 0 {
			t.Errorf("%s: no sections", path)
		}
		for verb, section := range env.Sections {
			parity(env.Page, verb, section)
		}
	}
	// Wave F's Evidence and System pages each render their verbs (OC-41..45).
	for _, id := range []string{"impact", "sessions", "operate", "decide"} {
		if p, ok := defaultConsoleRegistry().page(id); !ok || len(p.Verbs) == 0 {
			t.Errorf("page %s must declare the verbs it renders", id)
		}
	}

	// Wave G: the Learn pages render verbs, and a page may also carry a
	// section for a command it computes per request — the chosen project's
	// own `fathom capabilities --repo R --service S --json` on /learn. Every
	// such section equals that exact command's output too.
	for _, id := range []string{"learn", "learn-ask"} {
		if p, ok := defaultConsoleRegistry().page(id); !ok || len(p.Verbs) == 0 {
			t.Errorf("page %s must render a verb", id)
		}
	}
	perRequest := 0
	for _, p := range defaultConsoleRegistry().pages {
		if p.ID == "statusz" {
			continue
		}
		env := f.pageJSON(t, samplePath(p))
		for key, section := range env.Sections {
			// The page's rendered verbs were held above; a per-project
			// template section ("{svc}") is held below; a filtered page's
			// quoted command was held in the loop before this one.
			if containsString(env.Verbs, key) || !strings.HasPrefix(key, "fathom ") || strings.ContainsAny(key, `'"{`) {
				continue
			}
			perRequest++
			parity(p.ID, key, section)
		}
	}
	if perRequest == 0 {
		t.Error("no page carries a per-request section; /learn's capabilities section is missing")
	}
	// Every panel's verb chip is a command a shell runs (ADR-045 decision
	// 2): an agent-surface tool call ("staleness_report {…}") is offered as
	// a call inside a panel, never as the panel's verb — only a try-it
	// answer, whose whole point is the exact call an agent makes, carries
	// one (not on these sample paths).
	chipRE := regexp.MustCompile(`data-console-panel[^>]*data-console-verb="([^"]*)"`)
	toolCall := regexp.MustCompile(`^[a-z_]+ \{`)
	for _, p := range defaultConsoleRegistry().pages {
		_, _, page := f.get(t, samplePath(p))
		for _, m := range chipRE.FindAllStringSubmatch(page, -1) {
			if verb := html.UnescapeString(m[1]); toolCall.MatchString(verb) {
				t.Errorf("page %s: a panel's verb chip %q is an agent-surface call, not a command", p.ID, verb)
			}
		}
	}
	// The Learn pipeline's stage chips print their headlines:
	// TestConsoleLearn_StageChipsPrintTheirHeadlines runs each one.

	// Per-project verbs: a page that renders a verb's answer for each
	// project from a recorded instrument (ProjectVerbs) carries a section
	// per template, keyed by service, and each concrete verb is run and
	// held to it. The fleet here has real roots, and one plan is verified
	// at their HEAD by a real `fathom verify` run, so the recorded proof
	// has something to agree with.
	g := newConsoleFixtureOpts(t, consoleFixtureOpts{Rooted: true})
	payRoot, _ := rootedProject(t, g, "payment-service")
	plansDir, err := filepath.Abs(filepath.Join("testdata", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	var vout, verr bytes.Buffer
	// The run is recorded at the project's registered root (PE-24: a run
	// recorded anywhere else is not the project's proof).
	t.Chdir(payRoot)
	run([]string{"verify", "--service", "payment-service", "--url", g.ts.URL,
		"--plan", filepath.Join(plansDir, "fixture-a.md"), "--plan", filepath.Join(plansDir, "fixture-b.md")}, &vout, &verr)
	resetObsCache()
	compared := map[string]int{}
	for _, p := range defaultConsoleRegistry().pages {
		if len(p.ProjectVerbs) == 0 {
			continue
		}
		env := g.pageJSON(t, samplePath(p))
		if !reflect.DeepEqual(append([]string{}, env.ProjectVerbs...), append([]string{}, p.ProjectVerbs...)) {
			t.Errorf("page %s: JSON project verbs %v, registry %v", p.ID, env.ProjectVerbs, p.ProjectVerbs)
		}
		for _, tmpl := range p.ProjectVerbs {
			check, ok := consoleProjectVerbChecks[tmpl]
			if !ok {
				t.Errorf("page %s renders %q, but no check holds it to the verb", p.ID, tmpl)
				continue
			}
			var bySvc map[string]json.RawMessage
			if err := json.Unmarshal(env.Sections[tmpl], &bySvc); err != nil {
				t.Errorf("page %s: section %q is not keyed by service: %v", p.ID, tmpl, err)
				continue
			}
			for _, svc := range consoleRootedFixtureServices() {
				compared[tmpl] += check(t, g, svc, bySvc[svc])
			}
		}
	}
	if len(compared) == 0 {
		t.Error("no page declares a per-project verb; the parity framework proved nothing")
	}
	for tmpl, n := range compared {
		if n == 0 {
			t.Errorf("%q: nothing was compared (verify: %s %s)", tmpl, vout.String(), verr.String())
		}
	}
}

// consoleProjectVerbChecks hold a page's per-project section (the verb's
// shape, from a recorded instrument) to the verb itself, run for one
// service; each returns how many figures it compared.
var consoleProjectVerbChecks = map[string]func(t *testing.T, f *consoleFixture, svc string, section json.RawMessage) int{
	// plans status: every plan whose recorded run is at HEAD reads exactly
	// as the verb computes it now — proven, claimed-unproven (with the
	// PC-5 measured-stale split folded back) and total, and the rows the
	// ledger does not split (open, deferred, skipped) as one sum.
	verbPlansStatusFor: func(t *testing.T, f *consoleFixture, svc string, section json.RawMessage) int {
		var plans []nowPlanStatus
		if len(section) > 0 {
			if err := json.Unmarshal(section, &plans); err != nil {
				t.Fatalf("plans status section for %s: %v", svc, err)
			}
		}
		args := []string{"plans", "status", "--service", svc, "--url", f.ts.URL, "--json"}
		current := map[string]nowPlanStatus{}
		for _, p := range plans {
			if p.Stale == "" {
				args = append(args, "--plan", p.Plan)
				current[p.Plan] = p
			}
		}
		if len(current) == 0 {
			return 0
		}
		var out, errb bytes.Buffer
		if code := run(args, &out, &errb); code != 0 {
			t.Fatalf("plans status for %s exited %d: %s", svc, code, errb.String())
		}
		var rollups []planRollup
		if err := json.Unmarshal(out.Bytes(), &rollups); err != nil {
			t.Fatalf("plans status --json: %v\n%s", err, out.String())
		}
		n := 0
		for _, r := range rollups {
			p, ok := current[r.Plan]
			if !ok {
				t.Errorf("%s: plans status names %s, which the page does not", svc, r.Plan)
				continue
			}
			n++
			if p.Proven != r.Proven || p.ClaimedUnproven != r.ClaimedUnproven+r.MeasuredStale || p.Total != r.Total ||
				p.Unclaimed != r.Open+r.DeferredWithTrigger+r.SkippedOther {
				t.Errorf("%s %s: the page (recorded run at HEAD) says %+v; plans status says %+v", svc, r.Plan, p, r)
			}
		}
		if n != len(current) {
			t.Errorf("%s: the page shows %d current plans, plans status %d", svc, len(current), n)
		}
		return n
	},
	// certify status: the verb's own JSON, exactly.
	verbCertifyStatusFor: func(t *testing.T, f *consoleFixture, svc string, section json.RawMessage) int {
		if len(section) == 0 {
			t.Errorf("certify status section has no %s", svc)
			return 0
		}
		var out, errb bytes.Buffer
		if code := run([]string{"certify", "status", "--service", svc, "--store", f.store, "--json"}, &out, &errb); code != 0 {
			t.Fatalf("certify status for %s exited %d: %s", svc, code, errb.String())
		}
		var fromVerb, fromPage any
		if err := json.Unmarshal(out.Bytes(), &fromVerb); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(section, &fromPage); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(fromVerb, fromPage) {
			t.Errorf("%s: certify status says %s; the page %s", svc, out.String(), section)
		}
		return 1
	},
}

// TestConsoleMetricsAgreeAcrossPages pins OC-9: every page that shows a
// shared metric carries it in its JSON, and every page agrees on every
// metric ID. It also holds the projections to the fixture's truth: the call
// total counts unattributed rows, sessions are distinct sessions not rows,
// the store's own service list (statusz) matches the registry count, and
// the fleet tile renders the same call total the JSON carries.
func TestConsoleMetricsAgreeAcrossPages(t *testing.T) {
	f := newConsoleFixture(t)
	// Prime the model, then grow the ledger the way the Stop hook does on a
	// live daemon (a new session's first row): a page that re-reads the
	// ledger for itself would now disagree with the model-backed pages. One
	// metric is one read, so every page must still say the same thing.
	f.pageJSON(t, "/impact")
	ledger, err := os.OpenFile(f.ledger, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.WriteString(`{"schema_version":2,"session_id":"s-late","arm":"on","repo":"ledger-service","at":"2026-09-25T11:59:00Z"}` + "\n" +
		`{"schema_version":2,"session_id":"s-late-pay","arm":"on","repo":"payment-service","at":"2026-09-25T11:59:30Z"}` + "\n"); err != nil {
		t.Fatal(err)
	}
	ledger.Close()
	byMetric := map[string]map[string]string{} // metric → page → value JSON
	for _, p := range defaultConsoleRegistry().pages {
		if len(p.Shows) == 0 {
			continue
		}
		env := f.pageJSON(t, samplePath(p))
		for _, id := range p.Shows {
			raw, ok := env.Metrics[string(id)]
			if !ok {
				t.Errorf("page %s shows %s but its JSON does not carry it", p.ID, id)
				continue
			}
			if byMetric[string(id)] == nil {
				byMetric[string(id)] = map[string]string{}
			}
			byMetric[string(id)][p.ID] = string(raw)
		}
		if len(env.Metrics) != len(p.Shows) {
			t.Errorf("page %s JSON carries %d metrics, declares %d", p.ID, len(env.Metrics), len(p.Shows))
		}
	}
	// OC-H3: every metric a page shows is catalogued with the one question
	// it answers (the catalog admits no second id for a question —
	// TestMetricCatalog_OneMetricPerQuestion), and every catalogued metric
	// says the same thing on every page that shows it. Two pages therefore
	// cannot show two numbers for one question.
	catalogued := map[string]bool{}
	for _, spec := range console.MetricCatalog {
		catalogued[string(spec.ID)] = true
	}
	for id, pages := range byMetric {
		if !catalogued[id] {
			t.Errorf("pages %v show %s, which the metric catalog does not name — its question may already have a metric", keysOf(pages), id)
		}
	}
	for _, spec := range console.MetricCatalog {
		id := spec.ID
		pages := byMetric[string(id)]
		if len(pages) == 0 {
			t.Errorf("metric %s is shown on no page", id)
			continue
		}
		var first, firstPage string
		for page, v := range pages {
			if first == "" {
				first, firstPage = v, page
				continue
			}
			if v != first {
				t.Errorf("metric %s: page %s says %s, page %s says %s", id, firstPage, first, page, v)
			}
		}
	}
	// A page's panels show a shared metric through that one value too: the
	// activity tab's sessions panel ("the newest N of M distinct sessions")
	// and its JSON are project.sessions itself, never a recount from a
	// fresher read of the ledger of its own.
	actEnv := f.pageJSON(t, "/projects/payment-service/activity")
	var act struct {
		Activity struct {
			Sessions    json.RawMessage `json:"sessions"`
			SessionRows []struct {
				ID string `json:"id"`
			} `json:"session_rows"`
		} `json:"activity"`
	}
	if err := json.Unmarshal(actEnv.Model, &act); err != nil {
		t.Fatal(err)
	}
	var panelSessions, sharedSessions console.Value[int]
	if err := json.Unmarshal(act.Activity.Sessions, &panelSessions); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(actEnv.Metrics[string(console.MetricProjectSessions)], &sharedSessions); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(panelSessions, sharedSessions) {
		t.Errorf("the activity panel counts sessions itself: model.activity.sessions %+v, metrics.project.sessions %+v", panelSessions, sharedSessions)
	}
	for _, r := range act.Activity.SessionRows {
		if r.ID == "s-late-pay" {
			t.Errorf("the sessions panel lists a session the shared count was never read from: %+v", act.Activity.SessionRows)
		}
	}
	_, _, actPage := f.get(t, "/projects/payment-service/activity")
	band := tileValue(t, actPage, "distinct sessions")
	if !strings.Contains(actPage, "of "+band+" distinct sessions") {
		t.Errorf("the sessions caption disagrees with the band's %s distinct sessions", band)
	}
	value := func(id console.MetricID) console.Value[int] {
		for _, raw := range byMetric[string(id)] {
			var v console.Value[int]
			if err := json.Unmarshal([]byte(raw), &v); err != nil {
				t.Fatal(err)
			}
			return v
		}
		return console.Value[int]{}
	}
	if v := value(console.MetricCalls); v.State != console.StatePresent || v.V != f.callRows || !strings.Contains(v.Denominator, "2 unattributed") {
		t.Errorf("calls.total = %+v, want every one of the %d rows with the %d unattributed named", v, f.callRows, f.anonCalls)
	}
	if v := value(console.MetricSessions); v.V != 2 || !strings.Contains(v.Denominator, "from 3 ledger rows") || !v.AsOf.Equal(f.now) {
		t.Errorf("sessions.total = %+v, want 2 distinct sessions from 3 rows, as of the model's read (%s)", v, f.now)
	}
	// The HTML agrees with the JSON: the sessions page's stat band shows the
	// model's read, not a fresher one of its own.
	_, _, sessionsPage := f.get(t, "/sessions")
	if got := tileValue(t, sessionsPage, "distinct sessions (all time)"); got != "2" {
		t.Errorf("sessions tile shows %q, the shared metric 2", got)
	}
	if v := value(console.MetricServices); v.V != len(consoleFixtureServices) {
		t.Errorf("fleet.services = %+v, want %d", v, len(consoleFixtureServices))
	}
	if v := value(console.MetricObservations); v.State != console.StateAbsent {
		t.Errorf("observations.open = %+v, want absent (no fixture repo has an observations file)", v)
	}
	// Cross-sources: statusz reads the store's own list; impact's visible
	// tile reads the ledger through the impact fold.
	report := buildStatuszReport(t.Context(), f.client, f.now, f.now)
	if s := statuszServiceCount(report); s.V != value(console.MetricServices).V {
		t.Errorf("statusz counts %d services, the registry %d", s.V, value(console.MetricServices).V)
	}
	// Impact, Sessions and Operate show the shared metrics they declare, in
	// their HTML as in their JSON: the impact tile and the sessions table
	// read the model's one read of the ledger, the operate tile the model's
	// call total (wave F).
	_, _, impactPage := f.get(t, "/impact")
	if got := tileValue(t, impactPage, "distinct sessions (all time)"); got != fmtThousands(value(console.MetricSessions).V) {
		t.Errorf("impact's sessions tile says %q, the shared metric %d", got, value(console.MetricSessions).V)
	}
	var sessionsModel struct {
		Table console.SessionTableModel `json:"table"`
	}
	if err := json.Unmarshal(f.pageJSON(t, "/sessions").Model, &sessionsModel); err != nil {
		t.Fatal(err)
	}
	if sessionsModel.Table.Sessions.V != value(console.MetricSessions).V || sessionsModel.Table.Sessions.Denominator != value(console.MetricSessions).Denominator {
		t.Errorf("the sessions table lists %+v sessions, the shared metric %+v", sessionsModel.Table.Sessions, value(console.MetricSessions))
	}
	_, _, operatePage := f.get(t, "/operate")
	if !strings.Contains(operatePage, html.EscapeString(valuePlain(value(console.MetricCalls)))+" in the whole log") {
		t.Errorf("operate must show the shared call total (%s) beside its 7-day count", valuePlain(value(console.MetricCalls)))
	}
	// The HTML agrees with the JSON: the fleet tile shows the same total.
	_, _, fleet := f.get(t, "/fleet")
	if got := tileValue(t, fleet, "tool calls (log)"); got != fmtThousands(f.callRows) {
		t.Errorf("fleet tile shows %q calls, the metric says %d", got, f.callRows)
	}
	// The brief (the F0 proof's first defect) reads the call total and the
	// agent surface's size through the same projections: its "tool calls in
	// the call log" is the shared total, the attributed sum is named as a
	// subset, and the surface size is the measured tool count — never a
	// literal.
	tools := value(console.MetricAgentTools)
	if tools.State != console.StatePresent || tools.V == 0 {
		t.Fatalf("surface.agent_tools = %+v, want the fixture's measured agent surface", tools)
	}
	var brief briefModel
	if err := json.Unmarshal(f.pageJSON(t, "/brief").Model, &brief); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		fmtThousands(f.callRows) + " recorded tool calls in the call log (",
		"**Tool calls recorded:** " + fmtThousands(f.callRows) + " in the current call log (every row in the log;",
		"attributed to an indexed repository",
		", a " + fmtThousands(tools.V) + "-tool surface)",
		"the agent surface is " + fmtThousands(tools.V) + " tools",
	} {
		if !strings.Contains(brief.Markdown, want) {
			t.Errorf("the brief must read the shared metrics: missing %q in\n%s", want, brief.Markdown)
		}
	}
	if strings.Contains(brief.Markdown, "16-tool") {
		t.Error("the brief still names a literal surface size")
	}

	// The proof metrics (console wave C) equal their verbs summed over the
	// fleet: the plans headline is `plans status`'s claimed_unproven and
	// proven, the certifications totals are `certify gaps`'s summary.
	assertProofMetricsMatchVerbs(t, f, value)

	// The Knowledge metrics (console wave D) agree with their verbs.
	assertKnowledgeMetricsAgree(t, f, value(console.MetricServices), value(console.MetricClaims))

	// Wave G: the Learn pages render the shared metrics they carry — the
	// project count in /learn's heading, the agent surface's size in the
	// ask page's coverage line — from the same projections.
	if _, _, learn := f.get(t, "/learn"); !strings.Contains(learn, "one of "+valuePlain(value(console.MetricServices))+" indexed projects") {
		t.Errorf("/learn must name the shared project count %+v", value(console.MetricServices))
	}
	if _, _, ask := f.get(t, "/learn/ask"); !strings.Contains(ask, "of the "+fmtThousands(tools.V)+" on the agent surface") {
		t.Errorf("/learn/ask must name the shared agent-surface size %d", tools.V)
	}
	// The Learn pages' traffic is one read of the call log per project: the
	// pipeline's rules served are the ask page's obligations asks, its
	// tool calls answered the sum of every tool's asks, for every project,
	// in the same truth state.
	for _, svc := range consoleFixtureServices {
		var pipe learnModel
		if err := json.Unmarshal(f.pageJSON(t, "/learn?svc="+svc).Model, &pipe); err != nil {
			t.Fatal(err)
		}
		var ask askModel
		if err := json.Unmarshal(f.pageJSON(t, "/learn/ask?svc="+svc).Model, &ask); err != nil {
			t.Fatal(err)
		}
		asked := map[string]console.Value[int]{}
		for _, s := range ask.Guide.Shapes {
			asked[s.Tool] = s.Asked
		}
		for _, st := range pipe.Pipeline.Stages {
			if st.ID != console.StageServe {
				continue
			}
			if got, want := st.Headline.Value, asked["obligations"]; got.State != want.State || got.V != want.V {
				t.Errorf("%s: /learn rules served %+v, /learn/ask obligations asked %+v", svc, got, want)
			}
			if st.Counts[0].Value.V < st.Headline.Value.V {
				t.Errorf("%s: tool calls answered %+v below rules served %+v", svc, st.Counts[0].Value, st.Headline.Value)
			}
		}
	}

	// OC-H3: the Now and Fleet tiles and the brief's summary show the
	// unified metrics through their one value: the plan rows the gate
	// proves (the Plans page's headline, not a ledger sum beside it), the
	// current claims (the Layers page's), the live certificates (the
	// Certifications page's).
	proven := value(console.MetricPlansProven)
	if proven.State != console.StatePresent {
		t.Fatalf("plans.proven = %+v, want the portfolio's verified figure", proven)
	}
	_, _, nowBody := f.get(t, "/")
	_, _, fleetBody := f.get(t, "/fleet")
	if a, b := tileValue(t, nowBody, "Plan rows proven"), tileValue(t, fleetBody, "plan rows proven"); a != fmtThousands(proven.V) || a != b {
		t.Errorf("plan rows tile: Now %q, Fleet %q, plans.proven %d", a, b, proven.V)
	}
	var now nowReport
	if err := json.Unmarshal(f.pageJSON(t, "/").Model, &now); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(now.Summary.ClaimsCurrent, value(console.MetricClaims)) || !reflect.DeepEqual(now.Summary.CertifyLive, value(console.MetricCertLive)) {
		t.Errorf("the brief's summary claims %+v certify %+v; the metrics %+v %+v", now.Summary.ClaimsCurrent, now.Summary.CertifyLive, value(console.MetricClaims), value(console.MetricCertLive))
	}
	if a, b := tileValue(t, nowBody, "Live certificates"), tileValue(t, fleetBody, "live certificates"); a != b {
		t.Errorf("live certificates tile: Now %q, Fleet %q", a, b)
	}
}

// TestConsolePanels_CarryVerbQuestionAndSource pins OC-10: a panel states
// its question, its exact verb as a copyable chip, and a footer naming the
// source, denominator and freshness; each truth state renders distinctly.
func TestConsolePanels_CarryVerbQuestionAndSource(t *testing.T) {
	base := consolePanel{
		ID: "plans", Title: "Plans", Question: "What is planned and proven?",
		Verb: "fathom plans status --service fathom --json", Source: "the verify ledger",
		Denominator: "58 plans", Freshness: "as of 2026-09-25T12:00Z", SourceKey: "plans:fathom",
		Body: `<p>628 proven</p>`,
	}
	got := renderPanel(base)
	for _, want := range []string{
		`data-panel-question>What is planned and proven?</p>`,
		`<code class="min-w-0 [overflow-wrap:anywhere]">fathom plans status --service fathom --json</code>`,
		`data-console-copy="fathom plans status --service fathom --json"`,
		`hidden data-console-copy`, // revealed only when the script can copy
		// A source-keyed panel is live (OC-14): its freshness leads the
		// kit's reserved status line instead of repeating in the footer.
		`data-panel-footer>source: the verify ledger · denominator: 58 plans</p>`,
		`<span data-live-asof class="col-start-1 row-start-1">freshness: as of 2026-09-25T12:00Z</span>`,
		`data-console-source="plans:fathom"`, `data-truth-state="present"`, `id="plans"`, `<p>628 proven</p>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("panel missing %q", want)
		}
	}
	lines := map[console.State]string{
		console.StateAbsent:  "not measured — the ledger is missing",
		console.StateError:   "could not be read — the ledger is missing",
		console.StateStale:   "stale — the ledger is missing",
		console.StatePartial: "lower bound — the ledger is missing",
	}
	seen := map[string]bool{}
	for state, want := range lines {
		p := base
		p.State, p.Reason = state, "the ledger is missing"
		out := renderPanel(p)
		if !strings.Contains(out, want) || !strings.Contains(out, `data-truth-state="`+string(state)+`"`) {
			t.Errorf("state %s: missing %q", state, want)
		}
		// Each state is drawn differently (distinct classes).
		m := regexp.MustCompile(`<p class="([^"]*)" data-truth-line`).FindStringSubmatch(out)
		if m == nil || seen[m[1]] {
			t.Errorf("state %s is not drawn distinctly", state)
		} else {
			seen[m[1]] = true
		}
	}
	for _, s := range []console.State{console.StatePresent, console.StateZero} {
		p := base
		p.State = s
		if strings.Contains(renderPanel(p), "data-truth-line") {
			t.Errorf("state %s must let the number speak, not add a caveat line", s)
		}
	}
	// valueStat maps each state onto a kit tile without a fabricated zero.
	if s := valueStat("calls", console.Absent[int]("no log")); s.Value != "—" || s.Sub != "no log" {
		t.Errorf("absent tile = %+v", s)
	}
	if s := valueStat("calls", console.Partial(114, "7 of 18 repos reporting; lower bound")); s.Value != "≥\u00a0114" {
		t.Errorf("partial tile = %+v", s)
	}
	if s := valueStat("calls", console.Measured(0, "log")); s.Value != "0" {
		t.Errorf("zero tile = %+v", s)
	}
	if s := valueStat("calls", console.Errored[int]("EPERM")); s.Value == "0" || s.ValueRole != "danger" {
		t.Errorf("error tile = %+v", s)
	}
	assertRoleUtilitiesOnly(t, got)

	// The statusz page's panels carry the same parts on a live render.
	f := newConsoleFixture(t)
	_, _, page := f.get(t, "/statusz")
	if strings.Count(page, "data-console-panel") < 2 || strings.Count(page, "data-panel-footer") < 2 || !strings.Contains(page, "data-panel-question") {
		t.Error("statusz panels must carry question, verb and footer")
	}
	// A page that declares verbs shows them as chips in its heading.
	_, _, fleet := f.get(t, "/fleet")
	if !strings.Contains(fleet, `data-console-copy="fathom fleet report --json"`) {
		t.Error("the fleet page must show its verb as a copyable chip")
	}
}

// TestConsolePageViews_Counted pins OC-15: an HTML view of a page is
// counted per route pattern — never the concrete project or session — in
// memory (served on /metrics) and in the durable log; JSON reads and error
// pages are not views.
func TestConsolePageViews_Counted(t *testing.T) {
	dir := t.TempDir()
	views := newConsolePageViews(dir + "/views.jsonl")
	page := &consolePage{ID: "project", Route: "/projects/{svc}"}
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	countPageView(views, page, now)
	countPageView(views, page, now)
	var buf bytes.Buffer
	views.writeMetrics(&buf)
	if !strings.Contains(buf.String(), `fathom_console_page_views_total{route="/projects/{svc}"} 2`) {
		t.Errorf("metrics = %q", buf.String())
	}
	raw, _ := os.ReadFile(dir + "/views.jsonl")
	if strings.Count(string(raw), "\n") != 2 || !strings.Contains(string(raw), `"route":"/projects/{svc}"`) || !strings.Contains(string(raw), `"at":"2026-09-25T12:00:00Z"`) {
		t.Errorf("durable log = %q", raw)
	}
	countPageView(nil, page, now) // a nil counter never panics

	f := newConsoleFixture(t)
	for _, p := range []string{"/fleet", "/fleet", "/projects/payment-service", "/fleet?format=json", "/projects/nope-service"} {
		f.get(t, p)
	}
	// HEAD serves no page to anyone (link checkers, uptime probes, curl -I):
	// never a view, and the body is never built.
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest(http.MethodHead, f.ts.URL+"/fleet", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(resp.Header.Get("Content-Type"), "text/html") {
			t.Errorf("HEAD /fleet = %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
	_, _, metrics := f.get(t, "/metrics")
	for _, want := range []string{`fathom_console_page_views_total{route="/fleet"} 2`, `fathom_console_page_views_total{route="/projects/{svc}"} 1`, "fathom_console_event_subscribers 0"} {
		if !strings.Contains(metrics, want) {
			t.Errorf("/metrics missing %q:\n%s", want, metrics)
		}
	}
	logged, _ := os.ReadFile(f.viewsLog)
	if strings.Contains(string(logged), "payment-service") || strings.Contains(string(logged), "nope-service") {
		t.Errorf("the view log must hold route patterns, never concrete ids: %s", logged)
	}
	if n := strings.Count(string(logged), "\n"); n != 3 {
		t.Errorf("view log rows = %d, want 3 (two fleet views, one project view)", n)
	}
}

// TestConsoleBudgets pins OC-16: on the fixture fleet every registered
// page's warm render costs at most 150 ms of CPU server-side and its HTML
// stays within 250 KB. The render budget is CPU time, what the work cost
// however long the machine kept it waiting for a core (EW-7); where the
// platform has no CPU reading the render bound is skipped, the size bound
// stays.
func TestConsoleBudgets(t *testing.T) {
	const (
		maxRender = 150 * time.Millisecond
		maxBytes  = 250 * 1024
	)
	f := newConsoleFixture(t)
	paths := []string{}
	for _, p := range defaultConsoleRegistry().pages {
		paths = append(paths, samplePath(p))
	}
	// Filters and window sizes a reader can ask for keep the same budgets
	// (a replay's ?per= widens its JSON window, never its HTML page).
	paths = append(paths, "/impact?svc=payment-service&days=7", "/sessions?q=s-1%20x&sort=turns", "/sessions/s-1?per=2000", "/operate?svc=payment-service")
	for _, path := range paths {
		f.get(t, path) // cold: builds and caches the model
		best := time.Duration(1 << 62)
		size := 0
		for i := 0; i < 3; i++ {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			cpu0, cpuOK := processCPUTime()
			f.ts.Config.Handler.ServeHTTP(rec, req)
			cpu1, _ := processCPUTime()
			if !cpuOK {
				best = 0
			} else if d := cpu1 - cpu0; d < best {
				best = d
			}
			size = rec.Body.Len()
			if rec.Code != http.StatusOK {
				t.Fatalf("%s = %d", path, rec.Code)
			}
		}
		if best > maxRender {
			t.Errorf("%s: warm render %s exceeds %s", path, best, maxRender)
		}
		if size > maxBytes {
			t.Errorf("%s: HTML %d bytes exceeds %d", path, size, maxBytes)
		}
		t.Logf("%-28s warm %6.1f ms CPU  %6.1f KB", path, float64(best.Microseconds())/1000, float64(size)/1024)
	}
}

// TestConsoleSearch_PageFragmentAndJSON covers OC-11's server side: the
// same grouped results as a full page, as the palette's HTML fragment and
// as JSON; projects, plan rows, records, verbs and symbols from the store.
func TestConsoleSearch_PageFragmentAndJSON(t *testing.T) {
	f := newConsoleFixture(t)
	code, _, page := f.get(t, "/search?q=charge")
	if code != http.StatusOK || !strings.Contains(page, `<main id="observatory-main"`) || !strings.Contains(page, `id="search-symbols"`) {
		t.Fatalf("search page = %d:\n%.500s", code, page)
	}
	// The fragment is the ⌘K palette's (OC-12): one kit results fragment
	// answering exactly this query.
	code, _, frag := f.get(t, "/search?q=charge&format=fragment")
	if code != http.StatusOK || strings.Contains(frag, "<html") || !strings.Contains(frag, `data-palette-fragment data-palette-state="results" data-palette-query="charge"`) || !strings.Contains(frag, "ChargeRequest") {
		t.Errorf("fragment = %d:\n%.500s", code, frag)
	}
	env := f.pageJSON(t, "/search?q="+url.QueryEscape("payment"))
	var res console.SearchResults
	if err := json.Unmarshal(env.Model, &res); err != nil {
		t.Fatal(err)
	}
	groups := map[console.SearchGroup]bool{}
	for _, g := range res.Groups {
		groups[g.Group] = true
	}
	if !groups[console.GroupProjects] {
		t.Errorf("payment search groups = %v, want Projects", res.Groups)
	}
	for q, group := range map[string]console.SearchGroup{"fleet report": console.GroupVerbs, "Impact": console.GroupPages} {
		env := f.pageJSON(t, "/search?q="+url.QueryEscape(q))
		var r console.SearchResults
		_ = json.Unmarshal(env.Model, &r)
		found := false
		for _, g := range r.Groups {
			found = found || g.Group == group
		}
		if !found {
			t.Errorf("search %q: no %s group in %+v", q, group, r.Groups)
		}
	}
}

// A session ledger row names its repo by the directory label the Stop hook
// derives ("sextant"), not by service ID ("fathom"), and a label may hold a
// space ("2026 job hunt"). The session event maps the label to the service
// whose recorded root it names, so the source key a project panel carries
// (sessions:fathom) matches; a label that is not name-shaped still fires
// the unscoped "sessions" source — an event is never silently dropped.
func TestConsoleEvents_SessionEventsNameTheServiceAndNeverDrop(t *testing.T) {
	roots := map[string]string{"fathom": "/code/sextant", "app-kit": "/code/app-kit"}
	for label, want := range map[string]string{"sextant": "fathom", "app-kit": "app-kit", "Fathom": "fathom", "imbue": "imbue", "2026 job hunt": ""} {
		if got := sessionEventService(label, roots); got != want {
			t.Errorf("sessionEventService(%q) = %q, want %q", label, got, want)
		}
	}
	dir := t.TempDir()
	verify, sessions := dir+"/verify.jsonl", dir+"/sessions.jsonl"
	mustWrite(t, verify, "")
	mustWrite(t, sessions, "")
	bus := console.NewEventBus(console.BusOptions{})
	src := newConsoleEventSources(bus, nil, verify, sessions)
	src.roots = roots
	var warned []string
	src.warn = func(msg string) { warned = append(warned, msg) }
	src.verify.Prime()
	src.session.Prime()
	appendLine(t, sessions, `{"session_id":"probe-space-1","repo":"2026 job hunt"}`)
	appendLine(t, sessions, `{"session_id":"probe-sextant-1","repo":"sextant"}`)
	src.pollLedgers()
	_, events, _ := bus.Subscribe(0)
	if len(events) != 2 {
		t.Fatalf("events = %+v (warnings %v), want both session rows", events, warned)
	}
	if e := events[0]; e.Ref != "probe-space-1" || e.Service != "" || strings.Join(e.Sources(), ",") != "sessions" {
		t.Errorf("space-label session = %+v %v, want the unscoped sessions source", e, e.Sources())
	}
	if e := events[1]; e.Service != "fathom" || strings.Join(e.Sources(), ",") != "sessions,sessions:fathom" {
		t.Errorf("sextant session = %+v %v, want service fathom", e, e.Sources())
	}
}

// TestConsoleEvents_FeedersPublishNamesAndIDs covers OC-13's feeders: an
// appended verify-ledger row becomes a verify event naming the plans
// source, a session row a session event, the daemon announces itself, and
// GET /events on the real handler streams hello.
func TestConsoleEvents_FeedersPublishNamesAndIDs(t *testing.T) {
	dir := t.TempDir()
	verify, sessions := dir+"/verify.jsonl", dir+"/sessions.jsonl"
	mustWrite(t, verify, "")
	mustWrite(t, sessions, "")
	bus := console.NewEventBus(console.BusOptions{})
	src := newConsoleEventSources(bus, nil, verify, sessions)
	src.verify.Prime()
	src.session.Prime()
	appendLine(t, verify, `{"service":"fathom","plan":"docs/plans/observatory-console.md","failing":1}`)
	appendLine(t, sessions, `{"session_id":"abc-123","repo":"fathom","tokens_in":900}`)
	src.pollLedgers()
	publishDaemonEvent(bus, console.DaemonRecord{BinaryCommit: "old"}, true, "new")
	_, events, _ := bus.Subscribe(0)
	if len(events) != 3 {
		t.Fatalf("events = %+v, want verify, session, daemon", events)
	}
	if e := events[0]; e.Kind != console.EventVerify || e.Name != "observatory-console.md" || e.Ref != "fail" || e.Sources()[0] != "plans:fathom" {
		t.Errorf("verify event = %+v", e)
	}
	if e := events[1]; e.Kind != console.EventSession || e.Ref != "abc-123" {
		t.Errorf("session event = %+v", e)
	}
	if e := events[2]; e.Kind != console.EventDaemon || e.Name != "adopt" {
		t.Errorf("daemon event = %+v", e)
	}
	// The row's content (tokens_in: 900) must not ride along. Zero the
	// clock and the epoch first: a timestamp like "…43.900529Z" or a
	// base-36 epoch can contain "900" by chance, which made this flaky.
	for i := range events {
		events[i].At, events[i].Epoch = time.Time{}, ""
	}
	raw, _ := json.Marshal(events)
	if strings.Contains(string(raw), "900") || strings.Contains(string(raw), "tokens") {
		t.Errorf("events must carry names and IDs only: %s", raw)
	}
}

func appendLine(t *testing.T, path, line string) {
	t.Helper()
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer fh.Close()
	if _, err := fh.WriteString(line + "\n"); err != nil {
		t.Fatal(err)
	}
}

// TestResolveConsoleCallLog_DaemonRecordReachesVerbs covers OC-2's wiring:
// with a daemon record for the daemon a verb talks to, a metrics verb run
// from an unrelated directory reads the daemon's log — not a log relative
// to its working directory — and FATHOM_CALL_LOG still wins when set.
func TestResolveConsoleCallLog_DaemonRecordReachesVerbs(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("FATHOM_CALL_LOG", "")
	os.Unsetenv("FATHOM_CALL_LOG")
	t.Setenv("FATHOM_HTTP", "http://localhost:7599")
	daemonLog := dir + "/daemon-calls.jsonl"
	mustWrite(t, daemonLog, `{"ts":"`+time.Now().UTC().Format(time.RFC3339)+`","tool":"find_symbol","service":"x","surface":"agent","latency_ms":2}`+"\n")
	path, _ := console.DaemonRecordPath(dir, "127.0.0.1:7599")
	if err := console.WriteDaemonRecord(path, console.NewDaemonRecord("127.0.0.1:7599", 99, daemonLog, "", "c0ffee", time.Now())); err != nil {
		t.Fatal(err)
	}
	got := resolveConsoleCallLog("", "")
	if got.Path != daemonLog || got.Rule != console.RuleDaemon {
		t.Fatalf("resolved = %+v, want the daemon's log", got)
	}
	var out, errb bytes.Buffer
	if code := run([]string{"metrics", "traffic", "--json"}, &out, &errb); code != 0 {
		t.Fatalf("metrics traffic: %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"total_calls": 1`) {
		t.Errorf("metrics traffic must read the daemon's log:\n%s", out.String())
	}
	// Every metrics verb names the log it read and the rule that chose it
	// (OC-2's "naming which rule chose it"), in JSON and in text.
	root := t.TempDir()
	for _, verb := range [][]string{
		{"metrics", "traffic", "--json"},
		{"metrics", "series", "--json", "--root", root, "traffic_total_calls"},
		{"metrics", "rollup", "--json", "--root", root},
		{"metrics", "interventions", "--json"},
		{"instrument", "--adoption", "--json", "--root", root},
		{"flywheel", "--json"},
		{"research", "report", "--json", "--no-write", "--root", root},
	} {
		out.Reset()
		errb.Reset()
		run(verb, &out, &errb)
		var doc struct {
			CallLog console.ResolvedCallLog `json:"call_log"`
		}
		if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
			t.Errorf("%v: not JSON: %v\n%s%s", verb, err, out.String(), errb.String())
			continue
		}
		if doc.CallLog.Path != daemonLog || doc.CallLog.Rule != console.RuleDaemon || !strings.Contains(doc.CallLog.Note, "pid 99") {
			t.Errorf("%v: call_log = %+v, want the daemon's log named with its rule", verb, doc.CallLog)
		}
	}
	for _, verb := range [][]string{{"metrics", "traffic"}, {"metrics", "rotate", "--keep-days", "1"}} {
		out.Reset()
		errb.Reset()
		if code := run(verb, &out, &errb); code != 0 {
			t.Fatalf("%v: %d %s", verb, code, errb.String())
		}
		if !strings.Contains(out.String(), "call log: "+daemonLog+" (rule daemon") {
			t.Errorf("%v text must name the log and its rule:\n%s", verb, out.String())
		}
	}
	// rotate is a metrics verb too: it inspects the daemon's log, never a
	// cwd-relative legacy path (it said "no call log … nothing to rotate").
	if strings.Contains(out.String(), "nothing to rotate") && strings.Contains(out.String(), "no call log") {
		t.Errorf("metrics rotate must read the daemon's log:\n%s", out.String())
	}
	// review's T5-7 gate row goes where the readers read.
	if got := reviewCallLogPath("", "svc"); got != daemonLog {
		t.Errorf("review appends to %q, the readers read %q", got, daemonLog)
	}
	t.Setenv("FATHOM_CALL_LOG", dir+"/explicit.jsonl")
	if got := resolveConsoleCallLog("", ""); got.Rule != console.RuleEnv {
		t.Errorf("FATHOM_CALL_LOG must still win: %+v", got)
	}
	if got := resolveConsoleCallLog("/flag.jsonl", ""); got.Rule != console.RuleFlag {
		t.Errorf("--call-log must win: %+v", got)
	}
}

// A page's call-log provenance equals its verb's (OC-8): inside the daemon
// the pages resolve the daemon's own log, and a verb run from a terminal
// resolves the same log through the daemon's record — both must name it
// with the record's own words, or `?format=json` differs from the verb's
// --json in call_log.note alone.
func TestRecordServingDaemon_InProcessNoteIsTheRecords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("FATHOM_CALL_LOG", "")
	os.Unsetenv("FATHOM_CALL_LOG")
	const addr = "127.0.0.1:7598"
	t.Setenv("FATHOM_HTTP", "http://"+addr)
	resetServing := func() {
		servingCallLog.Lock()
		servingCallLog.set, servingCallLog.path, servingCallLog.note = false, "", ""
		servingCallLog.Unlock()
	}
	t.Cleanup(resetServing)
	rec, err := recordServingDaemon(addr, filepath.Join(dir, "calls.jsonl"), "", "c0ffee", time.Date(2026, 9, 26, 4, 28, 19, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	inProcess := resolveConsoleCallLog("", "")
	resetServing() // a verb in its own process: no serving daemon, only the record
	fromVerb := resolveConsoleCallLog("", "")
	if inProcess.Rule != console.RuleDaemon || fromVerb.Rule != console.RuleDaemon {
		t.Fatalf("both resolve the daemon's log: page %+v, verb %+v", inProcess, fromVerb)
	}
	if inProcess != fromVerb {
		t.Errorf("the page and the verb name the log differently:\npage: %+v\nverb: %+v", inProcess, fromVerb)
	}
	if !strings.Contains(inProcess.Note, rec.Note()) {
		t.Errorf("the note is the record's provenance %q: %q", rec.Note(), inProcess.Note)
	}
}
