package main

// console_palette_test.go — OC-12: the ⌘K palette over every console page
// (console_palette.go). The keyboard contract itself is the kit's
// (appkit/ui/palette, its own script tests and Gallery evidence) and the
// browser proof in the stage's reading; these tests pin fathom's side of
// it: every page mounts the trigger, the dialog and the script; the
// palette's fragment is exactly one kit fragment answering the query it
// asked, with the same ranked groups the search page and JSON carry; and
// without JavaScript the trigger is a link to a search page with its own
// form.

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/palette"
	"github.com/geoffmilleraz/fathom/internal/console"
)

var paletteHeadingRe = regexp.MustCompile(`<section data-palette-group><h3[^>]*>([^<]*)</h3>`)

func TestConsolePalette_OverEveryPageWithFragmentAndNoJSFallback(t *testing.T) {
	f := newConsoleFixture(t)

	// Every page — and the shell's error pages — carries one trigger (a
	// real link to /search), one dialog wired to the fragment URL with the
	// Mod+K shortcut, and the kit's script; the dialog's title is an h2, so
	// the page keeps exactly one h1.
	paths := []string{"/no-such-page"}
	for _, p := range defaultConsoleRegistry().pages {
		paths = append(paths, samplePath(p))
	}
	for _, path := range paths {
		_, _, body := f.get(t, path)
		checks := map[string]int{
			`<a href="/search" data-palette-open="console-palette"`: 1,
			`<dialog id="console-palette"`:                          1,
			`data-palette="console-palette" data-palette-action="/search" data-palette-url="/search?format=fragment" data-palette-param="q" data-palette-shortcut="mod+k"`: 1,
			`<script src="/assets/palette.js" defer></script>`:                     1,
			`<form role="search" action="/search" method="get" data-palette-form>`: 1,
		}
		for want, n := range checks {
			if got := strings.Count(body, want); got != n {
				t.Errorf("%s: %q appears %d times, want %d", path, want, got, n)
			}
		}
		if strings.Count(body, "<h1") != 1 {
			t.Errorf("%s: the palette dialog must not add an h1 (%d)", path, strings.Count(body, "<h1"))
		}
		if strings.Contains(body, `id="console-search-q"`) {
			t.Errorf("%s: the top bar's old search field is still rendered", path)
		}
	}

	// The kit script is served at a stable path.
	code, hdr, js := f.get(t, "/assets/palette.js")
	if code != http.StatusOK || !strings.Contains(hdr.Get("Content-Type"), "javascript") || !strings.Contains(js, "window.AppKitPalette") {
		t.Errorf("/assets/palette.js = %d %q", code, hdr.Get("Content-Type"))
	}

	// The palette's fragment: exactly one kit fragment root answering the
	// query asked, grouped and ranked as the JSON model is — by the URL the
	// dialog fetches and by the header the kit script sends.
	env := f.pageJSON(t, "/search?q=charge")
	var model console.SearchResults
	if err := json.Unmarshal(env.Model, &model); err != nil {
		t.Fatal(err)
	}
	for _, req := range []struct {
		path   string
		header []string
	}{
		{"/search?format=fragment&q=charge", nil},
		{"/search?q=charge", []string{palette.FragmentHeader, "1"}},
	} {
		code, hdr, frag := f.get(t, req.path, req.header...)
		frag = strings.TrimSpace(frag)
		if code != http.StatusOK || !strings.HasPrefix(frag, `<div data-palette-fragment data-palette-state="results" data-palette-query="charge"`) || !strings.HasSuffix(frag, "</div>") {
			t.Fatalf("%s = %d:\n%.400s", req.path, code, frag)
		}
		if strings.Count(frag, "data-palette-fragment") != 1 || strings.Contains(frag, "<html") || strings.Contains(frag, "<main") {
			t.Errorf("%s: not exactly one fragment", req.path)
		}
		if hdr.Get("Vary") != palette.FragmentHeader {
			t.Errorf("%s: Vary = %q", req.path, hdr.Get("Vary"))
		}
		// ChargeRequest: a prefix match the top 8 holds (since K17 the
		// fixture's struct fields are symbols too, and ChargeRequest's own
		// fields rank beside it, ahead of a substring match like
		// validateCharge).
		if !strings.Contains(frag, "ChargeRequest") || !strings.Contains(frag, `data-palette-option`) {
			t.Errorf("%s: the fragment lacks the ranked options", req.path)
		}
		var headings []string
		for _, m := range paletteHeadingRe.FindAllStringSubmatch(frag, -1) {
			headings = append(headings, strings.SplitN(m[1], " · ", 2)[0])
		}
		var want []string
		for _, g := range model.Groups {
			want = append(want, string(g.Group))
		}
		if strings.Join(headings, "|") != strings.Join(want, "|") {
			t.Errorf("%s: fragment groups %v, the search's %v", req.path, headings, want)
		}
	}

	// Without JavaScript the trigger lands on the full page: its own GET
	// form prefilled with the query, above the same results.
	code, hdr, page := f.get(t, "/search?q=charge")
	if code != http.StatusOK || !strings.Contains(page, `<form role="search" action="/search" method="get" class="flex flex-wrap`) ||
		!strings.Contains(page, `name="q" type="search" value="charge"`) || !strings.Contains(page, "data-console-search-results") || hdr.Get("Vary") != palette.FragmentHeader {
		t.Errorf("/search page = %d, Vary %q:\n%.600s", code, hdr.Get("Vary"), page)
	}

	// A query past the bounds answers the palette with the kit's error
	// state naming the bound, not its generic "unavailable".
	long := strings.Repeat("w ", console.MaxQueryTokens+1)
	code, _, frag := f.get(t, "/search?format=fragment&q="+url.QueryEscape(long))
	if code != http.StatusOK || !strings.Contains(frag, `data-palette-state="error"`) || !strings.Contains(frag, "at most 8") ||
		!strings.Contains(frag, `data-palette-query="`+strings.TrimSpace(long)+`"`) {
		t.Errorf("over-long fragment = %d:\n%.400s", code, frag)
	}
	// The empty query is the palette's idle state.
	if _, _, idle := f.get(t, "/search?format=fragment&q="); !strings.Contains(idle, `data-palette-state="idle"`) {
		t.Errorf("empty query fragment:\n%.300s", idle)
	}
}

