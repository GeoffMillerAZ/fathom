// Package code renders read-only source excerpts for review surfaces: numbered
// lines with an app-supplied per-line state, stable line anchors with
// permalinks, and a keyboard-scrollable viewport in which code never wraps.
//
// It is an L3b package: markup carries Tailwind role utilities, so a consumer
// scans appkit/ui/code/*.go and includes the kit theme CSS. There is no
// JavaScript and no syntax-highlighting engine; text renders escaped in the
// monospace font, which is what a review excerpt needs. A state is never
// expressed by color alone: each non-normal line carries a visible marker, a
// screen-reader label, and the excerpt lists every present state as text.
//
// # Provenance
//
// First-party, requested by Fathom's observatory console (FATHOM-KIT-04,
// docs/design/observatory-console.md §5) to show unexecuted changed lines under
// a refactor review. No licensed code-viewer composition was found to adopt;
// the structure is original.
package code

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/internal/linetext"
	"github.com/geoffmilleraz/app-kit/appkit/ui/internal/webref"
)

// LineState is the app-supplied review state of one line. The app decides what
// a state means for its data; the kit only presents it.
type LineState string

// Line states. The zero value "" is LineNormal.
const (
	LineNormal      LineState = "normal"
	LineChanged     LineState = "changed"
	LineNotExecuted LineState = "not-executed"
	LineHighlighted LineState = "highlighted"
)

// MaxLines bounds one excerpt. Longer input is refused rather than truncated:
// an excerpt is a reading aid, and every line number is a keyboard stop.
const MaxLines = 2000

// Line is one source line. Number is its 1-based line number in the source;
// numbers must strictly increase within an excerpt, and a jump renders an
// explicit "not shown" gap. Text is plain text without "\n".
type Line struct {
	Number int
	Text   string
	State  LineState
}

// StateLabels overrides the text label of a state (for example "Uncovered"
// instead of "Not executed"). Blank fields keep the defaults: Changed,
// Not executed and Highlighted.
type StateLabels struct {
	Changed, NotExecuted, Highlighted string
}

// Excerpt configures RenderExcerpt.
type Excerpt struct {
	// Label is the required accessible name of the scrollable viewport, e.g.
	// "internal/conserve/conserve.go, lines 118 to 134".
	Label string
	// Caption is optional visible text above the lines, typically a path.
	Caption string
	// SourceHref optionally links the caption row to the full source. Relative
	// references and absolute http(s) URLs are accepted; anything else is
	// refused. The app owns the destination and its authorization.
	SourceHref string
	// SourceLabel is the link text; empty renders "View source".
	SourceLabel string
	// AnchorPrefix namespaces line IDs: a line's id is AnchorPrefix+"L"+Number,
	// so the default gives GitHub-style "#L120". Give each excerpt on one page a
	// distinct prefix; document-wide uniqueness is the app's responsibility.
	AnchorPrefix string
	Lines        []Line
	Labels       StateLabels
}

var anchorPrefix = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// SplitLines numbers text from start as normal-state lines. A final newline
// ends the last line rather than adding an empty one; "\r" stays content.
func SplitLines(start int, text string) []Line {
	parts, _ := linetext.Split(text)
	lines := make([]Line, len(parts))
	for i, part := range parts {
		lines[i] = Line{Number: start + i, Text: part}
	}
	return lines
}

type stateStyle struct {
	marker, row string
}

// stateStyles map a state to its visible marker and row treatment. Soft
// backgrounds keep ink text readable; the left rule and the marker keep the
// state perceivable without color. Not-executed follows the coverage-report
// convention (danger); the light caution role is too faint for a 3:1 rule.
var stateStyles = map[LineState]stateStyle{
	LineNormal:      {marker: "", row: "border-transparent bg-surface-sunken"},
	LineChanged:     {marker: "~", row: "border-info bg-info-soft"},
	LineNotExecuted: {marker: "!", row: "border-danger bg-danger-soft"},
	LineHighlighted: {marker: ">", row: "border-accent bg-accent-soft"},
}

// legendOrder is the fixed order in which present states are listed.
var legendOrder = []LineState{LineChanged, LineNotExecuted, LineHighlighted}

func (l StateLabels) label(s LineState) string {
	pick := func(custom, fallback string) string {
		if strings.TrimSpace(custom) != "" {
			return custom
		}
		return fallback
	}
	switch s {
	case LineChanged:
		return pick(l.Changed, "Changed")
	case LineNotExecuted:
		return pick(l.NotExecuted, "Not executed")
	case LineHighlighted:
		return pick(l.Highlighted, "Highlighted")
	}
	return ""
}

