package gallery

import (
	"fmt"
	"html/template"
	"strings"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/code"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/hitl"
)

// code_review.go is the Gallery specimen for the two line-oriented review
// renderers: code.RenderExcerpt (numbered lines, labelled per-line state,
// anchors) and hitl.RenderUnifiedDiff (unified hunks with collapsed context).
// All source text here is synthetic and written for this page.

const reviewSource = `// Conserve applies a plan only when every changed symbol is covered.
func Conserve(plan Plan, cov Coverage) (Report, error) {
	if plan.Empty() {
		return Report{}, nil
	}
	var report Report
	for _, change := range plan.Changes {
		if !cov.Executed(change.Symbol) {
			report.Unexecuted = append(report.Unexecuted, change)
			continue
		}
		report.Conserved++
	}
	if len(report.Unexecuted) > 0 {
		return report, ErrPartial
	}
	return report, nil
}
`

const diffCurrent = `[service]
name = "ledger"
region = "eu-west"

[retry]
attempts = 3
backoff = "1s"
jitter = false

[timeouts]
connect = "2s"
read = "5s"
write = "5s"
idle = "60s"
shutdown = "10s"
drain = "30s"

[limits]
queue = 64
workers = 4
burst = 16
`

const diffProposed = `[service]
name = "ledger"
region = "eu-west"

[retry]
attempts = 5
backoff = "1s"
jitter = true
max_backoff = "30s"

[timeouts]
connect = "2s"
read = "5s"
write = "5s"
idle = "60s"
shutdown = "10s"
drain = "30s"

[limits]
queue = 128
workers = 4
burst = 16
`

type codeReviewData struct {
	Excerpt, Wide, Empty, Diff, Hunks, Same template.HTML
	Refusal                                 string
	ExcerptGo, DiffGo                       string
}

func buildCodeReview() (template.HTML, error) {
	lines := code.SplitLines(40, reviewSource)
	for i := range lines {
		switch lines[i].Number {
		case 47:
			lines[i].State = code.LineChanged
		case 48, 49:
			lines[i].State = code.LineNotExecuted
		case 54:
			lines[i].State = code.LineHighlighted
		}
	}
	// Keep 40–51 and 53–57 so the skipped line is stated, not hidden.
	shown := append(append([]code.Line(nil), lines[:12]...), lines[13:]...)
	excerpt, err := code.RenderExcerpt(code.Excerpt{
		Label:        "internal/conserve/conserve.go, lines 40 to 57",
		Caption:      "internal/conserve/conserve.go @ 3f2a9c1",
		SourceHref:   "/catalog/content.code-excerpt",
		SourceLabel:  "Capability contract",
		AnchorPrefix: "review-",
		Lines:        shown,
	})
	if err != nil {
		return "", err
	}

	wideLines := code.SplitLines(1, "package limits\n\n"+
		"// "+strings.Repeat("A deliberately long comment keeps its line intact and scrolls inside its own viewport. ", 3)+"\n"+
		"var defaultLimits = map[string]int{\"queue\": 64, \"workers\": 4, \"burst\": 16, \"retry_budget_per_minute_for_background_reconciliation\": 120}\n"+
		"\tconst role = \"user\u202e \u2066// admin\u2069\u2066\" // hidden direction controls are revealed\n"+
		"\tconst debug = false // disabled:\u2028sendTokens() — a line separator would end this comment in JavaScript\n"+
		"\tconst scope = \"user\U000E0061\U000E0064\" // zero-width tag characters are revealed\n")
	wideLines[3].State = code.LineChanged
	wideLines[4].State = code.LineHighlighted
	wideLines[5].State = code.LineHighlighted
	wideLines[6].State = code.LineHighlighted
	wide, err := code.RenderExcerpt(code.Excerpt{
		Label:        "limits.go, lines 1 to 7, long lines scroll horizontally",
		Caption:      "internal/limits/limits.go",
		AnchorPrefix: "wide-",
		Lines:        wideLines,
		Labels:       code.StateLabels{Highlighted: "Reviewer note"},
	})
	if err != nil {
		return "", err
	}

	empty, err := code.RenderExcerpt(code.Excerpt{Label: "empty excerpt", Caption: "internal/empty/empty.go"})
	if err != nil {
		return "", err
	}
	refusedHTML, refusal := code.RenderExcerpt(code.Excerpt{Label: "unsafe link", SourceHref: "javascript:alert(1)", Lines: lines[:1]})
	if refusal == nil || refusedHTML != "" {
		return "", fmt.Errorf("gallery: unsafe excerpt link was not refused")
	}

	diff, err := hitl.RenderUnifiedDiff(hitl.UnifiedDiff{Label: "deploy/ledger.toml", Old: diffCurrent, New: diffProposed})
	if err != nil {
		return "", err
	}
	computed, err := hitl.DiffText(diffCurrent, diffProposed)
	if err != nil {
		return "", err
	}
	hunks, err := hitl.RenderUnifiedDiff(hitl.UnifiedDiff{
		Label: "deploy/ledger.toml (precomputed hunks)", OldName: "main", NewName: "branch",
		Hunks: computed.Hunks(hitl.NoContext), Replaced: computed.Replaced,
	})
	if err != nil {
		return "", err
	}
	same, err := hitl.RenderUnifiedDiff(hitl.UnifiedDiff{Label: "deploy/ledger.toml (unchanged)", Old: diffCurrent, New: diffCurrent})
	if err != nil {
		return "", err
	}

	return htmlf("code", codeReviewHTML, codeReviewData{
		Excerpt: template.HTML(excerpt), Wide: template.HTML(wide), Empty: template.HTML(empty),
		Diff: template.HTML(diff), Hunks: template.HTML(hunks), Same: template.HTML(same),
		Refusal: refusal.Error(),
		ExcerptGo: `lines := code.SplitLines(40, source)
lines[7].State = code.LineChanged      // line 47; meaning is app-owned
lines[8].State = code.LineNotExecuted  // line 48
html, err := code.RenderExcerpt(code.Excerpt{
    Label:        "internal/conserve/conserve.go, lines 40 to 57",
    Caption:      "internal/conserve/conserve.go @ 3f2a9c1",
    SourceHref:   "/files/internal/conserve/conserve.go?rev=3f2a9c1",
    AnchorPrefix: "review-", // ids review-L40…; one prefix per excerpt
    Lines:        lines,
})`,
		DiffGo: `html, err := hitl.RenderUnifiedDiff(hitl.UnifiedDiff{
    Label: "deploy/ledger.toml",
    Old:   current, New: proposed,
    // or Hunks: diff.Hunks(hitl.NoContext), Replaced: diff.Replaced
})`,
	})
}

