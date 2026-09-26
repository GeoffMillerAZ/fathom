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
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
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
		// The breadcrumb trail's links carry the kit's own 44 px target and
		// focus outline (FATHOM-F0-KIT-03): no local wrapper patches them.
		if i := strings.Index(body, `aria-label="Breadcrumb"`); i >= 0 {
			trail := body[i:]
			trail = trail[:strings.Index(trail, "</nav>")]
			if strings.Contains(trail, "<a ") && !strings.Contains(trail, `class="inline-flex min-h-11 min-w-11`) {
				t.Errorf("%s: breadcrumb links need the kit's 44 px target", path)
			}
		}
		// Wave F0's stopgaps are gone now that the kit carries the fixes.
		for _, stopgap := range []string{`[&_a]:min-h-11 [&_a]:items-center [&_a]:rounded-sm`, `[&>svg]:max-w-full`, `relative overflow-hidden rounded-lg`} {
			if strings.Contains(body, stopgap) {
				t.Errorf("%s: F0 stopgap %q is still rendered", path, stopgap)
			}
		}
	}
	// /statusz renders the kit table: its header cells carry scope, written
	// by the kit itself (FATHOM-F0-KIT-01), inside the kit's positioned
	// scroll wrapper (FATHOM-F0-KIT-02).
	if _, _, st := f.get(t, "/statusz"); !strings.Contains(st, `<th scope="col"`) || !strings.Contains(st, `<div class="relative overflow-x-auto`) {
		t.Error("/statusz's table must carry th scope inside the kit's positioned scroll wrapper")
	}
	if got := renderKitTable(table.TableParams{Columns: []table.Column{{Key: "a", Label: "A"}}}); strings.Count(got, `scope="col"`) != len(thOpenRe.FindAllString(got, -1)) {
		t.Errorf("the kit table must scope every header cell itself:\n%s", got)
	}

	// The tool-load chart: drawn twice with the kit's Fluid option — a wide
	// drawing from md up, capped at its native width, and a narrow drawing
	// for phones — plus a table alternative behind a disclosure. Wave B
	// rebuilt /fleet as the project table; the chart still renders where
	// renderObsFleet does (the theme guide's specimen).
	fleet := renderObsFleet(cachedObservatoryModel(f.ts.URL, f.now))
	for _, want := range []string{
		`<div class="hidden max-w-[35rem] md:block" data-chart-width="wide"><svg xmlns="http://www.w3.org/2000/svg" width="100%" role="img" aria-label="calls per tool" viewBox="0 0 560 `,
		`<div class="max-w-80 md:hidden" data-chart-width="narrow"><svg xmlns="http://www.w3.org/2000/svg" width="100%" role="img" aria-label="calls per tool" viewBox="0 0 320 `,
		"calls per tool as a table", `data-table="fleet-tool-load"`,
	} {
		if !strings.Contains(fleet, want) {
			t.Errorf("/fleet tool chart missing %q", want)
		}
	}
	// Text-free sparklines are Fluid: they fill their card at every width.
	if strings.Contains(fleet, `aria-label="payment-service 14-day traffic"`) && !strings.Contains(fleet, `width="100%" role="img" aria-label="payment-service 14-day traffic"`) {
		t.Error("a service tile's sparkline must be the kit's Fluid drawing")
	}

	// A session replay's fixed-width timeline scrolls in its own region.
	st := renderSessionStory(sessionStory{SessionID: "s-1", Beats: []heartbeat.Beat{
		{Channel: "edit-hook", Kind: "emitted", At: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)},
		{Channel: "session-hook", At: time.Date(2026, 9, 25, 10, 5, 0, 0, time.UTC)},
	}})
	if !strings.Contains(st, `<div class="overflow-x-auto" role="region" tabindex="0" aria-label="session timeline — scrolls sideways"><div class="min-w-[900px] max-w-[900px] md:min-w-0"><svg xmlns="http://www.w3.org/2000/svg" width="100%"`) {
		t.Errorf("the session timeline must be Fluid from md up and scroll inside its own region below it:\n%.600s", st)
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
