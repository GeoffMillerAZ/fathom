package chrome

import (
	"fmt"
	"strings"
)

// Pagination renders prev/next + a windowed set of page links — a real expected
// capability for long lists/tables, server-rendered (plain links, no JS).
type Pagination struct {
	Page       int                   // current page (1-based)
	TotalPages int                   // total pages
	HrefFor    func(page int) string // URL builder; defaults to "?page=N"
	AriaLabel  string
}

// Every link is a 44px target with the kit's 2px focus-visible outline (the
// floor chrome.Breadcrumbs meets); the current page and the disabled ends keep
// the same height, and the row wraps instead of overflowing a narrow screen.
const (
	pageBox      = "inline-flex min-h-11 min-w-11 items-center justify-center rounded-md px-3"
	pageLinkRole = "border border-line text-ink-muted hover:border-line-strong focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus"
)

// RenderPagination returns the control as HTML. Returns "" when there is at most
// one page. The window shows first/last, the current page ±1, and ellipses.
func RenderPagination(p Pagination) string {
	if p.TotalPages <= 1 {
		return ""
	}
	page := p.Page
	if page < 1 {
		page = 1
	}
	if page > p.TotalPages {
		page = p.TotalPages
	}
	href := p.HrefFor
	if href == nil {
		href = func(n int) string { return fmt.Sprintf("?page=%d", n) }
	}
	aria := p.AriaLabel
	if aria == "" {
		aria = "Pagination"
	}

	// Build the page set: 1, last, current-1..current+1, with ellipses.
	show := map[int]bool{1: true, p.TotalPages: true}
	for d := -1; d <= 1; d++ {
		n := page + d
		if n >= 1 && n <= p.TotalPages {
			show[n] = true
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<nav aria-label="%s" class="flex flex-wrap items-center gap-1 text-sm">`, esc(aria))

	// Prev.
	if page > 1 {
		fmt.Fprintf(&b, `<a href="%s" rel="prev" class="%s %s">Prev</a>`, esc(href(page-1)), pageBox, pageLinkRole)
	} else {
		fmt.Fprintf(&b, `<span aria-disabled="true" class="%s border border-line text-ink-faint">Prev</span>`, pageBox)
	}

	prev := 0
	for n := 1; n <= p.TotalPages; n++ {
		if !show[n] {
			continue
		}
		if prev != 0 && n-prev > 1 {
			b.WriteString(`<span class="px-1 text-ink-faint" aria-hidden="true">…</span>`)
		}
		if n == page {
			fmt.Fprintf(&b, `<span aria-current="page" class="%s bg-accent font-medium text-accent-ink">%d</span>`, pageBox, n)
		} else {
			fmt.Fprintf(&b, `<a href="%s" class="%s %s">%d</a>`, esc(href(n)), pageBox, pageLinkRole, n)
		}
		prev = n
	}

	// Next.
	if page < p.TotalPages {
		fmt.Fprintf(&b, `<a href="%s" rel="next" class="%s %s">Next</a>`, esc(href(page+1)), pageBox, pageLinkRole)
	} else {
		fmt.Fprintf(&b, `<span aria-disabled="true" class="%s border border-line text-ink-faint">Next</span>`, pageBox)
	}

	b.WriteString(`</nav>`)
	return b.String()
}
