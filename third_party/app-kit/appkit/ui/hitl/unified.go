package hitl

// unified.go — a line-level text diff for reviewing a proposed change to a
// document, config file or source file. It complements the field-level
// DiffView: DiffView answers "which values change", RenderUnifiedDiff answers
// "which lines of this text change". Table-free, keyboard-scrollable, escaped,
// no JavaScript; unchanged context collapses behind native <details>.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/app-kit/appkit/ui/internal/linetext"
)

// UnifiedDiff configures RenderUnifiedDiff. Supply either the two texts (Old
// and New) or precomputed Hunks, never both.
type UnifiedDiff struct {
	// Label is the required accessible name, e.g. "config.toml".
	Label string
	// OldName and NewName name the two sides; empty renders "current" and
	// "proposed", matching DiffView.
	OldName, NewName string
	// Old and New are the complete texts. Unchanged runs outside the context
	// window collapse behind a native "Show N unchanged lines" disclosure.
	Old, New string
	// Hunks are precomputed hunks of one comparison, in order (for example
	// from LineDiff.Hunks or an app's own diff). A subset is allowed, such as
	// only the hunks that touch one symbol, so the lines between supplied hunks
	// may hold changes the renderer never received: each skipped range is
	// stated per side ("Lines 4–14 not included", or "Not included: current 2,
	// proposed 2–9"), never called unchanged, and cannot be expanded.
	Hunks []Hunk
	// Replaced carries LineDiff.Replaced with precomputed Hunks, so a diff
	// that exceeded DiffText's edit budget still says it is not minimal. The
	// renderer computes it itself from Old and New; setting it with texts is
	// refused.
	Replaced bool
	// Context is the number of unchanged lines kept around each change when
	// comparing Old and New: 0 means DefaultContext, NoContext means none.
	Context int
}

func (d UnifiedDiff) names() (string, string) {
	oldName, newName := d.OldName, d.NewName
	if strings.TrimSpace(oldName) == "" {
		oldName = "current"
	}
	if strings.TrimSpace(newName) == "" {
		newName = "proposed"
	}
	return oldName, newName
}

// RenderUnifiedDiff renders unified hunks with old and new line numbers, "+"
// and "-" markers as text, a screen-reader label per line, and role-tinted rows.
// Invalid input returns an error and no partial markup; identical texts render
// an explicit "No changes" state. Hidden, direction-changing and
// line-terminating characters are revealed as visible code points in the lines
// and in the label and side names, which are also isolated (<bdi>) so a name
// cannot reorder the text beside it. Scan appkit/ui/hitl/*.go (L3b).
func RenderUnifiedDiff(d UnifiedDiff) (string, error) {
	if strings.TrimSpace(d.Label) == "" {
		return "", fmt.Errorf("hitl: unified diff label is required")
	}
	var segments []segment
	replaced := d.Replaced
	if d.Hunks != nil {
		if d.Old != "" || d.New != "" {
			return "", fmt.Errorf("hitl: supply texts or precomputed hunks, not both")
		}
		// Validate and number a copy: the caller's hunks are never modified.
		hunks := append([]Hunk(nil), d.Hunks...)
		if err := validateHunks(hunks); err != nil {
			return "", err
		}
		for i := range hunks {
			segments = append(segments, segment{hunk: &hunks[i]})
		}
	} else {
		if d.Replaced {
			return "", fmt.Errorf("hitl: Replaced describes precomputed hunks; it is computed from Old and New")
		}
		diff, err := DiffText(d.Old, d.New)
		if err != nil {
			return "", err
		}
		replaced = diff.Replaced
		segments = diff.segments(effectiveContext(d.Context))
	}
	names := revealNames(d.names())
	added, removed := 0, 0
	for _, s := range segments {
		if s.hunk == nil {
			continue
		}
		for _, line := range s.hunk.Lines {
			switch line.Kind {
			case DiffAdded:
				added++
			case DiffRemoved:
				removed++
			}
		}
	}

	var b strings.Builder
	b.WriteString(`<figure class="min-w-0 overflow-hidden rounded-lg border border-line bg-surface-sunken text-ink" data-unified-diff>`)
	b.WriteString(`<figcaption class="flex flex-wrap items-baseline gap-x-4 gap-y-1 border-b border-line bg-surface-raised px-3 py-2 text-sm">`)
	fmt.Fprintf(&b, `<span class="min-w-0 font-medium text-ink [overflow-wrap:anywhere]">%s</span>`, bidiIsolated(d.Label))
	fmt.Fprintf(&b, `<span class="min-w-0 font-mono text-xs text-ink-muted [overflow-wrap:anywhere]"><span aria-hidden="true">- </span>%s <span aria-hidden="true">→ + </span><span class="sr-only">compared with </span>%s</span>`, names.oldHTML, names.newHTML)
	if len(segments) > 0 {
		fmt.Fprintf(&b, `<span class="text-xs tabular-nums text-ink-muted">%s added, %s removed</span>`, lineCount(added), lineCount(removed))
	}
	b.WriteString(`</figcaption>`)
	if replaced {
		b.WriteString(`<p class="border-b border-line bg-caution-soft px-3 py-2 text-xs text-ink" role="note">This comparison exceeded the diff budget, so the differing region is shown as one removal followed by one addition, not a minimal diff.</p>`)
	}
	if len(segments) == 0 {
		empty := "No changes: the texts are identical."
		if d.Hunks != nil {
			empty = "No changed lines were supplied."
		}
		fmt.Fprintf(&b, `<p class="px-3 py-3 text-sm text-ink-muted">%s</p></figure>`, empty)
		return b.String(), nil
	}
	fmt.Fprintf(&b, `<div class="overflow-x-auto focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-focus" tabindex="0" role="region" aria-label="%s">`, esc(linetext.Reveal(d.Label)+" line changes, "+names.oldText+" to "+names.newText))
	b.WriteString(`<div class="min-w-max py-1 font-mono text-xs leading-5">`)
	precomputed := d.Hunks != nil
	oldNext, newNext := 1, 1 // the first line after the previous hunk
	for _, s := range segments {
		if s.hunk == nil {
			first := s.hidden[0]
			writeCollapsed(&b, s.hidden, rangeWhere(first.OldNumber, first.NewNumber, len(s.hidden), names), names)
			continue
		}
		firstOld, firstNew := s.hunk.firstLines()
		if precomputed && (firstOld > oldNext || firstNew > newNext) {
			writeOmitted(&b, oldNext, firstOld-oldNext, newNext, firstNew-newNext, names)
		}
		writeHunk(&b, *s.hunk, names)
		oldNext, newNext = firstOld+s.hunk.OldCount, firstNew+s.hunk.NewCount
	}
	b.WriteString(`</div></div></figure>`)
	return b.String(), nil
}

