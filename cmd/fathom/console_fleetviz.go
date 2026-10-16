package main

// console_fleetviz.go — the small renderers the Now and Fleet pages share:
// the shared-scale traffic sparkline (OC-20), the realization rungs, the
// health badge, and the traffic table every sparkline has as its
// alternative.
//
// The sparkline is fathom's own, built just in time as the kit's
// originating consumer (vision 35 §6; returned as FATHOM-B-KIT-01 in
// docs/app-kit.md): viz.Sparkline scales every call to its own min and
// max and joins across a missing point, and the fleet needs the opposite —
// one scale for every row, and no line across a day the call log never
// observed. It keeps viz's L3a discipline: no class attribute, every color
// a var(--ak-color-<role>) on a presentation attribute, role="img" with a
// title, deterministic output.

import (
	"fmt"
	"html"
	"strings"

	"github.com/a-h/templ"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/callout"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/ui/table"
)

// Sparkline geometry: 8 px per day across a 14-day window.
const (
	sparkW   = 112
	sparkH   = 28
	sparkPad = 2.0
)

// sharedSparkline draws one project's calls per day on the fleet's shared
// scale: a line within each run of measured days, a dashed gap box on each
// day the call log holds no row, a baseline, and the latest measured day
// marked.
func sharedSparkline(s console.Spark, service string) string {
	n := len(s.Points)
	label := sparkLabel(s, service)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" data-shared-spark data-scale-max="%d">`,
		sparkW, sparkH, sparkW, sparkH, s.Max)
	b.WriteString(`<title>` + html.EscapeString(label) + `</title>`)
	if n == 0 {
		b.WriteString(`</svg>`)
		return b.String()
	}
	step := 0.0
	if n > 1 {
		step = (sparkW - 2*sparkPad) / float64(n-1)
	}
	x := func(i int) float64 {
		if n == 1 {
			return sparkW / 2
		}
		return sparkPad + step*float64(i)
	}
	top := float64(s.Max)
	y := func(v int) float64 {
		if top <= 0 {
			return sparkH - sparkPad
		}
		return sparkPad + (sparkH-2*sparkPad)*(1-float64(v)/top)
	}
	// Baseline: zero on the shared scale.
	fmt.Fprintf(&b, `<line x1="0" y1="%s" x2="%d" y2="%s" stroke="var(--ak-color-line)" stroke-width="1"/>`, f1(sparkH-sparkPad), sparkW, f1(sparkH-sparkPad))
	// Absent days: a dashed gap box, never a value.
	for i, p := range s.Points {
		if p.Calls != nil {
			continue
		}
		w := step
		if w < 6 {
			w = 6
		}
		left := x(i) - w/2
		if left < 0 {
			left = 0
		}
		if left+w > sparkW {
			w = sparkW - left
		}
		fmt.Fprintf(&b, `<rect x="%s" y="1" width="%s" height="%d" fill="var(--ak-color-surface-sunken)" stroke="var(--ak-color-ink-muted)" stroke-width="1" stroke-dasharray="2 2"><title>%s</title></rect>`,
			f1(left), f1(w), sparkH-2, html.EscapeString(p.Absent))
	}
	// One line per run of measured days.
	for _, run := range s.Runs {
		if run[0] == run[1] {
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="1.5" fill="var(--ak-color-accent)"/>`, f1(x(run[0])), f1(y(*s.Points[run[0]].Calls)))
			continue
		}
		pts := make([]string, 0, run[1]-run[0]+1)
		for i := run[0]; i <= run[1]; i++ {
			pts = append(pts, f1(x(i))+","+f1(y(*s.Points[i].Calls)))
		}
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="var(--ak-color-accent)" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/>`, strings.Join(pts, " "))
	}
	if k := len(s.Runs); k > 0 {
		last := s.Runs[k-1][1]
		fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="2.25" fill="var(--ak-color-accent)"/>`, f1(x(last)), f1(y(*s.Points[last].Calls)))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// sparkLabel says in words what the sparkline draws.
func sparkLabel(s console.Spark, service string) string {
	l := fmt.Sprintf("%s: calls per day over %d days on the shared scale 0–%s", service, len(s.Points), fmtThousands(s.Max))
	if s.Measured == 0 {
		return l + "; no day measured"
	}
	l += fmt.Sprintf("; %s measured", plural(s.Measured, "day"))
	if s.Absent > 0 {
		l += fmt.Sprintf(", %s with no call-log rows (gaps, not zeros)", plural(s.Absent, "day"))
	}
	for i := len(s.Points) - 1; i >= 0; i-- {
		if c := s.Points[i].Calls; c != nil {
			l += fmt.Sprintf("; %s on %s", plural(*c, "call"), s.Points[i].Date)
			break
		}
	}
	return l
}

func f1(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	return strings.TrimSuffix(s, ".0")
}

// rungPips renders the realization ladder as five labelled marks: met,
// unmet, not applicable and unknown are each a distinct glyph and outline,
// never color alone; the whole reads as one sentence to a screen reader,
// and hovering it lists each rung's evidence.
func rungPips(rungs []console.Rung, met, of int) string {
	if len(rungs) == 0 {
		return `<span class="italic text-ink-muted">no ladder — the registry did not answer for this project</span>`
	}
	var words, evidence []string
	var b strings.Builder
	for _, r := range rungs {
		cls, glyph, word := "border-dotted border-ink-muted text-ink-muted", "?", "unknown"
		switch r.State {
		case console.RungMet:
			cls, glyph, word = "border-positive bg-positive text-ink-inverse", "✓", "met"
		case console.RungUnmet:
			cls, glyph, word = "border-danger bg-danger-soft text-danger", "✕", "not met"
		case console.RungNotApplicable:
			cls, glyph, word = "border-dashed border-line bg-surface-sunken text-ink-muted", "–", "not applicable"
		}
		words = append(words, r.Name+": "+word)
		evidence = append(evidence, r.Name+": "+word+" — "+r.Evidence)
		b.WriteString(`<span class="` + cls + `">` + glyph + `</span>`)
	}
	count := fmt.Sprintf("%d of %d", met, of)
	return `<span class="inline-flex items-center gap-1 whitespace-nowrap" role="img" aria-label="` + html.EscapeString("realization: "+strings.Join(words, "; ")+"; "+count+" applicable rungs met") + `" title="` + html.EscapeString(strings.Join(evidence, "\n")) + `" data-rungs>` +
		`<span class="inline-flex gap-1 *:inline-flex *:size-4 *:items-center *:justify-center *:rounded-sm *:border *:text-[10px] *:font-bold *:leading-none" aria-hidden="true">` + b.String() + `</span>` +
		`<span class="ml-1 text-xs tabular-nums text-ink-muted" aria-hidden="true">` + count + `</span></span>`
}

// Below the breakpoint a kit table reads as one card per row: the header
// row hides, each row is a bordered block and each cell a label–value line
// (cellLabel), so a phone gets the same rows without a second rendering.
// Two literal class strings, one per breakpoint, so the CSS build sees them.
const (
	cardTableLG = `[&_td]:max-lg:flex [&_td]:max-lg:items-start [&_td]:max-lg:justify-between [&_td]:max-lg:gap-3 [&_td]:max-lg:px-3 [&_td]:max-lg:py-1.5 [&_caption]:max-lg:block [&_table]:max-lg:block [&_tbody]:max-lg:block [&_thead]:max-lg:hidden [&_tr]:max-lg:block [&_tr]:max-lg:py-2`
	cardTableXL = `[&_td]:max-xl:flex [&_td]:max-xl:items-start [&_td]:max-xl:justify-between [&_td]:max-xl:gap-3 [&_td]:max-xl:px-3 [&_td]:max-xl:py-1.5 [&_caption]:max-xl:block [&_table]:max-xl:block [&_tbody]:max-xl:block [&_thead]:max-xl:hidden [&_tr]:max-xl:block [&_tr]:max-xl:py-2`
)

// cellLabel is a cell's column name, shown only where the table reads as
// cards (below lg or xl).
func cellLabel(label, bp string) string {
	cls := "shrink-0 text-xs text-ink-muted lg:hidden"
	if bp == "xl" {
		cls = "shrink-0 text-xs text-ink-muted xl:hidden"
	}
	return `<span class="` + cls + `">` + html.EscapeString(label) + `</span>`
}

// labelled is a card cell: its label, then its value.
func labelled(label, bp, value string) string {
	cls := "min-w-0 text-right lg:text-left"
	if bp == "xl" {
		cls = "min-w-0 text-right xl:text-left"
	}
	return cellLabel(label, bp) + `<span class="` + cls + `">` + value + `</span>`
}

// healthBadge is a project's health as a kit badge: unknown is the
// neutral pill, never green.
func healthBadge(h console.Health) string {
	role := map[console.Health]string{console.HealthFailed: "danger", console.HealthDegraded: "caution", console.HealthHealthy: "positive"}[h]
	return `<span data-health="` + string(h) + `">` + callout.Badge(string(h), role) + `</span>`
}

// shortValue renders a table cell's projected value: the number, "≥ n" for
// a lower bound, or a short absence whose full reason is its title.
func shortValue(v console.Value[int], short string) string {
	switch v.State {
	case console.StateAbsent:
		return `<span class="italic text-ink-muted" title="` + html.EscapeString(v.Reason) + `" data-truth-state="absent">` + html.EscapeString(short) + `</span>`
	case console.StateError:
		return `<span class="text-danger" title="` + html.EscapeString(v.Reason) + `" data-truth-state="error">unreadable</span>`
	case console.StatePartial:
		return `<span class="tabular-nums" title="` + html.EscapeString(v.Reason) + `">≥ ` + fmtThousands(v.V) + `</span>`
	case console.StateStale:
		// Stale carries its age in words, never color alone.
		return `<span class="tabular-nums text-caution" title="stale: ` + html.EscapeString(v.Reason) + `" data-truth-state="stale">` + fmtThousands(v.V) + ` <span class="text-xs">stale</span></span>`
	}
	return `<span class="tabular-nums">` + fmtThousands(v.V) + `</span>`
}

// trafficTable is every sparkline's table alternative: each project's calls
// per day, an absent day written as "no rows" — never 0.
func trafficTable(id, caption string, days []string, rows []console.FleetRow) string {
	cols := []table.Column{{Key: "project", Label: "Project"}}
	for _, d := range days {
		cols = append(cols, table.Column{Key: d, Label: shortDate(d), Align: "right"})
	}
	trs := make([]table.Row, 0, len(rows))
	for _, r := range rows {
		cells := []string{r.Service}
		rich := []templ.Component{nil}
		for _, p := range r.Series.Points {
			if p.Calls == nil {
				cells = append(cells, "")
				rich = append(rich, templ.Raw(`<span class="italic text-ink-muted" title="`+htmlEsc(p.Absent)+`">no rows</span>`))
				continue
			}
			cells = append(cells, fmtThousands(*p.Calls))
			rich = append(rich, nil)
		}
		trs = append(trs, table.Row{ID: id + "-" + r.Service, Cells: cells, HTMLCells: rich})
	}
	return renderKitTable(table.TableParams{TableID: id, Caption: caption, Columns: cols, Rows: trs})
}

func htmlEsc(s string) string { return html.EscapeString(s) }

// kitBadge is the kit's status pill.
func kitBadge(label, role string) string { return callout.Badge(label, role) }
