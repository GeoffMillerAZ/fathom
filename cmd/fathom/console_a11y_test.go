package main

// console_a11y_test.go — design §2's accessibility floor on the fixture
// fleet: tables use th scope, targets are 44 px, charts have a table
// alternative and never scale their text below legibility, and a wide
// visualization scrolls inside its own region instead of panning the page
// at 390 px.

import (
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	heartbeat "github.com/geoffmilleraz/agentic-kit/foundations/telemetry/heartbeat"
)

var thOpenRe = regexp.MustCompile(`<th\b[^>]*>`)

func TestConsoleA11y_ScopeTargetsChartsAndOverflow(t *testing.T) {
	f := newConsoleFixture(t)
	for _, p := range defaultConsoleRegistry().pages {
		path := samplePath(p)
		_, _, body := f.get(t, path)
		for _, th := range thOpenRe.FindAllString(body, -1) {
			if !strings.Contains(th, `scope=`) {
				t.Errorf("%s: header cell without scope: %s", path, th)
			}
		}
		// The breadcrumb trail's links get a 44 px target.
		if i := strings.Index(body, `aria-label="Breadcrumb"`); i >= 0 && !strings.Contains(body[:i], `[&_a]:min-h-11`) {
			t.Errorf("%s: breadcrumb links need a 44 px target wrapper", path)
		}
	}
	// /statusz renders the kit table: its header cells carry scope too.
	if _, _, st := f.get(t, "/statusz"); !strings.Contains(st, `<th scope="col"`) {
		t.Error("/statusz's table must carry th scope")
	}

	// The tool-load chart: legible labels in a scroll region, and a table
	// alternative behind a disclosure.
	_, _, fleet := f.get(t, "/fleet")
	if strings.Contains(fleet, `[&>svg]:max-w-full"><svg role="img" aria-label="calls per tool"`) {
		t.Error("the tool chart must not scale its labels down to fit 390 px")
	}
	for _, want := range []string{`aria-label="calls per tool — scrolls sideways"`, "calls per tool as a table", `data-table="fleet-tool-load"`} {
		if !strings.Contains(fleet, want) {
			t.Errorf("/fleet tool chart missing %q", want)
		}
	}

	// A session replay's fixed-width timeline scrolls in its own region.
	st := renderSessionStory(sessionStory{SessionID: "s-1", Beats: []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "emitted", At: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)},
		{Channel: "session-hook", At: time.Date(2026, 9, 25, 10, 5, 0, 0, time.UTC)},
	}})
	if !strings.Contains(st, `<div class="overflow-x-auto" role="region" tabindex="0" aria-label="session timeline — scrolls sideways"><svg`) {
		t.Errorf("the session timeline must scroll inside its own region:\n%.600s", st)
	}

	// Recent-session links are 44 px targets.
	_, _, sessions := f.get(t, "/sessions")
	if strings.Contains(sessions, `href="/sessions/s-`) && !strings.Contains(sessions, `class="inline-flex min-h-11 items-center text-accent font-mono text-xs`) {
		t.Error("recent-session links need a 44 px target")
	}
}

// OC-5 covers the wiring steps too: a GET of a mutation endpoint (Enter in
// the address bar after Preview, a bookmark) and every refusal render on
// the shell with the reason and the way back to /wiring — never a bare
// text/plain body — and a step's page sits under Wiring in the trail.
func TestWiringSteps_ErrorsOnShellAndUnderWiring(t *testing.T) {
	f := newConsoleFixture(t)
	code, hdr, body := f.get(t, "/observatory/wiring/preview")
	if code != http.StatusMethodNotAllowed || !strings.Contains(hdr.Get("Content-Type"), "text/html") || !strings.Contains(body, `<main id="observatory-main"`) || !strings.Contains(body, `href="/wiring"`) {
		t.Errorf("GET preview = %d %q:\n%.600s", code, hdr.Get("Content-Type"), body)
	}
	post := func(form url.Values) (int, string, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, f.ts.URL+"/observatory/wiring/preview", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", f.ts.URL)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header.Get("Content-Type"), string(raw)
	}
	code, ct, body := post(url.Values{"service": {"nosuch"}})
	if code != http.StatusBadRequest || !strings.Contains(ct, "text/html") || !strings.Contains(body, `<main id="observatory-main"`) || !strings.Contains(body, "not in the registry") {
		t.Errorf("unknown service = %d %q:\n%.600s", code, ct, body)
	}
	// (The fixture's services are faux-ingested with no local root, so the
	// step page is rendered directly.)
	body = wiringStepPage("payment-service", "<p>preview</p>", observatoryModel{})
	crumbs := body[strings.Index(body, `aria-label="Breadcrumb"`):]
	crumbs = crumbs[:strings.Index(crumbs, "</nav>")]
	if !strings.Contains(crumbs, `href="/wiring"`) || !strings.Contains(crumbs, "payment-service") || !strings.Contains(body, ">Wiring · payment-service</h1>") {
		t.Errorf("a preview must sit under Wiring in the trail:\n%s", crumbs)
	}
}

// The copy chip's outcome reaches assistive tech: its accessible name is a
// fixed aria-label, so console.js announces "Copied …" / "Copy failed"
// through one polite status region instead of only changing button text.
func TestConsoleJS_AnnouncesCopyOutcome(t *testing.T) {
	f := newConsoleFixture(t)
	_, _, js := f.get(t, "/assets/console.js")
	for _, want := range []string{`'aria-live', 'polite'`, `'role', 'status'`, "data-console-announce", "region.textContent = 'Copied the command '", "Copy failed"} {
		if !strings.Contains(js, want) {
			t.Errorf("console.js missing %q", want)
		}
	}
}
