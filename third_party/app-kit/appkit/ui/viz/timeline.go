package viz

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Event is a point-in-time occurrence plotted as a dot on the axis.
type Event struct {
	At    time.Time
	Label string // shown in the dot's <title> (date is appended)
	Role  string // token color role for the dot
	Lane  string // optional swimlane group; when any event/span sets it, the
	// timeline renders one labeled row per distinct lane (see TimelineOptions).
}

// Span is a duration (Start→End) drawn as a bar. Without lanes it is packed into
// stacked rows above the axis; with lanes it sits on its lane's baseline.
type Span struct {
	Start time.Time
	End   time.Time
	Label string
	Role  string
	Lane  string // optional swimlane group (see Event.Lane)
}

// Marker is a key reference instant — a deadline, a hearing, "today" — drawn as
// a vertical line across the chart with a label at the top.
type Marker struct {
	At    time.Time
	Label string
	Role  string
}

// TimelineView selects how time maps to horizontal position.
type TimelineView string

const (
	// ViewProportional (default) maps time to x linearly: distances are true to
	// elapsed time. Honest, but long empty stretches waste space.
	ViewProportional TimelineView = ""
	// ViewSequence spaces the distinct instants evenly, ignoring how much time
	// separates them — best for reading order when magnitudes don't matter.
	ViewSequence TimelineView = "sequence"
	// ViewCondensed is proportional, but large empty gaps are collapsed to a
	// fixed-width break that is *labeled with the omitted duration* — so the
	// timeline condenses without lying. Populated regions stay relative.
	ViewCondensed TimelineView = "condensed"
	// ViewDensity bins the range into equal-time cells and shades each by event
	// count (a heatmap strip) — for spotting where activity concentrates over a
	// long range. With lanes, one heatmap row per lane. Spans are ignored.
	ViewDensity TimelineView = "density"
)

// TimelineOptions configures a Timeline render. Events are the primary input;
// Spans and Markers are overlays. The time range is computed from all of them,
// unless Start/End pin it explicitly.
type TimelineOptions struct {
	// W is the SVG width. Default 560. Height is computed from the content.
	W int
	// View selects the time→x mapping (default proportional).
	View TimelineView
	// AriaLabel is the accessible label.
	AriaLabel string
	// Start / End pin the axis range; zero values mean "derive from the data".
	Start time.Time
	End   time.Time
	// Spans and Markers are drawn alongside the events.
	Spans   []Span
	Markers []Marker
	// CondenseFraction is the share of the total range a single empty gap must
	// exceed to be elided in ViewCondensed. Default 0.15 (15%). Ignored in other
	// views.
	CondenseFraction float64
	// DensityBuckets sets the number of heatmap cells in ViewDensity. Default is
	// derived from the width. Ignored in other views.
	DensityBuckets int
}

func (o *TimelineOptions) withDefaults() TimelineOptions {
	out := *o
	if out.W <= 0 {
		out.W = 560
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "timeline"
	}
	if out.CondenseFraction <= 0 {
		out.CondenseFraction = 0.15
	}
	return out
}

const (
	tlPadX       = 14.0 // left/right inset for the axis
	tlLaneH      = 14.0 // span bar height
	tlLaneGap    = 3.0
	tlMarkerTopH = 16.0 // room for marker labels above the spans
	tlAxisGap    = 12.0 // gap between spans and the axis
	tlDotR       = 4.0
	tlLabelH     = 16.0 // room for the axis date labels below
	tlBreakW     = 30.0 // fixed width of an elided gap in ViewCondensed
	tlGutterW    = 88.0 // left gutter for swimlane labels
	tlRowH       = 24.0 // height of one swimlane row
)

