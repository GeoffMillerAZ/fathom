package dashboard

import (
	"fmt"
	"html/template"
	"strings"
)

// Widget is one dashboard card: a titled, bordered surface holding any content
// (a chart, a table, a list, KPIs). It composes with appkit/ui/viz, stat, table.
type Widget struct {
	Title  string        // card heading (optional)
	Action template.HTML // top-right slot (a link, menu, or filter)
	Body   template.HTML // card content
	Span   int           // column span on lg screens (1–4; 0 = 1)
}

// RenderWidget returns one dashboard card.
func RenderWidget(w Widget) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<section class="%s rounded-lg border border-line bg-surface-raised p-4 shadow-sm sm:p-5">`, spanClass(w.Span))
	if w.Title != "" || w.Action != "" {
		b.WriteString(`<div class="mb-3 flex items-center justify-between gap-3">`)
		if w.Title != "" {
			fmt.Fprintf(&b, `<h2 class="text-sm font-semibold text-ink">%s</h2>`, esc(w.Title))
		}
		if w.Action != "" {
			fmt.Fprintf(&b, `<div class="text-sm text-ink-muted">%s</div>`, string(w.Action))
		}
		b.WriteString(`</div>`)
	}
	fmt.Fprintf(&b, `<div class="text-sm text-ink">%s</div>`, string(w.Body))
	b.WriteString(`</section>`)
	return b.String()
}

// WidgetGridOptions configures the grid.
type WidgetGridOptions struct {
	// Cols is the max columns on large screens (1–4, default 3). The grid is
	// responsive: 1 col on mobile, 2 on sm, Cols on lg.
	Cols int
	// AriaLabel labels the grid region.
	AriaLabel string
}

// RenderWidgetGrid lays out pre-rendered widgets (use RenderWidget) in a
// responsive grid. Widget.Span is honored via per-card column-span classes.
func RenderWidgetGrid(widgets []string, o WidgetGridOptions) string {
	var b strings.Builder
	aria := ""
	if o.AriaLabel != "" {
		aria = fmt.Sprintf(` aria-label=%q`, esc(o.AriaLabel))
	}
	fmt.Fprintf(&b, `<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 %s"%s>`, gridColsClass(o.Cols), aria)
	for _, w := range widgets {
		b.WriteString(w)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// gridColsClass returns the lg: column count (literal strings so the Tailwind
// scanner emits them).
func gridColsClass(cols int) string {
	switch cols {
	case 1:
		return "lg:grid-cols-1"
	case 2:
		return "lg:grid-cols-2"
	case 4:
		return "lg:grid-cols-4"
	default: // 3 and unset
		return "lg:grid-cols-3"
	}
}

// spanClass returns the lg: column span for a widget (literal strings).
func spanClass(span int) string {
	switch span {
	case 2:
		return "lg:col-span-2"
	case 3:
		return "lg:col-span-3"
	case 4:
		return "lg:col-span-4"
	default: // 1 and unset
		return "lg:col-span-1"
	}
}
