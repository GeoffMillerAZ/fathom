// Package callout renders the kit's messaging components — callouts (info,
// success, caution, danger, note) and inline status badges — as deterministic,
// server-rendered HTML themed through the design tokens (D7). They carry the
// disclaimers, warnings, and emphasis a trustworthy app needs (e.g. a legal
// tool's "not legal advice" framing). Pure Go, no Node; output is escaped and
// uses only kit role utilities, so it re-themes light/dark with the token CSS.
package callout

import (
	"fmt"
	"html"
	"html/template"
	"strings"
)

// Kind selects the semantic intent of a callout, which drives its color role
// and ARIA role.
type Kind string

const (
	Info    Kind = "info"    // neutral information (info role)
	Success Kind = "success" // a positive outcome/confirmation (positive role)
	Caution Kind = "caution" // a caveat or risk to weigh (caution role)
	Danger  Kind = "danger"  // an error or serious warning (danger role)
	Note    Kind = "note"    // general emphasis / aside (accent role)
)

type kindCfg struct {
	role string // base token role; the soft variant tints the background
	aria string // ARIA live role
}

var kinds = map[Kind]kindCfg{
	Info:    {role: "info", aria: "note"},
	Success: {role: "positive", aria: "status"},
	Caution: {role: "caution", aria: "alert"},
	Danger:  {role: "danger", aria: "alert"},
	Note:    {role: "accent", aria: "note"},
}

// Callout is a block-level message.
type Callout struct {
	Kind  Kind
	Title string // optional bold heading
	Body  string // the message (plain text; escaped)
	// BodyHTML is trusted rich body (links, emphasis) used instead of Body when
	// set. The caller is responsible for its safety — pass only markup you
	// construct, never raw user input. For plain text, use Body (auto-escaped).
	BodyHTML template.HTML
	HideIcon bool // omit the leading icon

	// Details is optional elaboration, disclosed behind the summary. When set,
	// the callout renders as a native <details>/<summary> — no JavaScript, so
	// it works under a `script-src 'none'` CSP and with scripting disabled.
	//
	// The split is the point: Title and Body must carry the WHOLE meaning on
	// their own, because a reader who never expands must still be correctly
	// informed. Details is elaboration — the runbook step, the offending
	// value, who to ask — never the thing the reader needed.
	Details template.HTML

	// Open forces the disclosure open on first render. Use it for anything a
	// reader must not be able to miss: a warning nobody expands is a warning
	// nobody reads. Ignored when Details is empty.
	Open bool

	// DetailsLabel overrides the affordance text ("Details" by default). Name
	// what is inside — "Show the failing rule", "What changed" — because a
	// generic label makes every disclosure look equally uninteresting.
	DetailsLabel string
}

// bodyContent returns the rendered body: trusted BodyHTML if set, else the
// escaped plain Body.
func (c Callout) bodyContent() string {
	if c.BodyHTML != "" {
		return string(c.BodyHTML)
	}
	return esc(c.Body)
}

// hasBody reports whether the callout has any body content.
func (c Callout) hasBody() bool {
	return c.BodyHTML != "" || c.Body != ""
}

