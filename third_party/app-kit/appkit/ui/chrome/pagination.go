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
	fmt.Fprintf(&b, `<nav aria-label="%s" class="flex items-center gap-1 text-sm">`, esc(aria))

	// Prev.
	if page > 1 {
		fmt.Fprintf(&b, `<a href="%s" rel="prev" class="rounded-md border border-line px-2 py-1 text-ink-muted hover:border-line-strong">Prev</a>`, esc(href(page-1)))
	} else {
		b.WriteString(`<span aria-disabled="true" class="rounded-md border border-line px-2 py-1 text-ink-faint">Prev</span>`)
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
			fmt.Fprintf(&b, `<span aria-current="page" class="rounded-md bg-accent px-2.5 py-1 font-medium text-accent-ink">%d</span>`, n)
		} else {
			fmt.Fprintf(&b, `<a href="%s" class="rounded-md border border-line px-2.5 py-1 text-ink-muted hover:border-line-strong">%d</a>`, esc(href(n)), n)
		}
		prev = n
	}

	// Next.
	if page < p.TotalPages {
		fmt.Fprintf(&b, `<a href="%s" rel="next" class="rounded-md border border-line px-2 py-1 text-ink-muted hover:border-line-strong">Next</a>`, esc(href(page+1)))
	} else {
		b.WriteString(`<span aria-disabled="true" class="rounded-md border border-line px-2 py-1 text-ink-faint">Next</span>`)
	}

	b.WriteString(`</nav>`)
	return b.String()
}
