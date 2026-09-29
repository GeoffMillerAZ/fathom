package main

// console_kit_s0_test.go — the console adopts the four kit returns that
// landed in app-kit 9fa58e6 (docs/app-kit.md, "Mirror 9fa58e6"), each pinned
// so the local workaround it replaced cannot come back:
//
//   - FATHOM-S0-KIT-01: every page's one h1 is the kit page heading's, inside
//     main; the shell's top bar writes no heading (Shell.PageHeading) and
//     leads with the brand below md (Shell.MobileBrand).
//   - FATHOM-F0-KIT-05: pagers are the kit's own 44 px targets; no console
//     file wraps one to enlarge it.
//   - FATHOM-S0-KIT-02: the modal backdrop is the scrim role, so the role
//     check has no kit exception left.
//   - FATHOM-F0-KIT-06: the wiring preview's "Show both versions" label is
//     ink-muted, not ink-faint.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/internal/wiring"
)

// consoleKitS0ErrorPaths are two pages no registered route renders: the
// console's 404 and an unknown project, both on the shell.
var consoleKitS0ErrorPaths = []string{"/no-such-page", "/projects/no-such-service"}

// between returns s from the first open to the next close after it
// (inclusive), or "" when open is absent.
func between(s, open, close string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], close)
	if j < 0 {
		return s[i:]
	}
	return s[i : i+j+len(close)]
}

func TestConsoleKitS0_PageHeadingOwnsTheOneH1(t *testing.T) {
	f := newConsoleFixture(t)
	paths := append([]string{}, consoleKitS0ErrorPaths...)
	for _, pg := range defaultConsoleRegistry().pages {
		paths = append(paths, samplePath(pg))
	}
	mobileBrand := `<div class="flex min-w-0 items-center gap-3"><div class="flex shrink-0 items-center md:hidden">` + consoleBrand + `</div></div>`
	for _, path := range paths {
		_, _, body := f.get(t, path)
		if n := strings.Count(body, "<h1"); n != 1 {
			t.Errorf("%s: %d h1 elements, want 1", path, n)
			continue
		}
		topBar := between(body, `<header class="flex h-14`, `</header>`)
		if topBar == "" {
			t.Errorf("%s: no kit top bar", path)
			continue
		}
		if strings.Contains(topBar, "<h1") {
			t.Errorf("%s: the top bar still writes the h1:\n%s", path, topBar)
		}
		// With no Title the top bar's leading slot is the brand alone,
		// shown below md; the palette trigger stays at its end.
		if !strings.Contains(topBar, mobileBrand) || !strings.Contains(topBar, `data-palette-open`) {
			t.Errorf("%s: the top bar must lead with the kit's mobile brand slot and end with the palette trigger:\n%s", path, topBar)
		}
		if strings.Contains(body, "data-console-mobile-brand") {
			t.Errorf("%s: the local mobile brand is still rendered", path)
		}
		main := between(body, `<main id="`+consoleContentID+`"`, `</main>`)
		heading := between(main, `<header data-page-heading`, `</header>`)
		if !strings.Contains(heading, "<h1") {
			t.Errorf("%s: the h1 must be the kit page heading's, inside main", path)
			continue
		}
		// The trail is the kit heading's own; no second trail is drawn.
		if strings.Count(body, `aria-label="Breadcrumb"`) != 1 || !strings.Contains(heading, `aria-label="Breadcrumb"`) {
			t.Errorf("%s: the breadcrumb trail must render once, inside the kit heading", path)
		}
	}

	// The heading's meta line is the kit's wrapping list.
	_, _, fleet := f.get(t, "/fleet")
	meta := between(fleet, `<ul role="list" class="mt-2 flex flex-wrap`, `</ul>`)
	if strings.Count(meta, `<li class="min-w-0 [overflow-wrap:anywhere]">`) < 1 {
		t.Errorf("/fleet: the heading meta must be the kit's list, got %q", meta)
	}
}

