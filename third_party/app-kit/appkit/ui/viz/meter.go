// meter.go — single labeled meter (element-coverage confidence style).
package viz

import (
	"fmt"
	"strings"
)

// MeterOptions configures a MeterRow render.
type MeterOptions struct {
	// W is the SVG width. Default 200.
	W int
	// H is the total SVG height. Default 20.
	H int
	// TrackRole is the token role for the empty track. Default "line".
	TrackRole string
	// FillRole is the token role for the filled portion. Default "accent".
	FillRole string
	// Label is text rendered left of the bar (escaped internally).
	Label string
	// ShowPct renders a right-side percentage label when true.
	ShowPct bool
	// Fluid fills the container's width instead of drawing at W pixels:
	// the chart scales uniformly from its W×H viewBox (text included)
	// and the height follows. Off by default (fixed pixel size).
	Fluid bool
	// AriaLabel is the accessible label.
	AriaLabel string
}

func (o *MeterOptions) withDefaults() MeterOptions {
	out := *o
	if out.W <= 0 {
		out.W = 200
	}
	if out.H <= 0 {
		out.H = 20
	}
	if out.TrackRole == "" {
		out.TrackRole = "line"
	}
	if out.FillRole == "" {
		out.FillRole = "accent"
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "meter"
	}
	return out
}

// MeterRow returns an SVG string for a single labeled meter bar.
//
// value and max are clamped: negative → 0, value > max → max.
// max == 0 → empty track (no fill).
func MeterRow(value, max float64, o MeterOptions) string {
	opts := o.withDefaults()

	// Clamp.
	if value < 0 {
		value = 0
	}
	if max < 0 {
		max = 0
	}
	if max > 0 && value > max {
		value = max
	}

	w := float64(opts.W)
	h := float64(opts.H)

	// Layout: optional label on left, bar in middle, optional pct on right.
	labelW := 0.0
	if opts.Label != "" {
		labelW = 72.0
	}
	pctW := 0.0
	if opts.ShowPct {
		pctW = 36.0
	}
	barX := labelW
	barW := w - labelW - pctW
	if barW < 0 {
		barW = 0
	}

	barH := 8.0
	barY := (h - barH) / 2

	trackColor := cssColor(opts.TrackRole)
	fillColor := cssColor(opts.FillRole)
	inkMuted := cssColor("ink-muted")

	var b strings.Builder
	b.WriteString(chartOpen(opts.W, opts.H, opts.AriaLabel, opts.Fluid))
	b.WriteString(svgTitle(opts.AriaLabel))

	// Left label.
	if opts.Label != "" {
		fmt.Fprintf(&b,
			`<text x="0" y="%s" dominant-baseline="middle" fill="%s" font-size="11">%s</text>`,
			fmtNum(h/2), inkMuted, esc(opts.Label))
	}

	// Track.
	fmt.Fprintf(&b,
		`<rect x="%s" y="%s" width="%s" height="%s" rx="4" fill="%s"/>`,
		fmtNum(barX), fmtNum(barY), fmtNum(barW), fmtNum(barH), trackColor)

	// Fill.
	if max > 0 && value > 0 {
		fillW := (value / max) * barW
		fmt.Fprintf(&b,
			`<rect x="%s" y="%s" width="%s" height="%s" rx="4" fill="%s"/>`,
			fmtNum(barX), fmtNum(barY), fmtNum(fillW), fmtNum(barH), fillColor)
	}

	// Right pct label.
	if opts.ShowPct {
		pct := 0.0
		if max > 0 {
			pct = (value / max) * 100
		}
		pctX := w - pctW + 4
		fmt.Fprintf(&b,
			`<text x="%s" y="%s" dominant-baseline="middle" fill="%s" font-size="11">%s%%</text>`,
			fmtNum(pctX), fmtNum(h/2), inkMuted, fmtNum(pct))
	}

	b.WriteString("</svg>")
	return b.String()
}
