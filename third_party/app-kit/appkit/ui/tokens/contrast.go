// contrast.go — WCAG contrast measurement for token values.
//
// The kit's palette conventions are enforced by review (schema.cue), and review
// cannot see a contrast ratio: a reviewer reads `oklch(0.70 0.14 80)` and reads
// "caution", not "2.60:1 on surface". These helpers make the static half of the
// measure-never-read rule runnable with no browser and no Node (D2) — given two
// token values, they return the number.
//
// The composited half (alpha, ancestor opacity, color-mix, marks stacked on
// marks) still needs a real page: the procedure is in
// docs/theming-hazards.md, and ContrastOver covers its most common case.
//
// Values that cannot be resolved return an error rather than a ratio. A
// measurement you could not make must fail loudly — silently defaulting to a
// pass is the failure mode this file exists to prevent.
package tokens

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Contrast floors. Anything that carries meaning — a chart mark, a status dot,
// a legend swatch, a focus ring — clears MeaningFloor against the background it
// is actually drawn on. Text clears TextFloor (WCAG AA; large text may use
// MeaningFloor).
const (
	MeaningFloor = 3.0
	TextFloor    = 4.5
)

// srgb is a gamma-encoded sRGB color with alpha, each channel in 0..1.
type srgb struct{ R, G, B, A float64 }

// Contrast returns the WCAG 2.1 contrast ratio (1.0..21.0) between two token
// color values, in either order. Values may be `oklch(L C H)` (the kit's
// convention) or hex (`#rgb`, `#rrggbb`, `#rrggbbaa`).
//
// A translucent value is an error: a color with alpha has no contrast until it
// is composited over something. Use ContrastOver.
func Contrast(a, b string) (float64, error) {
	ca, err := parseColor(a)
	if err != nil {
		return 0, err
	}
	cb, err := parseColor(b)
	if err != nil {
		return 0, err
	}
	if ca.A < 1 || cb.A < 1 {
		return 0, fmt.Errorf("tokens: contrast: %q/%q is translucent — composite it first with ContrastOver", a, b)
	}
	return ratio(ca, cb), nil
}

// ContrastOver returns the contrast between a foreground drawn at `alpha`
// (0..1 — an SVG fill-opacity, a Tailwind /NN modifier, an inherited element
// opacity, or their product) over an opaque background, and that background.
//
// This is the measurement an opacity-based encoding needs: a role that clears
// the floor at full strength routinely fails it at the low end of an opacity
// ramp, because compositing moves the mark toward the background.
func ContrastOver(fg string, alpha float64, bg string) (float64, error) {
	cf, err := parseColor(fg)
	if err != nil {
		return 0, err
	}
	cb, err := parseColor(bg)
	if err != nil {
		return 0, err
	}
	if cb.A < 1 {
		return 0, fmt.Errorf("tokens: contrast: background %q is translucent — resolve it to the opaque color behind the mark", bg)
	}
	if alpha < 0 || alpha > 1 {
		return 0, fmt.Errorf("tokens: contrast: alpha %v out of range 0..1", alpha)
	}
	return ratio(composite(cf, alpha*cf.A, cb), cb), nil
}

// ContrastRoles measures one role against another within a token set, in the
// named scheme ("light" or "dark") — the form a review check wants:
//
//	ratio, err := tokens.ContrastRoles(tok, "caution", "surface", "light")
func ContrastRoles(t Tokens, fg, bg, scheme string) (float64, error) {
	fv, err := roleValue(t, fg, scheme)
	if err != nil {
		return 0, err
	}
	bv, err := roleValue(t, bg, scheme)
	if err != nil {
		return 0, err
	}
	return Contrast(fv, bv)
}

// roleValue resolves one role's value in a scheme.
func roleValue(t Tokens, role, scheme string) (string, error) {
	pair, ok := t.Colors[role]
	if !ok {
		return "", fmt.Errorf("tokens: contrast: token set %q defines no role %q", t.Name, role)
	}
	switch scheme {
	case "light":
		return pair.Light, nil
	case "dark":
		return pair.Dark, nil
	}
	return "", fmt.Errorf("tokens: contrast: unknown scheme %q (want \"light\" or \"dark\")", scheme)
}

// composite alpha-blends fg over an opaque bg. Browsers composite in the
// gamma-encoded space, so this does too — compositing in linear space would
// report a different (wrong) number.
func composite(fg srgb, alpha float64, bg srgb) srgb {
	return srgb{
		R: alpha*fg.R + (1-alpha)*bg.R,
		G: alpha*fg.G + (1-alpha)*bg.G,
		B: alpha*fg.B + (1-alpha)*bg.B,
		A: 1,
	}
}

