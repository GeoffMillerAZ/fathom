// heatmap.go — a themed 2D matrix heatmap, with optional cell truth states.
package viz

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// CellState is the truth state of one heatmap cell. A cell is a measured value,
// a measured zero, or one of three kinds of absence. Absence is never drawn as a
// low value: missing and unknown cells carry a hatch instead of a fill, and an
// error cell carries a danger cross, so none of them can be read as "0".
type CellState string

const (
	// CellValue is a finite measurement drawn on the opacity ramp. It is the zero
	// value of CellState, so an unset state defers to the numeric entry: NaN
	// becomes CellMissing, ±Inf becomes CellUnknown and 0 becomes CellZero.
	CellValue CellState = ""
	// CellZero is a measurement whose result is zero. It is drawn on the ramp
	// at 0 (at the ramp floor when every measured cell is zero, so there is no
	// span) and marked with a "0" glyph in cells of 14px or more, or with
	// ShowValues. A nonzero value never prints as "0" or "-0", so the glyph
	// keeps a measured zero distinct from a small value; below 14px without
	// ShowValues only the fill remains, which the ramp's lowest values share.
	CellZero CellState = "zero"
	// CellMissing means no measurement exists for the cell (not collected, not
	// applicable, not compiled). Drawn with a dashed outline and "/" hatch.
	CellMissing CellState = "missing"
	// CellUnknown means a measurement may exist but its value cannot be
	// determined or trusted (stale, unreadable, withheld). Drawn with a dotted
	// outline, cross-hatch and a "?" glyph.
	CellUnknown CellState = "unknown"
	// CellError means the attempt to measure failed. Drawn with a solid danger
	// outline and a danger cross.
	CellError CellState = "error"
)

// HeatmapCell declares one cell's truth state and the plain-text reason shown
// in its hover title (e.g. "not compiled", "index stale since 2026-09-01").
// Reason is escaped. For every state other than CellValue the numeric entry in
// rows is ignored.
type HeatmapCell struct {
	State  CellState
	Reason string
}

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
	// derived from the data (NaN cells ignored; in truth-state mode only value
	// and zero cells count).
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
	// TruthStates renders every cell with an explicit truth state (value, zero,
	// missing, unknown or error), a per-cell hover title naming the row, column,
	// state and reason, a <desc> state summary and a legend of the states
	// present. NaN entries become missing cells. Leave it false (and States nil)
	// to keep the original rendering byte for byte.
	TruthStates bool
	// States optionally declares per-cell truth states, row-major and parallel
	// to rows; a non-nil States implies TruthStates. The grid spans the union of
	// rows and States, so a state may describe a cell that has no numeric entry.
	// An unrecognized State renders as unknown, never as a value.
	States [][]HeatmapCell
}

// Heatmap renders rows (a 2D grid; each inner slice is one row, padded to the
// widest) as a themed heatmap: every cell is the Role color at an opacity
// proportional to its value's position in [Min,Max], with optional axis labels
// and per-cell values. A NaN value renders as an empty "hole" cell (a dashed
// outline) — use it for missing or infeasible points (e.g. an unconverged cell
// in an engagement-envelope map). Deterministic; all caller text is escaped.
//
// Set TruthStates or States to distinguish a measured zero from missing,
// unknown and error cells: absent cells are hatched or crossed rather than
// filled, each cell carries a title with its reason, and a legend and <desc>
// summary name the states present. See HeatmapCell and CellState.
func Heatmap(rows [][]float64, opts HeatmapOptions) string {
	if opts.TruthStates || opts.States != nil {
		return heatmapTruth(rows, opts)
	}
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
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="%s" fill-opacity="%s"/>`,
				cx, cy, cell, cell, cssColor(role), fmtNum(rampOpacity(v, vmin, span)))
			if opts.ShowValues {
				fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle" font-size="9" fill="%s">%s</text>`,
					cx+cell/2, cy+cell/2+3, cssColor("ink"), esc(fmtNum(v)))
			}
		}
	}
	writeHeatmapAxes(&b, opts, nRows, nCols, cell, gap, left, top, gridH)
	b.WriteString("</svg>")
	return b.String()
}

// rampOpacity maps v's position in [vmin, vmin+span] onto the 0.12..0.97
// opacity ramp. A zero span (every value equal) sits mid-ramp. The truth-state
// path uses truthOpacity instead.
func rampOpacity(v, vmin, span float64) float64 {
	norm := 0.5
	if span > 0 {
		norm = (v - vmin) / span
	}
	if norm < 0 {
		norm = 0
	} else if norm > 1 {
		norm = 1
	}
	return 0.12 + 0.85*norm
}