// paletteResults keeps every destination real and every total visible: a
// capped group names its exact total and ends with a link to its own search
// page, a result with no page of its own opens its search page, text the
// kit would refuse (control characters) is cleaned rather than dropping
// the fragment, and index notes become the kit's partial notice.
func TestConsolePalette_ResultsNameTotalsAndRealDestinations(t *testing.T) {
	res := console.SearchResults{
		Query: "plan",
		Notes: []string{"plans and intent records could not be read for: x (EPERM)"},
		Groups: []console.SearchGroupResult{
			{Group: console.GroupVerbs, Total: 1, Results: []console.SearchResult{{SearchDoc: console.SearchDoc{Group: console.GroupVerbs, Kind: "verb", Title: "plans status", Subtitle: "per-plan\tproof\x01 state"}}}},
			{Group: console.GroupSymbols, Total: 360, Results: []console.SearchResult{{SearchDoc: console.SearchDoc{Group: console.GroupSymbols, Kind: "symbol", Title: "PlanLedger", Service: "fathom", Ref: "internal/console/plans.go:12", Href: "/projects/fathom"}}}},
		},
	}
	out := paletteResults(res, "plan")
	if out.Query != "plan" || !strings.Contains(out.Notice, "EPERM") {
		t.Errorf("results = %+v", out)
	}
	if len(out.Groups) != 2 {
		t.Fatalf("groups = %+v", out.Groups)
	}
	verb := out.Groups[0].Results[0]
	if verb.Href != "/search?group=verbs&q=plans+status" || strings.ContainsRune(verb.Detail, '\x01') || verb.Meta != "verb" {
		t.Errorf("verb result = %+v", verb)
	}
	sym := out.Groups[1]
	if sym.Heading != "Symbols · top 1 of 360" || len(sym.Results) != 2 || sym.Results[1].Label != "Show all 360 in Symbols" || sym.Results[1].Href != "/search?group=symbols&q=plan" {
		t.Errorf("capped group = %+v", sym)
	}
	frag := renderPaletteFragment(res, "plan")
	if !strings.Contains(frag, `data-palette-state="partial"`) || !strings.Contains(frag, `href="/search?group=symbols&amp;q=plan"`) {
		t.Errorf("fragment:\n%s", frag)
	}
}