// ratio is the WCAG 2.1 contrast formula over two opaque colors.
func ratio(a, b srgb) float64 {
	l1, l2 := relLuminance(a), relLuminance(b)
	if l2 > l1 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// relLuminance is WCAG relative luminance: linearize each channel, then weight.
func relLuminance(c srgb) float64 {
	lin := func(v float64) float64 {
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// parseColor resolves a token value to sRGB. It accepts the two forms token
// sets actually contain: oklch() (the kit palette convention) and hex (what a
// pulled component or an app's brand color arrives as). Anything else is an
// error — an unmeasurable value must not be reported as measured.
func parseColor(v string) (srgb, error) {
	s := strings.TrimSpace(v)
	switch {
	case strings.HasPrefix(s, "#"):
		return parseHex(s)
	case strings.HasPrefix(strings.ToLower(s), "oklch("):
		return parseOKLCH(s)
	}
	return srgb{}, fmt.Errorf("tokens: contrast: cannot measure %q (want oklch(...) or #hex)", v)
}

func parseHex(s string) (srgb, error) {
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 || len(h) == 4 {
		var expanded strings.Builder
		for _, r := range h {
			expanded.WriteRune(r)
			expanded.WriteRune(r)
		}
		h = expanded.String()
	}
	if len(h) != 6 && len(h) != 8 {
		return srgb{}, fmt.Errorf("tokens: contrast: %q is not a 3/4/6/8-digit hex color", s)
	}
	ch := func(i int) (float64, error) {
		n, err := strconv.ParseUint(h[i:i+2], 16, 8)
		if err != nil {
			return 0, fmt.Errorf("tokens: contrast: %q is not valid hex", s)
		}
		return float64(n) / 255, nil
	}
	out := srgb{A: 1}
	var err error
	if out.R, err = ch(0); err != nil {
		return srgb{}, err
	}
	if out.G, err = ch(2); err != nil {
		return srgb{}, err
	}
	if out.B, err = ch(4); err != nil {
		return srgb{}, err
	}
	if len(h) == 8 {
		if out.A, err = ch(6); err != nil {
			return srgb{}, err
		}
	}
	return out, nil
}

// parseOKLCH converts `oklch(L C H[ / A])` to sRGB, clamping to gamut the way a
// browser does. L accepts 0..1 or a percentage; A accepts 0..1 or a percentage.
func parseOKLCH(s string) (srgb, error) {
	body := strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(s), "oklch("), ")")
	comps, alphaPart, hasAlpha := strings.Cut(body, "/")
	f := strings.Fields(strings.ReplaceAll(comps, ",", " "))
	if len(f) != 3 {
		return srgb{}, fmt.Errorf("tokens: contrast: %q is not oklch(L C H)", s)
	}
	num := func(x string, pctScale float64) (float64, error) {
		if strings.HasSuffix(x, "%") {
			n, err := strconv.ParseFloat(strings.TrimSuffix(x, "%"), 64)
			return n / 100 * pctScale, err
		}
		return strconv.ParseFloat(x, 64)
	}
	l, err := num(f[0], 1)
	if err != nil {
		return srgb{}, fmt.Errorf("tokens: contrast: %q has a non-numeric lightness", s)
	}
	c, err := num(f[1], 0.4)
	if err != nil {
		return srgb{}, fmt.Errorf("tokens: contrast: %q has a non-numeric chroma", s)
	}
	h, err := num(strings.TrimSuffix(f[2], "deg"), 360)
	if err != nil {
		return srgb{}, fmt.Errorf("tokens: contrast: %q has a non-numeric hue", s)
	}
	alpha := 1.0
	if hasAlpha {
		if alpha, err = num(strings.TrimSpace(alphaPart), 1); err != nil {
			return srgb{}, fmt.Errorf("tokens: contrast: %q has a non-numeric alpha", s)
		}
	}

	// OKLCH → OKLab → LMS → linear sRGB (Ottosson's matrices).
	rad := h * math.Pi / 180
	a, bb := c*math.Cos(rad), c*math.Sin(rad)
	lp := l + 0.3963377774*a + 0.2158037573*bb
	mp := l - 0.1055613458*a - 0.0638541728*bb
	sp := l - 0.0894841775*a - 1.2914855480*bb
	lc, mc, sc := lp*lp*lp, mp*mp*mp, sp*sp*sp
	lr := 4.0767416621*lc - 3.3077115913*mc + 0.2309699292*sc
	lg := -1.2684380046*lc + 2.6097574011*mc - 0.3413193965*sc
	lb := -0.0041960863*lc - 0.7034186147*mc + 1.7076147010*sc

	return srgb{R: encode(lr), G: encode(lg), B: encode(lb), A: alpha}, nil
}

// encode applies the sRGB transfer function, clamping out-of-gamut channels the
// way a browser clips them.
func encode(c float64) float64 {
	switch {
	case c <= 0:
		return 0
	case c >= 1:
		return 1
	case c <= 0.0031308:
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}
