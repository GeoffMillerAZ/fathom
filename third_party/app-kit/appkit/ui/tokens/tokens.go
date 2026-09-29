// Package tokens is the design-token pipeline (D7): CUE-authored tokens in,
// three deterministic artifacts out —
//
//   - Tailwind v4 `@theme inline` CSS wired for runtime light/dark switching
//     (EmitThemeCSS),
//   - a DTCG 2025.10 JSON export for design tooling (EmitDTCG),
//   - generated Go role-name constants for templ components (EmitGoNames).
//
// The schema (schema.cue, embedded) is the single source of truth for shape;
// Load validates every token set against it before anything is emitted.
// Emission is deterministic: same input bytes → same output bytes, no
// timestamps, sorted iteration — CI checks regeneration cleanliness.
//
// Theme-switching contract (consumed by the kit's settings UI in a later
// phase, honored by the CSS now):
//
//	<html>                     → follows the OS scheme (media query)
//	<html data-theme="light">  → forced light
//	<html data-theme="dark">   → forced dark
//	--ak-font-scale on :root   → user font-size display setting (1 = 100%)
package tokens

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"go/format"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed schema.cue
var schemaCUE []byte

//go:embed theme.css
var kitThemeCSS string

// ThemeCSS returns the kit's committed default stylesheet — the same bytes
// `appkit gen` writes to appkit/ui/tokens/theme.css from cue/tokens/kit.cue.
//
// It is embedded so the artifact travels with the module. `appkit vendor` and
// any consumer outside this repo need the theme without a checkout of
// cue/tokens/, and `go mod vendor` drops a directory containing no Go file at
// all — which is exactly how a downstream CSS build came to depend on a sibling
// clone at a fixed relative depth.
func ThemeCSS() string { return kitThemeCSS }

// StripDarkVariant returns generated theme CSS with its `@custom-variant dark
// (…);` statement removed — the DarkVariantNone form of an already-emitted
// stylesheet.
//
// It exists so vendoring can offer --dark-variant without re-running the CUE
// pipeline in a consumer's repo. The two paths are provably the same thing:
// TestStripDarkVariantEqualsNoneEmission asserts
// StripDarkVariant(EmitThemeCSS(t)) == EmitThemeCSSOpts(t, none) for every
// shipped token set, so this can never drift into a second, subtly different
// notion of "none".
func StripDarkVariant(css string) string {
	i := strings.Index(css, "@custom-variant dark ")
	if i < 0 {
		return css
	}
	j := strings.Index(css[i:], ";\n")
	if j < 0 {
		return css
	}
	rest := css[i+j+len(";\n"):]
	// The emitter writes the statement followed by a blank line; drop that too
	// so the output is byte-identical to never having written it.
	rest = strings.TrimPrefix(rest, "\n")
	return css[:i] + rest
}

// ColorPair is one semantic color role resolved per scheme.
type ColorPair struct {
	Light string `json:"light"`
	Dark  string `json:"dark"`
}

// Fonts are the token set's font stacks.
type Fonts struct {
	Sans    string `json:"sans"`
	Mono    string `json:"mono"`
	Display string `json:"display,omitempty"`
}

// Radius is the corner-radius scale.
type Radius struct {
	SM   string `json:"sm"`
	MD   string `json:"md"`
	LG   string `json:"lg"`
	XL   string `json:"xl"`
	Full string `json:"full"`
}

// Tokens is a complete, validated token set.
type Tokens struct {
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Fonts       Fonts                `json:"fonts"`
	Radius      Radius               `json:"radius"`
	Colors      map[string]ColorPair `json:"colors"`
}