// Timeline renders a horizontal timeline as an SVG string.
//
// Events plot as dots, Spans as rounded bars, and Markers as vertical reference
// lines. The View controls the time→x mapping (proportional / sequence /
// condensed). If any event or span sets a Lane, the timeline becomes a
// swimlane: one labeled row per distinct lane, sharing the time axis. A
// zero-width range (all times equal, or a single instant) places everything at
// the center without dividing by zero. Output is deterministic; caller strings
// are escaped; color roles are validated (invalid → ink-muted).
func Timeline(events []Event, o TimelineOptions) string {
	opts := o.withDefaults()
	w := float64(opts.W)

	lanes := timelineLanes(events, opts.Spans)
	gutterW := 0.0
	if len(lanes) > 0 {
		gutterW = tlGutterW
	}
	plotX0 := tlPadX + gutterW
	plotX1 := w - tlPadX
	scale := buildScale(events, opts, plotX0, plotX1-plotX0)

	if opts.View == ViewDensity {
		return renderDensity(events, opts, lanes, plotX0, plotX1)
	}
	if len(lanes) > 0 {
		return renderSwimlanes(events, opts, scale, lanes, plotX0, plotX1)
	}
	return renderSingleAxis(events, opts, scale, plotX0, plotX1)
}

// renderSingleAxis is the default layout: spans packed into stacked rows above
// one shared axis, events as dots on it.
func renderSingleAxis(events []Event, opts TimelineOptions, scale timeScale, x0, x1 float64) string {
	lanes, numLanes := packSpans(opts.Spans)
	topH := 0.0
	if len(opts.Markers) > 0 {
		topH = tlMarkerTopH
	}
	spansTop := topH
	spansBottom := spansTop + float64(numLanes)*(tlLaneH+tlLaneGap)
	axisY := spansBottom + tlAxisGap
	totalH := int(axisY + tlLabelH)

	var b strings.Builder
	b.WriteString(svgOpen(opts.W, totalH, opts.AriaLabel, ""))
	b.WriteString(svgTitle(opts.AriaLabel))

	for i, span := range opts.Spans {
		y := spansTop + float64(lanes[i])*(tlLaneH+tlLaneGap)
		drawSpanBar(&b, scale, span, y, tlLaneH, x1, true)
	}

	drawAxis(&b, scale, axisY, x0, x1)
	drawMarkers(&b, opts.Markers, scale, topH, axisY+5, x0, x1)

	for _, e := range events {
		drawEventDot(&b, scale.at(e.At), axisY, e)
	}

	drawTimeFooter(&b, scale, axisY, x0, x1)
	b.WriteString("</svg>")
	return b.String()
}

// renderSwimlanes draws one labeled row per lane, all sharing the time axis at
// the bottom. Events sit on their lane's baseline; spans are bars on it.
func renderSwimlanes(events []Event, opts TimelineOptions, scale timeScale, lanes []string, x0, x1 float64) string {
	topH := tlMarkerTopH // always reserve the marker-label strip for alignment
	rowsTop := topH
	axisY := rowsTop + float64(len(lanes))*tlRowH + tlAxisGap - tlLaneGap
	totalH := int(axisY + tlLabelH)

	laneIndex := map[string]int{}
	for i, l := range lanes {
		laneIndex[l] = i
	}
	baseline := func(lane string) float64 {
		return rowsTop + float64(laneIndex[lane])*tlRowH + tlRowH/2
	}

	var b strings.Builder
	b.WriteString(svgOpen(opts.W, totalH, opts.AriaLabel, ""))
	b.WriteString(svgTitle(opts.AriaLabel))

	// Row guides + lane labels.
	for i, lane := range lanes {
		y := rowsTop + float64(i)*tlRowH + tlRowH/2
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1"/>`,
			fmtNum(x0), fmtNum(y), fmtNum(x1), fmtNum(y), cssColor("line"))
		if lane != "" {
			fmt.Fprintf(&b,
				`<text x="%s" y="%s" dominant-baseline="middle" fill="%s" font-size="10">%s</text>`,
				fmtNum(tlPadX), fmtNum(y), cssColor("ink-muted"), esc(lane))
		}
	}

	// Spans on their lane baseline (bar centered on the row guide).
	for _, span := range opts.Spans {
		y := baseline(span.Lane) - tlLaneH/2
		drawSpanBar(&b, scale, span, y, tlLaneH, x1, false)
	}

	// Markers span all rows.
	drawMarkers(&b, opts.Markers, scale, topH, axisY+5, x0, x1)

	// Event dots on their lane baseline.
	for _, e := range events {
		drawEventDot(&b, scale.at(e.At), baseline(e.Lane), e)
	}

	drawAxis(&b, scale, axisY, x0, x1)
	drawTimeFooter(&b, scale, axisY, x0, x1)
	b.WriteString("</svg>")
	return b.String()
}

