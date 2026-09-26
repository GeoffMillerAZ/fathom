// funnel.go — vertical funnel (proposed → imported / excluded).
package viz

import (
	"fmt"
	"strings"
)

// Stage is one level of a Funnel.
type Stage struct {
	// Label is the stage name (escaped internally).
	Label string
	// Value is the count/magnitude for this stage.
	Value float64
	// Role is the token color role for this stage's bar.
	Role string
}

// FunnelOptions configures a Funnel render.
type FunnelOptions struct {
	// W is the SVG width. Default 160.
	W int
	// StageH is the height of each stage bar. Default 20.
	StageH int
	// Gap is the vertical gap between stages. Default 6.
	Gap int
	// Fluid fills the container's width instead of drawing at W pixels:
	// the chart scales uniformly from its W×height viewBox (text included)
	// and the height follows. Off by default (fixed pixel size).
	Fluid bool
	// AriaLabel is the accessible label.
	AriaLabel string
}

func (o *FunnelOptions) withDefaults() FunnelOptions {
	out := *o
	if out.W <= 0 {
		out.W = 160
	}
	if out.StageH <= 0 {
		out.StageH = 20
	}
	if out.Gap <= 0 {
		out.Gap = 6
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "funnel chart"
	}
	return out
}

// Funnel returns an SVG string for a vertical funnel chart.
//
// Bar widths are proportional to the first stage value (zero-safe: if the
// first stage is 0 all bars render at zero width). Count + label are rendered
// as SVG text (escaped). Each stage has a <title> for accessibility.
func Funnel(stages []Stage, o FunnelOptions) string {
	opts := o.withDefaults()

	n := len(stages)
	if n == 0 {
		totalH := opts.StageH
		var b strings.Builder
		b.WriteString(chartOpen(opts.W, totalH, opts.AriaLabel, opts.Fluid))
		b.WriteString(svgTitle(opts.AriaLabel))
		b.WriteString("</svg>")
		return b.String()
	}

	totalH := n*opts.StageH + (n-1)*opts.Gap
	w := float64(opts.W)
	stH := float64(opts.StageH)
	gap := float64(opts.Gap)

	maxVal := stages[0].Value // widths proportional to stage 0
	// Zero-safe: if first stage is 0, use the largest value to avoid all-zero.
	if maxVal == 0 {
		for _, s := range stages {
			if s.Value > maxVal {
				maxVal = s.Value
			}
		}
	}

	// Label column: fixed 80px on the right; bars fill the rest.
	labelW := 80.0
	barMaxW := w - labelW - 8 // 8px gap between bar and label

	var b strings.Builder
	b.WriteString(chartOpen(opts.W, totalH, opts.AriaLabel, opts.Fluid))
	b.WriteString(svgTitle(opts.AriaLabel))

	inkMuted := cssColor("ink-muted")
	ink := cssColor("ink")

	for i, stage := range stages {
		y := float64(i) * (stH + gap)

		barW := 0.0
		if maxVal > 0 {
			barW = (stage.Value / maxVal) * barMaxW
		}

		color := cssColor(stage.Role)

		// Bar rect.
		if barW > 0 {
			fmt.Fprintf(&b,
				`<rect x="0" y="%s" width="%s" height="%s" rx="3" fill="%s"><title>%s: %s</title></rect>`,
				fmtNum(y), fmtNum(barW), fmtNum(stH), color,
				esc(stage.Label), fmtNum(stage.Value))
		} else {
			// Zero-width bar: render a thin placeholder.
			fmt.Fprintf(&b,
				`<rect x="0" y="%s" width="2" height="%s" rx="1" fill="%s"/>`,
				fmtNum(y), fmtNum(stH), cssColor("line"))
		}

		// Label text (label name + value).
		textX := barMaxW + 8
		textY := y + stH/2

		fmt.Fprintf(&b,
			`<text x="%s" y="%s" dominant-baseline="middle" fill="%s" font-size="11">%s</text>`,
			fmtNum(textX), fmtNum(textY), inkMuted, esc(stage.Label))

		// Count right-aligned inside bar or just right of bar end.
		if stage.Value > 0 {
			countX := barW - 4
			if countX < 4 {
				countX = barW + 4
			}
			countFill := ink
			if barW > 24 {
				countFill = cssColor("ink-inverse")
			}
			fmt.Fprintf(&b,
				`<text x="%s" y="%s" text-anchor="end" dominant-baseline="middle" fill="%s" font-size="11" font-weight="500">%s</text>`,
				fmtNum(countX), fmtNum(textY), countFill, fmtNum(stage.Value))
		}
	}

	b.WriteString("</svg>")
	return b.String()
}
