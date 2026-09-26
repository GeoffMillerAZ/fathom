// Package chrome renders the structural/navigational "creature comfort" UI every
// app expects — empty states, breadcrumbs, pagination, tabs, skeletons — as
// deterministic server-rendered HTML themed through the design tokens (D7). Pure
// Go, no Node; escaped; plain links/markup only, so everything stays within the
// loopback invariant (D5) with no client-side framework.
//
// The bordered-surface "Card" primitive lives in its own package,
// appkit/ui/card (templ slot-based, composed by appkit/ui/kanban), so it is not
// duplicated here.
package chrome

import (
	"fmt"
	"html"
	"html/template"
	"strings"
)

// EmptyState is the standardized "nothing here yet" panel: an optional icon, a
// heading, body text, and an optional call-to-action.
type EmptyState struct {
	Icon    template.HTML // optional inline SVG
	Heading string
	Body    string
	Action  template.HTML // optional CTA (button/link)
}

// RenderEmptyState returns the empty state as an HTML string.
func RenderEmptyState(e EmptyState) string {
	var b strings.Builder
	b.WriteString(`<div class="rounded-lg border border-line bg-surface-sunken p-8 text-center">`)
	if e.Icon != "" {
		fmt.Fprintf(&b, `<div class="mx-auto mb-3 flex justify-center text-ink-faint">%s</div>`, string(e.Icon))
	}
	if e.Heading != "" {
		fmt.Fprintf(&b, `<h3 class="font-medium text-ink">%s</h3>`, esc(e.Heading))
	}
	if e.Body != "" {
		fmt.Fprintf(&b, `<p class="mx-auto mt-1 max-w-md text-sm text-ink-muted">%s</p>`, esc(e.Body))
	}
	if e.Action != "" {
		fmt.Fprintf(&b, `<div class="mt-4">%s</div>`, string(e.Action))
	}
	b.WriteString(`</div>`)
	return b.String()
}

// Crumb is one breadcrumb entry. The last crumb (or any with an empty Href) is
// rendered as the current page.
type Crumb struct {
	Label string
	Href  string
}

// crumbLinkClass gives every crumb link the kit's 44px minimum target and a
// visible keyboard focus outline. Consumers must scan appkit/ui/chrome/*.go so
// the utilities exist in their CSS.
const crumbLinkClass = "inline-flex min-h-11 min-w-11 items-center justify-center rounded-sm hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"

// Breadcrumbs renders a navigation trail. Returns "" for no crumbs. Each link
// is at least 44px square with a focus-visible outline; the current page and
// separators stay plain text.
func Breadcrumbs(crumbs []Crumb) string {
	if len(crumbs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav aria-label="Breadcrumb"><ol class="flex flex-wrap items-center gap-1.5 text-sm text-ink-muted">`)
	for i, c := range crumbs {
		if i > 0 {
			b.WriteString(`<li aria-hidden="true" class="text-ink-faint">/</li>`)
		}
		if i == len(crumbs)-1 || c.Href == "" {
			fmt.Fprintf(&b, `<li class="text-ink" aria-current="page">%s</li>`, esc(c.Label))
		} else {
			fmt.Fprintf(&b, `<li><a href="%s" class="%s">%s</a></li>`, esc(c.Href), crumbLinkClass, esc(c.Label))
		}
	}
	b.WriteString(`</ol></nav>`)
	return b.String()
}

func esc(s string) string { return html.EscapeString(s) }
