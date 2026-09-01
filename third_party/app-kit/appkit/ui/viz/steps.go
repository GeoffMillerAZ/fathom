package viz

import (
	"fmt"
	"math"
	"strings"
)

// StepState is a step's progress state.
type StepState string

const (
	StepDone     StepState = "done"    // completed
	StepCurrent  StepState = "current" // in progress now
	StepUpcoming StepState = ""        // not started (zero value)
)

// Step is one stage in a Steps progression.
type Step struct {
	Label string
	State StepState
}

// StepsOptions configures a Steps render.
type StepsOptions struct {
	// W is the SVG width. Default 480.
	W int
	// DoneRole colors completed steps + the connectors behind them.
	// Default "accent".
	DoneRole string
	// CurrentRole colors the current step. Default "accent".
	CurrentRole string
	// TrackRole colors upcoming steps + connectors. Default "line".
	TrackRole string
	// AriaLabel is the accessible label.
	AriaLabel string
}

func (o *StepsOptions) withDefaults() StepsOptions {
	out := *o
	if out.W <= 0 {
		out.W = 480
	}
	if out.DoneRole == "" {
		out.DoneRole = "accent"
	}
	if out.CurrentRole == "" {
		out.CurrentRole = "accent"
	}
	if out.TrackRole == "" {
		out.TrackRole = "line"
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "progress steps"
	}
	return out
}

// Steps renders a horizontal progress stepper — a case/workflow lifecycle at a
// glance (e.g. filed → discovery → trial → resolved), with done / current /
// upcoming states. Connectors behind completed steps take the done color.
// Deterministic; labels escaped; roles validated.
func Steps(steps []Step, o StepsOptions) string {
	opts := o.withDefaults()
	n := len(steps)

	const r, curR, labelGap = 9.0, 11.0, 13.0
	margin := curR + 4
	w := float64(opts.W)
	cy := curR + 2
	totalH := int(cy + curR + labelGap + 6)

	var b strings.Builder
	b.WriteString(svgOpen(opts.W, totalH, opts.AriaLabel, ""))
	b.WriteString(svgTitle(opts.AriaLabel))
	if n == 0 {
		b.WriteString("</svg>")
		return b.String()
	}

	// X position of step i.
	xOf := func(i int) float64 {
		if n == 1 {
			return w / 2
		}
		return margin + float64(i)*(w-2*margin)/float64(n-1)
	}

	done := cssColor(opts.DoneRole)
	track := cssColor(opts.TrackRole)
	current := cssColor(opts.CurrentRole)
	surface := cssColor("surface")

	// Connectors first (under the nodes). A segment is "done"-colored when the
	// step on its left is completed.
	for i := 0; i+1 < n; i++ {
		color := track
		if steps[i].State == StepDone {
			color = done
		}
		fmt.Fprintf(&b, `<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="2"/>`,
			fmtNum(xOf(i)+r), fmtNum(cy), fmtNum(xOf(i+1)-r), fmtNum(cy), color)
	}

	for i, s := range steps {
		x := xOf(i)
		switch s.State {
		case StepDone:
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, fmtNum(x), fmtNum(cy), fmtNum(r), done)
			// check glyph
			fmt.Fprintf(&b,
				`<path d="M%s %s %s %s %s %s" fill="none" stroke="%s" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/>`,
				fmtNum(x-3.5), fmtNum(cy), fmtNum(x-1), fmtNum(cy+2.5), fmtNum(x+3.5), fmtNum(cy-3), surface)
		case StepCurrent:
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="2.5"/>`,
				fmtNum(x), fmtNum(cy), fmtNum(curR), surface, current)
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="3.2" fill="%s"/>`, fmtNum(x), fmtNum(cy), current)
		default: // upcoming
			fmt.Fprintf(&b, `<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="2"/>`,
				fmtNum(x), fmtNum(cy), fmtNum(r), surface, track)
		}

		// Label below, anchored to stay inside at the ends.
		anchor, lx := "middle", x
		if i == 0 {
			anchor, lx = "start", math.Max(0, x-curR)
		} else if i == n-1 {
			anchor, lx = "end", math.Min(w, x+curR)
		}
		labelRole := "ink-muted"
		if s.State == StepCurrent {
			labelRole = "ink"
		}
		fmt.Fprintf(&b,
			`<text x="%s" y="%s" text-anchor="%s" fill="%s" font-size="10">%s</text>`,
			fmtNum(lx), fmtNum(cy+curR+labelGap), anchor, cssColor(labelRole), esc(s.Label))
	}

	b.WriteString("</svg>")
	return b.String()
}