func lineCount(n int) string {
	if n == 1 {
		return "1 line"
	}
	return strconv.Itoa(n) + " lines"
}

// firstLines returns the number of the first old and new line at the hunk's
// position: a zero-count range starts at the line before the hunk, as in git.
func (h Hunk) firstLines() (oldFirst, newFirst int) {
	oldFirst, newFirst = h.OldStart, h.NewStart
	if h.OldCount == 0 {
		oldFirst++
	}
	if h.NewCount == 0 {
		newFirst++
	}
	return oldFirst, newFirst
}

// validateHunks checks that precomputed hunks could come from one comparison:
// ordered and not overlapping on either side, every line number agreeing with
// its hunk's ranges, and "no newline at end of file" only on the last line of
// a side. The ranges skipped between hunks may differ in length per side: a
// subset of hunks leaves changes out of those ranges. Zero numbers are filled
// from the ranges. All hunks together are bounded like two MaxDiffLines texts.
func validateHunks(hunks []Hunk) error {
	total := 0
	for _, h := range hunks {
		total += len(h.Lines)
	}
	if total > 2*MaxDiffLines {
		return fmt.Errorf("hitl: precomputed hunks carry %d lines; at most %d", total, 2*MaxDiffLines)
	}
	oldNext, newNext := 1, 1 // the first line after the previous hunk
	for i := range hunks {
		h := &hunks[i]
		if h.OldStart < 0 || h.NewStart < 0 || h.OldCount < 0 || h.NewCount < 0 {
			return fmt.Errorf("hitl: hunk %d: negative range", i)
		}
		if (h.OldCount > 0 && h.OldStart < 1) || (h.NewCount > 0 && h.NewStart < 1) {
			return fmt.Errorf("hitl: hunk %d: a nonempty range starts at line 1 or later", i)
		}
		if len(h.Lines) == 0 {
			return fmt.Errorf("hitl: hunk %d: has no lines", i)
		}
		firstOld, firstNew := h.firstLines()
		if firstOld < oldNext || firstNew < newNext {
			return fmt.Errorf("hitl: hunk %d: overlaps or precedes the previous hunk", i)
		}
		lines := make([]DiffLine, len(h.Lines))
		copy(lines, h.Lines)
		oldSeen, newSeen := 0, 0
		for j := range lines {
			line := &lines[j]
			if strings.Contains(line.Text, "\n") {
				return fmt.Errorf("hitl: hunk %d line %d: text contains a newline", i, j)
			}
			wantOld, wantNew := 0, 0
			switch line.Kind {
			case DiffContext:
				wantOld, wantNew = firstOld+oldSeen, firstNew+newSeen
				oldSeen++
				newSeen++
			case DiffRemoved:
				wantOld = firstOld + oldSeen
				oldSeen++
			case DiffAdded:
				wantNew = firstNew + newSeen
				newSeen++
			default:
				return fmt.Errorf("hitl: hunk %d line %d: unknown kind %q", i, j, line.Kind)
			}
			if (line.OldNumber != 0 && line.OldNumber != wantOld) || (line.NewNumber != 0 && line.NewNumber != wantNew) {
				return fmt.Errorf("hitl: hunk %d line %d: numbers %d/%d disagree with the hunk range", i, j, line.OldNumber, line.NewNumber)
			}
			line.OldNumber, line.NewNumber = wantOld, wantNew
		}
		if oldSeen != h.OldCount || newSeen != h.NewCount {
			return fmt.Errorf("hitl: hunk %d: header counts -%d +%d, lines give -%d +%d", i, h.OldCount, h.NewCount, oldSeen, newSeen)
		}
		h.Lines = lines
		oldNext, newNext = firstOld+h.OldCount, firstNew+h.NewCount
	}
	return validateNoNewline(hunks)
}