// Load parses CUE source containing a `tokens:` value, validates it against
// the embedded schema, and decodes it. Validation failure returns the CUE
// diagnostic — the build must stop on an invalid token set, never emit from
// one.
func Load(src []byte) (Tokens, error) {
	ctx := cuecontext.New()

	schema := ctx.CompileBytes(schemaCUE)
	if err := schema.Err(); err != nil {
		return Tokens{}, fmt.Errorf("tokens: compile embedded schema: %w", err)
	}
	val := ctx.CompileBytes(src)
	if err := val.Err(); err != nil {
		return Tokens{}, fmt.Errorf("tokens: compile token source: %w", err)
	}

	tv := val.LookupPath(cue.ParsePath("tokens"))
	if !tv.Exists() {
		return Tokens{}, fmt.Errorf("tokens: source has no top-level `tokens:` value")
	}
	def := schema.LookupPath(cue.ParsePath("#Tokens"))
	if err := def.Err(); err != nil {
		return Tokens{}, fmt.Errorf("tokens: schema #Tokens: %w", err)
	}
	unified := def.Unify(tv)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return Tokens{}, fmt.Errorf("tokens: token set does not satisfy the schema: %w", err)
	}

	var t Tokens
	if err := unified.Decode(&t); err != nil {
		return Tokens{}, fmt.Errorf("tokens: decode: %w", err)
	}
	defaultScrim(&t)
	return t, nil
}

// defaultScrim fills an omitted scrim role (schema.cue marks it optional) so
// every loaded set emits the utility appkit/ui/overlay paints its backdrop
// with; a role that is missing is a class that is never generated (hazard H4).
// Each branch takes the kit's scrim for its MEASURED polarity, never its name
// (hazard H1): a dark-only set paints its "light" branch dark, and the lighter
// scrim would wash that page out. An unmeasurable surface takes the darker
// scrim, which dims any ground. An authored scrim is never replaced.
func defaultScrim(t *Tokens) {
	if _, ok := t.Colors["scrim"]; ok || t.Colors == nil {
		return
	}
	pick := func(surface string) string {
		if polarityFor(surface) == "light" {
			return defaultScrimOnLight
		}
		return defaultScrimOnDark
	}
	t.Colors["scrim"] = ColorPair{
		Light: pick(t.Colors["surface"].Light),
		Dark:  pick(t.Colors["surface"].Dark),
	}
}

// The scrim role's defaults: the veil a modal dialog or drawer draws over the
// page (appkit/ui/overlay paints it at 50%). One is a tinted near-black for a
// branch whose surface paints light, the other a deeper one for a branch whose
// surface paints dark — each dims its ground about as much as the black/40
// backdrop it replaced (TestScrimRoleDimsEveryShippedSurface). kit.cue authors
// the same pair, so the default set and the fallback are one value.
const (
	defaultScrimOnLight = "oklch(0.30 0.015 260)"
	defaultScrimOnDark  = "oklch(0.10 0.010 260)"
)

