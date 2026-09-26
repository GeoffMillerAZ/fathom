// stackbar.go — horizontal stacked bar (coverage, funnel ratios).
package viz

import (
	"fmt"
	"strings"
)

// Segment is one named portion of a stacked bar or donut.
type Segment struct {
	// Value is the numeric magnitude. Must be >= 0.
	Value float64
	// Role is the token color role for this segment.
	Role string
	// Label is the human-readable name (escaped internally).
	Label string
}

// StackBarOptions configures a StackBar render.
type StackBarOptions struct {
	// W is the SVG width. Default 200.
	W int
	// H is the SVG height (bar height). Default 12.
	H int
	// Radius is the corner radius (px). Default 3.
	Radius int
	// Fluid fills the container's width instead of drawing at W pixels:
	// the chart scales uniformly from its W×H viewBox (strokes included)
	// and the height follows. Off by default (fixed pixel size).
	Fluid bool
	// AriaLabel is the accessible label.
	AriaLabel string
}

func (o *StackBarOptions) withDefaults() StackBarOptions {
	out := *o
	if out.W <= 0 {
		out.W = 200
	}
	if out.H <= 0 {
		out.H = 12
	}
	if out.Radius <= 0 {
		out.Radius = 3
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "stacked bar"
	}
	return out
}

// StackBar returns an SVG string for one horizontal stacked bar.
//
// Zero-total → empty-state track rendered using the "line" role.
func StackBar(segments []Segment, o StackBarOptions) string {
	opts := o.withDefaults()

	var b strings.Builder
	b.WriteString(chartOpen(opts.W, opts.H, opts.AriaLabel, opts.Fluid))
	b.WriteString(svgTitle(opts.AriaLabel))

	total := 0.0
	for _, s := range segments {
		if s.Value > 0 {
			total += s.Value
		}
	}

	r := float64(opts.Radius)
	w := float64(opts.W)
	h := float64(opts.H)

	if total == 0 || len(segments) == 0 {
		// Empty-state track using "line" role color.
		emptyColor := cssColor("line")
		fmt.Fprintf(&b,
			`<rect x="0" y="0" width="%s" height="%s" rx="%s" fill="%s"/>`,
			fmtNum(w), fmtNum(h), fmtNum(r), emptyColor)
		b.WriteString("</svg>")
		return b.String()
	}

	// Render segments left-to-right; round corners only on the outermost ends.
	x := 0.0
	for i, seg := range segments {
		if seg.Value <= 0 {
			continue
		}
		segW := (seg.Value / total) * w
		color := cssColor(seg.Role)

		// Determine which corners to round.
		isFirst := (x == 0)
		isLast := (i == len(segments)-1)

		// SVG doesn't support per-corner rect radius directly; use path.
		// Build a path with individual corner arcs.
		tlr, trr, brr, blr := 0.0, 0.0, 0.0, 0.0
		if isFirst {
			tlr = r
			blr = r
		}
		if isLast {
			trr = r
			brr = r
		}

		path := roundedRectPath(x, 0, segW, h, tlr, trr, brr, blr)
		label := esc(seg.Label)
		fmt.Fprintf(&b,
			`<path d="%s" fill="%s"><title>%s</title></path>`,
			path, color, label)

		x += segW
	}

	b.WriteString("</svg>")
	return b.String()
}

// roundedRectPath builds an SVG path string for a rectangle with per-corner
// radii. Corners with radius 0 are sharp.
func roundedRectPath(x, y, w, h, tlr, trr, brr, blr float64) string {
	// Clamp radii to half the shorter dimension.
	maxR := min2(w, h) / 2
	tlr = clampR(tlr, maxR)
	trr = clampR(trr, maxR)
	brr = clampR(brr, maxR)
	blr = clampR(blr, maxR)

	// Path: start at top-left after TL radius, go clockwise.
	return fmt.Sprintf(
		"M %s %s "+
			"Q %s %s %s %s "+
			"L %s %s "+
			"Q %s %s %s %s "+
			"L %s %s "+
			"Q %s %s %s %s "+
			"L %s %s "+
			"Q %s %s %s %s "+
			"Z",
		// Start: just right of TL corner.
		fmtNum(x+tlr), fmtNum(y),
		// TL arc control → TR segment start.
		fmtNum(x), fmtNum(y), fmtNum(x), fmtNum(y+tlr),
		// Left edge down to BL.
		fmtNum(x), fmtNum(y+h-blr),
		// BL arc.
		fmtNum(x), fmtNum(y+h), fmtNum(x+blr), fmtNum(y+h),
		// Bottom edge right to BR.
		fmtNum(x+w-brr), fmtNum(y+h),
		// BR arc.
		fmtNum(x+w), fmtNum(y+h), fmtNum(x+w), fmtNum(y+h-brr),
		// Right edge up to TR.
		fmtNum(x+w), fmtNum(y+trr),
		// TR arc back to start.
		fmtNum(x+w), fmtNum(y), fmtNum(x+w-trr), fmtNum(y),
	)
}

func clampR(r, max float64) float64 {
	if r > max {
		return max
	}
	return r
}

func min2(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