// validateNoNewline allows NoNewline only on the last supplied line of the
// side it describes: the old side for context and removed lines, the new side
// for context and added lines.
func validateNoNewline(hunks []Hunk) error {
	type at struct{ hunk, line int }
	lastOld, lastNew := at{-1, -1}, at{-1, -1}
	for i, h := range hunks {
		for j, line := range h.Lines {
			if line.Kind != DiffAdded {
				lastOld = at{i, j}
			}
			if line.Kind != DiffRemoved {
				lastNew = at{i, j}
			}
		}
	}
	for i, h := range hunks {
		for j, line := range h.Lines {
			if !line.NoNewline {
				continue
			}
			here := at{i, j}
			if (line.Kind != DiffAdded && here != lastOld) || (line.Kind != DiffRemoved && here != lastNew) {
				return fmt.Errorf("hitl: hunk %d line %d: only the last line of a side can lack a final newline", i, j)
			}
		}
	}
	return nil
}

func span(from, to int) string {
	if from == to {
		return strconv.Itoa(from)
	}
	return strconv.Itoa(from) + "–" + strconv.Itoa(to)
}

func sideRange(name string, start, count int) string {
	if count == 0 {
		return "no " + name + " lines"
	}
	return name + " " + span(start, start+count-1)
}

// sideNames holds each side's name twice: as plain text with hidden
// characters replaced by their notation (spoken labels and attributes), and as
// isolated, revealed markup for visible text.
type sideNames struct {
	oldText, newText string
	oldHTML, newHTML string
}

func revealNames(oldName, newName string) sideNames {
	return sideNames{
		oldText: linetext.Reveal(oldName), newText: linetext.Reveal(newName),
		oldHTML: bidiIsolated(oldName), newHTML: bidiIsolated(newName),
	}
}

// bidiIsolated returns s escaped, with hidden characters revealed, inside <bdi> so
// right-to-left text in a name cannot reorder the markers around it.
func bidiIsolated(s string) string {
	var b strings.Builder
	b.WriteString("<bdi>")
	writeRevealed(&b, s)
	b.WriteString("</bdi>")
	return b.String()
}

// writeRevealed escapes ordinary text and styles each revealed notation (see
// linetext.Hidden), so a character that draws nothing, reorders the text or
// changes how it parses is shown as a code point.
func writeRevealed(b *strings.Builder, text string) {
	for _, segment := range linetext.Segments(text) {
		if segment.Revealed {
			fmt.Fprintf(b, `<span class="mx-px rounded-sm border border-danger px-0.5 text-ink" data-revealed>%s</span>`, esc(segment.Text))
			continue
		}
		b.WriteString(esc(segment.Text))
	}
}

// rangeWhere describes an unchanged run of count lines starting at oldFrom and
// newFrom as visible markup: "lines 7–13" when both sides agree, otherwise
// each side by name.
func rangeWhere(oldFrom, newFrom, count int, names sideNames) string {
	if oldFrom == newFrom {
		if count == 1 {
			return "line " + strconv.Itoa(oldFrom)
		}
		return "lines " + span(oldFrom, oldFrom+count-1)
	}
	return names.oldHTML + " " + span(oldFrom, oldFrom+count-1) + ", " + names.newHTML + " " + span(newFrom, newFrom+count-1)
}

func writeHunk(b *strings.Builder, h Hunk, names sideNames) {
	fmt.Fprintf(b, `<p class="border-y border-line bg-info-soft px-3 text-ink-muted" data-hunk-header><span aria-hidden="true">%s</span><span class="sr-only">Changed region: %s, %s</span></p>`,
		esc(h.Header()), esc(sideRange(names.oldText, h.OldStart, h.OldCount)), esc(sideRange(names.newText, h.NewStart, h.NewCount)))
	b.WriteString(`<ol role="list">`)
	for _, line := range h.Lines {
		writeDiffLine(b, line, names)
	}
	b.WriteString(`</ol>`)
}

