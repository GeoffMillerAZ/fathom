// oklch_terminal.go converts oklch() token values to sRGB hex for
// terminal color APIs (ANSI escapes, lipgloss.Color) — none of which have
// an OKLCH type. It backs EmitTerminalPalette (tokens.go); see that
// function's doc comment for the generated-file it feeds.
package tokens

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// OKLCHToHex converts a CSS `oklch(L C H[ / A])` string to a 6-digit
// `#rrggbb` sRGB hex color. Alpha, if present, is dropped: a terminal
// color role never carries translucency.
//
// A triple whose OKLab-derived linear sRGB falls outside [0,1] on any
// channel is brought into gamut by reducing CHROMA ONLY, holding L and H
// fixed, via bisection — the same strategy css-color-4 gamut mapping and
// browsers use. Post-hoc clipping each RGB channel independently (as a
// naive port might) shifts hue and lightness together and is visibly
// wrong on saturated tokens; this never does that.
func OKLCHToHex(s string) (string, error) {
	l, c, h, err := parseOKLCHTriple(s)
	if err != nil {
		return "", err
	}
	c = fitChroma(l, c, h)
	r, g, b := oklchToLinearSRGB(l, c, h)
	return fmt.Sprintf("#%02x%02x%02x", toByte(gammaEncode(r)), toByte(gammaEncode(g)), toByte(gammaEncode(b))), nil
}

// parseOKLCHTriple extracts L, C, H from `oklch(L C H[ / A])`. L and A
// accept 0..1 or a percentage; C accepts a raw number or a percentage of
// 0.4 (the CSS Color 4 reference range for chroma); H accepts a bare
// number or a `deg`-suffixed one.
func parseOKLCHTriple(s string) (l, c, h float64, err error) {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(strings.ToLower(trimmed), "oklch(") || !strings.HasSuffix(trimmed, ")") {
		return 0, 0, 0, fmt.Errorf("tokens: OKLCHToHex: %q is not oklch(L C H)", s)
	}
	body := trimmed[len("oklch(") : len(trimmed)-1]
	comps, _, _ := strings.Cut(body, "/") // alpha (if present) is dropped
	fields := strings.Fields(strings.ReplaceAll(comps, ",", " "))
	if len(fields) != 3 {
		return 0, 0, 0, fmt.Errorf("tokens: OKLCHToHex: %q is not oklch(L C H)", s)
	}
	num := func(x string, pctScale float64) (float64, error) {
		if strings.HasSuffix(x, "%") {
			n, err := strconv.ParseFloat(strings.TrimSuffix(x, "%"), 64)
			return n / 100 * pctScale, err
		}
		return strconv.ParseFloat(x, 64)
	}
	if l, err = num(fields[0], 1); err != nil {
		return 0, 0, 0, fmt.Errorf("tokens: OKLCHToHex: %q has a non-numeric lightness", s)
	}
	if c, err = num(fields[1], 0.4); err != nil {
		return 0, 0, 0, fmt.Errorf("tokens: OKLCHToHex: %q has a non-numeric chroma", s)
	}
	if h, err = num(strings.TrimSuffix(fields[2], "deg"), 360); err != nil {
		return 0, 0, 0, fmt.Errorf("tokens: OKLCHToHex: %q has a non-numeric hue", s)
	}
	return l, c, h, nil
}

// oklchToLinearSRGB is the OKLab → LMS → linear-sRGB step (Björn
// Ottosson's matrices, https://bottosson.github.io/posts/oklab/), applied
// with no clamping — callers decide what "out of range" means.
func oklchToLinearSRGB(l, c, h float64) (r, g, b float64) {
	rad := h * math.Pi / 180
	a, bb := c*math.Cos(rad), c*math.Sin(rad)
	lp := l + 0.3963377774*a + 0.2158037573*bb
	mp := l - 0.1055613458*a - 0.0638541728*bb
	sp := l - 0.0894841775*a - 1.2914855480*bb
	lc, mc, sc := lp*lp*lp, mp*mp*mp, sp*sp*sp
	r = 4.0767416621*lc - 3.3077115913*mc + 0.2309699292*sc
	g = -1.2684380046*lc + 2.6097574011*mc - 0.3413193965*sc
	b = -0.0041960863*lc - 0.7034186147*mc + 1.7076147010*sc
	return r, g, b
}

// gamutEps tolerates the float error bisection leaves at the boundary —
// small enough that no visible color error results, large enough that the
// search terminates in a fixed number of steps instead of chasing zero.
const gamutEps = 1e-6

func inGamut(l, c, h float64) bool {
	r, g, b := oklchToLinearSRGB(l, c, h)
	return r >= -gamutEps && r <= 1+gamutEps &&
		g >= -gamutEps && g <= 1+gamutEps &&
		b >= -gamutEps && b <= 1+gamutEps
}

// fitChroma returns the largest chroma <= c0 for which (l, c, h) is in the
// sRGB gamut, found by bisection over 30 steps (well past float64's useful
// precision for an 8-bit-per-channel target).
func fitChroma(l, c0, h float64) float64 {
	if c0 <= 0 || inGamut(l, c0, h) {
		return c0
	}
	lo, hi := 0.0, c0
	for i := 0; i < 30; i++ {
		mid := (lo + hi) / 2
		if inGamut(l, mid, h) {
			lo = mid
		} else {
			hi = mid
		}
	}
	return lo
}

// gammaEncode applies the sRGB transfer function, clamping the (already
// gamut-fitted, so this only catches float noise at the edges) result to
// [0,1].
func gammaEncode(v float64) float64 {
	switch {
	case v <= 0:
		return 0
	case v >= 1:
		return 1
	case v <= 0.0031308:
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

func toByte(v float64) int {
	n := int(math.Round(v * 255))
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}