// renderDensity bins the range into equal-time cells and shades each by event
// count — a heatmap. With lanes, one heatmap row per lane (counts normalized
// against the busiest cell across all rows, so rows are comparable). Spans are
// ignored; markers and the range labels are kept for orientation.
func renderDensity(events []Event, opts TimelineOptions, lanes []string, x0, x1 float64) string {
	_, min, max, have := gatherAnchors(events, opts)
	rows := lanes
	if len(rows) == 0 {
		rows = []string{""} // single strip
	}
	plotW := x1 - x0

	nBuckets := opts.DensityBuckets
	if nBuckets <= 0 {
		nBuckets = int(plotW / 16)
		if nBuckets < 8 {
			nBuckets = 8
		}
		if nBuckets > 64 {
			nBuckets = 64
		}
	}

	topH := tlMarkerTopH
	rowsTop := topH
	axisY := rowsTop + float64(len(rows))*tlRowH + tlAxisGap - tlLaneGap
	totalH := int(axisY + tlLabelH)

	var b strings.Builder
	b.WriteString(svgOpen(opts.W, totalH, opts.AriaLabel, ""))
	b.WriteString(svgTitle(opts.AriaLabel))

	if !have {
		b.WriteString("</svg>")
		return b.String()
	}

	// bucketOf maps a time to its cell index; range collapses to bucket 0.
	span := max.Sub(min).Seconds()
	bucketOf := func(t time.Time) int {
		if span <= 0 {
			return 0
		}
		i := int(t.Sub(min).Seconds() / span * float64(nBuckets))
		if i < 0 {
			i = 0
		}
		if i >= nBuckets {
			i = nBuckets - 1
		}
		return i
	}

	// Count events per (row, bucket); track the global peak for normalization.
	laneIndex := map[string]int{}
	for i, l := range rows {
		laneIndex[l] = i
	}
	counts := make([][]int, len(rows))
	for i := range counts {
		counts[i] = make([]int, nBuckets)
	}
	maxCount := 0
	for _, e := range events {
		ri, ok := laneIndex[e.Lane]
		if !ok { // event in a lane with no row (shouldn't happen) — skip
			continue
		}
		bi := bucketOf(e.At)
		counts[ri][bi]++
		if counts[ri][bi] > maxCount {
			maxCount = counts[ri][bi]
		}
	}

	cellW := plotW / float64(nBuckets)
	cellH := tlRowH - 8
	accent := cssColor("accent")
	track := cssColor("surface-sunken")
	bucketDur := time.Duration(span/float64(nBuckets)) * time.Second

	for ri, lane := range rows {
		rowY := rowsTop + float64(ri)*tlRowH + (tlRowH-cellH)/2
		if lane != "" {
			fmt.Fprintf(&b,
				`<text x="%s" y="%s" dominant-baseline="middle" fill="%s" font-size="10">%s</text>`,
				fmtNum(tlPadX), fmtNum(rowY+cellH/2), cssColor("ink-muted"), esc(lane))
		}
		for bi := 0; bi < nBuckets; bi++ {
			cx := x0 + float64(bi)*cellW
			c := counts[ri][bi]
			bStart := min.Add(time.Duration(bi) * bucketDur)
			bEnd := bStart.Add(bucketDur)
			if c == 0 {
				fmt.Fprintf(&b,
					`<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s" fill-opacity="0.35"/>`,
					fmtNum(cx), fmtNum(rowY), fmtNum(cellW-1.5), fmtNum(cellH), track)
				continue
			}
			op := 0.25 + 0.75*float64(c)/float64(maxCount)
			fmt.Fprintf(&b,
				`<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s" fill-opacity="%s"><title>%s – %s: %d</title></rect>`,
				fmtNum(cx), fmtNum(rowY), fmtNum(cellW-1.5), fmtNum(cellH), accent, fmtNum(op),
				esc(fmtDate(bStart)), esc(fmtDate(bEnd)), c)
		}
	}

	// Markers (linear position) + range labels.
	densityX := func(t time.Time) float64 {
		if span <= 0 {
			return x0 + plotW/2
		}
		return x0 + t.Sub(min).Seconds()/span*plotW
	}
	for _, m := range opts.Markers {
		mx := densityX(m.At)
		color := cssColor(m.Role)
		fmt.Fprintf(&b,
			`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1" stroke-dasharray="3 2"/>`,
			fmtNum(mx), fmtNum(topH), fmtNum(mx), fmtNum(axisY), color)
		anchor, lx := markerAnchor(mx, x0, x1)
		fmt.Fprintf(&b, `<text x="%s" y="11" text-anchor="%s" fill="%s" font-size="10">%s</text>`,
			fmtNum(lx), anchor, color, esc(m.Label))
	}
	labelY := axisY + 13
	fmt.Fprintf(&b, `<text x="%s" y="%s" fill="%s" font-size="10">%s</text>`,
		fmtNum(x0), fmtNum(labelY), cssColor("ink-faint"), esc(fmtDate(min)))
	if !max.Equal(min) {
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" fill="%s" font-size="10">%s</text>`,
			fmtNum(x1), fmtNum(labelY), cssColor("ink-faint"), esc(fmtDate(max)))
	}

	b.WriteString("</svg>")
	return b.String()
}

