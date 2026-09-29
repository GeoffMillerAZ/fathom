package dashboard

import (
	"fmt"
	"html/template"
	"strings"
)

// Shell is the dashboard application frame: a left sidebar (brand + nav), a top
// bar (title + actions), and a scrollable content region. All slots are
// caller-supplied HTML — the app owns its nav links, brand, and user menu; the
// kit owns the responsive, token-themed layout. Adapted from MatDash's FullLayout.
//
// The sidebar is shown from the `md` breakpoint up; on small screens it collapses
// to a top "Menu" disclosure (native <details>, no JS) so the layout is usable
// without a client framework.
type Shell struct {
	Title     string        // page title in the top bar (its h1 unless PageHeading)
	Brand     template.HTML // brand/logo slot (sidebar top); falls back to Title
	Nav       template.HTML // sidebar navigation (app-supplied links)
	Actions   template.HTML // top-bar right slot (search, user menu, buttons)
	Content   template.HTML // main content (typically a WidgetGrid)
	Footer    template.HTML // optional footer inside the content column
	AriaLabel string        // accessible label for the sidebar nav (default "Primary")

	// ContentID is the id put on <main>, and the target the skip link jumps to.
	// Defaults to "content".
	//
	// It exists because the skip link below needs a destination and the app is
	// the only party that knows whether "content" collides with something it
	// already emits — but the DEFAULT is the kit's, so an app that does not
	// care gets a working skip link without knowing this field is here.
	ContentID string

	// SkipLink, when true, emits the first focusable element on the page: a
	// keyboard user must be able to get past the sidebar without tabbing
	// through every nav item.
	//
	// It is opt-in rather than always-on because the shell may be embedded in a
	// page that already has one, and two skip links is worse than none — the
	// second sends a keyboard user somewhere they did not ask to go. Every app
	// rendering the shell as its whole page should set it.
	//
	// Before this shipped, every consumer either re-implemented the link or
	// omitted it, which is accessibility as a per-app decision. The treatment
	// is `sr-only focus:not-sr-only`: visually hidden until focused, the only
	// treatment that serves a screen-reader user and a sighted keyboard user
	// without adding a control nobody else needs.
	SkipLink bool

	// PageHeading, when true, says Content writes the page's one h1 (for
	// example chrome.RenderPageHeading). The top bar then writes Title as plain
	// text rather than an h1 — or nothing when Title is empty — so the page
	// keeps a single h1. Off, the top bar's Title is the h1, for pages with no
	// heading of their own (FATHOM-S0-KIT-01).
	PageHeading bool

	// MobileBrand, when true, repeats Brand at the start of the top bar below
	// the md breakpoint, where the sidebar that carries it is hidden. Brand is
	// then written twice, so it must not carry an id, and a link in it brings
	// its own 44px target and focus treatment.
	MobileBrand bool
}

// RenderShell returns the full dashboard frame as an HTML string.
func RenderShell(s Shell) string {
	navAria := s.AriaLabel
	if navAria == "" {
		navAria = "Primary"
	}
	brand := string(s.Brand)
	if brand == "" {
		brand = fmt.Sprintf(`<span class="text-base font-semibold text-ink">%s</span>`, esc(s.Title))
	}

	contentID := s.ContentID
	if contentID == "" {
		contentID = "content"
	}

	var b strings.Builder
	// The skip link is FIRST in source order, before the wrapper, because
	// "first focusable element" is a statement about document order and a link
	// nested after the sidebar is one the reader tabs to after the thing it
	// exists to skip.
	if s.SkipLink {
		fmt.Fprintf(&b, `<a href="#%s" class="sr-only focus:not-sr-only focus:absolute focus:left-3 focus:top-3 focus:z-50 `+
			`focus:rounded-md focus:bg-accent focus:px-3 focus:py-2 focus:text-sm focus:font-medium `+
			`focus:text-accent-ink focus:outline-2 focus:outline-offset-2 focus:outline-focus">Skip to content</a>`, esc(contentID))
	}
	b.WriteString(`<div class="flex min-h-screen w-full bg-surface-sunken text-ink">`)

	// Sidebar (md+).
	b.WriteString(`<aside class="hidden w-64 shrink-0 flex-col border-r border-line bg-surface-raised md:flex">`)
	fmt.Fprintf(&b, `<div class="flex h-14 items-center border-b border-line px-4">%s</div>`, brand)
	fmt.Fprintf(&b, `<nav class="flex-1 overflow-y-auto p-3" aria-label=%q>%s</nav>`, esc(navAria), string(s.Nav))
	b.WriteString(`</aside>`)

	// Main column.
	b.WriteString(`<div class="flex min-w-0 flex-1 flex-col">`)

	// Top bar.
	b.WriteString(`<header class="flex h-14 shrink-0 items-center justify-between gap-3 border-b border-line bg-surface px-4">`)
	title := fmt.Sprintf(`<h1 class="truncate text-lg font-semibold text-ink">%s</h1>`, esc(s.Title))
	if s.PageHeading {
		title = ""
		if s.Title != "" {
			title = fmt.Sprintf(`<p class="truncate text-lg font-semibold text-ink">%s</p>`, esc(s.Title))
		}
	}
	if s.PageHeading || s.MobileBrand {
		// The leading slot is always written in these modes, even empty, so
		// justify-between keeps the actions at the end of the bar.
		b.WriteString(`<div class="flex min-w-0 items-center gap-3">`)
		if s.MobileBrand {
			fmt.Fprintf(&b, `<div class="flex shrink-0 items-center md:hidden">%s</div>`, brand)
		}
		b.WriteString(title)
		b.WriteString(`</div>`)
	} else {
		b.WriteString(title)
	}
	if s.Actions != "" {
		fmt.Fprintf(&b, `<div class="flex items-center gap-3">%s</div>`, string(s.Actions))
	}
	b.WriteString(`</header>`)

	// Mobile nav disclosure (no JS) — only when nav is supplied.
	if s.Nav != "" {
		b.WriteString(`<details class="border-b border-line bg-surface-raised md:hidden">`)
		b.WriteString(`<summary class="min-h-[44px] cursor-pointer select-none px-4 py-2 text-sm font-medium text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Menu</summary>`)
		fmt.Fprintf(&b, `<nav class="px-3 pb-3" aria-label=%q>%s</nav>`, esc(navAria+" (mobile)"), string(s.Nav))
		b.WriteString(`</details>`)
	}

	// Content.
	fmt.Fprintf(&b, `<main id=%q tabindex="-1" class="flex-1 overflow-y-auto p-4 md:p-6">`, esc(contentID))
	b.WriteString(string(s.Content))
	if s.Footer != "" {
		fmt.Fprintf(&b, `<footer class="mt-6 border-t border-line pt-4 text-sm text-ink-muted">%s</footer>`, string(s.Footer))
	}
	b.WriteString(`</main>`)

	b.WriteString(`</div>`) // main column
	b.WriteString(`</div>`) // root
	return b.String()
}
