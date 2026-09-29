package main

// console_evidence.go — the small render pieces the Evidence and System
// pages (wave F: /impact, /sessions, /operate, /decide) share: a table cell
// that draws a value's truth state, filter chips that are plain links, a
// chart framed with its table alternative, a query-preserving page link and
// the kit's pagination at a 44 px target. Every piece composes kit parts
// (chrome, table, viz, form) with role utilities only; none of them holds a
// number of its own.

import (
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// evCell draws one value in a table cell by its truth state: a measured
// number (zero included), a "≥" lower bound, a stale number, or — for an
// absence — a short label with the full reason for assistive tech and on
// hover. No absence ever renders as a digit.
func evCell(v console.Value[int], absentLabel string) string {
	switch v.State {
	case console.StateAbsent:
		if absentLabel == "" {
			absentLabel = "not measured"
		}
		return `<span class="italic text-ink-muted" data-truth-state="absent" title="` + html.EscapeString(v.Reason) + `">` + html.EscapeString(absentLabel) + `<span class="sr-only"> — ` + html.EscapeString(v.Reason) + `</span></span>`
	case console.StateError:
		return `<span class="font-medium text-danger" data-truth-state="error" title="` + html.EscapeString(v.Reason) + `">unreadable<span class="sr-only"> — ` + html.EscapeString(v.Reason) + `</span></span>`
	case console.StatePartial:
		return `<span class="tabular-nums" data-truth-state="partial">≥ ` + fmtThousands(v.V) + `</span>`
	case console.StateStale:
		return `<span class="tabular-nums text-caution" data-truth-state="stale" title="` + html.EscapeString(v.Reason) + `">` + fmtThousands(v.V) + `</span>`
	}
	return `<span class="tabular-nums" data-truth-state="` + html.EscapeString(string(v.State)) + `">` + fmtThousands(v.V) + `</span>`
}

// templComponent is a kit table cell's rich-content type.
type templComponent = templ.Component

// evRaw wraps trusted HTML as a table cell component.
func evRaw(s string) templ.Component { return templ.Raw(s) }

// evChip is one filter chip: a link that sets a query value.
type evChip struct {
	Label    string
	Href     string
	Selected bool
	Count    string // optional count shown after the label
}

// evChips renders filter chips as a labelled list of links; the selected
// ones carry aria-current, and color is never the only channel (a selected
// chip is also marked with a check).
func evChips(label string, chips []evChip) string {
	var b strings.Builder
	b.WriteString(`<div class="mb-3 flex flex-wrap items-center gap-2" role="group" aria-label="` + html.EscapeString(label) + `">`)
	b.WriteString(`<span class="text-xs font-medium text-ink-muted">` + html.EscapeString(label) + `</span>`)
	for _, c := range chips {
		cls := "inline-flex min-h-11 items-center gap-1 rounded-full border px-3 text-xs focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus "
		cur := ""
		mark := ""
		if c.Selected {
			cls += "border-accent bg-accent-soft font-medium text-ink"
			cur = ` aria-current="true"`
			mark = `<span aria-hidden="true">✓</span>`
		} else {
			cls += "border-line bg-surface text-ink-muted hover:border-line-strong hover:text-ink"
		}
		count := ""
		if c.Count != "" {
			count = ` <span class="tabular-nums text-ink-muted">` + html.EscapeString(c.Count) + `</span>`
		}
		b.WriteString(`<a href="` + html.EscapeString(c.Href) + `" class="` + cls + `"` + cur + `>` + mark + html.EscapeString(c.Label) + count + `</a>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// evChart frames a chart: the drawing (inside a keyboard-reachable region
// that scrolls on its own below md when it is wider than a phone) and, below
// it, its table alternative behind the kit's disclosure. minWidthClass is
// the chart's floor below md as a LITERAL Tailwind class the caller writes
// out (e.g. "min-w-[560px]"): Tailwind emits only classes it can see in the
// source, so a class built at run time never reaches app.css.
func evChart(label, svg, minWidthClass, tableSummary, table string) string {
	var b strings.Builder
	if minWidthClass != "" {
		b.WriteString(`<div class="overflow-x-auto" role="region" tabindex="0" aria-label="` + html.EscapeString(label) + ` — scrolls sideways on a narrow screen"><div class="` + html.EscapeString(minWidthClass) + ` md:min-w-0">`)
		b.WriteString(svg)
		b.WriteString(`</div></div>`)
	} else {
		b.WriteString(svg)
	}
	if table != "" {
		b.WriteString(`<div class="mt-3">` + chrome.RenderDisclosure(chrome.Disclosure{Summary: tableSummary, Body: template.HTML(table)}) + `</div>`)
	}
	return b.String()
}

// evHref rebuilds the request's path with one query value set (or removed
// when value is ""), keeping the rest; page resets unless it is the key.
func evHref(path string, q url.Values, key, value string) string {
	next := url.Values{}
	for k, vs := range q {
		if k == "format" {
			continue
		}
		next[k] = append([]string{}, vs...)
	}
	if key != "page" {
		next.Del("page")
	}
	if value == "" {
		next.Del(key)
	} else {
		next.Set(key, value)
	}
	if enc := next.Encode(); enc != "" {
		return path + "?" + enc
	}
	return path
}

// evPagination renders the kit's pagination over query-preserving links
// (the kit's own 44 px targets since FATHOM-F0-KIT-05), or nothing for one
// page.
func evPagination(path string, q url.Values, page, pages int, label string) string {
	nav := chrome.RenderPagination(chrome.Pagination{Page: page, TotalPages: pages, AriaLabel: label,
		HrefFor: func(n int) string { return evHref(path, q, "page", strconv.Itoa(n)) }})
	if nav == "" {
		return ""
	}
	return `<div class="mt-3 flex flex-wrap items-center gap-3">` + nav +
		`<span class="text-xs text-ink-muted">page ` + strconv.Itoa(page) + ` of ` + strconv.Itoa(pages) + `</span></div>`
}

// evPage reads a positive page number from the query (default 1).
func evPage(q url.Values) int {
	n, err := strconv.Atoi(q.Get("page"))
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// evPlainCell is a table cell of escaped text.
func evPlainCell(s string) templ.Component { return templ.Raw(html.EscapeString(s)) }

// evShortTime renders an RFC 3339 instant as "2006-01-02 15:04" (UTC).
func evShortTime(ts string) string {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.UTC().Format("2006-01-02 15:04")
		}
	}
	return ts
}

// evPercent renders a fraction or percentage with one decimal.
func evPercent(p float64) string { return strconv.FormatFloat(p, 'f', 1, 64) + "%" }

// evMS renders a latency in ms or s.
func evMS(ms float64) string {
	if ms >= 1000 {
		return strconv.FormatFloat(ms/1000, 'f', 1, 64) + " s"
	}
	return strconv.FormatFloat(ms, 'f', 0, 64) + " ms"
}

// evCommand renders a verb for a panel's chip with the request's flags,
// each value quoted where a shell would split or expand it, so the copied
// chip runs exactly what the page ran (design §2: "the exact command,
// copyable").
func evCommand(base string, flags ...string) string {
	parts := []string{base}
	for i := 0; i+1 < len(flags); i += 2 {
		if flags[i+1] != "" {
			parts = append(parts, flags[i], shellQuote(flags[i+1]))
		}
	}
	return strings.Join(append(parts, "--json"), " ")
}

// shellSafe matches a word a POSIX shell passes through unchanged.
var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote is s as one shell word: unchanged when nothing in it is
// special, else single-quoted, an embedded quote written as close-quote,
// backslash-quote, reopen-quote.
func shellQuote(s string) string {
	if s != "" && shellSafe.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// evStackItem is one table row restated for a phone: a heading line and
// the row's other cells as terms and details.
type evStackItem struct {
	Heading string // trusted HTML
	Items   []chrome.DescItem
}

// evTableOrStack is a table from md up and the same rows as a stacked list
// below md (the sessions and catches pattern): at 390 px a table that
// scrolls sideways hides the columns that carry its answer, so a phone
// reads each row whole instead. id names both (data-stack="<id>").
func evTableOrStack(id, aria, tableHTML string, items []evStackItem) string {
	var b strings.Builder
	b.WriteString(`<div class="hidden md:block" data-table-wide="` + html.EscapeString(id) + `">` + tableHTML + `</div>`)
	b.WriteString(`<ul class="flex flex-col gap-3 md:hidden" aria-label="` + html.EscapeString(aria) + `" data-stack="` + html.EscapeString(id) + `">`)
	for _, it := range items {
		b.WriteString(`<li class="min-w-0 rounded-lg border border-line bg-surface-raised px-3 pb-3 pt-2 [overflow-wrap:anywhere]"><p class="font-medium text-ink">` + it.Heading + `</p>` + chrome.RenderDescriptionList(it.Items) + `</li>`)
	}
	b.WriteString(`</ul>`)
	return b.String()
}