// timelineLanes returns the distinct lane keys in first-appearance order across
// events then spans. Returns nil when nothing sets a lane (single-axis mode).
// When some items are laned and others aren't, the unlabeled "" lane is kept in
// its natural position so nothing is dropped.
func timelineLanes(events []Event, spans []Span) []string {
	var order []string
	seen := map[string]bool{}
	any := false
	consider := func(lane string) {
		if lane != "" {
			any = true
		}
		if !seen[lane] {
			seen[lane] = true
			order = append(order, lane)
		}
	}
	for _, e := range events {
		consider(e.Lane)
	}
	for _, s := range spans {
		consider(s.Lane)
	}
	if !any {
		return nil
	}
	return order
}

// drawSpanBar renders a span as a rounded bar; rightLabel places the label to
// the right of the bar (single-axis mode) when there's room.
func drawSpanBar(b *strings.Builder, scale timeScale, span Span, y, h, x1 float64, rightLabel bool) {
	a, z := scale.at(span.Start), scale.at(span.End)
	if z < a {
		a, z = z, a
	}
	bw := z - a
	if bw < 2 {
		bw = 2
	}
	fmt.Fprintf(b,
		`<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s"><title>%s (%s – %s)</title></rect>`,
		fmtNum(a), fmtNum(y), fmtNum(bw), fmtNum(h), cssColor(span.Role),
		esc(span.Label), esc(fmtDate(span.Start)), esc(fmtDate(span.End)))
	if rightLabel && z+4 < x1 {
		fmt.Fprintf(b,
			`<text x="%s" y="%s" dominant-baseline="middle" fill="%s" font-size="10">%s</text>`,
			fmtNum(z+4), fmtNum(y+h/2), cssColor("ink-muted"), esc(span.Label))
	}
}

// drawEventDot renders one event as a token-colored dot with a surface ring.
func drawEventDot(b *strings.Builder, cx, cy float64, e Event) {
	fmt.Fprintf(b,
		`<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="1.5"><title>%s (%s)</title></circle>`,
		fmtNum(cx), fmtNum(cy), fmtNum(tlDotR), cssColor(e.Role), cssColor("surface"),
		esc(e.Label), esc(fmtDate(e.At)))
}

// drawMarkers renders each marker as a vertical reference line from top to
// bottomY with a label in the top strip.
func drawMarkers(b *strings.Builder, markers []Marker, scale timeScale, topH, bottomY, x0, x1 float64) {
	for _, m := range markers {
		mx := scale.at(m.At)
		color := cssColor(m.Role)
		fmt.Fprintf(b,
			`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1" stroke-dasharray="3 2"/>`,
			fmtNum(mx), fmtNum(topH), fmtNum(mx), fmtNum(bottomY), color)
		anchor, lx := markerAnchor(mx, x0, x1)
		fmt.Fprintf(b,
			`<text x="%s" y="11" text-anchor="%s" fill="%s" font-size="10">%s</text>`,
			fmtNum(lx), anchor, color, esc(m.Label))
	}
}

