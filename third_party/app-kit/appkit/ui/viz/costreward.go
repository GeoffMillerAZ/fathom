package viz

import (
	"fmt"
	"strings"
)

// CostRewardItem is one row: a reward (extends right) versus a cost (extends
// left) from a shared center axis. Both magnitudes are non-negative.
type CostRewardItem struct {
	Label  string
	Reward float64
	Cost   float64
}

// CostRewardOptions configures a CostReward render.
type CostRewardOptions struct {
	W          int     // SVG width. Default 420.
	RowH       int     // row height. Default 24.
	Gap        int     // gap between rows. Default 6.
	LabelW     int     // left label column. Default 90.
	Max        float64 // half-axis scale; 0 derives from the largest reward/cost.
	Prefix     string  // value-label prefix (e.g. "$")
	RewardRole string  // default "positive"
	CostRole   string  // default "caution"
	AriaLabel  string
	Fluid      bool // fill the container width: the W viewBox scales uniformly, text included. Default off (fixed pixels).
}

func (o *CostRewardOptions) withDefaults() CostRewardOptions {
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
		out.LabelW = 90
	}
	if out.RewardRole == "" {
		out.RewardRole = "positive"
	}
	if out.CostRole == "" {
		out.CostRole = "caution"
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "cost vs reward"
	}
	return out
}

// CostReward renders a diverging bar chart: reward right (positive role), cost
// left (caution role), from a center axis — a per-row reward-vs-cost read.
// Magnitudes clamp at 0; bars scale to Max or the largest magnitude. Empty-safe,
// escaped, role-validated, deterministic.
func CostReward(items []CostRewardItem, o CostRewardOptions) string {
	opts := o.withDefaults()
	n := len(items)
	rowH := float64(opts.RowH)
	gap := float64(opts.Gap)
	totalH := opts.RowH
	if n > 0 {
		totalH = n*opts.RowH + (n-1)*opts.Gap
	}
	w := float64(opts.W)

	var b strings.Builder
	b.WriteString(chartOpen(opts.W, totalH, opts.AriaLabel, opts.Fluid))
	b.WriteString(svgTitle(opts.AriaLabel))
	if n == 0 {
		b.WriteString("</svg>")
		return b.String()
	}

	maxVal := opts.Max
	if maxVal <= 0 {
		for _, it := range items {
			for _, v := range []float64{it.Reward, it.Cost} {
				if v > maxVal {
					maxVal = v
				}
			}
		}
	}
	if maxVal <= 0 {
		maxVal = 1
	}

	labelW := float64(opts.LabelW)
	plotX := labelW + 6
	center := (plotX + w) / 2
	half := (w - plotX) / 2

	clampLen := func(v float64) float64 {
		if v < 0 {
			v = 0
		}
		return v / maxVal * half
	}
	rewardC := cssColor(opts.RewardRole)
	costC := cssColor(opts.CostRole)
	ink := cssColor("ink-muted")

	// Center axis.
	fmt.Fprintf(&b, `<line x1="%s" y1="0" x2="%s" y2="%s" stroke="%s" stroke-width="1"/>`,
		fmtNum(center), fmtNum(center), fmtNum(float64(totalH)), cssColor("line"))

	for i, it := range items {
		y := float64(i) * (rowH + gap)
		barY := y + rowH*0.2
		barH := rowH * 0.6
		mid := y + rowH/2

		fmt.Fprintf(&b, `<text x="0" y="%s" dominant-baseline="middle" fill="%s" font-size="11">%s</text>`,
			fmtNum(mid), ink, esc(it.Label))

		// Cost (left of center).
		cl := clampLen(it.Cost)
		if cl > 0 {
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"><title>%s cost: %s%s</title></rect>`,
				fmtNum(center-cl), fmtNum(barY), fmtNum(cl), fmtNum(barH), costC,
				esc(it.Label), esc(opts.Prefix), fmtNum(it.Cost))
		}
		// Reward (right of center).
		rl := clampLen(it.Reward)
		if rl > 0 {
			fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="2" fill="%s"><title>%s reward: %s%s</title></rect>`,
				fmtNum(center), fmtNum(barY), fmtNum(rl), fmtNum(barH), rewardC,
				esc(it.Label), esc(opts.Prefix), fmtNum(it.Reward))
		}
	}

	b.WriteString("</svg>")
	return b.String()
}
