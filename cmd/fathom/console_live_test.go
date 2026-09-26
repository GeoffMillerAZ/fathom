package main

// console_live_test.go — OC-14: live freshness on the kit's live helper
// (console_live.go). The kit owns the script and its connection contract
// (its own tests and Gallery run) and the browser proof is in the stage's
// reading; these tests pin fathom's side: every panel names the sources it
// follows, in fathom's key and the kit's; every page not built from panels
// is one region following the sources its model reads; the page starts the
// stream from the position it was rendered at; every followed key is one an
// event can name; and an event published after a render reaches that page
// naming its key.

import (
	"bufio"
	"context"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

var (
	panelOpenRe  = regexp.MustCompile(`<div class="[^"]*" data-console-panel[^>]*>`)
	liveCursorRe = regexp.MustCompile(`data-live-last-event-id="([^"]+)"`)
)

func TestConsoleLive_PanelsCarrySourcesAndMarkWithoutMoving(t *testing.T) {
	// A source-keyed panel is a kit region: fathom's key and the kit's on
	// one focusable element, and a status line whose "Updated" layer shares
	// the as-of layer's grid cell (hidden until an event marks it), so
	// marking moves nothing. Its freshness leads that line, once.
	got := renderPanel(consolePanel{
		ID: "plans", Title: "Plans", Question: "What is proven?", Verb: "fathom plans status --json",
		Source: "the verify ledger", Denominator: "58 plans", Freshness: "as of 12:00 UTC",
		SourceKey: "plans:fathom verify:fathom", Body: "<p>628 proven</p>",
	})
	for _, want := range []string{
		`id="plans" tabindex="-1" data-live-source="plans:fathom verify:fathom" data-live-label="Plans" data-console-source="plans:fathom verify:fathom"`,
		`<span data-live-asof class="col-start-1 row-start-1">freshness: as of 12:00 UTC</span>`,
		`<span data-live-updated class="invisible col-start-1 row-start-1 text-ink">freshness: as of 12:00 UTC · <strong class="font-semibold">Updated</strong> — <a data-live-refresh href="#plans"`,
		`data-panel-footer>source: the verify ledger · denominator: 58 plans</p>`,
		`focus-visible:outline-focus"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("live panel missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "as of 12:00 UTC") != 2 || strings.Contains(got, "freshness: as of 12:00 UTC</p>") {
		t.Errorf("freshness must lead the status line once, not repeat in the footer:\n%s", got)
	}
	// A key outside the kit's grammar degrades to a plain panel, never a
	// broken region.
	if bad := renderPanel(consolePanel{ID: "x", Title: "X", SourceKey: "*"}); strings.Contains(bad, "data-live-source") || !strings.Contains(bad, `data-console-source="*"`) {
		t.Errorf("an invalid key must render a plain panel:\n%s", bad)
	}

	f := newConsoleFixture(t)
	producible := producibleSourceKeys()

	for _, p := range defaultConsoleRegistry().pages {
		path := samplePath(p)
		_, _, body := f.get(t, path)
		// Every panel carries its source name.
		for _, open := range panelOpenRe.FindAllString(body, -1) {
			if !strings.Contains(open, "data-console-source=") || !strings.Contains(open, "data-live-source=") {
				t.Errorf("%s: a panel without its source: %s", path, open)
			}
		}
		keys := liveKeysOf(body)
		if len(keys) == 0 {
			if len(p.Live) > 0 {
				t.Errorf("%s declares live sources %v but renders no region", path, p.Live)
			}
			if strings.Contains(body, "data-live-page") || strings.Contains(body, consoleLiveScriptPath) {
				t.Errorf("%s: a page with no region must not start the stream", path)
			}
			continue
		}
		// A live page starts the kit script once, against /events, from
		// the stream position it was rendered at, following every kind.
		for want, n := range map[string]int{
			`data-live-page data-live-url="/events" data-live-events="ingest compile verify session daemon gap"`: 1,
			`<script src="/assets/live.js" defer></script>`:                                                      1,
		} {
			if c := strings.Count(body, want); c != n {
				t.Errorf("%s: %q appears %d times, want %d", path, want, c, n)
			}
		}
		m := liveCursorRe.FindStringSubmatch(body)
		if m == nil || !strings.HasPrefix(m[1], f.deps.bus.Epoch()+"-") {
			t.Errorf("%s: the page must carry this bus's cursor, got %v", path, m)
		}
		// Every key a page follows is a key some event names.
		for _, k := range keys {
			if !producible[k] {
				t.Errorf("%s follows %q, which no event ever names", path, k)
			}
		}
		// A page not built from panels is one region leading with its
		// status line.
		if len(p.Live) > 0 && !strings.Contains(body, `<div id="page-live" tabindex="-1" data-live-source="`) {
			t.Errorf("%s: no page region", path)
		}
	}
	// The project page fills its own service into its keys.
	_, _, project := f.get(t, "/projects/payment-service")
	if !strings.Contains(project, `data-live-source="index:payment-service layers:payment-service sessions:payment-service plans:payment-service"`) {
		t.Errorf("the project page must follow its own service's sources")
	}

	// The kit script is served at a stable path.
	code, hdr, js := f.get(t, "/assets/live.js")
	if code != http.StatusOK || !strings.Contains(hdr.Get("Content-Type"), "javascript") || !strings.Contains(js, "AppKitLive") {
		t.Errorf("/assets/live.js = %d %q", code, hdr.Get("Content-Type"))
	}

	// End to end: a verify-ledger row after /impact rendered reaches a
	// stream opened from the page's own cursor, naming "plans", a key the
	// page follows — the event the page marks itself updated on.
	_, _, impact := f.get(t, "/impact")
	cursor := liveCursorRe.FindStringSubmatch(impact)[1]
	if !strings.Contains(impact, `data-live-source="sessions plans"`) {
		t.Fatalf("/impact must follow sessions and plans")
	}
	if _, err := f.deps.bus.Publish(console.Event{Kind: console.EventVerify, Service: "payment-service", Name: "plan.md", Ref: "pass"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, f.ts.URL+"/events", nil)
	req.Header.Set("Last-Event-ID", cursor)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	var stream []string
	for sc.Scan() {
		stream = append(stream, sc.Text())
		if strings.HasPrefix(sc.Text(), "data: {\"id\"") {
			break
		}
	}
	joined := strings.Join(stream, "\n")
	if !strings.Contains(joined, "event: verify") || !strings.Contains(joined, `"plans"`) || strings.Contains(joined, "event: gap") {
		t.Errorf("the stream from the page's cursor must replay the verify event naming plans, with no gap:\n%s", joined)
	}
}

// liveKeysOf returns every source key a rendered page's regions follow.
func liveKeysOf(body string) []string {
	var out []string
	for _, m := range regexp.MustCompile(`data-live-source="([^"]*)"`).FindAllStringSubmatch(body, -1) {
		out = append(out, strings.Fields(m[1])...)
	}
	return out
}

// producibleSourceKeys is every key an event of each kind names for the
// fixture fleet's services.
func producibleSourceKeys() map[string]bool {
	keys := map[string]bool{}
	for _, svc := range consoleFixtureServices {
		for _, e := range []console.Event{
			{Kind: console.EventIngest, Service: svc},
			{Kind: console.EventCompile, Service: svc, Name: "adr-rules"},
			{Kind: console.EventVerify, Service: svc},
			{Kind: console.EventSession, Service: svc},
			{Kind: console.EventDaemon},
		} {
			for _, k := range e.Sources() {
				keys[k] = true
			}
		}
	}
	return keys
}