// drawTimeFooter renders the range date labels and any condensed-gap labels
// below the axis.
func drawTimeFooter(b *strings.Builder, scale timeScale, axisY, x0, x1 float64) {
	if !scale.haveRange {
		return
	}
	labelY := axisY + 13
	fmt.Fprintf(b, `<text x="%s" y="%s" fill="%s" font-size="10">%s</text>`,
		fmtNum(x0), fmtNum(labelY), cssColor("ink-faint"), esc(fmtDate(scale.min)))
	if !scale.max.Equal(scale.min) {
		fmt.Fprintf(b, `<text x="%s" y="%s" text-anchor="end" fill="%s" font-size="10">%s</text>`,
			fmtNum(x1), fmtNum(labelY), cssColor("ink-faint"), esc(fmtDate(scale.max)))
	}
	drawBreakLabels(b, scale, axisY)
}

// ---- piecewise time scale -------------------------------------------------

// scalePoint is a control point: time t maps to coordinate x. The scale is
// piecewise-linear between consecutive points.
type scalePoint struct {
	t time.Time
	x float64
}

// breakSeg records an elided gap (ViewCondensed) for glyph + label rendering.
type breakSeg struct {
	x0, x1   float64
	from, to time.Time
}

type timeScale struct {
	pts       []scalePoint
	breaks    []breakSeg
	min, max  time.Time
	haveRange bool
	centerX   float64 // used when there is no real range
}

// at maps an instant to an x coordinate, clamping to the scale's ends and
// interpolating linearly within the containing segment.
func (s timeScale) at(t time.Time) float64 {
	if !s.haveRange || len(s.pts) == 0 {
		return s.centerX
	}
	if len(s.pts) == 1 {
		return s.pts[0].x
	}
	if !t.After(s.pts[0].t) {
		return s.pts[0].x
	}
	last := s.pts[len(s.pts)-1]
	if !t.Before(last.t) {
		return last.x
	}
	for i := 0; i+1 < len(s.pts); i++ {
		a, b := s.pts[i], s.pts[i+1]
		if !t.Before(a.t) && !t.After(b.t) {
			span := b.t.Sub(a.t).Seconds()
			if span <= 0 {
				return a.x
			}
			frac := t.Sub(a.t).Seconds() / span
			return a.x + frac*(b.x-a.x)
		}
	}
	return last.x
}

// buildScale constructs the time→x scale for the chosen view from the distinct
// instants present in the data.
func buildScale(events []Event, o TimelineOptions, plotX0, plotW float64) timeScale {
	anchors, min, max, have := gatherAnchors(events, o)
	s := timeScale{min: min, max: max, haveRange: have, centerX: plotX0 + plotW/2}
	if !have || len(anchors) == 1 {
		if have {
			s.pts = []scalePoint{{t: min, x: s.centerX}}
		}
		return s
	}

	switch o.View {
	case ViewSequence:
		n := len(anchors)
		s.pts = make([]scalePoint, n)
		for i, t := range anchors {
			s.pts[i] = scalePoint{t: t, x: plotX0 + plotW*float64(i)/float64(n-1)}
		}
	case ViewCondensed:
		s.pts, s.breaks = condensedPoints(anchors, o.Spans, max.Sub(min), o.CondenseFraction, plotX0, plotW)
	default: // ViewProportional
		total := max.Sub(min).Seconds()
		s.pts = make([]scalePoint, len(anchors))
		for i, t := range anchors {
			s.pts[i] = scalePoint{t: t, x: plotX0 + plotW*(t.Sub(min).Seconds()/total)}
		}
	}
	return s
}