// writeHeatmapAxes draws the optional frontier curve and the axis labels, in
// the order the original renderer emitted them.
func writeHeatmapAxes(b *strings.Builder, opts HeatmapOptions, nRows, nCols, cell, gap, left, top, gridH int) {
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
			fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`, d.String(), cssColor(frole))
		}
	}
	for ri := 0; ri < nRows && ri < len(opts.YLabels); ri++ {
		cy := top + ri*(cell+gap) + cell/2 + 3
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="end" font-size="10" fill="%s">%s</text>`,
			left-8, cy, cssColor("ink-muted"), esc(opts.YLabels[ri]))
	}
	for ci := 0; ci < nCols && ci < len(opts.XLabels); ci++ {
		cx := left + ci*(cell+gap) + cell/2
		fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="10" fill="%s">%s</text>`,
			cx, top+gridH+15, cssColor("ink-muted"), esc(opts.XLabels[ci]))
	}
}

// ---- truth states ----------------------------------------------------------

// truthOrder is the fixed legend and summary order.
var truthOrder = []CellState{CellValue, CellZero, CellMissing, CellUnknown, CellError}

// truthCell is one cell after its declared state and numeric entry are
// reconciled. value is meaningful for CellValue only (CellZero is 0).
type truthCell struct {
	state  CellState
	value  float64
	reason string
}

func joinReason(derived, declared string) string {
	if declared == "" {
		return derived
	}
	return derived + "; " + declared
}

// resolveTruthCell reconciles the declared state at (r, c) with its numeric
// entry. Absence always wins over a number: a declared missing/unknown/error
// cell is never drawn on the ramp, whatever value accompanies it.
func resolveTruthCell(rows [][]float64, states [][]HeatmapCell, r, c int) truthCell {
	v := math.NaN()
	if r < len(rows) && c < len(rows[r]) {
		v = rows[r][c]
	}
	var decl HeatmapCell
	if r < len(states) && c < len(states[r]) {
		decl = states[r][c]
	}
	switch decl.State {
	case CellValue:
		switch {
		case math.IsNaN(v):
			return truthCell{CellMissing, 0, joinReason("no value supplied", decl.Reason)}
		case math.IsInf(v, 0):
			return truthCell{CellUnknown, 0, joinReason("value is not finite", decl.Reason)}
		case v == 0:
			return truthCell{CellZero, 0, decl.Reason}
		default:
			return truthCell{CellValue, v, decl.Reason}
		}
	case CellZero, CellMissing, CellUnknown, CellError:
		return truthCell{decl.State, 0, decl.Reason}
	default:
		return truthCell{CellUnknown, 0, joinReason(fmt.Sprintf("unrecognized state %q", string(decl.State)), decl.Reason)}
	}
}

// truthOpacity is rampOpacity for truth states, except that a measured zero
// with no span to place it on (every measured cell is zero, or the explicit
// range is degenerate) sits at the ramp floor rather than mid-ramp, where it
// would look like a grid of equal positive values.
func truthOpacity(v, vmin, span float64) float64 {
	if v == 0 && !(span > 0) {
		return 0.12
	}
	return rampOpacity(v, vmin, span)
}

// fmtTruthNum formats a measured number for a truth-state label, title,
// legend, summary or Validate message. It is fmtNum unless that would round a
// nonzero value to "0" or "-0"; such a value keeps two significant digits
// (0.004, -0.0041, 1e-07), so only a measured zero ever reads as 0.
func fmtTruthNum(v float64) string {
	if v == 0 { // including -0, which fmtNum prints as "-0"
		return "0"
	}
	s := fmtNum(v)
	if s == "0" || s == "-0" {
		return strconv.FormatFloat(v, 'g', 2, 64)
	}
	return s
}

// truthGlyphMin is the smallest cell edge that carries a text glyph ("0", "?").
const truthGlyphMin = 14

// truthFontSize scales cell text with the cell: 9px (the original value
// label size) below 40px cells, 12px from there up.
func truthFontSize(s int) int {
	if s >= 40 {
		return 12
	}
	return 9
}

// hatchPath returns "/" (up) or "\" (down) diagonal hatch segments inset one
// pixel inside the s×s square at (x, y), with integer endpoints.
func hatchPath(d *strings.Builder, x, y, s, step int, up bool) {
	x, y, s = x+1, y+1, s-2
	if s <= 0 || step <= 0 {
		return
	}
	if up { // u + w = k
		for k := step; k < 2*s; k += step {
			if k <= s {
				fmt.Fprintf(d, "M%d %d L%d %d", x, y+k, x+k, y)
			} else {
				fmt.Fprintf(d, "M%d %d L%d %d", x+k-s, y+s, x+s, y+k-s)
			}
		}
		return
	}
	for k := -s + step; k < s; k += step { // u - w = k
		if k >= 0 {
			fmt.Fprintf(d, "M%d %d L%d %d", x+k, y, x+s, y+s-k)
		} else {
			fmt.Fprintf(d, "M%d %d L%d %d", x, y-k, x+s+k, y+s)
		}
	}
}

// writeTruthMark draws one cell (or legend swatch) of edge s at (x, y) in its
// truth state. title, when non-empty, is already escaped and becomes the
// cell's first child so pointer users see the reason.
func writeTruthMark(b *strings.Builder, tc truthCell, x, y, s int, role string, op float64, showValue bool, title string) {
	t := ""
	if title != "" {
		t = "<title>" + title + "</title>"
	}
	switch tc.state {
	case CellValue, CellZero:
		fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="%s" fill-opacity="%s"`,
			x, y, s, s, cssColor(role), fmtNum(op))
		if t == "" {
			b.WriteString("/>")
		} else {
			b.WriteString(">" + t + "</rect>")
		}
		label := ""
		if tc.state == CellZero && (showValue || s >= truthGlyphMin) {
			label = "0"
		} else if tc.state == CellValue && showValue {
			label = fmtTruthNum(tc.value)
		}
		if label != "" {
			fs := truthFontSize(s)
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="%d" fill="%s">%s</text>`,
				x+s/2, y+s/2+fs/3, fs, cssColor("ink"), esc(label))
		}
	case CellMissing, CellUnknown:
		// Hatch spacing grows with the cell so a large cell reads as texture,
		// not as a dense fill: about six "/" lines, or four each way for "?".
		dash, glyph := "2 2", ""
		var d strings.Builder
		hatchPath(&d, x, y, s, max(5, s/6), true)
		if tc.state == CellUnknown {
			dash = "1 2"
			d.Reset()
			hatchPath(&d, x, y, s, max(7, s/4), true)
			hatchPath(&d, x, y, s, max(7, s/4), false)
			if s >= truthGlyphMin {
				glyph = "?"
			}
		}
		fmt.Fprintf(b, `<g>%s<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="none" pointer-events="all" stroke="%s" stroke-dasharray="%s"/>`,
			t, x, y, s, s, cssColor("ink-muted"), dash)
		if d.Len() > 0 {
			fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="1"/>`, d.String(), cssColor("ink-muted"))
		}
		if glyph != "" {
			gs := truthFontSize(s) + 1
			if plate := gs + 4; s >= 2*plate {
				// A surface plate keeps the glyph legible over the cross-hatch.
				fmt.Fprintf(b, `<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="%s"/>`,
					x+s/2-plate/2, y+s/2-plate/2, plate, plate, cssColor("surface"))
			}
			fmt.Fprintf(b, `<text x="%d" y="%d" text-anchor="middle" font-size="%d" font-weight="600" fill="%s">%s</text>`,
				x+s/2, y+s/2+gs*2/5, gs, cssColor("ink"), glyph)
		}
		b.WriteString("</g>")
	case CellError:
		in := max(3, s/5)
		fmt.Fprintf(b, `<g>%s<rect x="%d" y="%d" width="%d" height="%d" rx="2" fill="none" pointer-events="all" stroke="%s" stroke-width="1.5"/>`,
			t, x, y, s, s, cssColor("danger"))
		if s > 2*in {
			fmt.Fprintf(b, `<path d="M%d %d L%d %d M%d %d L%d %d" fill="none" stroke="%s" stroke-width="1.5" stroke-linecap="round"/>`,
				x+in, y+in, x+s-in, y+s-in, x+s-in, y+in, x+in, y+s-in, cssColor("danger"))
		}
		b.WriteString("</g>")
	}
}