const codeReviewHTML = `
<h1 class="text-2xl font-semibold text-ink">Code review</h1>
<p class="mt-2 max-w-3xl text-ink-muted">Two experimental, line-oriented review renderers. <code class="font-mono text-sm text-ink">appkit/ui/code</code> shows a source excerpt with numbered lines, an app-supplied state per line, stable anchors and permalinks. <code class="font-mono text-sm text-ink">hitl.RenderUnifiedDiff</code> shows a proposed text change as unified hunks, beside the field-level <code class="font-mono text-sm text-ink">hitl.DiffView</code>. Both are server-rendered, need no JavaScript and keep long lines on one row inside their own keyboard-scrollable viewport.</p>
<p class="mt-3 max-w-3xl font-medium text-ink">All source text, revisions and states on this page are synthetic examples.</p>

<section id="code-excerpt" class="mt-10" aria-labelledby="code-excerpt-title">
  <h2 id="code-excerpt-title" class="text-lg font-medium text-ink">Code excerpt</h2>
  <p class="mt-2 max-w-3xl text-sm text-ink-muted">Each state has a marker, a screen-reader label and a legend entry, so color is never the only channel. Line numbers are permalinks: <a class="text-accent underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" href="#review-L49">jump to line 49</a> to see the targeted-line outline. Skipped lines are announced as a gap.</p>
  <div class="mt-4 grid gap-4 lg:grid-cols-2">
    <div class="min-w-0">{{.Excerpt}}</div>
    <div class="min-w-0 rounded-lg border border-line bg-surface-sunken p-4">
      <pre class="max-w-full overflow-x-auto font-mono text-xs leading-relaxed text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="Code excerpt code example"><code>{{.ExcerptGo}}</code></pre>
    </div>
  </div>
</section>

<section class="mt-10" aria-labelledby="code-wide-title">
  <h2 id="code-wide-title" class="text-lg font-medium text-ink">Long lines, custom labels and hidden characters</h2>
  <p class="mt-2 max-w-3xl text-sm text-ink-muted">Code never wraps. Focus the viewport and use the arrow keys to scroll; the line numbers and markers stay pinned. Direction-override, zero-width, tag and line-separator characters are shown as their code points instead of silently reordering, hiding or splitting the line.</p>
  <div class="mt-4 min-w-0">{{.Wide}}</div>
</section>

<section class="mt-10" aria-labelledby="code-states-title">
  <h2 id="code-states-title" class="text-lg font-medium text-ink">Empty excerpt and refused input</h2>
  <div class="mt-4 grid gap-4 lg:grid-cols-2">
    <div class="min-w-0">{{.Empty}}</div>
    <div class="min-w-0 rounded-lg border border-line bg-surface-raised p-4 text-sm">
      <p class="font-medium text-ink">A <code class="font-mono">javascript:</code> source link is refused before any markup is written:</p>
      <p class="mt-2 font-mono text-xs text-ink [overflow-wrap:anywhere]" data-refusal>{{.Refusal}}</p>
    </div>
  </div>
</section>

<section id="unified-diff" class="mt-12" aria-labelledby="unified-diff-title">
  <h2 id="unified-diff-title" class="text-lg font-medium text-ink">Unified diff</h2>
  <p class="mt-2 max-w-3xl text-sm text-ink-muted">A deterministic Myers line diff with three lines of context. Markers are text (<code class="font-mono">+</code>, <code class="font-mono">-</code>); each line also carries a spoken label with its line numbers. Unchanged runs collapse behind a native disclosure.</p>
  <div class="mt-4 grid gap-4 lg:grid-cols-2">
    <div class="min-w-0">{{.Diff}}</div>
    <div class="min-w-0 rounded-lg border border-line bg-surface-sunken p-4">
      <pre class="max-w-full overflow-x-auto font-mono text-xs leading-relaxed text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="Unified diff code example"><code>{{.DiffGo}}</code></pre>
    </div>
  </div>
</section>

<section class="mt-10" aria-labelledby="diff-hunks-title">
  <h2 id="diff-hunks-title" class="text-lg font-medium text-ink">Precomputed hunks and no changes</h2>
  <p class="mt-2 max-w-3xl text-sm text-ink-muted">Given only hunks, the renderer never invents the lines between them: each skipped range is stated per side, is never called unchanged and cannot be expanded. Identical texts say so instead of drawing an empty viewport.</p>
  <div class="mt-4 grid gap-4 lg:grid-cols-2">
    <div class="min-w-0">{{.Hunks}}</div>
    <div class="min-w-0">{{.Same}}</div>
  </div>
</section>
`
