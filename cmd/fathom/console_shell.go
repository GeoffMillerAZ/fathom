package main

// console_shell.go — OC-4 (docs/design/observatory-console.md §2): every
// console page renders on one shell. The kit owns the frame
// (dashboard.RenderShell with its skip link and mobile disclosure), the
// grouped sidebar (dashboard.RenderNav over the registry's groups), the
// trail (chrome.Breadcrumbs) and the theme (appearance: a bootstrap script
// before the stylesheet plus the native control, storage key fathom-theme).
// The page heading's meta-and-actions block is composed here from role
// utilities: it is FATHOM-KIT-07, which fathom builds just in time as the
// originating consumer and returns to the kit (vision 35 §6).

import (
	_ "embed"
	"html"
	"html/template"
	"net/http"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/appearance"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/form"
)

// consoleThemeKey is the appearance storage key (design §2).
const consoleThemeKey = "fathom-theme"

// consoleContentID is the skip link's target and the main landmark's id.
const consoleContentID = "observatory-main"

//go:embed ui/console.js
var consoleJS []byte

// consoleShell is one page's frame.
type consoleShell struct {
	PageID      string         // the registered page (nav highlight); "" for none
	Title       string         // the h1
	DocTitle    string         // <title>; defaults to Title · fathom observatory
	Crumbs      []chrome.Crumb // the full trail, Observatory first; current page last
	Question    string         // what the page answers
	Meta        []string       // heading meta line
	Verbs       []string       // the verbs the page renders, as copyable chips
	Actions     string         // trusted heading actions HTML
	Body        string         // trusted page content
	Footer      string         // trusted footer HTML; a default is used when empty
	SearchQuery string         // prefills the top-bar search field
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
	content.WriteString(s.Body)
	content.WriteString(`</div>`)

	footer := s.Footer
	if footer == "" {
		footer = `<p class="mx-auto max-w-7xl text-xs text-ink-muted">loopback-only; sharing is a human verb (law 13) — <code class="font-mono">fathom brief</code> writes the shareable file · JSON: add <code class="font-mono">?format=json</code> to any page</p>`
	}

	shell := dashboard.RenderShell(dashboard.Shell{
		Title:     s.Title,
		Brand:     template.HTML(`<a href="/" class="text-base font-semibold text-ink hover:text-accent">fathom observatory</a>`),
		Nav:       template.HTML(consoleNav(reg, reg.navCurrentPath(s.PageID))),
		Actions:   template.HTML(consoleSearchForm(s.SearchQuery)),
		Content:   template.HTML(content.String()),
		Footer:    template.HTML(footer),
		ContentID: consoleContentID,
		SkipLink:  true,
	})
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">`)
	b.WriteString(`<title>` + html.EscapeString(doc) + `</title>`)
	b.WriteString(`<link rel="icon" href="data:image/svg+xml,<svg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 100 100%22><text y=%22.9em%22 font-size=%2290%22>🔱</text></svg>">`)
	// The appearance bootstrap runs BEFORE the stylesheet so a saved dark or
	// light preference never flashes the other theme.
	b.WriteString(`<script>` + appearance.BootstrapScript(appearance.Options{StorageKey: consoleThemeKey}) + `</script>`)
	b.WriteString(`<link rel="stylesheet" href="/assets/app.css"><script src="/assets/console.js" defer></script>`)
	b.WriteString(`</head><body class="min-h-screen bg-surface font-sans text-sm text-ink">`)
	b.WriteString(shell)
	b.WriteString(`</body></html>`)
	return b.String()
}

// consoleNav is the grouped sidebar plus the theme control beneath it.
func consoleNav(reg *consoleRegistry, currentPath string) string {
	return dashboard.RenderNav(reg.navGroups(), currentPath) +
		`<div class="mt-4 border-t border-line px-3 pt-4">` + appearance.RenderControl(appearance.Options{StorageKey: consoleThemeKey}) + `</div>`
}

// consoleSearchForm is the top bar's search: a real GET form to /search, so
// every palette result is also a page without JavaScript (design §2).
func consoleSearchForm(q string) string {
	return `<form role="search" action="/search" method="get" class="flex min-w-0 items-center">` +
		`<label for="console-search-q" class="sr-only">Search projects, plans, symbols, rules, verbs and pages</label>` +
		`<div class="w-36 sm:w-64 [&>input]:min-h-11">` +
		form.RenderInput(form.Input{ID: "console-search-q", Name: "q", Type: "search", Value: q, Placeholder: "Search…", Autocomplete: "off"}) +
		`</div></form>`
}

// renderConsoleHeading is the page heading block under the top bar's h1:
// the trail, the question, the meta line, and the verb chips and actions.
func renderConsoleHeading(s consoleShell) string {
	var b strings.Builder
	b.WriteString(`<div class="mb-4 flex flex-wrap items-start justify-between gap-3 border-b border-line pb-3" data-console-heading>`)
	b.WriteString(`<div class="min-w-0 max-w-3xl">`)
	// The kit's trail links carry their own 44 px target and focus-visible
	// outline since mirror d9dd789 (FATHOM-F0-KIT-03, returned from F0).
	b.WriteString(chrome.Breadcrumbs(s.Crumbs))
	if s.Question != "" {
		b.WriteString(`<p class="mt-1 text-ink-muted">` + html.EscapeString(s.Question) + `</p>`)
	}
	if len(s.Meta) > 0 {
		b.WriteString(`<p class="mt-1 text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(strings.Join(s.Meta, " · ")) + `</p>`)
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
