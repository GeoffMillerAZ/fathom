package chrome

// disclosure.go — two more structural creature comforts: a collapsible
// Disclosure (native <details>/<summary>, no JS) and a DescriptionList for
// term/detail metadata. Both deterministic, token-themed, escaped.

import (
	"fmt"
	"html/template"
	"strings"
)

// Disclosure is a collapsible section built on the native <details>/<summary>
// elements — open/close with zero JS. Use it for "show more" panels, optional
// detail, and progressive disclosure.
type Disclosure struct {
	Summary string        // the always-visible clickable label
	Body    template.HTML // revealed content (trusted)
	Open    bool          // rendered expanded
}

// RenderDisclosure returns the disclosure as an HTML string.
func RenderDisclosure(d Disclosure) string {
	var b strings.Builder
	b.WriteString(`<details class="group rounded-md border border-line bg-surface-raised"`)
	b.WriteString(boolAttr("open", d.Open))
	b.WriteString(`>`)
	fmt.Fprintf(&b,
		`<summary class="flex cursor-pointer items-center justify-between gap-3 px-4 py-3 text-sm font-medium text-ink marker:content-none [&::-webkit-details-marker]:hidden focus:outline-none focus:ring-2 focus:ring-focus">%s`,
		esc(d.Summary))
	// Chevron rotates when open (CSS only, via the group-open variant).
	b.WriteString(`<svg class="shrink-0 text-ink-muted transition-transform group-open:rotate-180" width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`)
	b.WriteString(`</summary>`)
	fmt.Fprintf(&b, `<div class="border-t border-line px-4 py-3 text-sm text-ink">%s</div>`, string(d.Body))
	b.WriteString(`</details>`)
	return b.String()
}

// boolAttr renders ` name` when on, else "". (Shared within the package.)
func boolAttr(name string, on bool) string {
	if !on {
		return ""
	}
	return " " + name
}

// DescItem is one term/detail pair in a DescriptionList.
type DescItem struct {
	Term   string
	Detail string
	// DetailHTML is trusted rich detail (badges, links, lists) used instead of
	// the escaped Detail string when set.
	DetailHTML template.HTML
}

// detailOf returns the rendered detail for an item — rich HTML when present,
// else the escaped string.
func detailOf(it DescItem) string {
	if it.DetailHTML != "" {
		return string(it.DetailHTML)
	}
	return esc(it.Detail)
}

// RenderDescriptionList returns a <dl> of term/detail pairs laid out as an
// aligned two-column grid — for compact metadata (key dates, figures, source
// facts). Empty input yields an empty <dl>.
func RenderDescriptionList(items []DescItem) string {
	var b strings.Builder
	b.WriteString(`<dl class="grid grid-cols-[max-content_1fr] gap-x-6 gap-y-2 text-sm">`)
	for _, it := range items {
		fmt.Fprintf(&b,
			`<dt class="text-ink-muted">%s</dt><dd class="text-ink">%s</dd>`,
			esc(it.Term), detailOf(it))
	}
	b.WriteString(`</dl>`)
	return b.String()
}

// DescListOptions configures RenderDescriptionListCard.
type DescListOptions struct {
	Title    string // optional card heading
	Subtitle string // optional sub-heading under the title
}

// RenderDescriptionListCard renders term/detail pairs as a bordered card with a
// header and divided rows (the TailwindPlus "Description Lists · Left-aligned in
// card" pattern, re-themed to kit tokens) — for detail screens (participant,
// source, case facts). Each row aligns term + detail in a responsive grid; rich
// details (badges/links) come through DetailHTML.
func RenderDescriptionListCard(items []DescItem, o DescListOptions) string {
	var b strings.Builder
	b.WriteString(`<div class="overflow-hidden rounded-lg border border-line bg-surface-raised shadow-sm">`)
	if o.Title != "" || o.Subtitle != "" {
		b.WriteString(`<div class="px-4 py-5 sm:px-6">`)
		if o.Title != "" {
			fmt.Fprintf(&b, `<h3 class="text-base font-semibold text-ink">%s</h3>`, esc(o.Title))
		}
		if o.Subtitle != "" {
			fmt.Fprintf(&b, `<p class="mt-1 max-w-2xl text-sm text-ink-muted">%s</p>`, esc(o.Subtitle))
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`<div class="border-t border-line"><dl class="divide-y divide-line">`)
	for _, it := range items {
		b.WriteString(`<div class="px-4 py-4 sm:grid sm:grid-cols-3 sm:gap-4 sm:px-6">`)
		fmt.Fprintf(&b, `<dt class="text-sm font-medium text-ink">%s</dt>`, esc(it.Term))
		fmt.Fprintf(&b, `<dd class="mt-1 text-sm text-ink-muted sm:col-span-2 sm:mt-0">%s</dd>`, detailOf(it))
		b.WriteString(`</div>`)
	}
	b.WriteString(`</dl></div></div>`)
	return b.String()
}