// colorNames returns the role names in deterministic (sorted) order.
func colorNames(t Tokens) []string {
	names := make([]string, 0, len(t.Colors))
	for name := range t.Colors {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// radiusValues returns the radius scale as ordered (name, value) pairs — the
// single source both EmitThemeCSS (for --radius-* vars) and EmitGoNames (for
// RadiusNames) iterate, so the CSS utilities and the Go vocabulary can't drift
// apart. Order is scale order (sm→full), not alphabetical.
func radiusValues(t Tokens) [][2]string {
	return [][2]string{
		{"sm", t.Radius.SM},
		{"md", t.Radius.MD},
		{"lg", t.Radius.LG},
		{"xl", t.Radius.XL},
		{"full", t.Radius.Full},
	}
}

// fontValues returns the font roles as ordered (name, value) pairs. display
// falls back to sans when unset (matching the emitted CSS).
func fontValues(t Tokens) [][2]string {
	display := t.Fonts.Display
	if display == "" {
		display = t.Fonts.Sans
	}
	return [][2]string{
		{"sans", t.Fonts.Sans}, {"mono", t.Fonts.Mono}, {"display", display},
	}
}

// DarkVariant selects how (or whether) the emitted CSS defines Tailwind's
// `dark:` variant.
type DarkVariant string

const (
	// DarkVariantAttribute emits `@custom-variant dark (…)` keyed on
	// data-theme. The default, and what `appkit gen` uses.
	DarkVariantAttribute DarkVariant = "attribute"
	// DarkVariantNone emits no custom variant at all, leaving Tailwind's
	// built-in `dark:` on `prefers-color-scheme` where the consuming app left
	// it. Use this whenever the kit's CSS is being dropped into an app that
	// already has `dark:` utilities of its own — see the hazard on
	// EmitThemeCSSOpts.
	DarkVariantNone DarkVariant = "none"
)

// EmitOptions configures EmitThemeCSSOpts. The zero value reproduces
// EmitThemeCSS exactly, so every existing caller is unaffected.
type EmitOptions struct {
	// DarkVariant selects the `dark:` strategy. Empty means
	// DarkVariantAttribute.
	DarkVariant DarkVariant
}

// EmitThemeCSS renders the token set as Tailwind v4 CSS with the kit's default
// options. It is a thin wrapper over [EmitThemeCSSOpts]; `appkit gen` calls
// this, so its output is byte-identical to EmitThemeCSSOpts with
// DarkVariantAttribute and that identity is asserted by
// TestEmitThemeCSS_WrapperIsByteIdentical.
//
// Apps import the result from their input.css AFTER `@import "tailwindcss"`.
func EmitThemeCSS(t Tokens) string {
	return EmitThemeCSSOpts(t, EmitOptions{})
}

// EmitThemeCSSOpts renders the token set as Tailwind v4 CSS:
//
//   - `@theme inline` maps every role to a runtime variable, so utilities
//     like bg-surface / text-ink / border-line resolve through
//     `var(--ak-color-*)` and switch scheme without a rebuild;
//   - `:root` carries the light values and `[data-theme="dark"]` the dark
//     values, with a SYMMETRIC `[data-theme="light"]` block so an island of
//     either scheme is expressible inside a page of the other — a dark island
//     was expressible and a light one was not, which made a two-scheme
//     comparison work only on a light page;
//   - a `prefers-color-scheme` block covers the no-attribute
//     (follow-the-OS) case;
//   - a `dark:` custom variant keyed on data-theme for the rare component
//     that needs per-scheme structure rather than token indirection —
//     omitted entirely under [DarkVariantNone];
//   - `--ak-polarity` and `color-scheme` per branch, DERIVED from that
//     branch's own surface color (see [polarityFor]);
//   - the `--ak-font-scale` hook implements the font-size display setting.
//
// # The hazard DarkVariantNone exists for
//
// `@custom-variant dark (…)` does not add a variant — it REPLACES Tailwind's
// built-in one. Emitting it re-keys every `dark:` utility in the consuming app
// from `prefers-color-scheme` to a `data-theme` attribute that app may never
// set, so its dark mode silently stops responding to the OS. There is no error,
// no warning, and nothing in the built CSS to grep for: the utilities are still
// generated, they just never match. Vendoring into an app that owns its own
// `dark:` utilities therefore defaults to DarkVariantNone; `appkit gen` keeps
// DarkVariantAttribute, so every Go consumer is byte-unchanged.
//
// # A constraint on this emitter
//
// The output must parse under Tailwind v4.0, not just the v4.3.0 the kit pins:
// a vendoring consumer may resolve an older v4 minor. `@theme inline` and
// `@custom-variant` are v4.0; **`@source inline(...)` is v4.1+ and must never
// appear in emitted output.**
func EmitThemeCSSOpts(t Tokens, o EmitOptions) string {
	var b strings.Builder
	names := colorNames(t)

	// Polarity is derived per branch from the branch's own surface, never from
	// the branch's NAME. The two disagree: synthwave's "light" branch is
	// dark-valued on purpose, and every rule that assumed otherwise inverted
	// figure and ground on it (docs/theming-hazards.md H1).
	lightPolarity := polarityFor(t.Colors["surface"].Light)
	darkPolarity := polarityFor(t.Colors["surface"].Dark)

	fmt.Fprintf(&b, "/* Code generated by `appkit gen` from token set %q. DO NOT EDIT. */\n\n", t.Name)

	if o.DarkVariant != DarkVariantNone {
		// dark: variant — explicit data-theme (see package doc for the
		// switching contract). When the DEFAULT branch is already dark-valued,
		// the attribute alone is wrong: a page that sets no data-theme paints
		// dark while every `dark:` utility sits switched off. Such a token set
		// therefore also matches the un-forced root.
		sel := `[data-theme="dark"], [data-theme="dark"] *`
		if lightPolarity == "dark" {
			sel += `, :root:not([data-theme="light"]), :root:not([data-theme="light"]) *`
		}
		fmt.Fprintf(&b, "@custom-variant dark (&:where(%s));\n\n", sel)
	}

	// @theme inline: utilities resolve through the runtime vars.
	b.WriteString("@theme inline {\n")
	for _, fv := range fontValues(t) {
		fmt.Fprintf(&b, "  --font-%s: var(--ak-font-%s);\n", fv[0], fv[0])
	}
	for _, rv := range radiusValues(t) {
		fmt.Fprintf(&b, "  --radius-%s: var(--ak-radius-%s);\n", rv[0], rv[0])
	}
	for _, name := range names {
		fmt.Fprintf(&b, "  --color-%s: var(--ak-color-%s);\n", name, name)
	}
	b.WriteString("}\n\n")

	// Default scheme + font-scale hook.
	b.WriteString(":root {\n")
	b.WriteString("  font-size: calc(100% * var(--ak-font-scale, 1));\n")
	// Font and shape use the same runtime indirection as color. A separately
	// linked named theme must change these too, without rebuilding utilities.
	for _, fv := range fontValues(t) {
		fmt.Fprintf(&b, "  --ak-font-%s: %s;\n", fv[0], fv[1])
	}
	for _, rv := range radiusValues(t) {
		fmt.Fprintf(&b, "  --ak-radius-%s: %s;\n", rv[0], rv[1])
	}
	writePolarity(&b, "  ", lightPolarity)
	for _, name := range names {
		fmt.Fprintf(&b, "  --ak-color-%s: %s;\n", name, t.Colors[name].Light)
	}
	b.WriteString("}\n\n")

	// Forced dark.
	b.WriteString("[data-theme=\"dark\"] {\n")
	writePolarity(&b, "  ", darkPolarity)
	for _, name := range names {
		fmt.Fprintf(&b, "  --ak-color-%s: %s;\n", name, t.Colors[name].Dark)
	}
	b.WriteString("}\n\n")

	// Forced LIGHT, symmetric with forced dark.
	//
	// Without this block a dark island is expressible and a light island is
	// not: `[data-theme="dark"]` re-sets the tokens on any subtree, but a
	// subtree inside a dark page has no way back — `:root` carries the light
	// values and `:root` matches one element. So a two-scheme comparison only
	// worked on a light page, and every consumer that wanted to show both had
	// to either write the palette a second time or tell the reader to use the
	// toggle. The portal's style guide states that limitation in prose today,
	// which is a design system apologising for its generator.
	//
	// It is emitted AFTER the dark block. The two selectors are mutually
	// exclusive on any one element so order does not decide a conflict — but
	// order is what a reader checks first when one does, and "the later block
	// wins" is the rule they will assume.
	b.WriteString("[data-theme=\"light\"] {\n")
	writePolarity(&b, "  ", lightPolarity)
	for _, name := range names {
		fmt.Fprintf(&b, "  --ak-color-%s: %s;\n", name, t.Colors[name].Light)
	}
	b.WriteString("}\n\n")

	// Follow the OS when no explicit choice is set.
	b.WriteString("@media (prefers-color-scheme: dark) {\n")
	b.WriteString("  :root:not([data-theme=\"light\"]):not([data-theme=\"dark\"]) {\n")
	writePolarity(&b, "    ", darkPolarity)
	for _, name := range names {
		fmt.Fprintf(&b, "    --ak-color-%s: %s;\n", name, t.Colors[name].Dark)
	}
	b.WriteString("  }\n")
	b.WriteString("}\n")

	return b.String()
}

// writePolarity emits the derived polarity hook and the matching color-scheme
// for one branch. An unresolvable surface emits neither, rather than guessing:
// a wrong color-scheme repaints every form control and scrollbar.
func writePolarity(b *strings.Builder, indent, polarity string) {
	if polarity == "" {
		return
	}
	fmt.Fprintf(b, "%s--ak-polarity: %s;\n", indent, polarity)
	fmt.Fprintf(b, "%scolor-scheme: %s;\n", indent, polarity)
}

// polarityFor reports whether a branch paints "light" or "dark", derived from
// that branch's own surface color — measured, never authored, and never
// inferred from the branch's name.
//
// The distinction is load-bearing because the kit's two shipped token sets
// disagree with their own labels: `synthwave`'s light and dark values are
// identical and both dark, so its `:root` (nominally the light branch) paints
// near-black. Anything that read "light branch" as "light page" got it exactly
// backwards there — which is hazard H1 in docs/theming-hazards.md, and it
// shipped twice.
//
// The test is the standard one: a color is dark when it contrasts more with
// white than with black. That crosses over at relative luminance ≈ 0.179,
// which is the perceptual midpoint rather than the arithmetic one. Returns ""
// when the value cannot be parsed — a measurement you could not make must not
// silently become a default (contrast.go).
func polarityFor(surface string) string {
	c, err := parseColor(surface)
	if err != nil {
		return ""
	}
	white := srgb{R: 1, G: 1, B: 1, A: 1}
	black := srgb{R: 0, G: 0, B: 0, A: 1}
	if ratio(c, white) > ratio(c, black) {
		return "dark"
	}
	return "light"
}

// EmitDTCG renders the token set as DTCG-format JSON (Design Tokens
// Community Group, 2025.10 draft shape): $type/$value per token, dark values
// under $extensions["com.appkit.scheme"].dark. Deterministic via
// json.MarshalIndent over sorted-key maps (encoding/json sorts map keys).
func EmitDTCG(t Tokens) []byte {
	type dtcgToken struct {
		Type       string         `json:"$type"`
		Value      string         `json:"$value"`
		Extensions map[string]any `json:"$extensions,omitempty"`
	}
	doc := map[string]any{
		"$description": t.Description,
		"name":         t.Name,
		"font": map[string]any{
			"sans": dtcgToken{Type: "fontFamily", Value: t.Fonts.Sans},
			"mono": dtcgToken{Type: "fontFamily", Value: t.Fonts.Mono},
		},
		"radius": map[string]any{
			"sm":   dtcgToken{Type: "dimension", Value: t.Radius.SM},
			"md":   dtcgToken{Type: "dimension", Value: t.Radius.MD},
			"lg":   dtcgToken{Type: "dimension", Value: t.Radius.LG},
			"xl":   dtcgToken{Type: "dimension", Value: t.Radius.XL},
			"full": dtcgToken{Type: "dimension", Value: t.Radius.Full},
		},
	}
	colors := map[string]any{}
	for _, name := range colorNames(t) {
		pair := t.Colors[name]
		colors[name] = dtcgToken{
			Type:  "color",
			Value: pair.Light,
			Extensions: map[string]any{
				"com.appkit.scheme": map[string]string{"dark": pair.Dark},
			},
		}
	}
	doc["color"] = colors

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(doc); err != nil {
		// Marshalling string-only maps cannot fail; treat as programmer error.
		panic("tokens: EmitDTCG: " + err.Error())
	}
	return buf.Bytes()
}

// EmitGoNames renders a generated Go file of color-role name constants for
// the package named pkg, so templ components reference roles by constant
// (compile-checked) rather than string literal. Output is gofmt-formatted —
// consumers gate on gofmt-cleanliness and generated code must pass it.
func EmitGoNames(t Tokens, pkg string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "// Code generated by `appkit gen` from token set %q. DO NOT EDIT.\n\npackage %s\n\n", t.Name, pkg)
	b.WriteString("// Color role names — each emits Tailwind utilities (bg-<role>, text-<role>, border-<role>, ...).\nconst (\n")
	for _, name := range colorNames(t) {
		fmt.Fprintf(&b, "\tColor%s = %q\n", goName(name), name)
	}
	b.WriteString(")\n\n// ColorNames lists every color role in the token set, sorted.\nvar ColorNames = []string{\n")
	for _, name := range colorNames(t) {
		fmt.Fprintf(&b, "\t%q,\n", name)
	}
	b.WriteString("}\n\n// RadiusNames lists the corner-radius scale (scale order); each emits a\n// rounded-<name> utility wired to --radius-<name>.\nvar RadiusNames = []string{\n")
	for _, rv := range radiusValues(t) {
		fmt.Fprintf(&b, "\t%q,\n", rv[0])
	}
	b.WriteString("}\n\n// FontNames lists the font roles; each emits a font-<name> utility wired to\n// --font-<name>.\nvar FontNames = []string{\n")
	for _, fv := range fontValues(t) {
		fmt.Fprintf(&b, "\t%q,\n", fv[0])
	}
	b.WriteString("}\n")
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		// Unreachable for the template above; fail loudly, never emit
		// unparseable Go.
		panic("tokens: EmitGoNames produced invalid Go: " + err.Error())
	}
	return formatted
}