func normalize(s LineState) LineState {
	if s == "" {
		return LineNormal
	}
	return s
}

// validate refuses the whole excerpt before any markup is produced.
func (e Excerpt) validate() error {
	if strings.TrimSpace(e.Label) == "" {
		return fmt.Errorf("code: excerpt label is required")
	}
	if e.AnchorPrefix != "" && !anchorPrefix.MatchString(e.AnchorPrefix) {
		return fmt.Errorf("code: anchor prefix %q must start with a letter and contain only letters, digits, '-' or '_'", e.AnchorPrefix)
	}
	if e.SourceHref != "" && !webref.Allowed(e.SourceHref) {
		return fmt.Errorf("code: source link must be a relative reference or an absolute http(s) URL")
	}
	if len(e.Lines) > MaxLines {
		return fmt.Errorf("code: excerpt has %d lines; at most %d are rendered", len(e.Lines), MaxLines)
	}
	previous := 0
	for i, line := range e.Lines {
		if line.Number < 1 {
			return fmt.Errorf("code: line %d: number must be at least 1", i)
		}
		if line.Number <= previous {
			return fmt.Errorf("code: line %d: number %d does not follow %d", i, line.Number, previous)
		}
		previous = line.Number
		if strings.Contains(line.Text, "\n") {
			return fmt.Errorf("code: line %d: text contains a newline", i)
		}
		if _, ok := stateStyles[normalize(line.State)]; !ok {
			return fmt.Errorf("code: line %d: unknown state %q", i, line.State)
		}
	}
	return nil
}

// RenderExcerpt returns the excerpt as an HTML <figure>. Invalid input returns
// an error and no partial markup. An excerpt with no lines renders an explicit
// empty state instead of an empty viewport.
//
// Each line is an <li id="{AnchorPrefix}L{n}"> whose number is a native link to
// that anchor (the permalink); a targeted line gains a focus-colored frame
// through CSS :target. The viewport is a named region with tabindex="0" and a
// visible focus ring, so keyboard users can scroll long lines horizontally.
// Hidden, direction-changing and line-terminating characters are revealed as
// visible code points in the lines, the caption and the link text; the
// accessible name carries the same notations as plain text.
func RenderExcerpt(e Excerpt) (string, error) {
	if err := e.validate(); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(`<figure class="min-w-0 overflow-hidden rounded-lg border border-line bg-surface-sunken text-ink" data-code-excerpt>`)
	writeCaption(&b, e)
	if len(e.Lines) == 0 {
		b.WriteString(`<p class="px-3 py-3 text-sm text-ink-muted">No lines to show in this excerpt.</p></figure>`)
		return b.String(), nil
	}
	fmt.Fprintf(&b, `<div class="overflow-x-auto focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="%s">`, esc(linetext.Reveal(e.Label)))
	// leading-6 makes every row, and so every line-number permalink stretched
	// over it, 24px tall at a 24px pitch: the WCAG 2.2 2.5.8 minimum. Dense
	// code rows cannot reach the kit's 44px floor for standalone controls.
	b.WriteString(`<ol class="min-w-max py-2 font-mono text-xs leading-6" role="list">`)
	counts := map[LineState]int{}
	for i, line := range e.Lines {
		if i > 0 && line.Number > e.Lines[i-1].Number+1 {
			writeGap(&b, e.Lines[i-1].Number+1, line.Number-1)
		}
		state := normalize(line.State)
		counts[state]++
		writeLine(&b, e, line, state)
	}
	b.WriteString(`</ol></div>`)
	writeLegend(&b, e.Labels, counts)
	b.WriteString(`</figure>`)
	return b.String(), nil
}

func writeCaption(b *strings.Builder, e Excerpt) {
	if e.Caption == "" && e.SourceHref == "" {
		return
	}
	b.WriteString(`<figcaption class="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 border-b border-line bg-surface-raised px-3 py-1.5 text-sm">`)
	// The caption says what is under review, so it gets the same reveal as the
	// lines, isolated so a path cannot reorder the text beside it.
	if e.Caption != "" {
		b.WriteString(`<span class="min-w-0 py-1 font-mono text-ink [overflow-wrap:anywhere]"><bdi>`)
		writeText(b, e.Caption)
		b.WriteString(`</bdi></span>`)
	}
	if e.SourceHref != "" {
		label := e.SourceLabel
		if strings.TrimSpace(label) == "" {
			label = "View source"
		}
		fmt.Fprintf(b, `<a href="%s" class="inline-flex min-h-11 items-center rounded-sm text-accent underline underline-offset-2 hover:text-accent-hover focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-focus"><bdi>`, esc(e.SourceHref))
		writeText(b, label)
		b.WriteString(`</bdi></a>`)
	}
	b.WriteString(`</figcaption>`)
}

