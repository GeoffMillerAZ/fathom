// donut.go — distribution donut (ring chart).
package viz

import (
	"fmt"
	"math"
	"strings"
)

// DonutOptions configures a Donut render.
type DonutOptions struct {
	// Size is the outer diameter in pixels. Default 96.
	Size int
	// Thickness is the ring thickness in pixels. Default 10.
	Thickness int
	// CenterText is optional text in the center of the ring (escaped internally).
	CenterText string
	// Fluid fills the container's width instead of drawing at Size pixels:
	// the chart scales uniformly from its Size×Size viewBox (text included)
	// and the height follows. Off by default (fixed pixel size).
	Fluid bool
	// AriaLabel is the accessible label.
	AriaLabel string
}

func (o *DonutOptions) withDefaults() DonutOptions {
	out := *o
	if out.Size <= 0 {
		out.Size = 96
	}
	if out.Thickness <= 0 {
		out.Thickness = 10
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "donut chart"
	}
	return out
}

// Donut returns an SVG string for a distribution ring chart.
//
// Zero-total → empty ring using the "line" role. Segments are rendered in
// the order provided, starting at 12 o'clock, going clockwise.
func Donut(segments []Segment, o DonutOptions) string {
	opts := o.withDefaults()
	size := float64(opts.Size)
	cx, cy := size/2, size/2
	r := (size - float64(opts.Thickness)) / 2

	var b strings.Builder
	b.WriteString(chartOpen(opts.Size, opts.Size, opts.AriaLabel, opts.Fluid))
	b.WriteString(svgTitle(opts.AriaLabel))

	total := 0.0
	for _, s := range segments {
		if s.Value > 0 {
			total += s.Value
		}
	}

	if total == 0 || len(segments) == 0 {
		// Empty ring.
		emptyColor := cssColor("line")
		fmt.Fprintf(&b,
			`<circle cx="%s" cy="%s" r="%s" fill="none" stroke="%s" stroke-width="%d"/>`,
			fmtNum(cx), fmtNum(cy), fmtNum(r), emptyColor, opts.Thickness)
	} else {
		// Draw arc segments.
		startAngle := -math.Pi / 2 // 12 o'clock
		for _, seg := range segments {
			if seg.Value <= 0 {
				continue
			}
			sweep := (seg.Value / total) * 2 * math.Pi
			endAngle := startAngle + sweep
			color := cssColor(seg.Role)

			// Arc path using stroke-dasharray trick for thick arcs.
			// We use actual arc paths so segments are visually separated by a 1px gap.
			gapAngle := 0.012 // radians gap between segments (~0.7°)
			aStart := startAngle + gapAngle/2
			aEnd := endAngle - gapAngle/2
			if aEnd <= aStart {
				// Segment too small for gap — render without gap.
				aStart = startAngle
				aEnd = endAngle
			}

			x1 := cx + r*math.Cos(aStart)
			y1 := cy + r*math.Sin(aStart)
			x2 := cx + r*math.Cos(aEnd)
			y2 := cy + r*math.Sin(aEnd)
			largeArc := 0
			if aEnd-aStart > math.Pi {
				largeArc = 1
			}

			label := esc(seg.Label)
			fmt.Fprintf(&b,
				`<path d="M %s %s A %s %s 0 %d 1 %s %s" fill="none" stroke="%s" stroke-width="%d"><title>%s</title></path>`,
				fmtNum(x1), fmtNum(y1),
				fmtNum(r), fmtNum(r),
				largeArc,
				fmtNum(x2), fmtNum(y2),
				color, opts.Thickness,
				label)

			startAngle = endAngle
		}
	}

	// Optional center text.
	if opts.CenterText != "" {
		inkColor := cssColor("ink")
		fmt.Fprintf(&b,
			`<text x="%s" y="%s" text-anchor="middle" dominant-baseline="middle" fill="%s" font-size="14">%s</text>`,
			fmtNum(cx), fmtNum(cy), inkColor, esc(opts.CenterText))
	}

	b.WriteString("</svg>")
	return b.String()
}
