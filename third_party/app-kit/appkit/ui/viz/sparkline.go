// sparkline.go — inline trend sparkline.
package viz

import (
	"fmt"
	"math"
	"strings"
)

// SparklineOptions configures a Sparkline render.
type SparklineOptions struct {
	// W is the SVG width in pixels. Default 120.
	W int
	// H is the SVG height in pixels. Default 28.
	H int
	// StrokeRole is the token role for the line stroke. Default "accent".
	StrokeRole string
	// FillRole is the token role for the area fill. Empty = no fill.
	FillRole string
	// AriaLabel is the accessible label. Defaults to "sparkline".
	AriaLabel string
}

func (o *SparklineOptions) withDefaults() SparklineOptions {
	out := *o
	if out.W <= 0 {
		out.W = 120
	}
	if out.H <= 0 {
		out.H = 28
	}
	if out.StrokeRole == "" {
		out.StrokeRole = "accent"
	}
	if out.AriaLabel == "" {
		out.AriaLabel = "sparkline"
	}
	return out
}

// Sparkline returns an SVG string for an inline trend line.
//
// NaN values are filtered. len==0 and len==1 render an empty/flat midline
// respectively. All-equal values render a flat midline.
func Sparkline(values []float64, o SparklineOptions) string {
	opts := o.withDefaults()

	// Filter NaN.
	clean := make([]float64, 0, len(values))
	for _, v := range values {
		if !math.IsNaN(v) {
			clean = append(clean, v)
		}
	}

	var b strings.Builder
	b.WriteString(svgOpen(opts.W, opts.H, opts.AriaLabel, ""))
	b.WriteString(svgTitle(opts.AriaLabel))

	if len(clean) == 0 {
		b.WriteString("</svg>")
		return b.String()
	}

	pad := 1.5 // pixel inset so stroke doesn't clip at edges
	w := float64(opts.W)
	h := float64(opts.H)

	// Compute min/max.
	mn, mx := clean[0], clean[0]
	for _, v := range clean {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}

	// Map value → y coordinate (inverted: high value = low y).
	yFor := func(v float64) float64 {
		if mx == mn {
			return pad + (h-2*pad)/2 // flat midline
		}
		return pad + (h-2*pad)*(1-(v-mn)/(mx-mn))
	}

	// Map index → x coordinate.
	n := len(clean)
	xFor := func(i int) float64 {
		if n == 1 {
			return w / 2
		}
		return pad + (w-2*pad)*float64(i)/float64(n-1)
	}

	strokeColor := cssColor(opts.StrokeRole)

	// Build polyline points.
	pts := make([]string, n)
	for i, v := range clean {
		pts[i] = fmt.Sprintf("%s,%s", fmtNum(xFor(i)), fmtNum(yFor(v)))
	}
	pointsStr := strings.Join(pts, " ")

	// Optional fill area: close the path back along the bottom edge.
	if opts.FillRole != "" {
		fillColor := cssColor(opts.FillRole)
		// Build closed path for fill: line → bottom-right → bottom-left.
		var path strings.Builder
		path.WriteString("M ")
		for i, v := range clean {
			if i > 0 {
				path.WriteString(" L ")
			}
			path.WriteString(fmt.Sprintf("%s %s", fmtNum(xFor(i)), fmtNum(yFor(v))))
		}
		// Close bottom.
		path.WriteString(fmt.Sprintf(" L %s %s", fmtNum(xFor(n-1)), fmtNum(h-pad)))
		path.WriteString(fmt.Sprintf(" L %s %s", fmtNum(xFor(0)), fmtNum(h-pad)))
		path.WriteString(" Z")
		fmt.Fprintf(&b, `<path d="%s" fill="%s" fill-opacity="0.15" stroke="none"/>`,
			path.String(), fillColor)
	}

	// Polyline stroke.
	fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>`,
		pointsStr, strokeColor)

	b.WriteString("</svg>")
	return b.String()
}
