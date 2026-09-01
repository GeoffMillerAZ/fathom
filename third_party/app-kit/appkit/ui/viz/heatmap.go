// heatmap.go — a themed 2D matrix heatmap.
package viz

import (
	"fmt"
	"math"
	"strings"
)

// HeatmapOptions configures Heatmap.
type HeatmapOptions struct {
	// Role is the semantic color role for cell intensity (default "accent").
	// Every cell is this one role at a modulated opacity, so the heatmap stays
	// fully themeable (light/dark) — no literal colors, per the kit token rule.
	Role string
	// Cell is the cell edge length in px (default 26). Gap is the spacing
	// between cells in px (default 2).
	Cell, Gap int
	// XLabels are column labels along the bottom; YLabels are row labels along
	// the left (top-to-bottom). Either may be empty/short — extras are ignored,
	// missing ones are simply not drawn.
	XLabels, YLabels []string
	// Min/Max set the value range for the opacity ramp. If both are 0 they are
	// derived from the data (NaN cells ignored).
	Min, Max float64
	// ShowValues renders each cell's numeric value centered in the cell.
	ShowValues bool
	// Frontier, when non-empty, draws a boundary curve over the grid: Frontier[ci]
	// is the fractional ROW coordinate of the curve at column ci (0 = top edge,
	// len(rows) = bottom edge). Use it for an engagement frontier (latest-intercept),
	// a compliance threshold, or an SLA edge over a coverage map. A NaN entry breaks
	// the line. FrontierRole is the curve's color role (default "caution").
	Frontier     []float64
	FrontierRole string
	AriaLabel    string
}

// Heatmap renders rows (a 2D grid; each inner slice is one row, padded to the
// widest) as a themed heatmap: every cell is the Role color at an opacity
// proportional to its value's position in [Min,Max], with optional axis labels
// and per-cell values. A NaN value renders as an empty "hole" cell (a dashed
// outline) — use it for missing or infeasible points (e.g. an unconverged cell
// in an engagement-envelope map). Deterministic; all caller text is escaped.
func Heatmap(rows [][]float64, opts HeatmapOptions) string {
	role := opts.Role
	if role == "" {
		role = "accent"
	}
	cell := opts.Cell
	if cell <= 0 {
		cell = 26
	}
	gap := opts.Gap
	if gap < 0 {
		gap = 0
	} else if gap == 0 {
		gap = 2
	}

	nRows := len(rows)
	nCols := 0
	for _, r := range rows {
		if len(r) > nCols {
			nCols = len(r)
		}
	}
	aria := opts.AriaLabel
	if aria == "" {
		aria = "heatmap"
	}
	if nRows == 0 || nCols == 0 {
		return svgOpen(8, 8, aria, "") + svgTitle(aria) + "</svg>"
	}

	// derive the value range (ignoring NaN) when not supplied
	vmin, vmax := opts.Min, opts.Max
	if vmin == 0 && vmax == 0 {
		first := true
		for _, r := range rows {
			for _, v := range r {
				if math.IsNaN(v) {
					continue
				}
				if first {
					vmin, vmax, first = v, v, false
				} else {
					if v < vmin {
						vmin = v
					}
					if v > vmax {
						vmax = v
					}
				}
			}
		}
	}
	span := vmax - vmin

	left := 8
	if len(opts.YLabels) > 0 {
		left = 70
	}
	bottom := 8
	if len(opts.XLabels) > 0 {
		bottom = 24
	}
	const top, right = 8, 8
	gridW := nCols*cell + (nCols-1)*gap
	gridH := nRows*cell + (nRows-1)*gap
	w := left + gridW + right
	h := top + gridH + bottom

	var b strings.Builder
	b.WriteString(svgOpen(w, h, aria, ""))
	b.WriteString(svgTitle(aria))

	for ri, r := range rows {
		cy := top + ri*(cell+gap)
		for ci := 0; ci < nCols; ci++ {
			cx := left + ci*(cell+gap)
			v := math.NaN()
			if ci < len(r) {
				v = r[ci]
			}
			if math.IsNaN(v) {
				fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="none" stroke="%s" stroke-opacity="0.4" stroke-dasharray="2 2"/>`,
					cx, cy, cell, cell, cssColor("ink-muted"))
				continue
			}
			norm := 0.5
			if span > 0 {
				norm = (v - vmin) / span
			}
			if norm < 0 {
				norm = 0
			} else if norm > 1 {
				norm = 1
			}
			op := 0.12 + 0.85*norm
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="%s" fill-opacity="%s"/>`,
				cx, cy, cell, cell, cssColor(role), fmtNum(op))
			if opts.ShowValues {
				fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle" font-size="9" fill="%s">%s</text>`,
					cx+cell/2, cy+cell/2+3, cssColor("ink"), esc(fmtNum(v)))
			}
		}
	}
	if len(opts.Frontier) > 0 {
		frole := opts.FrontierRole
		if frole == "" {
			frole = "caution"
		}
		var d strings.Builder
		started := false
		for ci := 0; ci < nCols && ci < len(opts.Frontier); ci++ {
			fy := opts.Frontier[ci]
			if math.IsNaN(fy) {
				started = false
				continue
			}
			x := left + ci*(cell+gap) + cell/2
			y := top + int(fy*float64(cell+gap)+0.5)
			if !started {
				fmt.Fprintf(&d, "M%d %d", x, y)
				started = true
			} else {
				fmt.Fprintf(&d, " L%d %d", x, y)
			}
		}
		if d.Len() > 0 {
			fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`, d.String(), cssColor(frole))
		}
	}
	for ri := 0; ri < nRows && ri < len(opts.YLabels); ri++ {
		cy := top + ri*(cell+gap) + cell/2 + 3
		fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="end" font-size="10" fill="%s">%s</text>`,
			left-8, cy, cssColor("ink-muted"), esc(opts.YLabels[ri]))
	}
	for ci := 0; ci < nCols && ci < len(opts.XLabels); ci++ {
		cx := left + ci*(cell+gap) + cell/2
		fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" fill="%s">%s</text>`,
			cx, top+gridH+15, cssColor("ink-muted"), esc(opts.XLabels[ci]))
	}
	b.WriteString("</svg>")
	return b.String()
}