func writeGap(b *strings.Builder, from, to int) {
	text := "Line " + strconv.Itoa(from) + " not shown"
	if to > from {
		text = "Lines " + strconv.Itoa(from) + "–" + strconv.Itoa(to) + " not shown"
	}
	fmt.Fprintf(b, `<li class="flex border-y border-l-2 border-dashed border-line bg-surface-sunken text-ink-muted" data-code-gap><span class="sticky left-0 w-17 shrink-0 bg-surface-sunken pr-8 text-right" aria-hidden="true">⋯</span><span class="pr-4 font-sans">%s</span></li>`, text)
}

func writeLine(b *strings.Builder, e Excerpt, line Line, state LineState) {
	style := stateStyles[state]
	id := e.AnchorPrefix + "L" + strconv.Itoa(line.Number)
	n := strconv.Itoa(line.Number)
	// The targeted-line frame is an overlay above the pinned gutter: an outline
	// on the row itself would be painted beneath the sticky number column.
	fmt.Fprintf(b, `<li id="%s" class="relative flex scroll-mt-16 border-l-2 %s target:after:pointer-events-none target:after:absolute target:after:inset-0 target:after:z-10 target:after:border-2 target:after:border-focus" data-line="%s" data-state="%s">`, id, style.row, n, state)
	// Number and marker stay pinned while long lines scroll horizontally; the
	// wrapper inherits the row's opaque background so code slides beneath it.
	fmt.Fprintf(b, `<span class="sticky left-0 flex shrink-0 bg-inherit"><a href="#%s" class="w-12 pr-3 text-right tabular-nums text-ink-muted no-underline select-none hover:text-ink hover:underline focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-focus" aria-label="Line %s">%s</a>`, id, n, n)
	fmt.Fprintf(b, `<span class="w-5 text-center font-semibold select-none" aria-hidden="true">%s</span></span>`, esc(style.marker))
	// The state label is announced but never copied with the code.
	if label := e.Labels.label(state); label != "" {
		fmt.Fprintf(b, `<span class="sr-only select-none">%s: </span>`, esc(label))
	}
	b.WriteString(`<code class="whitespace-pre pr-4 [tab-size:4]">`)
	writeText(b, line.Text)
	b.WriteString(`</code></li>`)
}

// writeText escapes ordinary text and styles each revealed notation (see
// linetext.Hidden: controls, format and tag characters, line and paragraph
// separators, default-ignorable fillers and variation selectors, invalid
// bytes), so a character that draws nothing, reorders the line or changes how
// it parses is shown as a code point instead of passing review unseen.
func writeText(b *strings.Builder, text string) {
	for _, segment := range linetext.Segments(text) {
		if segment.Revealed {
			fmt.Fprintf(b, `<span class="mx-px rounded-sm border border-danger px-0.5 text-ink" data-revealed>%s</span>`, esc(segment.Text))
			continue
		}
		b.WriteString(esc(segment.Text))
	}
}

func writeLegend(b *strings.Builder, labels StateLabels, counts map[LineState]int) {
	present := false
	for _, state := range legendOrder {
		present = present || counts[state] > 0
	}
	if !present {
		return
	}
	b.WriteString(`<ul class="flex flex-wrap gap-x-4 gap-y-1 border-t border-line bg-surface-raised px-3 py-2 text-xs text-ink-muted" aria-label="Line states">`)
	for _, state := range legendOrder {
		count := counts[state]
		if count == 0 {
			continue
		}
		noun := "lines"
		if count == 1 {
			noun = "line"
		}
		fmt.Fprintf(b, `<li class="inline-flex items-center gap-1.5"><span class="inline-flex w-5 justify-center border-l-2 font-mono font-semibold text-ink %s" aria-hidden="true">%s</span><span><span class="font-medium text-ink">%s</span> · %d %s</span></li>`,
			stateStyles[state].row, esc(stateStyles[state].marker), esc(labels.label(state)), count, noun)
	}
	b.WriteString(`</ul>`)
}

func esc(s string) string { return html.EscapeString(s) }
