package docs

// toc.go — three renderings of the same heading outline.
//
// RenderToC is the boxed nested-list treatment: a document rendered inline
// (a printable bundle, a style-guide specimen) that wants its ToC prepended
// to the body it indexes. RenderToCPanel and RenderToCRail are the two
// viewport treatments of a document page's own "on this page" outline — a
// narrow-screen disclosure and a wide-screen sticky rail, emitted together
// because a zero-JavaScript page has no way to measure the viewport and
// choose one: CSS picks between them (`.xl:hidden` / `.hidden.xl:block` on
// the caller's side), so both render on every request.
//
// All three share one nesting pass (nest) so a document's structure —
// including a skipped level, an h4 directly under an h2 — renders as
// authored rather than as we wish it were, in every treatment.

import (
	"fmt"
	"html/template"
	"strings"
)

// Options configures the label RenderToCPanel and RenderToCRail show above
// their outline. Both take the same struct because the two viewport
// treatments differ in shell, not in vocabulary — "On this page" is the
// same sentence whether it sits in a <summary> or an <h2>.
type Options struct {
	// LabelHTML is the label's content: trusted markup, so a caller with its
	// own label typography (a tabular-figures instrument register, a
	// small-caps treatment) can wrap it without this package baking that
	// choice in as a default every consumer inherits — see D17's "never
	// enforce an editorial rule inside a shared renderer". Empty renders the
	// plain text "On this page".
	LabelHTML template.HTML
}

// label resolves the option to what actually renders.
func (o Options) label() template.HTML {
	if o.LabelHTML != "" {
		return o.LabelHTML
	}
	return "On this page"
}

// RenderToC renders a boxed table of contents: a bordered, nested <ol>
// prepended to the document it indexes. Returns "" for no headings — an
// empty ToC shell is a surface implying content that is not there.
func RenderToC(headings []Heading) string {
	if len(headings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="doc-toc" aria-label="Table of contents">`)
	writeBoxedList(&b, nest(headings))
	b.WriteString(`</nav>`)
	return b.String()
}

func writeBoxedList(b *strings.Builder, nodes []*node) {
	b.WriteString(`<ol class="doc-toc__list">`)
	for _, n := range nodes {
		fmt.Fprintf(b, `<li class="doc-toc__item doc-toc__item--h%d"><a href="#%s">%s</a>`,
			n.H.Level, esc(n.H.ID), esc(n.H.Text))
		if len(n.Kids) > 0 {
			writeBoxedList(b, n.Kids)
		}
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol>`)
}

// panelIndent maps a heading level to a padding utility for the narrow
// disclosure. A lookup table, not a computed inline style: a page under a
// strict `style-src 'self'` CSP (no inline styles) needs the indent to be a
// class, and a document nested deeper than the table shares its last entry —
// six levels down is telling us something about the document that more
// indentation would not fix.
var panelIndent = []string{"pl-0", "pl-3", "pl-6", "pl-9", "pl-9"}

// railIndent is panelIndent for the sticky rail, where the list hangs off a
// rule and every level is already inset by one step.
var railIndent = []string{"pl-3", "pl-6", "pl-9", "pl-12", "pl-12"}

// indentFor clamps a heading level into an indent table. Levels below 2 clamp
// to the table's first entry rather than panicking: a caller of this package
// is not guaranteed to have dropped the H1 the way the portal's docHeadings
// does before calling in (the H1 is normally the page's own title, rendered
// above the outline), and a promoted-to-a-second-consumer package that
// panics on an un-filtered level 1 is a worse failure than one flat indent.
func indentFor(level int, table []string) string {
	idx := level - 2
	if idx < 0 {
		idx = 0
	}
	if idx > len(table)-1 {
		idx = len(table) - 1
	}
	return table[idx]
}

// RenderToCPanel renders the outline as a native <details> disclosure — open
// by default, collapsible with no JavaScript. Returns "" for no headings.
func RenderToCPanel(headings []Heading, opts Options) string {
	if len(headings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<details class="border-y border-line" open>`)
	fmt.Fprintf(&b, `<summary class="cursor-pointer py-2 text-ink `+
		`marker:content-none [&::-webkit-details-marker]:hidden `+
		`focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">%s</summary>`,
		opts.label())
	b.WriteString(`<ol class="space-y-1 border-t border-line py-2 text-sm">`)
	for _, h := range headings {
		fmt.Fprintf(&b, `<li class=%q><a class="text-ink-muted hover:text-accent" href="#%s">%s</a></li>`,
			indentFor(h.Level, panelIndent), esc(h.ID), esc(h.Text))
	}
	b.WriteString(`</ol></details>`)
	return b.String()
}

// RenderToCRail renders the outline as a sticky right-hand column, hidden by
// the caller below its own wide-viewport breakpoint. Returns "" for no
// headings.
func RenderToCRail(headings []Heading, opts Options) string {
	if len(headings) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="sticky top-6 space-y-3">`)
	fmt.Fprintf(&b, `<h2 class="text-xs font-medium uppercase tracking-wide text-ink-faint">%s</h2>`, opts.label())
	b.WriteString(`<nav aria-label="Table of contents">`)
	b.WriteString(`<ol class="space-y-1.5 border-l border-line text-sm">`)
	for _, h := range headings {
		fmt.Fprintf(&b, `<li class=%q><a class="block text-ink-muted hover:text-accent" href="#%s">%s</a></li>`,
			indentFor(h.Level, railIndent), esc(h.ID), esc(h.Text))
	}
	b.WriteString(`</ol></nav></div>`)
	return b.String()
}

// node is the nesting form of the flat heading list, shared by every
// treatment above.
type node struct {
	H    Heading
	Kids []*node
}

// nest nests headings by level. A skipped level (an h4 directly under an h2)
// nests under the nearest shallower heading rather than being dropped: the
// document's structure renders as authored, not as we wish it were.
func nest(hs []Heading) []*node {
	var roots, stack []*node
	for _, h := range hs {
		n := &node{H: h}
		for len(stack) > 0 && stack[len(stack)-1].H.Level >= h.Level {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			roots = append(roots, n)
		} else {
			parent := stack[len(stack)-1]
			parent.Kids = append(parent.Kids, n)
		}
		stack = append(stack, n)
	}
	return roots
}
