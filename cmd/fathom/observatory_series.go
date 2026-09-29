package main

// observatory_series.go — phase Y RG-6/RG-7 (docs/design/realization-grade.md
// §5 "activity over time", §6 "grade distribution over time", §7 charting
// discipline): the 30-day series behind the service page and the fleet
// scoreboard.
//
// Two folds, both pure over their inputs plus an injected clock:
//
//   - dimensionSeries: one service's ladder dimensions (queried, discovering,
//     preventing, gating) as a per-day series over the last N days, built
//     from REAL call-log rows. A day the log holds no rows at all is Absent
//     with a reason naming the date — the instrument was not observing, and
//     "0" would claim it was. A day the log was alive but this service or
//     dimension saw nothing is a measured zero, which is a different and
//     true claim.
//   - gradeDistributionSeries: the fleet's letter distribution per day. A
//     day without a persisted fleet-grade snapshot is Absent; nothing here
//     persists snapshots yet, so the series is today's fold plus 29 named
//     absences, and its note says so. The scoreboard accrues once daily
//     snapshots are recorded — it never back-fills.
//
// Rendering follows the house rules (cmd/fathom/AGENTS.md, binding): kit
// charts only (viz.Sparkline per contiguous run of measured days, viz.Bars
// for the distribution), stat.Absent for every absence, role utilities
// only. A sparkline is drawn once per RUN of measured days — an absent day
// is never interpolated across (the kit Sparkline has no gap notion; it
// drops NaN and joins the neighbours, which is exactly the line a gap must
// not have), so the strip is segment · gap chip · segment, and every gap
// chip names its dates and reason.

import (
	"fmt"
	"html"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/app-kit/appkit/ui/dashboard"
	"github.com/geoffmilleraz/app-kit/appkit/ui/stat"
	"github.com/geoffmilleraz/app-kit/appkit/ui/viz"
	"github.com/geoffmilleraz/fathom/internal/metrics"
)

// seriesDays is the window both series cover.
const seriesDays = 30

