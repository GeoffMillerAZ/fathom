package main

// console_shell.go — OC-4 (docs/design/observatory-console.md §2): every
// console page renders on one shell. The kit owns the frame
// (dashboard.RenderShell with its skip link and mobile disclosure), the
// grouped sidebar (dashboard.RenderNav over the registry's groups), the
// trail (chrome.Breadcrumbs) and the theme (appearance: a bootstrap script
// before the stylesheet plus the native control, storage key fathom-theme).
// The page heading is the kit's chrome.RenderPageHeading (FATHOM-KIT-07):
// the trail, the page's one h1 and its meta list. The shell's top bar writes
// no heading (Shell.PageHeading) and shows the brand below md
// (Shell.MobileBrand), both returned as FATHOM-S0-KIT-01. The question, verb
// chips and actions sit beside the kit heading. The top bar carries the ⌘K
// palette trigger and every page carries its dialog (console_palette.go,
// OC-12).

import (
	_ "embed"
	"html"
	"html/template"
	"net/http"
	"strings"
	"unicode"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/appearance"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/dashboard"
)

// consoleThemeKey is the appearance storage key (design §2).
const consoleThemeKey = "fathom-theme"

// consoleContentID is the skip link's target and the main landmark's id.
const consoleContentID = "observatory-main"

//go:embed ui/console.js
var consoleJS []byte

// consoleShell is one page's frame.
type consoleShell struct {
	PageID   string         // the registered page (nav highlight); "" for none
	Title    string         // the h1 (the kit page heading's)
	DocTitle string         // <title>; defaults to Title · fathom observatory
	Crumbs   []chrome.Crumb // the full trail, Observatory first; current page last
	Question string         // what the page answers
	Meta     []string       // heading meta line
	Verbs    []string       // the verbs the page renders, as copyable chips
	Actions  string         // trusted heading actions HTML
	Body     string         // trusted page content
	Footer   string         // trusted footer HTML; a default is used when empty
	// Live is the page's live-freshness status line (consoleLiveStatus),
	// set when the body carries live regions; it also loads the kit script.
	Live string
}

// renderConsoleShell renders a complete HTML document on the console shell.
func renderConsoleShell(reg *consoleRegistry, s consoleShell) string {
	doc := s.DocTitle
	if doc == "" {
		doc = s.Title + " · fathom observatory"
	}
	var content strings.Builder
	content.WriteString(`<div class="mx-auto w-full max-w-7xl">`)
	content.WriteString(renderConsoleHeading(s))
	content.WriteString(s.Live)
	content.WriteString(s.Body)
	content.WriteString(`</div>`)

	footer := s.Footer
	if footer == "" {
		footer = `<p class="mx-auto max-w-7xl text-xs text-ink-muted">loopback-only; sharing is a human verb (law 13) — <code class="font-mono">fathom brief</code> writes the shareable file · JSON: add <code class="font-mono">?format=json</code> to any page</p>`
	}

	// The page heading in Content writes the page's one h1, so the top bar
	// writes none and, with no Title, leads with the brand below md only.
	shell := dashboard.RenderShell(dashboard.Shell{
		Brand:       template.HTML(consoleBrand),
		Nav:         template.HTML(consoleNav(reg, reg.navCurrentPath(s.PageID))),
		Actions:     template.HTML(consolePaletteTrigger()),
		Content:     template.HTML(content.String()),
		Footer:      template.HTML(footer),
		ContentID:   consoleContentID,
		SkipLink:    true,
		PageHeading: true,
		MobileBrand: true,
	})
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">`)
	b.WriteString(`<title>` + html.EscapeString(doc) + `</title>`)
	b.WriteString(`<link rel="icon" href="data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>🔱</text></svg>">`)
	// The appearance bootstrap runs BEFORE the stylesheet so a saved dark or
	// light preference never flashes the other theme.
	b.WriteString(`<script>` + appearance.BootstrapScript(appearance.Options{StorageKey: consoleThemeKey}) + `</script>`)
	b.WriteString(`<link rel="stylesheet" href="/assets/app.css"><script src="/assets/console.js" defer></script>`)
	b.WriteString(`<script src="` + consolePaletteScriptPath + `" defer></script>`)
	if s.Live != "" {
		b.WriteString(`<script src="` + consoleLiveScriptPath + `" defer></script>`)
	}
	// The kit shell's <main> is overflow-y:auto inside an unbounded
	// min-h-screen column: it never scrolls, yet as a scroll container it
	// pins every position:sticky descendant to a box that never moves. A
	// page that positions something sticky marks itself
	// data-console-sticky, and main then clips sideways (as auto did)
	// without being a scroll container (FATHOM-G-KIT-04). The classes are
	// literal — the Tailwind scanner reads source text, not Go values — so
	// consoleStickyMainClasses names consoleContentID by hand, and
	// TestConsoleShell_StickyPagesFreeMain holds the two together.
	b.WriteString(`</head><body class="min-h-screen bg-surface font-sans text-sm text-ink ` + consoleStickyMainClasses + `">`)
	b.WriteString(shell)
	// The palette's dialog, once per page (OC-12): Mod+K or the top bar's
	// trigger opens it; without JavaScript the trigger is a link to /search.
	b.WriteString(consolePaletteDialog())
	b.WriteString(`</body></html>`)
	return b.String()
}

