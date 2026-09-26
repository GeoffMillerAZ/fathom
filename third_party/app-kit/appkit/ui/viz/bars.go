package viz

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
)

// Bar is one row of a Bars chart.
type Bar struct {
	Label string
	Value float64
	Role  string
	// Suffix is appended to the value label (e.g. "%"). Optional.
	Suffix string
}

// BarsOptions configures a Bars render.
type BarsOptions struct {
	// W is the SVG width. Default 320.
	W int
	// RowH is the height of each bar row. Default 22.
	RowH int
	// Gap is the vertical gap between rows. Default 6.
	Gap int
	// LabelW is the left label column width. Default 96.
	LabelW int
	// Max pins the scale; 0 derives it from the largest value.
	Max float64
	// ShowValue draws the value at the end of each bar. Off by default (the
	// zero value) — an earlier comment claimed "default true", which a bool
	// zero value can never deliver. Values >= 1000 render with thousands
	// separators.
	ShowValue bool
	// Fluid fills the container's width instead of drawing at W pixels:
	// the chart scales uniformly from its W×height viewBox (text included)
	// and the height follows. Off by default (fixed pixel size).
	Fluid bool
	// AriaLabel is the accessible label.
	AriaLabel string
}

func (o *BarsOptions) withDefaults() BarsOptions {
	out := *o
	if out.W <= 0 {
		out.W = 320
	}
	if out.RowH <= 0 {
		out.RowH = 22
	}
	if out.Gap <= 0 {
		out.Gap = 6
	}
	if out.LabelW <= 0 {
		out.LabelW = 96
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "bar chart"
	}
	return out
}

// Bars renders a horizontal labeled bar chart — for comparisons, probabilities,
// or an expected-outcome distribution (each category its own role-colored bar
// with a value label). Bar widths are proportional to Max (or the largest
// value). Negative values clamp to zero. Deterministic; strings escaped; roles
// validated.
func Bars(bars []Bar, o BarsOptions) string {
	opts := o.withDefaults()
	n := len(bars)
	rowH := float64(opts.RowH)
	gap := float64(opts.Gap)
	totalH := 0
	if n > 0 {
		totalH = n*opts.RowH + (n-1)*opts.Gap
	} else {
		totalH = opts.RowH
	}

	var b strings.Builder
	b.WriteString(chartOpen(opts.W, totalH, opts.AriaLabel, opts.Fluid))
	b.WriteString(svgTitle(opts.AriaLabel))
	if n == 0 {
		b.WriteString("</svg>")
		return b.String()
	}

	maxVal := opts.Max
	if maxVal <= 0 {
		for _, bar := range bars {
			if bar.Value > maxVal {
				maxVal = bar.Value
			}
		}
	}

	w := float64(opts.W)
	labelW := float64(opts.LabelW)
	valueW := 0.0
	if opts.ShowValue {
		valueW = 56
		for _, bar := range bars {
			// Reserve the complete printed value, including units. One em per
			// display cell is deliberately conservative for the 11px labels;
			// actual consumer font geometry still needs browser verification.
			label := fmtGrouped(bar.Value) + bar.Suffix
			valueW = math.Max(valueW, float64(uniseg.StringWidth(label))*11+8)
		}
	}
	trackX := labelW + 6
	barMaxW := w - trackX - valueW
	if barMaxW < 0 {
		barMaxW = 0
	}

	ink := cssColor("ink")
	inkMuted := cssColor("ink-muted")
	track := cssColor("surface-sunken")

	for i, bar := range bars {
		y := float64(i) * (rowH + gap)
		mid := y + rowH/2
		v := bar.Value
		if v < 0 {
			v = 0
		}
		bw := 0.0
		if maxVal > 0 {
			bw = v / maxVal * barMaxW
		}

		// Label (left). This low-level primitive does not wrap or truncate;
		// the caller must choose fitting columns or a wrapping composition.
		fmt.Fprintf(&b,
			`<text x="0" y="%s" dominant-baseline="middle" fill="%s" font-size="11">%s</text>`,
			fmtNum(mid), inkMuted, esc(bar.Label))

		// Track + bar.
		fmt.Fprintf(&b,
			`<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s" fill-opacity="0.5"/>`,
			fmtNum(trackX), fmtNum(y+rowH*0.15), fmtNum(barMaxW), fmtNum(rowH*0.7), track)
		if bw > 0 {
			fmt.Fprintf(&b,
				`<rect x="%s" y="%s" width="%s" height="%s" rx="3" fill="%s"><title>%s: %s%s</title></rect>`,
				fmtNum(trackX), fmtNum(y+rowH*0.15), fmtNum(bw), fmtNum(rowH*0.7), cssColor(bar.Role),
				esc(bar.Label), fmtGrouped(bar.Value), esc(bar.Suffix))
		}

		// Value label.
		if opts.ShowValue {
			fmt.Fprintf(&b,
				`<text x="%s" y="%s" text-anchor="end" dominant-baseline="middle" fill="%s" font-size="11">%s%s</text>`,
				fmtNum(w), fmtNum(mid), ink, fmtGrouped(bar.Value), esc(bar.Suffix))
		}
	}

	b.WriteString("</svg>")
	return b.String()
}

// fmtGrouped renders an integral value with thousands separators (12,345);
// non-integral values fall back to fmtNum unchanged. Large numbers without
// grouping are misread — a UI law, not a style choice.
func fmtGrouped(v float64) string {
	if v != math.Trunc(v) || math.Abs(v) < 1000 {
		return fmtNum(v)
	}
	s := strconv.FormatInt(int64(v), 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}
