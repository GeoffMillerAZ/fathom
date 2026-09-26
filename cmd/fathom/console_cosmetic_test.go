package main

// console_cosmetic_test.go — the F0 browser proof's minor items (its
// items 3, 5, 6, 7 and 8), each pinned so it cannot quietly return: paths
// and commands wrap at their separators, /statusz's table fits a phone, a
// nav label is its page's title, the session page names its session once
// and states law 14 once, and the brand stays in the top bar below md.

import (
	"regexp"
	"strings"
	"testing"
)

func TestConsoleCosmetics_F0ProofMinorItems(t *testing.T) {
	// Item 3: a path wraps after its separators and sits left-aligned; a
	// remedy command wraps at its spaces; nothing breaks mid-word by rule.
	p := pathHTML("/Volumes/m2ext/code/geoffmilleraz/sextant")
	if !strings.Contains(p, "/<wbr>code/<wbr>geoffmilleraz/<wbr>sextant") || !strings.Contains(p, "text-left") || strings.Contains(p, "break-all") {
		t.Errorf("pathHTML = %s", p)
	}
	if r := remedy("fathom post-commit-hook --dir /x --service y --timeout 2m"); strings.Contains(r, "break-all") || !strings.Contains(r, "[overflow-wrap:anywhere]") {
		t.Errorf("remedy = %s", r)
	}
	wiring := renderWiringIndex([]registeredService{{Service: "fathom", Root: "/Volumes/m2ext/code/geoffmilleraz/sextant"}}, nil)
	if !strings.Contains(wiring, "geoffmilleraz/<wbr>sextant") {
		t.Error("a wiring card's root must wrap at its separators")
	}
	project, ok := renderObsService(observatoryModel{Services: []obsServiceRow{{fleetServiceRow: fleetServiceRow{Service: "fathom", Path: "/Volumes/m2ext/code/sextant"}}}}, "fathom")
	if !ok || !strings.Contains(project, "code/<wbr>sextant") {
		t.Error("the project page's path row must wrap at its separators")
	}

	f := newConsoleFixture(t)
	brandRe := regexp.MustCompile(`<a href="/" class="[^"]*md:hidden" data-console-mobile-brand>fathom</a><a href="/search" data-palette-open`)
	for _, pg := range defaultConsoleRegistry().pages {
		path := samplePath(pg)
		_, _, body := f.get(t, path)
		if strings.Contains(body, "break-all") {
			t.Errorf("%s: a value still breaks mid-word by rule", path)
		}
		// Item 8: the top bar keeps a brand below md, beside the palette.
		if !brandRe.MatchString(body) {
			t.Errorf("%s: no mobile brand in the top bar", path)
		}
		// Item 6: a navigable page's nav label is its title. Home is the
		// one deliberate exception: vision 35's IA names the group "Now"
		// and its page "Home", and the page's heading answers "Now".
		if pg.Nav && pg.NavLabel != pg.Title && pg.ID != "home" {
			t.Errorf("page %s: nav label %q differs from its title %q", pg.ID, pg.NavLabel, pg.Title)
		}
	}
	_, _, statusz := f.get(t, "/statusz")
	if !regexp.MustCompile(`<a href="/statusz"[^>]*><span[^>]*>Daemon status</span></a>`).MatchString(statusz) || strings.Contains(statusz, ">Statusz<") {
		t.Error("the nav must name /statusz by its title, Daemon status")
	}
	// Item 5: the services table fits a phone — the service name is the
	// drill-down link (no "Open →" column), and no cell forces a width.
	for _, bad := range []string{"Open →", "min-w-48", "min-w-64"} {
		if strings.Contains(statusz, bad) {
			t.Errorf("/statusz still renders %q", bad)
		}
	}
	if !strings.Contains(statusz, `<a href="/projects/payment-service" class="inline-flex min-h-11`) {
		t.Error("/statusz's service names must be the drill-down links")
	}
	// Below lg the same rows stack as one description list per service
	// (four columns squeezed the text to a word per line at 390 px); one
	// rendering is displayed at a width.
	if !strings.Contains(statusz, `<div class="hidden lg:block" data-statusz-table>`) || !strings.Contains(statusz, `<ul class="flex flex-col gap-3 lg:hidden" aria-label="services" data-statusz-list>`) ||
		strings.Count(statusz, `<li class="min-w-0 rounded-lg border border-line bg-surface-raised px-3 pb-3"><a href="/projects/`) != len(consoleFixtureServices) {
		t.Error("/statusz must render its services as a table from lg up and a stacked list below it")
	}

	// Item 7: the session page's h1 is short enough not to truncate, the
	// full id is in the meta line, law 14 is stated once, and no
	// uppercased label repeats the id.
	env := f.pageJSON(t, "/sessions/8c867d31-40ca-4619-b77b-f8f6b680fc2f")
	if env.Title != "Session 8c867d31" {
		t.Errorf("session title = %q", env.Title)
	}
	_, _, session := f.get(t, "/sessions/8c867d31-40ca-4619-b77b-f8f6b680fc2f")
	if !strings.Contains(session, ">Session 8c867d31</h1>") || !strings.Contains(session, "session id 8c867d31-40ca-4619-b77b-f8f6b680fc2f") {
		t.Error("the session page must name its session in the h1 and its full id in the meta line")
	}
	if n := strings.Count(session, "never evidence (law 14)"); n != 1 {
		t.Errorf("the law-14 caveat appears %d times, want once", n)
	}
	if strings.Contains(strings.ToLower(session), ">session 8c867d31-40ca") {
		t.Error("no section label may repeat the session id")
	}
}
