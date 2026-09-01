package viz

import (
	"fmt"
	"sort"
	"strings"
)

// TornadoItem is one factor's swing: the outcome value when the factor is at its
// Low and High. The bar spans Low→High across the baseline; its width is the
// factor's sensitivity (how much it moves the outcome).
type TornadoItem struct {
	Label string
	Low   float64
	High  float64
	// Role optionally overrides the side coloring. Empty uses positive for the
	// portion above the baseline and danger for the portion below.
	Role   string
	Prefix string // value-label prefix (e.g. "$")
	Suffix string // value-label suffix (e.g. "%")
}

// TornadoOptions configures a Tornado render.
type TornadoOptions struct {
	W         int     // SVG width. Default 420.
	RowH      int     // row height. Default 24.
	Gap       int     // gap between rows. Default 6.
	LabelW    int     // left label column. Default 120.
	Baseline  float64 // center value the swings pivot around (the base case).
	Prefix    string  // applied to the baseline + end labels when item has none.
	Suffix    string
	AriaLabel string
}

func (o *TornadoOptions) withDefaults() TornadoOptions {
	out := *o
	if out.W <= 0 {
		out.W = 420
	}
	if out.RowH <= 0 {
		out.RowH = 24
	}
	if out.Gap <= 0 {
		out.Gap = 6
	}
	if out.LabelW <= 0 {
		out.LabelW = 120
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "tornado sensitivity chart"
	}
	return out
}

// Tornado renders a tornado / sensitivity chart: one horizontal bar per factor,
// spanning that factor's Low→High outcome values across a central Baseline, with
// the widest swing (most influential factor) at the top. The portion above the
// baseline is positive-role, the portion below danger-role (unless Role overrides).
// Answers "what moves the outcome most." Empty-safe; deterministic (stable sort
// by swing, then label); strings escaped; roles validated.
func Tornado(items []TornadoItem, o TornadoOptions) string {
	opts := o.withDefaults()
	n := len(items)
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

	// Sort a copy by swing magnitude (widest first), then label for determinism.
	sorted := make([]TornadoItem, len(items))
	copy(sorted, items)
	sort.SliceStable(sorted, func(i, j int) bool {
		si, sj := swing(sorted[i]), swing(sorted[j])
		if si != sj {
			return si > sj
		}
		return sorted[i].Label < sorted[j].Label
	})

	// Value domain spans every Low/High and the baseline.
	vMin, vMax := opts.Baseline, opts.Baseline
	for _, it := range sorted {
		lo, hi := it.Low, it.High
		if lo > hi {
			lo, hi = hi, lo
		}
		if lo < vMin {
			vMin = lo
		}
		if hi > vMax {
			vMax = hi
		}
	}
	span := vMax - vMin
	if span == 0 {
		span = 1
	}

	w := float64(opts.W)
	labelW := float64(opts.LabelW)
	trackX := labelW + 6
	trackW := w - trackX
	if trackW < 0 {
		trackW = 0
	}
	xOf := func(v float64) float64 {
		return trackX + (v-vMin)/span*trackW
	}

	rowH := float64(opts.RowH)
	gap := float64(opts.Gap)
	baseX := xOf(opts.Baseline)
	ink := cssColor("ink")
	inkMuted := cssColor("ink-muted")
	line := cssColor("line-strong")

	// Baseline guide spanning the plot height.
	fmt.Fprintf(&b, `<line x1="%s" y1="0" x2="%s" y2="%s" stroke="%s" stroke-width="1" stroke-dasharray="2 2"/>`,
		fmtNum(baseX), fmtNum(baseX), fmtNum(float64(totalH)), line)

	for i, it := range sorted {
		lo, hi := it.Low, it.High
		if lo > hi {
			lo, hi = hi, lo
		}
		y := float64(i) * (rowH + gap)
		barY := y + rowH*0.2
		barH := rowH * 0.6
		mid := y + rowH/2

		// Left label.
		fmt.Fprintf(&b, `<text x="0" y="%s" dominant-baseline="middle" fill="%s" font-size="11">%s</text>`,
			fmtNum(mid), inkMuted, esc(it.Label))

		// Below-baseline portion (danger) and above-baseline portion (positive).
		belowRole, aboveRole := "danger", "positive"
		if it.Role != "" {
			belowRole, aboveRole = it.Role, it.Role
		}
		if lo < opts.Baseline {
			x1 := xOf(lo)
			x2 := xOf(min(hi, opts.Baseline))
			drawSeg(&b, x1, x2, barY, barH, cssColor(belowRole))
		}
		if hi > opts.Baseline {
			x1 := xOf(max(lo, opts.Baseline))
			x2 := xOf(hi)
			drawSeg(&b, x1, x2, barY, barH, cssColor(aboveRole))
		}

		// End value labels (low at left edge of bar, high at right).
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="end" dominant-baseline="middle" fill="%s" font-size="9">%s%s%s</text>`,
			fmtNum(xOf(lo)-2), fmtNum(mid), ink, esc(it.Prefix), fmtNum(lo), esc(it.Suffix))
		fmt.Fprintf(&b, `<text x="%s" y="%s" text-anchor="start" dominant-baseline="middle" fill="%s" font-size="9">%s%s%s</text>`,
			fmtNum(xOf(hi)+2), fmtNum(mid), ink, esc(it.Prefix), fmtNum(hi), esc(it.Suffix))

		// Accessible per-bar title.
		fmt.Fprintf(&b, `<title>%s: %s%s%s to %s%s%s</title>`,
			esc(it.Label), esc(it.Prefix), fmtNum(lo), esc(it.Suffix), esc(it.Prefix), fmtNum(hi), esc(it.Suffix))
	}

	b.WriteString("</svg>")
	return b.String()
}

// swing is the absolute width of an item's Low→High range.
func swing(it TornadoItem) float64 {
	d := it.High - it.Low
	if d < 0 {
		return -d
	}
	return d
}

func drawSeg(b *strings.Builder, x1, x2, y, h float64, fill string) {
	w := x2 - x1
	if w < 1 {
		w = 1
	}
	fmt.Fprintf(b, `<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"/>`,
		fmtNum(x1), fmtNum(y), fmtNum(w), fmtNum(h), fill)
}