func writeCollapsed(b *strings.Builder, lines []DiffLine, where string, names sideNames) {
	// Native disclosure: keyboard, state announcement and no-JS behavior come
	// from <details>; the verb follows the open state through CSS alone.
	fmt.Fprintf(b, `<details class="group border-y border-dashed border-line" data-diff-collapsed><summary class="min-h-11 cursor-pointer px-3 py-3 font-sans leading-5 text-ink-muted hover:text-ink focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-focus"><span class="group-open:hidden">Show</span><span class="hidden group-open:inline">Hide</span> %d unchanged %s (%s)</summary><ol role="list">`, len(lines), noun(len(lines)), where)
	for _, line := range lines {
		writeDiffLine(b, line, names)
	}
	b.WriteString(`</ol></details>`)
}

// writeOmitted states the ranges skipped before a precomputed hunk. The
// renderer never received those lines, so it cannot say whether they changed,
// and a supplied subset of hunks can skip different amounts on each side.
func writeOmitted(b *strings.Builder, oldFrom, oldCount, newFrom, newCount int, names sideNames) {
	var text string
	switch {
	case oldFrom == newFrom && oldCount == newCount && oldCount == 1:
		text = "Line " + strconv.Itoa(oldFrom) + " not included"
	case oldFrom == newFrom && oldCount == newCount:
		text = "Lines " + span(oldFrom, oldFrom+oldCount-1) + " not included"
	default:
		side := func(nameHTML string, from, count int) string {
			if count == 0 {
				return "no " + nameHTML + " lines"
			}
			return nameHTML + " " + span(from, from+count-1)
		}
		text = "Not included: " + side(names.oldHTML, oldFrom, oldCount) + ", " + side(names.newHTML, newFrom, newCount)
	}
	fmt.Fprintf(b, `<p class="border-y border-dashed border-line px-3 py-1 font-sans text-ink-muted" data-diff-omitted>%s</p>`, text)
}

func noun(count int) string {
	if count == 1 {
		return "line"
	}
	return "lines"
}

type diffStyle struct {
	marker, row, label string
}

var diffStyles = map[DiffLineKind]diffStyle{
	DiffContext: {marker: " ", row: "border-transparent bg-surface-sunken", label: "Unchanged"},
	DiffAdded:   {marker: "+", row: "border-positive bg-positive-soft", label: "Added"},
	DiffRemoved: {marker: "-", row: "border-danger bg-danger-soft", label: "Removed"},
}

func writeDiffLine(b *strings.Builder, line DiffLine, names sideNames) {
	oldName, newName := names.oldText, names.newText
	style := diffStyles[line.Kind]
	number := func(n int) string {
		if n == 0 {
			return ""
		}
		return strconv.Itoa(n)
	}
	var where string
	switch line.Kind {
	case DiffAdded:
		where = newName + " line " + strconv.Itoa(line.NewNumber)
	case DiffRemoved:
		where = oldName + " line " + strconv.Itoa(line.OldNumber)
	default:
		where = "line " + strconv.Itoa(line.OldNumber)
		if line.OldNumber != line.NewNumber {
			where = oldName + " line " + strconv.Itoa(line.OldNumber) + ", " + newName + " line " + strconv.Itoa(line.NewNumber)
		}
	}
	fmt.Fprintf(b, `<li class="flex border-l-2 %s" data-kind="%s" data-old-line="%s" data-new-line="%s">`, style.row, line.Kind, number(line.OldNumber), number(line.NewNumber))
	fmt.Fprintf(b, `<span class="sticky left-0 flex shrink-0 bg-inherit select-none" aria-hidden="true"><span class="w-12 pr-2 text-right tabular-nums text-ink-muted">%s</span><span class="w-12 pr-2 text-right tabular-nums text-ink-muted">%s</span><span class="w-5 text-center font-semibold">%s</span></span>`,
		number(line.OldNumber), number(line.NewNumber), style.marker)
	fmt.Fprintf(b, `<span class="sr-only select-none">%s, %s: </span>`, style.label, esc(where))
	b.WriteString(`<code class="whitespace-pre pr-4 [tab-size:4]">`)
	writeRevealed(b, line.Text)
	b.WriteString(`</code></li>`)
	if line.NoNewline {
		b.WriteString(`<li class="flex border-l-2 border-transparent bg-surface-sunken text-ink-muted" data-no-newline><span class="sticky left-0 w-29 shrink-0 bg-surface-sunken" aria-hidden="true"></span><span class="pr-4 font-sans">No newline at end of file</span></li>`)
	}
}