// A page's fields are shaped to the kit heading's bounds, so a field the kit
// would refuse can never cost the page its h1.
func TestConsoleKitS0_HeadingNeverRefusesAPage(t *testing.T) {
	long := strings.Repeat("x", 400)
	var meta []string
	for i := 0; i < chrome.MaxHeadingMeta+3; i++ {
		meta = append(meta, fmt.Sprintf("m%d", i))
	}
	meta = append(meta, "  ", "tab\there")
	got := consolePageHeading(consoleShell{
		Title:  "Title\nwith a newline " + long,
		Crumbs: []chrome.Crumb{{Label: "Observatory", Href: "/"}, {Label: "bad", Href: "javascript:alert(1)"}, {Label: "  "}, {Label: "Here"}},
		Meta:   meta,
	})
	if strings.Count(got, "<h1") != 1 {
		t.Fatalf("heading refused:\n%s", got)
	}
	if strings.Contains(got, "javascript:") || !strings.Contains(got, `>bad</li>`) {
		t.Errorf("a refused crumb destination must be written as plain text:\n%s", got)
	}
	if n := strings.Count(got, `<li class="min-w-0 [overflow-wrap:anywhere]">`); n != chrome.MaxHeadingMeta {
		t.Errorf("meta items = %d, want %d (the rest folded into the last)", n, chrome.MaxHeadingMeta)
	}
	if !strings.Contains(got, "tab here") || !strings.Contains(got, "…</h1>") {
		t.Errorf("control characters become spaces and a long title ends in …:\n%.400s", got)
	}
	if empty := consolePageHeading(consoleShell{}); !strings.Contains(empty, ">Untitled page</h1>") {
		t.Errorf("an empty title still renders an h1:\n%s", empty)
	}
}

// pagerWrapperRe matches the local wrappers that lifted the kit's old
// ~28 px page links: descendant min-height variants on a pager's links or
// its disabled ends.
var pagerWrapperRe = regexp.MustCompile(`\[&_(?:a|span)\]:(?:inline-flex|min-h-11|items-center)`)

func TestConsoleKitS0_PagersAreTheKitsOwnTargets(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		if !strings.Contains(src, "RenderPagination") && !strings.Contains(src, "consolePager(") {
			continue
		}
		for _, line := range strings.Split(src, "\n") {
			if strings.Contains(line, "[&_span]:") || (strings.Contains(line, "RenderPagination") || strings.Contains(line, "data-console-pager") || strings.Contains(line, `"page " + strconv`)) && pagerWrapperRe.MatchString(line) {
				t.Errorf("%s: a pager is still wrapped to enlarge the kit's links: %s", name, strings.TrimSpace(line))
			}
		}
	}
	out := consolePager(chrome.Pagination{Page: 2, TotalPages: 3, AriaLabel: "Plans pages", HrefFor: func(n int) string { return fmt.Sprintf("/plans?page=%d", n) }})
	if pagerWrapperRe.MatchString(out) || !strings.Contains(out, `<a href="/plans?page=3" class="inline-flex min-h-11 min-w-11`) || !strings.Contains(out, "focus-visible:outline-focus") {
		t.Errorf("the pager must be the kit's own 44 px targets with a focus outline:\n%s", out)
	}
}

func TestConsoleKitS0_NoKitPaletteColorException(t *testing.T) {
	// The role check exempts nothing: the kit's old backdrop is flagged. The
	// class is assembled so the CSS build's source scan never emits it.
	if got := paletteColorClasses(`<dialog class="rounded-lg backdrop:bg-` + `black/40"></dialog>`); len(got) != 1 {
		t.Errorf("paletteColorClasses must flag the old kit backdrop, got %q", got)
	}
	f := newConsoleFixture(t)
	_, _, home := f.get(t, "/")
	dialog := between(home, `<dialog`, `>`)
	if !strings.Contains(dialog, "backdrop:bg-scrim/50") {
		t.Errorf("the palette dialog's backdrop must be the scrim role: %s", dialog)
	}
	if drawer := between(renderRowDrawer(), `<dialog`, `>`); !strings.Contains(drawer, "backdrop:bg-scrim/50") {
		t.Errorf("the plan drawer's backdrop must be the scrim role: %s", drawer)
	}
	if got := paletteColorClasses(home); len(got) != 0 {
		t.Errorf("/ renders palette colors: %q", got)
	}
}

func TestConsoleKitS0_DetailsLabelIsInkMuted(t *testing.T) {
	out := renderWiringPreview(wiringPreview{Service: "svc", Root: "/r", Plan: wiring.Plan{Root: "/r", Items: []wiring.Item{{
		File: "CLAUDE.md", Target: "stanza", Kind: wiring.KindRegion, Verdict: wiring.VerdictBlocked,
		Cause: wiring.CauseDrift, Reason: "hand-edited", Present: true, Before: "mine", After: "fathom's",
	}}}})
	label := between(out, `<span class="ml-auto flex shrink-0 items-center gap-1 text-xs`, `</span>`)
	if !strings.Contains(label, "text-ink-muted") || strings.Contains(label, "text-ink-faint") || !strings.Contains(label, "Show both versions") {
		t.Errorf("the Details label must be ink-muted: %s", label)
	}
}