// EmitTerminalPalette renders the token set's color roles as a generated Go
// source file for a terminal palette: a Role type, one constant per role, a
// sorted Roles slice, and Light/Dark hex tables converted from the token
// set's oklch() values via [OKLCHToHex]. It is `appkit gen`'s terminal
// counterpart to EmitGoNames — same DO-NOT-EDIT header, same byte-identity
// gating — but the output is sRGB hex rather than Tailwind role-name
// constants: no terminal color API (ANSI escapes, lipgloss.Color) has an
// OKLCH type, so this is the one place in the pipeline that must resolve
// L/C/H to concrete bytes. appkit/cli/theme consumes the result (palette_gen.go).
func EmitTerminalPalette(t Tokens, pkg string) ([]byte, error) {
	var b bytes.Buffer
	names := colorNames(t)
	fmt.Fprintf(&b, "// Code generated by `appkit gen` from token set %q. DO NOT EDIT.\n\npackage %s\n\n", t.Name, pkg)
	b.WriteString("// Role identifies a semantic terminal color role, generated from the kit's\n// CUE color tokens (cue/tokens/kit.cue).\ntype Role string\n\nconst (\n")
	for _, name := range names {
		fmt.Fprintf(&b, "\tRole%s Role = %q\n", goName(name), name)
	}
	b.WriteString(")\n\n// Roles lists every color role, sorted.\nvar Roles = []Role{\n")
	for _, name := range names {
		fmt.Fprintf(&b, "\tRole%s,\n", goName(name))
	}
	b.WriteString("}\n\n// LightHex and DarkHex map each role to its sRGB hex value, converted from\n// the token set's oklch() values via OKLCHToHex (see oklch_terminal.go).\nvar LightHex = map[Role]string{\n")
	for _, name := range names {
		hex, err := OKLCHToHex(t.Colors[name].Light)
		if err != nil {
			return nil, fmt.Errorf("tokens: EmitTerminalPalette: role %q light: %w", name, err)
		}
		fmt.Fprintf(&b, "\tRole%s: %q,\n", goName(name), hex)
	}
	b.WriteString("}\n\nvar DarkHex = map[Role]string{\n")
	for _, name := range names {
		hex, err := OKLCHToHex(t.Colors[name].Dark)
		if err != nil {
			return nil, fmt.Errorf("tokens: EmitTerminalPalette: role %q dark: %w", name, err)
		}
		fmt.Fprintf(&b, "\tRole%s: %q,\n", goName(name), hex)
	}
	b.WriteString("}\n")
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("tokens: EmitTerminalPalette produced invalid Go: %w", err)
	}
	return formatted, nil
}

// goName converts a kebab-case role name to a Go identifier fragment
// (surface-raised → SurfaceRaised).
func goName(kebab string) string {
	parts := strings.Split(kebab, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}
