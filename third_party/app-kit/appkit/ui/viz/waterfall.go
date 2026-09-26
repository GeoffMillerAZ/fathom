package viz

import (
	"fmt"
	"strings"
)

// WaterfallStep is one column of a Waterfall: a signed Delta (up/down), or an
// absolute running Total checkpoint.
type WaterfallStep struct {
	Label string
	Delta float64 // signed contribution; ignored when Total
	Total bool    // render as an absolute bar from 0 to the running total
	Role  string  // optional color override; default: positive (up) / danger (down) / accent (total)
}

// WaterfallOptions configures a Waterfall render.
type WaterfallOptions struct {
	W         int    // SVG width. Default 420.
	H         int    // plot height (bars). Default 140.
	Prefix    string // value-label prefix (e.g. "$")
	Suffix    string // value-label suffix (e.g. "%")
	AriaLabel string
	Fluid     bool // fill the container width: the W viewBox scales uniformly, text included. Default off (fixed pixels).
}

func (o *WaterfallOptions) withDefaults() WaterfallOptions {
	out := *o
	if out.W <= 0 {
		out.W = 420
	}
	if out.H <= 0 {
		out.H = 140
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "waterfall chart"
	}
	return out
}

// Waterfall renders a vertical waterfall: signed deltas stack into a running
// total, with optional absolute Total checkpoints. Up deltas are positive-role,
// down deltas danger-role, totals accent (unless Role overrides). Handles
// negative running totals; empty-safe; deterministic; escaped; roles validated.
func Waterfall(steps []WaterfallStep, o WaterfallOptions) string {
	opts := o.withDefaults()
	n := len(steps)

	const labelH, valueH, padX = 16.0, 14.0, 8.0
	plotH := float64(opts.H)
	totalH := int(plotH + labelH + valueH + 4)
	w := float64(opts.W)

	var b strings.Builder
	b.WriteString(chartOpen(opts.W, totalH, opts.AriaLabel, opts.Fluid))
	b.WriteString(svgTitle(opts.AriaLabel))
	if n == 0 {
		b.WriteString("</svg>")
		return b.String()
	}

	// Compute each column's [start,end] value and the running total.
	type seg struct {
		start, end, val float64
		total           bool
	}
	segs := make([]seg, n)
	running := 0.0
	vMin, vMax := 0.0, 0.0
	for i, s := range steps {
		if s.Total {
			segs[i] = seg{start: 0, end: running, val: running, total: true}
		} else {
			start := running
			running += s.Delta
			segs[i] = seg{start: start, end: running, val: s.Delta}
		}
		for _, v := range []float64{segs[i].start, segs[i].end} {
			if v < vMin {
				vMin = v
			}
			if v > vMax {
				vMax = v
			}
		}
	}
	span := vMax - vMin
	if span == 0 {
		span = 1
	}
	yTop := valueH
	yOf := func(v float64) float64 { return yTop + (vMax-v)/span*plotH }

	colW := (w - 2*padX) / float64(n)
	barW := colW * 0.6
	zeroY := yOf(0)

	// Zero baseline.
	fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1"/>`,
		fmtNum(padX), fmtNum(zeroY), fmtNum(w-padX), fmtNum(zeroY), cssColor("line"))

	for i, s := range steps {
		cx := padX + (float64(i)+0.5)*colW
		bx := cx - barW/2
		sg := segs[i]
		top := yOf(sg.end)
		bot := yOf(sg.start)
		if bot < top {
			top, bot = bot, top
		}
		bh := bot - top
		if bh < 2 {
			bh = 2
		}

		role := s.Role
		if role == "" {
			switch {
			case sg.total:
				role = "accent"
			case sg.val < 0:
				role = "danger"
			default:
				role = "positive"
			}
		}
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"><title>%s: %s%s%s</title></rect>`,
			fmtNum(bx), fmtNum(top), fmtNum(barW), fmtNum(bh), cssColor(role),
			esc(s.Label), esc(opts.Prefix), fmtNum(sg.val), esc(opts.Suffix))

		// Connector to the next column's start (running-total guide).
		if i+1 < n && !steps[i+1].Total {
			ny := yOf(sg.end)
			fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1" stroke-dasharray="2 2"/>`,
				fmtNum(bx+barW), fmtNum(ny), fmtNum(cx+colW-barW/2), fmtNum(ny), cssColor("line-strong"))
		}

		// Value label above the bar.
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="middle" fill="%s" font-size="10">%s%s%s</text>`,
			fmtNum(cx), fmtNum(top-3), cssColor("ink"), esc(opts.Prefix), fmtNum(sg.val), esc(opts.Suffix))

		// Column label below the plot.
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="middle" fill="%s" font-size="10">%s</text>`,
			fmtNum(cx), fmtNum(yTop+plotH+12), cssColor("ink-muted"), esc(s.Label))
	}

	b.WriteString("</svg>")
	return b.String()
}
