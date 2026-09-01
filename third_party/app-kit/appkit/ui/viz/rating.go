package viz

import (
	"fmt"
	"math"
	"strings"
)

// RatingOptions configures a Rating render.
type RatingOptions struct {
	// Max is the number of pips. Default 5.
	Max int
	// PipR is the pip radius in px. Default 6.
	PipR int
	// Gap is the space between pips in px. Default 6.
	Gap int
	// Role is the filled-pip color. Default "accent". Ignored when Graded.
	Role string
	// TrackRole is the empty-pip color. Default "line".
	TrackRole string
	// Graded colors filled pips by the filled/max fraction (danger/caution/
	// positive) — an at-a-glance strength scale (e.g. evidence strength).
	Graded bool
	// AriaLabel is the accessible label. A sensible "N of M" default is used
	// when empty.
	AriaLabel string
}

func (o *RatingOptions) withDefaults() RatingOptions {
	out := *o
	if out.Max <= 0 {
		out.Max = 5
	}
	if out.PipR <= 0 {
		out.PipR = 6
	}
	if out.Gap <= 0 {
		out.Gap = 6
	}
	if out.Role == "" {
		out.Role = "accent"
	}
	if out.TrackRole == "" {
		out.TrackRole = "line"
	}
	return out
}

// Rating renders a row of pips with `filled` of Max filled — a compact strength
// / quality indicator (evidence strength, confidence, priority). filled is
// rounded and clamped to [0, Max]. Graded mode colors the filled pips by the
// fraction. Deterministic; strings escaped; roles validated.
func Rating(filled float64, o RatingOptions) string {
	opts := o.withDefaults()
	n := opts.Max
	f := min(max(int(math.Round(filled)), 0), n)

	r := float64(opts.PipR)
	gap := float64(opts.Gap)
	w := float64(n)*(2*r+gap) - gap + 2 // +2 so edge strokes aren't clipped
	h := 2*r + 2

	role := opts.Role
	if opts.Graded {
		frac := 0.0
		if n > 0 {
			frac = float64(f) / float64(n)
		}
		role = GradedRole(frac)
	}

	aria := opts.AriaLabel
	if aria == "" {
		aria = fmt.Sprintf("%d of %d", f, n)
	}

	var b strings.Builder
	b.WriteString(svgOpen(int(math.Ceil(w)), int(math.Ceil(h)), aria, ""))
	b.WriteString(svgTitle(aria))
	filledC := cssColor(role)
	trackC := cssColor(opts.TrackRole)
	for i := range n {
		cx := 1 + r + float64(i)*(2*r+gap)
		color := trackC
		if i < f {
			color = filledC
		}
		fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`,
			fmtNum(cx), fmtNum(1+r), fmtNum(r), color)
	}
	b.WriteString("</svg>")
	return b.String()
}