// condensedPoints builds proportional control points but replaces each empty
// segment longer than condenseFraction of the total span with a fixed-width
// break. Non-break segments share the remaining width in proportion to their
// real duration, so populated regions stay relative to one another.
func condensedPoints(anchors []time.Time, spans []Span, total time.Duration, frac, plotX0, plotW float64) ([]scalePoint, []breakSeg) {
	n := len(anchors)
	segDur := make([]float64, n-1)
	isBreak := make([]bool, n-1)
	threshold := frac * total.Seconds()
	var keepSum, breakCount float64
	for i := 0; i+1 < n; i++ {
		d := anchors[i+1].Sub(anchors[i]).Seconds()
		segDur[i] = d
		// Elide only long stretches that are genuinely empty — never one a span
		// covers (the bar would otherwise straddle an "omitted" break).
		if total.Seconds() > 0 && d > threshold && !spanCovers(spans, anchors[i], anchors[i+1]) {
			isBreak[i] = true
			breakCount++
		} else {
			keepSum += d
		}
	}
	// Width budget: fixed slice per break, the rest shared by kept segments.
	breakBudget := breakCount * tlBreakW
	if breakBudget > plotW*0.6 { // never let breaks dominate
		breakBudget = plotW * 0.6
	}
	perBreak := 0.0
	if breakCount > 0 {
		perBreak = breakBudget / breakCount
	}
	keepWidth := plotW - perBreak*breakCount
	if keepWidth < 0 {
		keepWidth = 0
	}

	pts := make([]scalePoint, n)
	var breaks []breakSeg
	x := plotX0
	pts[0] = scalePoint{t: anchors[0], x: x}
	for i := 0; i+1 < n; i++ {
		var segW float64
		if isBreak[i] {
			segW = perBreak
			breaks = append(breaks, breakSeg{x0: x, x1: x + segW, from: anchors[i], to: anchors[i+1]})
		} else if keepSum > 0 {
			segW = keepWidth * (segDur[i] / keepSum)
		} else {
			segW = keepWidth / float64(n-1) // all-equal fallback
		}
		x += segW
		pts[i+1] = scalePoint{t: anchors[i+1], x: x}
	}
	return pts, breaks
}

// spanCovers reports whether any span overlaps the open interval (a, b) — used
// to keep span-covered stretches at true scale in the condensed view.
func spanCovers(spans []Span, a, b time.Time) bool {
	for _, s := range spans {
		start, end := s.Start, s.End
		if end.Before(start) {
			start, end = end, start
		}
		if start.Before(b) && end.After(a) {
			return true
		}
	}
	return false
}

// gatherAnchors returns the sorted, de-duplicated control instants the scale is
// built from. Anchors come from events and span endpoints only — the populated
// regions. Markers and Start/End extend the [min, max] range (so a "today" line
// or a pinned window is visible) but are NOT anchors, so a marker falling in an
// empty stretch renders inside a single clean break rather than splitting it.
func gatherAnchors(events []Event, o TimelineOptions) (anchors []time.Time, min, max time.Time, have bool) {
	var anchorTs, rangeTs []time.Time
	anchor := func(t time.Time) {
		if !t.IsZero() {
			anchorTs = append(anchorTs, t)
			rangeTs = append(rangeTs, t)
		}
	}
	rng := func(t time.Time) {
		if !t.IsZero() {
			rangeTs = append(rangeTs, t)
		}
	}
	for _, e := range events {
		anchor(e.At)
	}
	for _, s := range o.Spans {
		anchor(s.Start)
		anchor(s.End)
	}
	for _, m := range o.Markers {
		rng(m.At)
	}
	rng(o.Start)
	rng(o.End)
	if len(rangeTs) == 0 {
		return nil, time.Time{}, time.Time{}, false
	}

	sort.Slice(rangeTs, func(i, j int) bool { return rangeTs[i].Before(rangeTs[j]) })
	min, max = rangeTs[0], rangeTs[len(rangeTs)-1]
	if !o.Start.IsZero() {
		min = o.Start
	}
	if !o.End.IsZero() {
		max = o.End
	}

	// Anchors = event/span instants clamped to [min, max], with the range ends
	// guaranteed present so the scale spans the full axis.
	anchorTs = append(anchorTs, min, max)
	sort.Slice(anchorTs, func(i, j int) bool { return anchorTs[i].Before(anchorTs[j]) })
	for _, t := range anchorTs {
		if t.Before(min) || t.After(max) {
			continue
		}
		if len(anchors) == 0 || !t.Equal(anchors[len(anchors)-1]) {
			anchors = append(anchors, t)
		}
	}
	return anchors, min, max, true
}

