package viz

import (
	"fmt"
	"strings"
)

// RangeItem is one labeled low–high interval with an optional point marker (a
// most-likely / expected value within the range).
type RangeItem struct {
	Label    string
	Low      float64
	High     float64
	Point    float64 // most-likely marker; drawn only when HasPoint
	HasPoint bool
	Role     string
	Suffix   string // appended to value labels (e.g. "%", or use a $ Prefix)
	Prefix   string // prepended to value labels (e.g. "$")
}

// RangeBarsOptions configures a RangeBars render.
type RangeBarsOptions struct {
	W         int     // SVG width. Default 360.
	RowH      int     // row height. Default 26.
	Gap       int     // gap between rows. Default 6.
	LabelW    int     // left label column. Default 96.
	Max       float64 // scale max; 0 derives from the largest High.
	HideRange bool    // hide the "low–high" value labels at the right (shown by default).
	AriaLabel string
}

func (o *RangeBarsOptions) withDefaults() RangeBarsOptions {
	out := *o
	if out.W <= 0 {
		out.W = 360
	}
	if out.RowH <= 0 {
		out.RowH = 26
	}
	if out.Gap <= 0 {
		out.Gap = 6
	}
	if out.LabelW <= 0 {
		out.LabelW = 96
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "range chart"
	}
	return out
}

// RangeBars renders horizontal low–high interval bars — for value ranges such
// as a damages low/high per theory, with an optional most-likely point marker.
// Bars scale to Max (or the largest High). Negative lows clamp to 0; an inverted
// item (High < Low) is swapped. Deterministic; strings escaped; roles validated.
func RangeBars(items []RangeItem, o RangeBarsOptions) string {
	opts := o.withDefaults()
	n := len(items)
	rowH := float64(opts.RowH)
	gap := float64(opts.Gap)
	totalH := opts.RowH
	if n > 0 {
		totalH = n*opts.RowH + (n-1)*opts.Gap
	}

	var b strings.Builder
	b.WriteString(svgOpen(opts.W, totalH, opts.AriaLabel, ""))
	b.WriteString(svgTitle(opts.AriaLabel))
	if n == 0 {
		b.WriteString("</svg>")
		return b.String()
	}

	maxVal := opts.Max
	if maxVal <= 0 {
		for _, it := range items {
			hi := it.High
			if it.Low > hi {
				hi = it.Low
			}
			if hi > maxVal {
				maxVal = hi
			}
		}
	}

	w := float64(opts.W)
	labelW := float64(opts.LabelW)
	showRange := !opts.HideRange
	valueW := 0.0
	if showRange {
		valueW = 96
	}
	trackX := labelW + 6
	trackW := w - trackX - valueW
	if trackW < 0 {
		trackW = 0
	}

	xOf := func(v float64) float64 {
		if maxVal <= 0 {
			return trackX
		}
		if v < 0 {
			v = 0
		}
		if v > maxVal {
			v = maxVal
		}
		return trackX + v/maxVal*trackW
	}

	ink := cssColor("ink")
	inkMuted := cssColor("ink-muted")
	track := cssColor("surface-sunken")

	for i, it := range items {
		lo, hi := it.Low, it.High
		if hi < lo {
			lo, hi = hi, lo
		}
		y := float64(i) * (rowH + gap)
		mid := y + rowH/2
		barY := y + rowH*0.28
		barH := rowH * 0.44

		// Label (left).
		fmt.Fprintf(&b, `<text x="0" y="%s" dominant-baseline="middle" fill="%s" font-size="11">%s</text>`,
			fmtNum(mid), inkMuted, esc(it.Label))

		// Track.
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s" fill-opacity="0.5"/>`,
			fmtNum(trackX), fmtNum(barY), fmtNum(trackW), fmtNum(barH), track)

		// Interval bar low→high.
		x1, x2 := xOf(lo), xOf(hi)
		bw := x2 - x1
		if bw < 2 {
			bw = 2
		}
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s"><title>%s: %s%s%s – %s%s%s</title></rect>`,
			fmtNum(x1), fmtNum(barY), fmtNum(bw), fmtNum(barH), cssColor(it.Role),
			esc(it.Label), esc(it.Prefix), fmtNum(lo), esc(it.Suffix), esc(it.Prefix), fmtNum(hi), esc(it.Suffix))

		// Most-likely point marker (vertical tick spanning the row).
		if it.HasPoint {
			px := xOf(it.Point)
			fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="2"><title>most likely: %s%s%s</title></line>`,
				fmtNum(px), fmtNum(y+rowH*0.18), fmtNum(px), fmtNum(y+rowH*0.82), ink,
				esc(it.Prefix), fmtNum(it.Point), esc(it.Suffix))
		}

		// Range value label (right).
		if showRange {
			fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" dominant-baseline="middle" fill="%s" font-size="10">%s%s%s–%s%s%s</text>`,
				fmtNum(w), fmtNum(mid), ink,
				esc(it.Prefix), fmtNum(lo), esc(it.Suffix), esc(it.Prefix), fmtNum(hi), esc(it.Suffix))
		}
	}

	b.WriteString("</svg>")
	return b.String()
}