// truthLabel is the word a title, legend and summary use for a state.
func truthLabel(s CellState) string {
	if s == CellValue {
		return "value"
	}
	return string(s)
}

// truncateRunes bounds legend text without splitting a UTF-8 sequence.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

func heatmapAxisName(labels []string, i int, kind string) string {
	if i < len(labels) && strings.TrimSpace(labels[i]) != "" {
		return labels[i]
	}
	return fmt.Sprintf("%s %d", kind, i+1)
}

type legendItem struct {
	state CellState
	label string
	width int
}

// heatmapTruth is the truth-state renderer behind Heatmap.
func heatmapTruth(rows [][]float64, opts HeatmapOptions) string {
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
	aria := opts.AriaLabel
	if aria == "" {
		aria = "heatmap"
	}

	nRows := max(len(rows), len(opts.States))
	nCols := 0
	for _, r := range rows {
		nCols = max(nCols, len(r))
	}
	for _, r := range opts.States {
		nCols = max(nCols, len(r))
	}
	if nRows == 0 || nCols == 0 {
		return svgOpen(8, 8, aria, "") + svgTitle(aria) + "<desc>No cells.</desc></svg>"
	}

	grid := make([][]truthCell, nRows)
	counts := map[CellState]int{}
	reasons := map[CellState]string{}
	uniform := map[CellState]bool{}
	first := true
	var dmin, dmax float64
	for ri := range grid {
		grid[ri] = make([]truthCell, nCols)
		for ci := range grid[ri] {
			tc := resolveTruthCell(rows, opts.States, ri, ci)
			grid[ri][ci] = tc
			if counts[tc.state] == 0 {
				reasons[tc.state], uniform[tc.state] = tc.reason, true
			} else if reasons[tc.state] != tc.reason {
				uniform[tc.state] = false
			}
			counts[tc.state]++
			if tc.state == CellValue || tc.state == CellZero {
				if first {
					dmin, dmax, first = tc.value, tc.value, false
				} else {
					dmin, dmax = math.Min(dmin, tc.value), math.Max(dmax, tc.value)
				}
			}
		}
	}
	vmin, vmax := opts.Min, opts.Max
	if vmin == 0 && vmax == 0 {
		vmin, vmax = dmin, dmax
	}
	span := vmax - vmin
	rangeText := fmtTruthNum(vmin)
	if span != 0 {
		rangeText += "–" + fmtTruthNum(vmax)
	}

	left := 8
	if len(opts.YLabels) > 0 {
		left = 70
	}
	bottom := 8
	if len(opts.XLabels) > 0 {
		bottom = 24
	}
	const top, right = 8, 8
	const sw, rowH, itemGap = 14, 20, 14
	gridW := nCols*cell + (nCols-1)*gap
	gridH := nRows*cell + (nRows-1)*gap

	// Legend: one entry per state present, in truthOrder; a state whose cells
	// all share one reason names it. Laid out in rows that wrap at the wider of
	// the grid and 220px; the SVG widens for an entry that alone is wider.
	var items []legendItem
	for _, s := range truthOrder {
		if counts[s] == 0 {
			continue
		}
		label := truthLabel(s)
		switch {
		case s == CellValue:
			label += " " + rangeText
		case uniform[s] && reasons[s] != "":
			label += ": " + truncateRunes(reasons[s], 40)
		}
		swatch := sw
		if s == CellValue {
			swatch = 3 * 10
		}
		items = append(items, legendItem{s, label, swatch + 5 + 6*utf8.RuneCountInString(label)})
	}
	avail := max(gridW, 220)
	var lines [][]legendItem
	lineW, widest := 0, 0
	for _, it := range items {
		if len(lines) == 0 || lineW+itemGap+it.width > avail && lineW > 0 {
			lines = append(lines, nil)
			lineW = 0
		} else {
			lineW += itemGap
		}
		lines[len(lines)-1] = append(lines[len(lines)-1], it)
		lineW += it.width
		widest = max(widest, lineW)
	}
	legendTop := top + gridH + bottom + 2
	w := max(left+gridW+right, left+widest+right)
	h := legendTop + len(lines)*rowH + 4

	var b strings.Builder
	b.WriteString(svgOpen(w, h, aria, ""))
	b.WriteString(svgTitle(aria))
	var summary []string
	for _, s := range truthOrder {
		if counts[s] == 0 {
			continue
		}
		part := fmt.Sprintf("%s %d", truthLabel(s), counts[s])
		if s == CellValue {
			part += " (range " + rangeText + ")"
		}
		summary = append(summary, part)
	}
	fmt.Fprintf(&b, "<desc>%s</desc>", esc(fmt.Sprintf("%d cells: %s.", nRows*nCols, strings.Join(summary, "; "))))

	for ri, r := range grid {
		cy := top + ri*(cell+gap)
		rowName := heatmapAxisName(opts.YLabels, ri, "row")
		for ci, tc := range r {
			cx := left + ci*(cell+gap)
			state := truthLabel(tc.state)
			switch tc.state {
			case CellValue:
				state = fmtTruthNum(tc.value)
			case CellZero:
				state = "0 (zero)"
			}
			title := rowName + ", " + heatmapAxisName(opts.XLabels, ci, "column") + ": " + state
			if tc.reason != "" {
				title += " — " + tc.reason
			}
			op := 0.0
			if tc.state == CellValue || tc.state == CellZero {
				op = truthOpacity(tc.value, vmin, span)
			}
			writeTruthMark(&b, tc, cx, cy, cell, role, op, opts.ShowValues, esc(title))
		}
	}
	writeHeatmapAxes(&b, opts, nRows, nCols, cell, gap, left, top, gridH)

	for li, line := range lines {
		x, y := left, legendTop+li*rowH
		for _, it := range line {
			if it.state == CellValue {
				for i, op := range []float64{0.12, 0.545, 0.97} {
					fmt.Fprintf(&b, `<rect x="%d" y="%d" width="10" height="%d" fill="%s" fill-opacity="%s"/>`,
						x+10*i, y+2, sw, cssColor(role), fmtNum(op))
				}
			} else {
				op := truthOpacity(0, vmin, span)
				writeTruthMark(&b, truthCell{state: it.state}, x, y+2, sw, role, op, false, "")
			}
			swatch := sw
			if it.state == CellValue {
				swatch = 30
			}
			fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="10" fill="%s">%s</text>`,
				x+swatch+5, y+13, cssColor("ink-muted"), esc(it.label))
			x += it.width + itemGap
		}
	}
	b.WriteString("</svg>")
	return b.String()
}

// Validate reports input that Heatmap would have to reinterpret: an invalid
// Role or FrontierRole (rendered as the fallback role), an inverted explicit
// Min/Max range, a non-finite numeric entry, an unrecognized cell State
// (rendered as unknown) and a zero-state cell whose numeric entry is a nonzero
// number (rendered as 0). Heatmap itself stays total and never refuses; wire
// Validate into the caller's tests or admission path to fail before rendering.
// Errors are reported in row-major order.
func (o HeatmapOptions) Validate(rows [][]float64) error {
	var errs []error
	for _, r := range []struct{ name, role string }{{"Role", o.Role}, {"FrontierRole", o.FrontierRole}} {
		if r.role != "" && !roleRe.MatchString(r.role) {
			errs = append(errs, fmt.Errorf("viz: heatmap %s %q is not a token role name", r.name, r.role))
		}
	}
	if (o.Min != 0 || o.Max != 0) && o.Min > o.Max {
		errs = append(errs, fmt.Errorf("viz: heatmap Min %s exceeds Max %s", fmtTruthNum(o.Min), fmtTruthNum(o.Max)))
	}
	nRows := max(len(rows), len(o.States))
	for ri := 0; ri < nRows; ri++ {
		nCols := 0
		if ri < len(rows) {
			nCols = len(rows[ri])
		}
		if ri < len(o.States) {
			nCols = max(nCols, len(o.States[ri]))
		}
		for ci := 0; ci < nCols; ci++ {
			v := math.NaN()
			if ri < len(rows) && ci < len(rows[ri]) {
				v = rows[ri][ci]
			}
			var decl HeatmapCell
			if ri < len(o.States) && ci < len(o.States[ri]) {
				decl = o.States[ri][ci]
			}
			at := fmt.Sprintf("row %d, column %d", ri+1, ci+1)
			switch decl.State {
			case CellValue, CellMissing, CellUnknown, CellError:
			case CellZero:
				if !math.IsNaN(v) && v != 0 {
					errs = append(errs, fmt.Errorf("viz: heatmap %s is declared zero but carries %s", at, fmtTruthNum(v)))
				}
			default:
				errs = append(errs, fmt.Errorf("viz: heatmap %s has unrecognized state %q", at, string(decl.State)))
			}
			if math.IsInf(v, 0) && decl.State == CellValue {
				errs = append(errs, fmt.Errorf("viz: heatmap %s value is not finite", at))
			}
		}
	}
	return errors.Join(errs...)
}
