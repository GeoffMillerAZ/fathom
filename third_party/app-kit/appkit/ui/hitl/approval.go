// Package hitl renders human-in-the-loop UI for agentic apps: approval cards
// for proposed agent actions and run traces for "what's happening under the
// hood." Deterministic server-rendered HTML themed through the design tokens
// (D7); pure Go, no Node; escaped. Action affordances are caller-supplied HTML
// (forms/links) so they stay within the loopback invariant (D5) — no
// shell-specific JS bindings.
package hitl

import (
	"fmt"
	"html"
	"html/template"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
)

// Risk levels for a proposed action, mapped to kit status roles.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

func riskRole(r Risk) string {
	switch r {
	case RiskHigh:
		return "danger"
	case RiskMedium:
		return "caution"
	case RiskLow:
		return "positive"
	default:
		return "info"
	}
}

// Metric is one deterministic before→after effect of a proposed action, shown
// in the card's impact strip (e.g. {"Weekly tuition", "$8,304", "$8,479"}).
type Metric struct {
	Label string
	From  string
	To    string
}

// ApprovalCard is a proposed agent action awaiting a human decision.
type ApprovalCard struct {
	// Intent is a short verb-phrase ("Send email", "Delete 12 records") shown
	// as a badge; destructive intents should set a high Risk.
	Intent string
	// Title is the headline (what specifically is proposed).
	Title string
	// Summary is the human-readable explanation (plain text; escaped).
	Summary string
	// SummaryHTML is trusted rich explanation used instead of Summary when set.
	SummaryHTML template.HTML
	// Risk drives the risk badge (low/medium/high).
	Risk Risk
	// Confidence (0..1) renders a small pip rating when HasConfidence is set.
	Confidence    float64
	HasConfidence bool
	// Effect is a one-line plain-language consequence ("+$175/wk once enrolled")
	// shown in the impact strip. Optional.
	Effect string
	// Changes are deterministic before→after metrics shown in the impact strip —
	// the concrete effect of approving, so a human decides on facts, not just the
	// intent. Optional.
	Changes []Metric
	// Actions is caller-supplied HTML (approve/reject/edit forms or links). The
	// kit does not impose a transport — the consuming app wires the loopback POST.
	Actions template.HTML
}

// RenderApprovalCard returns the card as an HTML string.
func RenderApprovalCard(c ApprovalCard) string {
	var b strings.Builder
	b.WriteString(`<div role="group" aria-label="Proposed action" class="rounded-lg border border-line bg-surface-raised p-4">`)

	// Header: intent + risk badges (left), confidence rating (right).
	b.WriteString(`<div class="flex items-center justify-between gap-3">`)
	b.WriteString(`<div class="flex items-center gap-2">`)
	if c.Intent != "" {
		b.WriteString(callout.Badge(c.Intent, "accent"))
	}
	if c.Risk != "" {
		b.WriteString(callout.Badge(string(c.Risk)+" risk", riskRole(c.Risk)))
	}
	b.WriteString(`</div>`)
	if c.HasConfidence {
		b.WriteString(`<span class="flex items-center gap-1 text-xs text-ink-muted">confidence ` +
			viz.Rating(c.Confidence*5, viz.RatingOptions{Max: 5, PipR: 5, Graded: true, AriaLabel: "confidence"}) + `</span>`)
	}
	b.WriteString(`</div>`)

	if c.Title != "" {
		fmt.Fprintf(&b, `<p class="mt-2 font-medium text-ink">%s</p>`, esc(c.Title))
	}
	if body := bodyOf(c); body != "" {
		fmt.Fprintf(&b, `<div class="mt-1 text-sm text-ink-muted">%s</div>`, body)
	}
	// Impact strip: the deterministic effect of approving (before → after).
	if c.Effect != "" || len(c.Changes) > 0 {
		var parts []string
		if c.Effect != "" {
			parts = append(parts, esc(c.Effect))
		}
		for _, ch := range c.Changes {
			parts = append(parts, fmt.Sprintf(`%s %s → <span class="font-semibold text-ink">%s</span>`, esc(ch.Label), esc(ch.From), esc(ch.To)))
		}
		fmt.Fprintf(&b, `<div class="mt-2 border-t border-line pt-2 text-xs text-ink-muted">%s</div>`, strings.Join(parts, ` · `))
	}
	if c.Actions != "" {
		fmt.Fprintf(&b, `<div class="mt-4 flex flex-wrap gap-2">%s</div>`, string(c.Actions))
	}
	b.WriteString(`</div>`)
	return b.String()
}

func bodyOf(c ApprovalCard) string {
	if c.SummaryHTML != "" {
		return string(c.SummaryHTML)
	}
	return esc(c.Summary)
}

func esc(s string) string { return html.EscapeString(s) }
