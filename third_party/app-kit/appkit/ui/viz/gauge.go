package viz

import (
	"fmt"
	"math"
	"strings"
)

// GaugeOptions configures a Gauge render.
type GaugeOptions struct {
	// W is the SVG width. Default 160. Height is derived (semicircle + labels).
	W int
	// FillRole is the arc fill role. Default "accent". Ignored when Graded.
	FillRole string
	// TrackRole is the unfilled arc. Default "line".
	TrackRole string
	// Graded auto-colors the fill by the value fraction — a built-in
	// at-a-glance scale for scores like case strength or likelihood:
	// <34% danger, <67% caution, else positive.
	Graded bool
	// CenterText overrides the big center label (defaults to the rounded percent).
	CenterText string
	// Label is the caption below the gauge.
	Label string
	// AriaLabel is the accessible label.
	AriaLabel string
}

func (o *GaugeOptions) withDefaults() GaugeOptions {
	out := *o
	if out.W <= 0 {
		out.W = 160
	}
	if out.FillRole == "" {
		out.FillRole = "accent"
	}
	if out.TrackRole == "" {
		out.TrackRole = "line"
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "gauge"
	}
	return out
}

// GradedRole maps a 0..1 fraction to a kit status role: a quick semantic scale
// for strength / confidence / likelihood (low → danger, mid → caution, high →
// positive). Exposed so callers can color related elements consistently.
func GradedRole(frac float64) string {
	switch {
	case frac < 0.34:
		return "danger"
	case frac < 0.67:
		return "caution"
	default:
		return "positive"
	}
}

// Gauge renders a 180° semicircular gauge for value/max — an at-a-glance score
// dial (case strength, likelihood of success, confidence). The arc fills
// clockwise from the left; a center number and optional caption sit below.
// value is clamped to [0, max]; max <= 0 renders an empty track without
// dividing by zero. Deterministic; strings escaped; roles validated.
func Gauge(value, max float64, o GaugeOptions) string {
	opts := o.withDefaults()

	frac := 0.0
	if max > 0 {
		frac = value / max
	}
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}

	w := float64(opts.W)
	stroke := w * 0.075 // arc thickness scales with size
	pad := stroke/2 + 2 // keep the round cap inside the viewbox
	cx := w / 2         //
	r := w/2 - pad      //
	cy := pad + r       // baseline of the semicircle
	labelH := 0.0
	if opts.Label != "" {
		labelH = 16
	}
	totalH := int(cy + stroke/2 + 6 + labelH)

	fillRole := opts.FillRole
	if opts.Graded {
		fillRole = GradedRole(frac)
	}

	// Point on the semicircle at fraction f (0 = left end, 1 = right end).
	pt := func(f float64) (float64, float64) {
		theta := math.Pi * (1 - f) // 180°→0°
		return cx + r*math.Cos(theta), cy - r*math.Sin(theta)
	}
	lx, ly := pt(0)
	rx, ry := pt(1)

	var b strings.Builder
	b.WriteString(svgOpen(opts.W, totalH, opts.AriaLabel, ""))
	b.WriteString(svgTitle(opts.AriaLabel))

	// Track (full semicircle).
	fmt.Fprintf(&b,
		`<path d="M%s %s A %s %s 0 0 1 %s %s" fill="none" stroke="%s" stroke-width="%s" stroke-linecap="round"/>`,
		fmtNum(lx), fmtNum(ly), fmtNum(r), fmtNum(r), fmtNum(rx), fmtNum(ry),
		cssColor(opts.TrackRole), fmtNum(stroke))

	// Fill (left to the fraction point).
	if frac > 0 {
		fx, fy := pt(frac)
		fmt.Fprintf(&b,
			`<path d="M%s %s A %s %s 0 0 1 %s %s" fill="none" stroke="%s" stroke-width="%s" stroke-linecap="round"/>`,
			fmtNum(lx), fmtNum(ly), fmtNum(r), fmtNum(r), fmtNum(fx), fmtNum(fy),
			cssColor(fillRole), fmtNum(stroke))
	}

	// Center value.
	center := opts.CenterText
	if center == "" {
		center = fmtNum(math.Round(frac*100)) + "%"
	}
	fmt.Fprintf(&b,
		`<text x="%s" y="%s" text-anchor="middle" fill="%s" font-size="%s" font-weight="600">%s</text>`,
		fmtNum(cx), fmtNum(cy-2), cssColor("ink"), fmtNum(w*0.16), esc(center))

	// Caption.
	if opts.Label != "" {
		fmt.Fprintf(&b,
			`<text x="%s" y="%s" text-anchor="middle" fill="%s" font-size="11">%s</text>`,
			fmtNum(cx), fmtNum(cy+stroke/2+12), cssColor("ink-muted"), esc(opts.Label))
	}

	b.WriteString("</svg>")
	return b.String()
}
