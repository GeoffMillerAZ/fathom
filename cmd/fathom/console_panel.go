package main

// console_panel.go — OC-10 (docs/design/observatory-console.md §2): every
// number carries its instrument. A panel is a kit widget (dashboard
// .RenderWidget) that states the question it answers, the exact verb that
// produces it as a copyable chip, and a footer naming its source,
// denominator and freshness. Its truth state renders distinctly: zero is a
// number, absent names its reason, stale carries its age, partial names its
// lower bound, error names what failed — never a colored zero.

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/chrome"
	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/table"
	"github.com/geoffmilleraz/fathom/internal/console"
)

// consolePanel is one panel.
type consolePanel struct {
	ID          string // anchor id (a "refresh this panel" link targets it)
	Title       string
	Question    string // the question the panel answers
	Verb        string // the exact command that produces it
	Source      string // where the figure was read
	Denominator string // what it was counted over
	Freshness   string // when it was true ("as of …", "live")
	// State is the panel's headline truth state; Reason explains any state
	// other than present or zero.
	State  console.State
	Reason string
	// SourceKey names the live-freshness sources the panel follows
	// (data-console-source), e.g. "plans:fathom"; several keys are
	// space-separated ("index layers"). An event naming one marks the panel
	// updated without moving it (OC-14, console_live.go); the panel's
	// freshness then leads its reserved status line.
	SourceKey string
	Body      string // trusted panel content
	Span      int
}

// renderPanel renders p as a kit widget with its question, verb chip, truth
// state and instrument footer.
func renderPanel(p consolePanel) string {
	var body strings.Builder
	if p.Question != "" {
		body.WriteString(`<p class="mb-3 text-ink-muted" data-panel-question>` + html.EscapeString(p.Question) + `</p>`)
	}
	if line := truthStateLine(p.State, p.Reason); line != "" {
		body.WriteString(line)
	}
	body.WriteString(p.Body)
	// A live panel is a kit region (console_live.go): its freshness leads
	// the region's reserved status line, where "Updated — refresh this
	// panel" appears without moving anything.
	freshness := p.Freshness
	if freshness == "" {
		freshness = "as rendered"
	}
	regionAttrs, status, isLive := "", "", false
	if p.SourceKey != "" && p.ID != "" {
		regionAttrs, status, isLive = consoleLiveRegion(p.ID, p.Title, "freshness: "+freshness, p.SourceKey)
	}
	var foot []string
	if p.Source != "" {
		foot = append(foot, "source: "+p.Source)
	}
	if p.Denominator != "" {
		foot = append(foot, "denominator: "+p.Denominator)
	}
	if p.Freshness != "" && !isLive {
		foot = append(foot, "freshness: "+p.Freshness)
	}
	if len(foot) > 0 {
		body.WriteString(`<p class="mt-3 border-t border-line pt-2 text-xs text-ink-muted [overflow-wrap:anywhere]" data-panel-footer>` + html.EscapeString(strings.Join(foot, " · ")) + `</p>`)
	}
	if isLive {
		body.WriteString(status)
	}
	var action template.HTML
	if p.Verb != "" {
		action = template.HTML(renderVerbChip(p.Verb))
	}
	widget := dashboard.RenderWidget(dashboard.Widget{Title: p.Title, Action: action, Body: template.HTML(body.String()), Span: p.Span})
	state := p.State
	if state == "" {
		state = console.StatePresent
	}
	attrs := fmt.Sprintf(` data-console-panel data-truth-state="%s"`, html.EscapeString(string(state)))
	class := "min-w-0"
	switch {
	case isLive:
		attrs += " " + regionAttrs
		class = consoleRegionClass
	case p.ID != "":
		attrs += ` id="` + html.EscapeString(p.ID) + `"`
		if p.SourceKey != "" {
			attrs += ` data-console-source="` + html.EscapeString(p.SourceKey) + `"`
		}
	}
	if p.Verb != "" {
		attrs += ` data-console-verb="` + html.EscapeString(p.Verb) + `"`
	}
	return `<div class="` + class + `"` + attrs + `>` + widget + `</div>`
}

// truthStateLine renders a non-plain truth state in words, each state drawn
// differently; present and zero render nothing (the number speaks). A
// reason can carry long paths with no break point, so the line wraps
// anywhere rather than running off its card (FATHOM-UI-01: long
// diagnostics stay left-aligned and wrapping).
func truthStateLine(s console.State, reason string) string {
	r := html.EscapeString(reason)
	const wrap = ` min-w-0 [overflow-wrap:anywhere]`
	switch s {
	case console.StateAbsent:
		return `<p class="mb-2 italic text-ink-muted` + wrap + `" data-truth-line="absent">not measured — ` + r + `</p>`
	case console.StateError:
		return `<p class="mb-2 font-medium text-danger` + wrap + `" data-truth-line="error">could not be read — ` + r + `</p>`
	case console.StateStale:
		return `<p class="mb-2 text-caution` + wrap + `" data-truth-line="stale">stale — ` + r + `</p>`
	case console.StatePartial:
		return `<p class="mb-2 text-ink-muted` + wrap + `" data-truth-line="partial">lower bound — ` + r + `</p>`
	}
	return ""
}