// seriesPoint is one calendar day of one dimension. Value nil = absent,
// and Reason then says why (law 3); a measured point carries no reason.
type seriesPoint struct {
	Date   string `json:"date"`
	Value  *int   `json:"value,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// dimSeries is one ladder dimension's series, oldest day first.
type dimSeries struct {
	Name     string        `json:"name"`
	What     string        `json:"what"`
	Points   []seriesPoint `json:"points"`
	Measured int           `json:"measured_days"`
	Absent   int           `json:"absent_days"`
	// Total sums the measured days only, and is nil (omitted from JSON)
	// when no day was measured: a reader must never see "total": 0 beside
	// thirty absences — a measured zero over measured days is "total": 0
	// WITH measured_days > 0, which is a different and true claim.
	Total *int `json:"total,omitempty"`
}

// seriesDimension names one call-log-derived rung and how a row counts
// toward it. Ladder order is fixed — the order is the story (a repository
// whose "queried" holds while "preventing" falls off is hooks breaking).
type seriesDimension struct {
	name  string
	what  string
	match func(adoptionCallEntry) bool
}

var seriesDimensions = []seriesDimension{
	{"queried", "calls this service answered", func(adoptionCallEntry) bool { return true }},
	{"discovering", "index-discovery calls (find_symbol, references, read_source, …)", func(e adoptionCallEntry) bool { return metrics.IsDiscoveryIndexTool(e.Tool) }},
	{"preventing", "rule injections (obligations) served before edits", func(e adoptionCallEntry) bool { return e.Tool == "obligations" }},
	{"gating", "verify-gate runs (verify_plan)", func(e adoptionCallEntry) bool { return e.Tool == "verify_plan" }},
}

// windowDates lists the last `days` calendar dates (UTC) ending today,
// oldest first. days <= 0 means seriesDays.
func windowDates(now time.Time, days int) []string {
	if days <= 0 {
		days = seriesDays
	}
	out := make([]string, 0, days)
	for i := days - 1; i >= 0; i-- {
		out = append(out, now.UTC().AddDate(0, 0, -i).Format("2006-01-02"))
	}
	return out
}

// dimensionSeries folds call-log rows into one series per ladder dimension
// for `service` ("" = fleet-wide, every row) over the last `days` days.
//
// Absence is decided by the LOG, not by the service: a date with no rows
// from anyone is absent for every service and dimension (the daemon may
// have been down; nobody can tell), while a date with rows elsewhere but
// none here is a measured zero. Pure: same rows, same clock, same series,
// regardless of row order.
func dimensionSeries(rows []adoptionCallEntry, service string, now time.Time, days int) []dimSeries {
	dates := windowDates(now, days)
	logged := map[string]bool{}
	counts := make([]map[string]int, len(seriesDimensions))
	for i := range counts {
		counts[i] = map[string]int{}
	}
	svc := strings.ToLower(service)
	for _, e := range rows {
		if len(e.TS) < 10 {
			continue
		}
		d := e.TS[:10]
		logged[d] = true
		if svc != "" && strings.ToLower(e.Service) != svc {
			continue
		}
		for i, dim := range seriesDimensions {
			if dim.match(e) {
				counts[i][d]++
			}
		}
	}
	out := make([]dimSeries, 0, len(seriesDimensions))
	for i, dim := range seriesDimensions {
		s := dimSeries{Name: dim.name, What: dim.what, Points: make([]seriesPoint, 0, len(dates))}
		total := 0
		for _, d := range dates {
			if !logged[d] {
				s.Points = append(s.Points, seriesPoint{Date: d, Reason: "no call-log rows for " + d})
				s.Absent++
				continue
			}
			n := counts[i][d]
			s.Points = append(s.Points, seriesPoint{Date: d, Value: &n})
			s.Measured++
			total += n
		}
		if s.Measured > 0 {
			s.Total = &total
		}
		out = append(out, s)
	}
	return out
}

// seriesSegment is one contiguous run of a series: either measured (Values
// non-nil, one per day) or absent (Values nil, Reason set). Segments
// partition the window in order, so the rendered strip reads as the
// calendar does — segment, gap, segment — and no line crosses a gap.
type seriesSegment struct {
	Run    dateRun
	Values []float64
	Reason string
}

// seriesSegments splits a series into its measured and absent runs.
func seriesSegments(s dimSeries) []seriesSegment {
	var segs []seriesSegment
	for _, p := range s.Points {
		measured := p.Value != nil
		if n := len(segs); n > 0 && (segs[n-1].Values != nil) == measured {
			last := &segs[n-1]
			last.Run.To = p.Date
			last.Run.N++
			if measured {
				last.Values = append(last.Values, float64(*p.Value))
			}
			continue
		}
		seg := seriesSegment{Run: dateRun{From: p.Date, To: p.Date, N: 1}}
		if measured {
			seg.Values = []float64{float64(*p.Value)}
		} else {
			seg.Reason = p.Reason
		}
		segs = append(segs, seg)
	}
	// An absent run's reason is its first day's; widen it to the span so
	// the chip names the whole gap, not just its first date.
	for i := range segs {
		if segs[i].Values == nil {
			segs[i].Reason = "no call-log rows for " + segs[i].Run.span()
		}
	}
	return segs
}

// seriesStripWidth is the pixel budget the measured runs of one strip
// share, split by day count so the x-axis stays "measured days" across
// segments; a run is never narrower than seriesSegmentMinW.
const (
	seriesStripWidth  = 280
	seriesSegmentMinW = 20
)

// renderSeriesStrip draws one dimension's strip: a kit Sparkline per
// contiguous run of measured days and a dashed chip per absent run, in
// calendar order. Each segment scales to its own run (the kit Sparkline
// carries no shared axis), which the widget's note states; what the strip
// guarantees is that no stroke ever spans an absent day.
func renderSeriesStrip(d dimSeries) string {
	segs := seriesSegments(d)
	var b strings.Builder
	b.WriteString(`<div class="mt-1 flex flex-wrap items-center gap-2" role="group" aria-label="` + html.EscapeString(fmt.Sprintf("%s — %d-day strip: %s, %d absent", d.Name, len(d.Points), plural(d.Measured, "measured day"), d.Absent)) + `">`)
	for _, seg := range segs {
		if seg.Values == nil {
			b.WriteString(`<span class="shrink-0 rounded-md border border-dashed border-line px-2 py-1 text-xs italic text-ink-muted" title="` + html.EscapeString(seg.Reason) + `" aria-label="` + html.EscapeString("absent — "+seg.Reason) + `">` + html.EscapeString(seg.Run.label()) + `</span>`)
			continue
		}
		w := seriesSegmentMinW
		if d.Measured > 0 {
			if pw := seriesStripWidth * seg.Run.N / d.Measured; pw > w {
				w = pw
			}
		}
		b.WriteString(`<span class="shrink-0">` + viz.Sparkline(seg.Values, viz.SparklineOptions{
			W: w, H: 32, FillRole: "accent-soft",
			AriaLabel: fmt.Sprintf("%s — %s, %s", d.Name, seg.Run.span(), plural(seg.Run.N, "measured day")),
		}) + `</span>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

// ─── RG-7: the grade distribution ────────────────────────────────────────

// gradeCount is one letter's count on one day.
type gradeCount struct {
	Grade string `json:"grade"`
	Count int    `json:"count"`
}

// gradeDay is one day of the fleet distribution. Counts nil = absent (no
// snapshot for that date), with Reason saying so.
type gradeDay struct {
	Date   string       `json:"date"`
	Counts []gradeCount `json:"counts,omitempty"`
	// Total is omitted on an absent day (a measured day always graded at
	// least one repository, so 0 here only ever means "not measured").
	Total  int    `json:"total,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// gradeSeries is the fleet's letter distribution over the window.
type gradeSeries struct {
	Days     []gradeDay `json:"days"`
	Measured int        `json:"measured_days"`
	Absent   int        `json:"absent_days"`
	// Note states, in words, how much of the window is measured and why
	// the rest is not — never a silent gap.
	Note string `json:"note"`
}

// gradeLadder is the render order for letters; anything else sorts after,
// alphabetically.
//
//fathom:tenancy-allow pkg-map read-only lookup table of constants, never written after init: not a cache
var gradeLadder = map[string]int{"A": 0, "B": 1, "C": 2, "D": 3, "F": 4, "idle": 5}

func gradeRank(letter string) int {
	if r, ok := gradeLadder[letter]; ok {
		return r
	}
	return len(gradeLadder)
}

// gradeLetterOf folds "B — not preventing" to "B": the distribution counts
// letters; the first-gap suffix belongs to the service page.
func gradeLetterOf(grade string) string {
	letter, _, _ := strings.Cut(grade, " — ")
	return strings.TrimSpace(letter)
}

func lessGrade(a, b string) bool {
	ra, rb := gradeRank(a), gradeRank(b)
	if ra != rb {
		return ra < rb
	}
	return a < b
}

// gradeDistributionSeries folds per-day grade lists (date → the fleet's
// grades that day) into a per-day distribution over the last `days` days.
// A date with no entry is Absent with a reason; an entry outside the window
// is ignored. Pure over (gradesByDay, now, days).
func gradeDistributionSeries(gradesByDay map[string][]string, now time.Time, days int) gradeSeries {
	dates := windowDates(now, days)
	gs := gradeSeries{Days: make([]gradeDay, 0, len(dates))}
	for _, d := range dates {
		grades, ok := gradesByDay[d]
		if !ok || len(grades) == 0 {
			gs.Days = append(gs.Days, gradeDay{Date: d, Reason: "no fleet-grade snapshot for " + d})
			gs.Absent++
			continue
		}
		byLetter := map[string]int{}
		total := 0
		for _, g := range grades {
			letter := gradeLetterOf(g)
			if letter == "" {
				continue
			}
			byLetter[letter]++
			total++
		}
		if total == 0 {
			gs.Days = append(gs.Days, gradeDay{Date: d, Reason: "no fleet-grade snapshot for " + d})
			gs.Absent++
			continue
		}
		day := gradeDay{Date: d, Total: total}
		for letter, n := range byLetter {
			day.Counts = append(day.Counts, gradeCount{Grade: letter, Count: n})
		}
		sort.Slice(day.Counts, func(i, j int) bool { return lessGrade(day.Counts[i].Grade, day.Counts[j].Grade) })
		gs.Days = append(gs.Days, day)
		gs.Measured++
	}
	switch {
	case gs.Measured == 0:
		gs.Note = fmt.Sprintf("no fleet-grade snapshot on any of the last %d days — nothing folded, nothing drawn", len(dates))
	default:
		latest := ""
		for i := len(gs.Days) - 1; i >= 0; i-- {
			if gs.Days[i].Counts != nil {
				latest = gs.Days[i].Date
				break
			}
		}
		gs.Note = fmt.Sprintf("%d of %d days measured; %d have no fleet-grade snapshot — the fleet page does not persist daily grade snapshots yet, so history begins with the most recent fold (%s) and accrues from here; it is never back-filled", gs.Measured, len(dates), gs.Absent, latest)
	}
	return gs
}

// Letters lists every letter seen on a measured day, ladder order.
func (gs gradeSeries) Letters() []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range gs.Days {
		for _, c := range d.Counts {
			if !seen[c.Grade] {
				seen[c.Grade] = true
				out = append(out, c.Grade)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return lessGrade(out[i], out[j]) })
	return out
}

// latestMeasured returns the most recent day that carries counts.
func (gs gradeSeries) latestMeasured() (gradeDay, bool) {
	for i := len(gs.Days) - 1; i >= 0; i-- {
		if gs.Days[i].Counts != nil {
			return gs.Days[i], true
		}
	}
	return gradeDay{}, false
}

// gradeLetterRole maps a letter to its kit role — the same mapping the
// fleet chips use, so the chart and the tiles never disagree.
func gradeLetterRole(letter string) string {
	switch letter {
	case "A", "B":
		return "positive"
	case "C", "D":
		return "caution"
	case "F":
		return "danger"
	default:
		return "ink-muted"
	}
}

// ─── absence runs ────────────────────────────────────────────────────────

// dateRun is a contiguous run of absent dates — rendered as ONE
// stat.Absent tile naming its span rather than thirty identical tiles.
type dateRun struct {
	From, To string
	N        int
}

// span renders "2026-08-24" or "2026-07-27 to 2026-08-22 (27 days)".
func (r dateRun) span() string {
	if r.N == 1 {
		return r.From
	}
	return fmt.Sprintf("%s to %s (%d days)", r.From, r.To, r.N)
}

// label renders the short tile label: "Aug 24" or "Jul 27 – Aug 22".
func (r dateRun) label() string {
	if r.N == 1 {
		return shortDate(r.From)
	}
	return shortDate(r.From) + " – " + shortDate(r.To)
}

func shortDate(d string) string {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	return t.Format("Jan 2")
}

// absentRuns collapses consecutive absent dates into runs. dates are the
// window's dates in order; absent reports whether index i is absent.
func absentRuns(dates []string, absent func(i int) bool) []dateRun {
	var runs []dateRun
	for i, d := range dates {
		if !absent(i) {
			continue
		}
		if n := len(runs); n > 0 && runs[n-1].To == dates[i-1] {
			runs[n-1].To = d
			runs[n-1].N++
			continue
		}
		runs = append(runs, dateRun{From: d, To: d, N: 1})
	}
	return runs
}

func seriesAbsentNote(s string) string {
	return `<span class="italic text-ink-muted">` + html.EscapeString(s) + `</span>`
}

// ─── rendering ───────────────────────────────────────────────────────────

// renderDimensionSeries is the service page's "activity over time" widget:
// one kit stat tile per dimension (Absent when no day was measured) with a
// strip of sparkline segments — one per run of measured days, a chip per
// run of absent days — then one Absent tile per absent run naming its
// dates. callLogNote is the model's CallLogNote: when the log itself could
// not be read no series exists, and the widget says why instead of
// pretending thirty days of "no rows".
func renderDimensionSeries(s obsServiceRow, callLogNote string) string {
	title := fmt.Sprintf("activity — last %d days, per dimension (call log)", seriesDays)
	if len(s.Series) == 0 {
		reason := "no 30-day series was built for this service — the call log was not folded into this model"
		if callLogNote != "" {
			reason = "no 30-day series — " + callLogNote
		}
		return dashboard.RenderWidget(dashboard.Widget{
			Title: title, Span: 2,
			Body: template.HTML(seriesAbsentNote(reason)),
		})
	}
	var body strings.Builder
	body.WriteString(`<div class="grid grid-cols-1 gap-3 md:grid-cols-2">`)
	for _, d := range s.Series {
		body.WriteString(`<div>`)
		if d.Measured == 0 || d.Total == nil {
			body.WriteString(stat.Render(stat.Absent(d.Name, fmt.Sprintf("no call-log rows on any of the last %d days — %s", len(d.Points), d.What))))
		} else {
			body.WriteString(stat.Render(stat.Stat{
				Label: d.Name,
				Value: fmtThousands(*d.Total),
				Sub:   fmt.Sprintf("over %s · %d absent — %s", plural(d.Measured, "measured day"), d.Absent, d.What),
			}))
			body.WriteString(renderSeriesStrip(d))
		}
		body.WriteString(`</div>`)
	}
	body.WriteString(`</div>`)

	// The absence criterion is the log itself, so the runs are the same for
	// every dimension: list them once.
	first := s.Series[0]
	dates := make([]string, len(first.Points))
	for i, p := range first.Points {
		dates[i] = p.Date
	}
	runs := absentRuns(dates, func(i int) bool { return first.Points[i].Value == nil })
	if len(runs) > 0 {
		note := "days with no call-log rows are drawn as absent, not zero — the daemon may not have been observing; no sparkline is drawn because nothing was measured"
		if first.Measured > 0 {
			note = "days with no call-log rows are drawn as absent, not zero — the daemon may not have been observing; each sparkline segment covers one unbroken run of measured days and is scaled to that run alone, a dashed chip marks each gap, and no line is ever drawn across one"
		}
		body.WriteString(`<p class="mt-3 text-xs text-ink-muted">` + html.EscapeString(note) + `</p>`)
		body.WriteString(`<div class="mt-2 flex flex-wrap gap-2">`)
		for _, r := range runs {
			body.WriteString(stat.Render(stat.Absent(r.label(), "no call-log rows for "+r.span())))
		}
		body.WriteString(`</div>`)
	}
	return dashboard.RenderWidget(dashboard.Widget{Title: title, Span: 2, Body: template.HTML(body.String())})
}

// renderGradeDistribution is the fleet page's scoreboard: today's letter
// distribution as kit Bars (only letters some repository carries — a
// letter nobody holds is not a zero bar), per-letter sparklines once more
// than one day is measured, and one Absent tile per run of snapshot-less
// days. The clock is the model's own generated_at, so HTML and JSON views
// of one model agree.
func renderGradeDistribution(m observatoryModel) string {
	var b strings.Builder
	b.WriteString(sectionHeading(fmt.Sprintf("grade distribution — last %d days (the phase-U scoreboard)", seriesDays)))
	now, err := time.Parse(time.RFC3339, m.GeneratedAt)
	if err != nil {
		b.WriteString(dashboard.RenderWidget(dashboard.Widget{
			Body: template.HTML(seriesAbsentNote("grade history not rendered — the model's generated_at (" + m.GeneratedAt + ") is not a clock the series can anchor to")),
		}))
		return b.String()
	}
	var grades []string
	noLadder := 0
	for _, s := range m.Services {
		if s.Realization == nil {
			noLadder++
			continue
		}
		grades = append(grades, s.Realization.Grade)
	}
	byDay := map[string][]string{}
	if len(grades) > 0 {
		byDay[now.UTC().Format("2006-01-02")] = grades
	}
	gs := gradeDistributionSeries(byDay, now, seriesDays)

	var body strings.Builder
	if latest, ok := gs.latestMeasured(); ok {
		bars := make([]viz.Bar, 0, len(latest.Counts))
		for _, c := range latest.Counts {
			bars = append(bars, viz.Bar{Label: c.Grade, Value: float64(c.Count), Role: gradeLetterRole(c.Grade)})
		}
		// A compact distribution fits the 256px content column at a 320px
		// viewport. Set intrinsic geometry rather than shrinking SVG text with CSS.
		body.WriteString(viz.Bars(bars, viz.BarsOptions{W: 256, LabelW: 60, ShowValue: true, AriaLabel: "grade distribution on " + latest.Date}))
		body.WriteString(`<p class="mt-1 text-xs text-ink-muted">` + html.EscapeString(fmt.Sprintf("%s graded on %s", plural(latest.Total, "repo"), latest.Date)) + `</p>`)
	}
	if gs.Measured > 1 {
		body.WriteString(`<div class="mt-3 flex flex-wrap gap-2">`)
		for _, letter := range gs.Letters() {
			var vals []float64
			last := 0
			for _, d := range gs.Days {
				if d.Counts == nil {
					continue
				}
				n := 0
				for _, c := range d.Counts {
					if c.Grade == letter {
						n = c.Count
					}
				}
				vals = append(vals, float64(n))
				last = n
			}
			body.WriteString(stat.Render(stat.Stat{
				Label: letter, Value: fmtThousands(last),
				Sub:   "over " + plural(gs.Measured, "measured day"),
				Spark: vals, SparkRole: gradeLetterRole(letter),
			}))
		}
		body.WriteString(`</div>`)
	}
	dates := make([]string, len(gs.Days))
	for i, d := range gs.Days {
		dates[i] = d.Date
	}
	if runs := absentRuns(dates, func(i int) bool { return gs.Days[i].Counts == nil }); len(runs) > 0 {
		body.WriteString(`<div class="mt-3 flex flex-wrap gap-2">`)
		for _, r := range runs {
			body.WriteString(stat.Render(stat.Absent(r.label(), "no fleet-grade snapshot for "+r.span())))
		}
		body.WriteString(`</div>`)
	}
	note := gs.Note
	if noLadder > 0 {
		if noLadder == 1 {
			note += " · 1 repository has no ladder (realization not folded) and is not counted"
		} else {
			note += fmt.Sprintf(" · %d repositories have no ladder (realization not folded) and are not counted", noLadder)
		}
	}
	body.WriteString(`<p class="mt-3 text-xs text-ink-muted">` + html.EscapeString(note) + `</p>`)
	b.WriteString(dashboard.RenderWidget(dashboard.Widget{Body: template.HTML(body.String())}))
	return b.String()
}