// ---- rendering helpers ----------------------------------------------------

// drawAxis draws the baseline as one line per non-break segment, leaving a
// slashed gap where the condensed view elides time.
func drawAxis(b *strings.Builder, s timeScale, axisY, x0, x1 float64) {
	stroke := cssColor("line-strong")
	if len(s.breaks) == 0 || !s.haveRange {
		fmt.Fprintf(b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.5"/>`,
			fmtNum(x0), fmtNum(axisY), fmtNum(x1), fmtNum(axisY), stroke)
		return
	}
	// Draw axis up to each break, render the break glyph, continue after it.
	cursor := x0
	for _, br := range s.breaks {
		fmt.Fprintf(b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.5"/>`,
			fmtNum(cursor), fmtNum(axisY), fmtNum(br.x0), fmtNum(axisY), stroke)
		// Break glyph: two diagonal slashes straddling the gap.
		mid := (br.x0 + br.x1) / 2
		fmt.Fprintf(b,
			`<path d="M%s %s l4 -6 M%s %s l4 -6" stroke="%s" stroke-width="1.5" fill="none"/>`,
			fmtNum(mid-5), fmtNum(axisY+3), fmtNum(mid+1), fmtNum(axisY+3), stroke)
		cursor = br.x1
	}
	fmt.Fprintf(b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.5"/>`,
		fmtNum(cursor), fmtNum(axisY), fmtNum(x1), fmtNum(axisY), stroke)
}

// drawBreakLabels annotates each elided gap with the omitted duration, so the
// condensation is honest rather than silent.
func drawBreakLabels(b *strings.Builder, s timeScale, axisY float64) {
	for _, br := range s.breaks {
		mid := (br.x0 + br.x1) / 2
		fmt.Fprintf(b,
			`<text x="%s" y="%s" text-anchor="middle" fill="%s" font-size="9">%s<title>%s – %s omitted</title></text>`,
			fmtNum(mid), fmtNum(axisY-7), cssColor("ink-faint"), esc(humanDur(br.to.Sub(br.from))),
			esc(fmtDate(br.from)), esc(fmtDate(br.to)))
	}
}

// packSpans assigns each span to the lowest lane whose latest end is at or
// before the span's start, opening a new lane when none fits. Deterministic:
// spans process in input order, lanes fill from 0. Returns one lane index per
// span plus the lane count.
func packSpans(spans []Span) (lanes []int, count int) {
	lanes = make([]int, len(spans))
	var laneEnds []time.Time
	for i, s := range spans {
		start, end := s.Start, s.End
		if end.Before(start) {
			start, end = end, start
		}
		placed := false
		for l := range laneEnds {
			if !laneEnds[l].After(start) { // laneEnd <= start
				lanes[i] = l
				laneEnds[l] = end
				placed = true
				break
			}
		}
		if !placed {
			lanes[i] = len(laneEnds)
			laneEnds = append(laneEnds, end)
		}
	}
	return lanes, len(laneEnds)
}

// markerAnchor keeps a marker label inside the plot region [x0, x1]: anchored to
// the start near the left edge, the end near the right edge, centered otherwise.
func markerAnchor(mx, x0, x1 float64) (anchor string, lx float64) {
	span := x1 - x0
	switch {
	case mx < x0+span*0.15:
		return "start", x0
	case mx > x0+span*0.85:
		return "end", x1
	default:
		return "middle", mx
	}
}

// fmtDate formats an instant as a short, locale-independent, deterministic date.
func fmtDate(t time.Time) string {
	return t.Format("2 Jan 2006")
}

// humanDur renders a duration at the coarsest sensible unit, for gap labels.
func humanDur(d time.Duration) string {
	days := d.Hours() / 24
	switch {
	case days >= 547: // ~1.5 years
		return fmt.Sprintf("%.0f y", days/365.25)
	case days >= 46: // ~1.5 months
		return fmt.Sprintf("%.0f mo", days/30.44)
	case days >= 14:
		return fmt.Sprintf("%.0f w", days/7)
	default:
		return fmt.Sprintf("%.0f d", days)
	}
}