// consoleStickyMainClasses free the shell's main from being a scroll
// container on a page marked data-console-sticky (FATHOM-G-KIT-04).
const consoleStickyMainClasses = `[&:has([data-console-sticky])_#observatory-main]:overflow-x-clip [&:has([data-console-sticky])_#observatory-main]:overflow-y-visible`

// consoleBrand is the app's brand link: the sidebar's first row and, through
// the kit shell's MobileBrand, the top bar's first item below md (F0 proof,
// item 8). It is a 44 px target with the focus outline and carries no id,
// because the shell writes it twice.
const consoleBrand = `<a href="/" class="inline-flex min-h-11 items-center rounded-sm text-base font-semibold text-ink hover:text-accent focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">fathom observatory</a>`

// consoleNav is the grouped sidebar plus the theme control beneath it.
func consoleNav(reg *consoleRegistry, currentPath string) string {
	return dashboard.RenderNav(reg.navGroups(), currentPath) +
		`<div class="mt-4 border-t border-line px-3 pt-4">` + appearance.RenderControl(appearance.Options{StorageKey: consoleThemeKey}) + `</div>`
}

// renderConsoleHeading is the page heading block: the kit's page heading
// (the trail, the page's one h1, the meta list) with the question under it,
// and the verb chips and actions beside it (the kit heading's own actions are
// form buttons only).
func renderConsoleHeading(s consoleShell) string {
	var b strings.Builder
	b.WriteString(`<div class="mb-4 flex flex-wrap items-start justify-between gap-3 border-b border-line pb-3" data-console-heading>`)
	b.WriteString(`<div class="min-w-0 max-w-3xl">`)
	b.WriteString(consolePageHeading(s))
	if s.Question != "" {
		b.WriteString(`<p class="mt-2 text-ink-muted">` + html.EscapeString(s.Question) + `</p>`)
	}
	b.WriteString(`</div>`)
	if len(s.Verbs) > 0 || s.Actions != "" {
		b.WriteString(`<div class="flex min-w-0 flex-wrap items-center gap-2">`)
		for _, v := range s.Verbs {
			b.WriteString(renderVerbChip(v))
		}
		b.WriteString(s.Actions)
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// consolePageHeading renders the kit page heading from a page's fields,
// shaped to the kit's bounds so a refusal can never cost a page its h1:
// control characters become spaces, a field past the kit's length bound is
// shortened, blank meta items and blank crumbs are dropped, meta past
// chrome.MaxHeadingMeta folds into the last item, and a crumb whose
// destination the kit refuses is written as plain text. The kit escapes
// every value.
func consolePageHeading(s consoleShell) string {
	h := chrome.PageHeading{Title: headingText(s.Title)}
	if h.Title == "" {
		h.Title = "Untitled page"
	}
	for _, c := range s.Crumbs {
		if c.Label = headingText(c.Label); c.Label == "" {
			continue
		}
		if c.Href != "" && (chrome.PageHeading{Title: h.Title, Breadcrumbs: []chrome.Crumb{c}}).Validate() != nil {
			c.Href = ""
		}
		h.Breadcrumbs = append(h.Breadcrumbs, c)
	}
	for _, m := range s.Meta {
		if m = headingText(m); m != "" {
			h.Meta = append(h.Meta, m)
		}
	}
	if len(h.Meta) > chrome.MaxHeadingMeta {
		rest := headingText(strings.Join(h.Meta[chrome.MaxHeadingMeta-1:], " · "))
		h.Meta = append(h.Meta[:chrome.MaxHeadingMeta-1], rest)
	}
	out, err := chrome.RenderPageHeading(h)
	if err != nil {
		// Unreachable once shaped; the title alone always validates.
		out, _ = chrome.RenderPageHeading(chrome.PageHeading{Title: h.Title})
	}
	return out
}

// headingMaxRunes is the kit heading's per-field bound (chrome's unexported
// maxHeadingRunes); a longer field is refused, so it is shortened here.
const headingMaxRunes = 300

// headingText makes s a valid kit heading field: invalid UTF-8 is replaced,
// control characters become spaces, surrounding space is trimmed, and text
// past the bound ends in "…".
func headingText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(s, "\uFFFD"))
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > headingMaxRunes {
		s = strings.TrimSpace(string(r[:headingMaxRunes-1])) + "…"
	}
	return s
}

// consoleCrumbs builds the trail for a page: Observatory, the navigable
// parent when the page has one, then the current title.
func consoleCrumbs(reg *consoleRegistry, pageID, title string) []chrome.Crumb {
	crumbs := []chrome.Crumb{{Label: "Observatory", Href: "/"}}
	if p, ok := reg.page(pageID); ok && p.NavParent != "" {
		if parent, ok := reg.page(p.NavParent); ok && parent.Href != "" {
			crumbs = append(crumbs, chrome.Crumb{Label: parent.Title, Href: parent.Href})
		}
	}
	if pageID == "home" {
		return []chrome.Crumb{{Label: "Observatory", Href: "/"}, {Label: title}}
	}
	return append(crumbs, chrome.Crumb{Label: title})
}

// consoleAssetsJSHandler serves the console's small L2 script (copy chips).
func consoleAssetsJSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(consoleJS)
	}
}
