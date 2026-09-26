package main

// console_ladder.go — the project overview's realization ladder (OC-25,
// RG-5; design §3 "Project workspace, Overview"): wired → fresh → queried →
// preventing → gating, each rung with its evidence, and the first unmet
// rung with its remedy as the exact command. The panel renders `fathom
// fleet realization --service S --json` (the ladder the grade folds,
// console.RealizationLadder), so the rungs and the letter cannot disagree.

import (
	"fmt"
	"html"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// realizationVerb is the ladder's verb for one service.
func realizationVerb(service string) string {
	return "fathom fleet realization --service " + service + " --json"
}

// rungSurface and rungInk color a rung by its state (complete literals for
// the CSS build); the state is always also words.
var rungSurface = map[string]string{
	console.RungMet: "border-positive", console.RungUnmet: "border-danger",
	console.RungUnknown: "border-line", console.RungNotApplicable: "border-line",
}

var rungInk = map[string]string{
	console.RungMet: "text-positive", console.RungUnmet: "text-danger",
	console.RungUnknown: "text-ink-muted", console.RungNotApplicable: "text-ink-muted",
}

var rungWords = map[string]string{
	console.RungMet: "met", console.RungUnmet: "not met",
	console.RungUnknown: "unknown — not measured", console.RungNotApplicable: "does not apply",
}

// renderLadderPanel draws the ladder as an ordered list of rungs (the list
// is its own table alternative: each rung's state is in words beside its
// evidence) with the remedy under the first unmet rung.
func renderLadderPanel(l console.Ladder) string {
	var b strings.Builder
	b.WriteString(`<ol class="grid gap-2 sm:grid-cols-5" aria-label="realization ladder, bottom rung first" data-ladder>`)
	for _, r := range l.Rungs {
		b.WriteString(`<li class="min-w-0 rounded-md border-l-4 bg-surface-sunken px-3 py-2 ` + rungSurface[r.State] + `" data-rung="` + html.EscapeString(r.Name) + `" data-rung-state="` + html.EscapeString(r.State) + `">`)
		b.WriteString(`<p class="text-sm font-semibold text-ink">` + fmt.Sprintf("%d. ", r.Order) + html.EscapeString(r.Name) + `</p>`)
		b.WriteString(`<p class="text-xs font-medium ` + rungInk[r.State] + `">` + html.EscapeString(rungWords[r.State]) + `</p>`)
		b.WriteString(`<p class="mt-1 text-xs text-ink-muted [overflow-wrap:anywhere]">` + html.EscapeString(r.Evidence) + `</p>`)
		b.WriteString(`</li>`)
	}
	b.WriteString(`</ol>`)
	switch {
	case l.FirstUnmet != "":
		lead := "Remedy for <strong>" + html.EscapeString(l.FirstUnmet) + "</strong>, the first rung not met"
		if l.Idle {
			lead += " (no sessions in the window, so the repository grades idle, not failing)"
		}
		b.WriteString(`<p class="mt-3 text-sm text-ink" data-ladder-remedy>` + lead + `:</p>` + remedy(l.Remedy))
	case l.Idle:
		b.WriteString(`<p class="mt-3 text-sm text-ink-muted">No sessions in the window: the repository grades idle, not failing, and every rung that could be judged is met.</p>`)
	default:
		b.WriteString(`<p class="mt-3 text-sm text-positive">Every rung that applies is met.</p>`)
	}
	return b.String()
}

// ladderPanel is the ladder as a console panel.
func ladderPanel(l console.Ladder) string {
	return renderPanel(consolePanel{
		ID: "realization", Title: "Realization", SourceKey: "index:" + l.Service + " sessions:" + l.Service,
		Question:    "Is fathom actually doing its job here? The first unmet rung carries the remedy.",
		Verb:        realizationVerb(l.Service),
		Source:      "the registry, the call log the daemon writes (14-day window), the session ledger and the repository's plans",
		Denominator: fmt.Sprintf("%d of %d rungs that apply are met — grade %s", l.Realized, l.Available, l.Grade),
		Freshness:   "as of the observatory model's read",
		Body:        renderLadderPanel(l),
	})
}
