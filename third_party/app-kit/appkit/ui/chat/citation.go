package chat

import (
	"fmt"
	"strings"
)

// Citation is a source the assistant referenced. It renders as a chip carrying
// data-cite-id / data-cite-kind so the consuming app can wire a click to its own
// provenance view (a drawer, a modal — the kit owns no transport, D5). Unresolved
// citations (the app couldn't match the id to real evidence) render with a warning
// style and are never silently dropped — the anti-fabrication signal.
type Citation struct {
	ID         string // span/authority id, emitted as data-cite-id
	Kind       string // e.g. "span" | "authority", emitted as data-cite-kind
	Label      string // visible chip text
	Unresolved bool   // render the warning style + tooltip
}

// SafetyFlag is one injection/safety finding surfaced on an assistant message.
type SafetyFlag struct {
	Severity string // low | medium | high | critical
	RuleID   string
	Section  string // where it fired (optional)
	Excerpt  string // the triggering text (optional)
}

// disclosure wraps body in a token-themed native <details> (no JS).
func disclosure(summary, body string) string {
	return `<details class="chat-disclosure mt-2 text-xs">` +
		`<summary class="cursor-pointer select-none text-ink-muted hover:text-ink">` + esc(summary) + `</summary>` +
		`<div class="mt-2">` + body + `</div></details>`
}

// renderCitations renders the "Sources (N)" disclosure with click-to-trace chips.
func renderCitations(cs []Citation) string {
	var chips strings.Builder
	chips.WriteString(`<div class="flex flex-wrap gap-1.5" aria-label="Citations">`)
	for _, c := range cs {
		cls := "inline-flex items-center gap-1 rounded-md border border-line bg-surface px-2 py-0.5 text-xs text-ink hover:bg-surface-sunken"
		title := ""
		if c.Unresolved {
			cls = "inline-flex items-center gap-1 rounded-md border border-caution bg-caution-soft px-2 py-0.5 text-xs text-caution"
			title = ` title="Unverified — this citation could not be matched to case evidence"`
		}
		fmt.Fprintf(&chips, `<button type="button" class=%q data-cite-id=%q data-cite-kind=%q aria-label=%q%s>%s%s</button>`,
			cls, esc(c.ID), esc(c.Kind), esc("View source for "+c.Label), title,
			unresolvedMark(c.Unresolved), esc(c.Label))
	}
	chips.WriteString(`</div>`)
	return disclosure(fmt.Sprintf("Sources (%d)", len(cs)), chips.String())
}

func unresolvedMark(u bool) string {
	if !u {
		return ""
	}
	return `<span aria-hidden="true">⚠</span>`
}

// renderSafetyFlags renders the "Safety flags (N)" disclosure.
func renderSafetyFlags(fs []SafetyFlag) string {
	var list strings.Builder
	list.WriteString(`<ul class="flex flex-col gap-2" aria-label="Safety flags">`)
	for _, f := range fs {
		list.WriteString(`<li class="flex flex-wrap items-center gap-2">`)
		role := severityRole(f.Severity)
		fmt.Fprintf(&list, `<span class="rounded-full bg-%s-soft px-1.5 py-0.5 text-[0.625rem] font-medium text-%s">%s</span>`,
			role, role, esc(f.Severity))
		if f.RuleID != "" {
			fmt.Fprintf(&list, `<span class="font-mono text-ink">%s</span>`, esc(f.RuleID))
		}
		if f.Section != "" {
			fmt.Fprintf(&list, `<span class="text-ink-faint">%s</span>`, esc(f.Section))
		}
		if f.Excerpt != "" {
			fmt.Fprintf(&list, `<code class="rounded bg-surface-sunken px-1 py-0.5 text-ink-muted">%s</code>`, esc(f.Excerpt))
		}
		list.WriteString(`</li>`)
	}
	list.WriteString(`</ul>`)
	return disclosure(fmt.Sprintf("Safety flags (%d)", len(fs)), list.String())
}

// severityRole maps a severity label to a kit token role.
func severityRole(sev string) string {
	switch strings.ToLower(sev) {
	case "critical", "high":
		return "danger"
	case "medium":
		return "caution"
	default: // low / unknown
		return "info"
	}
}