// Render returns the callout as an HTML string. An unknown Kind falls back to
// Note so output is always well-formed.
//
// With Details set the outer element is a <details> and the summary row is the
// clickable affordance; without it, the markup is unchanged from before, so a
// caller that never sets Details sees no difference.
func Render(c Callout) string {
	cfg, ok := kinds[c.Kind]
	if !ok {
		cfg = kinds[Note]
	}
	role := cfg.role
	expandable := strings.TrimSpace(string(c.Details)) != ""

	var b strings.Builder
	switch {
	case expandable:
		openAttr := ""
		if c.Open {
			openAttr = " open"
		}
		// `group` scopes the chevron's rotation to THIS disclosure; role is
		// omitted on <details> deliberately — an ARIA role would override the
		// native disclosure semantics screen readers already handle well.
		fmt.Fprintf(&b,
			`<details%s class="group rounded-md border border-%s bg-%s-soft p-4">`,
			openAttr, role, role)
		b.WriteString(`<summary class="flex cursor-pointer list-none items-start gap-3 ` +
			`focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus ` +
			`[&::-webkit-details-marker]:hidden">`)
	default:
		fmt.Fprintf(&b,
			`<div role="%s" class="rounded-md border border-%s bg-%s-soft p-4">`,
			cfg.aria, role, role)
		b.WriteString(`<div class="flex gap-3">`)
	}
	if !c.HideIcon {
		fmt.Fprintf(&b, `<span class="text-%s shrink-0">%s</span>`, role, icon(c.Kind))
	}
	// Body container: a <p> for plain/inline text, a <div> when BodyHTML carries
	// block content (lists, multiple paragraphs) so it stays valid HTML.
	tag := "p"
	if c.BodyHTML != "" {
		tag = "div"
	}
	b.WriteString(`<div class="min-w-0">`)
	if c.Title != "" {
		fmt.Fprintf(&b, `<p class="font-medium text-ink">%s</p>`, esc(c.Title))
		if c.hasBody() {
			fmt.Fprintf(&b, `<%s class="mt-1 text-sm text-ink-muted">%s</%s>`, tag, c.bodyContent(), tag)
		}
	} else if c.hasBody() {
		fmt.Fprintf(&b, `<%s class="text-sm text-ink">%s</%s>`, tag, c.bodyContent(), tag)
	}
	b.WriteString(`</div>`)

	if expandable {
		// The chevron sits last in the summary row and rotates on open. It is
		// decorative — the label beside it carries the meaning — so it is
		// aria-hidden and the label is real text.
		label := c.DetailsLabel
		if strings.TrimSpace(label) == "" {
			label = "Details"
		}
		fmt.Fprintf(&b, `<span class="ml-auto flex shrink-0 items-center gap-1 text-xs text-ink-faint">`+
			`<span>%s</span>`+
			`<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" `+
			`stroke-linecap="round" stroke-linejoin="round" width="16" height="16" aria-hidden="true" `+
			`class="transition-transform group-open:rotate-90 motion-reduce:transition-none">`+
			`<path d="M7.5 5 12.5 10 7.5 15"/></svg></span>`, esc(label))
		b.WriteString(`</summary>`)
		fmt.Fprintf(&b, `<div class="mt-3 border-t border-%s pt-3 text-sm text-ink-muted">%s</div>`,
			role, c.Details)
		b.WriteString(`</details>`)
		return b.String()
	}

	b.WriteString(`</div></div>`)
	return b.String()
}

// Disclaimer renders the standard "informational, not advice" caveat as a
// caution callout — the framing a regulated/legal app should show by default.
// Pass an empty body to use the generic wording.
func Disclaimer(body string) string {
	if strings.TrimSpace(body) == "" {
		body = "This is generated information to support your judgment, not professional advice. Verify against primary sources before relying on it."
	}
	return Render(Callout{Kind: Caution, Title: "For information only", Body: body})
}

// icon returns a small inline SVG glyph for the kind. Glyphs are simple, valid
// SVG drawn with currentColor so the enclosing text-<role> class colors them.
func icon(k Kind) string {
	const open = `<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" width="20" height="20" aria-hidden="true">`
	const dot = `<circle cx="10" cy="6" r="0.9" fill="currentColor" stroke="none"/>`
	switch k {
	case Success:
		return open + `<circle cx="10" cy="10" r="8"/><path d="M6.5 10.5 9 13 14 7"/></svg>`
	case Caution:
		return open + `<path d="M10 3 18 16 H2 Z"/><line x1="10" y1="8" x2="10" y2="12"/><circle cx="10" cy="14.5" r="0.9" fill="currentColor" stroke="none"/></svg>`
	case Danger:
		return open + `<circle cx="10" cy="10" r="8"/><line x1="7" y1="7" x2="13" y2="13"/><line x1="13" y1="7" x2="7" y2="13"/></svg>`
	case Note:
		return open + `<circle cx="10" cy="10" r="8"/><circle cx="10" cy="10" r="2.5" fill="currentColor" stroke="none"/></svg>`
	default: // Info
		return open + `<circle cx="10" cy="10" r="8"/><line x1="10" y1="9.5" x2="10" y2="14"/>` + dot + `</svg>`
	}
}

func esc(s string) string { return html.EscapeString(s) }
