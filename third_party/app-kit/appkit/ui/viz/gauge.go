package viz

import (
	"fmt"
	"math"
	"strings"

	"github.com/rivo/uniseg"
)

const (
	gaugeCaptionLineHeight  = 14
	gaugeCaptionLineReserve = 16
	gaugeCaptionFontSize    = 11
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
	// Label is the caption below the gauge. Long captions wrap conservatively
	// at grapheme boundaries. Put explanatory prose and source context in an
	// HTML figure (for example, ui/vizframe); font geometry needs browser review.
	Label string
	// Fluid fills the container's width instead of drawing at W pixels:
	// the chart scales uniformly from its W×height viewBox (text included)
	// and the height follows. Off by default (fixed pixel size).
	Fluid bool
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

// gaugeCaptionGraphemeBudget reserves one font em for each grapheme cluster in
// the drawable width. It is deliberately conservative for wide scripts and
// emoji, but does not measure a browser's actual font geometry.
func gaugeCaptionGraphemeBudget(w, pad float64) int {
	budget := int(math.Floor((w - 2*pad) / gaugeCaptionFontSize))
	if budget < 1 {
		return 1
	}
	return budget
}

// wrapGaugeCaption splits a caption at word boundaries into SVG text runs. It
// counts grapheme clusters, not bytes or runes, so an unbreakable Unicode word
// is retained without splitting combining sequences or ZWJ emoji. The caller
// supplies a conservative width budget; a single cluster may still exceed it,
// and browser review remains the font-geometry oracle.
func wrapGaugeCaption(caption string, maxGraphemes int) []string {
	words := strings.Fields(caption)
	if len(words) == 0 {
		return nil
	}

	var lines []string
	var line []string
	flush := func() {
		if len(line) > 0 {
			lines = append(lines, strings.Join(line, ""))
			line = nil
		}
	}
	appendWord := func(word []string) {
		for len(word) > maxGraphemes {
			lines = append(lines, strings.Join(word[:maxGraphemes], ""))
			word = word[maxGraphemes:]
		}
		if len(word) > 0 {
			line = append(line, word...)
		}
	}

	for _, word := range words {
		var graphemes []string
		segmenter := uniseg.NewGraphemes(word)
		for segmenter.Next() {
			graphemes = append(graphemes, segmenter.Str())
		}
		if len(line) == 0 {
			appendWord(graphemes)
			continue
		}
		if len(line)+1+len(graphemes) <= maxGraphemes {
			line = append(line, " ")
			line = append(line, graphemes...)
			continue
		}
		flush()
		appendWord(graphemes)
	}
	flush()
	return lines
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
	captionLines := wrapGaugeCaption(opts.Label, gaugeCaptionGraphemeBudget(w, pad))
	captionLineCount := 0
	if opts.Label != "" {
		captionLineCount = 1
		if len(captionLines) > 1 {
			captionLineCount = len(captionLines)
		}
	}
	totalH := int(cy + stroke/2 + 6 + float64(captionLineCount*gaugeCaptionLineReserve))

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
	b.WriteString(chartOpen(opts.W, totalH, opts.AriaLabel, opts.Fluid))
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
		captionY := fmtNum(cy + stroke/2 + 12)
		if len(captionLines) <= 1 {
			// Preserve the compact, existing SVG for normal one-line captions.
			fmt.Fprintf(&b,
				`<text x="%s" y="%s" text-anchor="middle" fill="%s" font-size="11">%s</text>`,
				fmtNum(cx), captionY, cssColor("ink-muted"), esc(opts.Label))
		} else {
			// tspans concatenate in SVG textContent, so retain the normalized
			// complete caption in desc without adding spaces to split words.
			b.WriteString(`<desc>` + esc(strings.Join(strings.Fields(opts.Label), " ")) + `</desc>`)
			fmt.Fprintf(&b, `<text text-anchor="middle" fill="%s" font-size="11">`, cssColor("ink-muted"))
			for i, line := range captionLines {
				if i == 0 {
					fmt.Fprintf(&b, `<tspan x="%s" y="%s">%s</tspan>`, fmtNum(cx), captionY, esc(line))
					continue
				}
				fmt.Fprintf(&b, `<tspan x="%s" dy="%d">%s</tspan>`, fmtNum(cx), gaugeCaptionLineHeight, esc(line))
			}
			b.WriteString(`</text>`)
		}
	}

	b.WriteString("</svg>")
	return b.String()
}