// renderVerbChip renders the exact command as selectable monospace text with
// a copy button the console script reveals (hidden without JavaScript, when
// the text itself is still selectable).
func renderVerbChip(cmd string) string {
	c := html.EscapeString(cmd)
	return `<span class="inline-flex max-w-full items-center gap-1 rounded-md border border-line bg-surface-sunken py-0.5 pl-2 pr-0.5 font-mono text-xs text-ink" data-verb-chip>` +
		`<code class="min-w-0 [overflow-wrap:anywhere]">` + c + `</code>` +
		`<button type="button" hidden data-console-copy="` + c + `" aria-label="Copy the command ` + c + `" class="inline-flex min-h-11 min-w-11 items-center justify-center rounded px-2 font-sans text-xs font-medium text-accent hover:bg-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus">Copy</button>` +
		`</span>`
}

// consolePager is the kit's pagination with 44 px targets (design §2
// Accessibility): its links are about 30 px tall (FATHOM-C-KIT-05,
// docs/app-kit.md), so every console pager wraps them. "" when there is one
// page.
func consolePager(p chrome.Pagination) string {
	nav := chrome.RenderPagination(p)
	if nav == "" {
		return ""
	}
	return `<div class="mt-2 [&_a]:inline-flex [&_a]:min-h-11 [&_a]:items-center [&_span]:inline-flex [&_span]:min-h-11 [&_span]:items-center" data-console-pager>` + nav + `</div>`
}

// valueStat maps a projected value onto a kit stat tile without losing its
// truth state: absent is stat.Absent with its reason, error names what
// failed, partial is a "≥" lower bound, stale carries its reason, zero is
// the number 0.
func valueStat(label string, v console.Value[int]) stat.Stat {
	switch v.State {
	case console.StateAbsent:
		s := stat.Absent(label, v.Reason)
		s.ValueRole = "ink-muted"
		return s
	case console.StateError:
		return stat.Stat{Label: label, Value: "unreadable", ValueRole: "danger", Sub: v.Reason}
	case console.StatePartial:
		return stat.Stat{Label: label, Value: "≥ " + fmtThousands(v.V), Sub: v.Reason}
	case console.StateStale:
		return stat.Stat{Label: label, Value: fmtThousands(v.V), ValueRole: "caution", Sub: "stale: " + v.Reason}
	}
	return stat.Stat{Label: label, Value: fmtThousands(v.V), Sub: v.Denominator}
}

// valueText renders a projected value inline: the number, "≥ n" for a lower
// bound, or the named absence — never a bare 0 for something unmeasured.
func valueText(v console.Value[int]) string {
	switch v.State {
	case console.StateAbsent:
		return `<span class="italic text-ink-muted">not measured — ` + html.EscapeString(v.Reason) + `</span>`
	case console.StateError:
		return `<span class="text-danger">unreadable — ` + html.EscapeString(v.Reason) + `</span>`
	case console.StatePartial:
		return "≥ " + fmtThousands(v.V)
	}
	return fmtThousands(v.V)
}

// valuePlain renders a projected value as plain text (markdown, logs): the
// number, "≥ n (reason)" for a lower bound, or "not measured (reason)".
func valuePlain(v console.Value[int]) string {
	switch v.State {
	case console.StateAbsent:
		return "not measured (" + v.Reason + ")"
	case console.StateError:
		return "unreadable (" + v.Reason + ")"
	case console.StatePartial:
		return "≥ " + fmtThousands(v.V) + " (" + v.Reason + ")"
	case console.StateStale:
		return fmtThousands(v.V) + " (stale: " + v.Reason + ")"
	}
	return fmtThousands(v.V)
}

// renderKitTable renders the kit's table.Table (a templ component) to a
// string. Since mirror d9dd789 the kit itself writes scope="col" on every
// header cell and makes its scroll wrapper the containing block of its
// sr-only header labels (FATHOM-F0-KIT-01/02, returned from wave F0), so
// fathom no longer patches either.
func renderKitTable(p table.TableParams) string {
	var buf bytes.Buffer
	_ = table.Table(p).Render(context.Background(), &buf)
	return buf.String()
}
