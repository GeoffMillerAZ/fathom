package tokens

import (
	"os"
	"strings"
	"testing"
)

func synthwaveSource(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile("../../../cue/tokens/synthwave/synthwave.cue")
	if err != nil {
		t.Fatalf("read synthwave.cue: %v", err)
	}
	return src
}

func mustLoad(t *testing.T, src []byte) Tokens {
	t.Helper()
	tok, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return tok
}

// TestEmitThemeCSS_WrapperIsByteIdentical — EmitThemeCSS must stay a pure
// wrapper. `appkit gen` calls it, `task check` gates on the regenerated
// artifacts being byte-identical to the committed ones, and seven Go consumers
// build their CSS from those artifacts. Adding an option must not move a byte
// for anyone who did not ask for one.
func TestEmitThemeCSS_WrapperIsByteIdentical(t *testing.T) {
	for name, src := range map[string][]byte{"kit": kitSource(t), "synthwave": synthwaveSource(t)} {
		t.Run(name, func(t *testing.T) {
			tok := mustLoad(t, src)
			if got, want := EmitThemeCSS(tok), EmitThemeCSSOpts(tok, EmitOptions{DarkVariant: DarkVariantAttribute}); got != want {
				t.Error("EmitThemeCSS is not byte-identical to EmitThemeCSSOpts with the attribute variant")
			}
			// The zero-value EmitOptions must also mean "attribute": a caller
			// passing EmitOptions{} is asking for the default, not for none.
			if got, want := EmitThemeCSS(tok), EmitThemeCSSOpts(tok, EmitOptions{}); got != want {
				t.Error("EmitOptions{} does not reproduce the default")
			}
		})
	}
}

// TestEmitThemeCSSOpts_NoneOmitsCustomVariant — the whole reason the option
// exists. `@custom-variant dark (…)` REPLACES Tailwind's built-in `dark:`,
// re-keying every dark: utility in the consuming app from prefers-color-scheme
// to an attribute that app may never set. Nothing errors; the utilities are
// still generated and simply never match. A vendoring consumer must be able to
// take the palette without losing its own dark mode.
func TestEmitThemeCSSOpts_NoneOmitsCustomVariant(t *testing.T) {
	tok := mustLoad(t, kitSource(t))

	none := EmitThemeCSSOpts(tok, EmitOptions{DarkVariant: DarkVariantNone})
	if strings.Contains(none, "@custom-variant") {
		t.Error("DarkVariantNone still emits @custom-variant — the consuming app's dark: utilities would be silently re-keyed")
	}
	attr := EmitThemeCSSOpts(tok, EmitOptions{DarkVariant: DarkVariantAttribute})
	if !strings.Contains(attr, "@custom-variant dark") {
		t.Error("DarkVariantAttribute did not emit @custom-variant dark")
	}

	// Everything else is unchanged: the palette, the theme block and the
	// scheme branches all still ship under `none`. Dropping the variant must
	// not drop the theming.
	for _, want := range []string{"@theme inline {", "--ak-color-surface:", `[data-theme="dark"]`, "@media (prefers-color-scheme: dark)"} {
		if !strings.Contains(none, want) {
			t.Errorf("DarkVariantNone output is missing %q — only the variant should be omitted", want)
		}
	}

	// Emitted CSS must parse under Tailwind v4.0, not just the v4.3.0 the kit
	// pins: a vendoring consumer may resolve an older v4 minor.
	// `@source inline(...)` is v4.1+.
	for _, out := range []string{none, attr} {
		if strings.Contains(out, "@source") {
			t.Error("emitted theme CSS contains an @source directive; v4.1+ syntax must never reach a vendored consumer")
		}
	}
}

// TestPolarityDerivedFromSurfaces — polarity is measured from the branch's own
// surface, never inferred from the branch's name. The two shipped token sets
// disagree with their own labels: synthwave's `:root` (nominally light) is
// near-black, which is hazard H1 and which shipped twice.
func TestPolarityDerivedFromSurfaces(t *testing.T) {
	kit := EmitThemeCSSOpts(mustLoad(t, kitSource(t)), EmitOptions{})
	root, dark := block(t, kit, ":root {"), block(t, kit, `[data-theme="dark"] {`)

	if !strings.Contains(root, "--ak-polarity: light;") {
		t.Errorf("kit :root must derive polarity light from its own surface; got:\n%s", root)
	}
	if !strings.Contains(root, "color-scheme: light;") {
		t.Error("kit :root must declare color-scheme: light")
	}
	if !strings.Contains(dark, "--ak-polarity: dark;") || !strings.Contains(dark, "color-scheme: dark;") {
		t.Errorf("kit dark branch must derive polarity dark; got:\n%s", dark)
	}

	// synthwave is dark-valued in BOTH branches. Its :root must say so.
	sw := EmitThemeCSSOpts(mustLoad(t, synthwaveSource(t)), EmitOptions{})
	swRoot := block(t, sw, ":root {")
	if !strings.Contains(swRoot, "--ak-polarity: dark;") {
		t.Errorf("synthwave :root paints dark and must derive polarity dark, not light; got:\n%s", swRoot)
	}
	if !strings.Contains(swRoot, "color-scheme: dark;") {
		t.Error("synthwave :root must declare color-scheme: dark — without it every form control and scrollbar renders light on a near-black page")
	}

	// And the consequence the polarity derivation exists to fix: on a
	// dark-valued token set, a page that sets NO data-theme paints dark, so
	// `dark:` utilities must fire there. Keyed on the attribute alone they sit
	// switched off and the page renders its light-mode structure over a dark
	// palette.
	if !strings.Contains(sw, `:root:not([data-theme="light"])`) {
		t.Error("synthwave's dark: variant does not match an un-forced root; a page with no data-theme paints dark with dark: utilities off")
	}
	// The kit set is genuinely light by default and must NOT acquire that arm.
	if strings.Contains(block(t, kit, "@custom-variant"), `:root:not([data-theme="light"])`) {
		t.Error("kit dark: variant must stay keyed on the attribute only — its default branch is light")
	}
}

// TestPolarityForUnparseableSurfaceEmitsNothing — a measurement you could not
// make must not silently become a default. A wrong color-scheme repaints every
// form control and scrollbar on the page.
func TestPolarityForUnparseableSurfaceEmitsNothing(t *testing.T) {
	if got := polarityFor("var(--something)"); got != "" {
		t.Errorf("polarityFor(unparseable) = %q, want \"\"", got)
	}
	if got := polarityFor(""); got != "" {
		t.Errorf("polarityFor(empty) = %q, want \"\"", got)
	}
	// The crossover is perceptual, not arithmetic. "Contrasts more with white
	// than with black" crosses at relative luminance ≈ 0.179, well below the
	// arithmetic midpoint: mid-grey #808080 (L ≈ 0.216) is a LIGHT ground —
	// it reads better under black text than under white. An "L < 0.5" test
	// would call it dark and invert every page built on it.
	for value, want := range map[string]string{
		"#ffffff": "light",
		"#000000": "dark",
		"#808080": "light", // L 0.216 — above the 0.179 crossover
		"#595959": "dark",  // L 0.098 — below it
		"#e5e5e5": "light",
	} {
		if got := polarityFor(value); got != want {
			t.Errorf("polarityFor(%s) = %q, want %q", value, got, want)
		}
	}
}

// block returns the CSS block opened by the first line containing marker, up to
// its closing brace at the same nesting level. Good enough for asserting on
// generated output; it is not a CSS parser.
func block(t *testing.T, css, marker string) string {
	t.Helper()
	i := strings.Index(css, marker)
	if i < 0 {
		t.Fatalf("generated CSS has no %q", marker)
	}
	rest := css[i:]
	if j := strings.Index(rest, "\n}"); j >= 0 {
		return rest[:j]
	}
	if j := strings.Index(rest, "\n\n"); j >= 0 {
		return rest[:j]
	}
	return rest
}

// TestStripDarkVariantEqualsNoneEmission — the two ways to get "no dark:
// variant" must be the same bytes. `appkit vendor` strips the statement from
// the embedded stylesheet (it has no CUE token set in a consumer's repo);
// `appkit gen` could re-emit. If those ever diverge, a vendored consumer and a
// generating consumer get different CSS from the same kit version, which is the
// class of bug a lockfile cannot catch because both files hash correctly.
func TestStripDarkVariantEqualsNoneEmission(t *testing.T) {
	for name, src := range map[string][]byte{"kit": kitSource(t), "synthwave": synthwaveSource(t)} {
		t.Run(name, func(t *testing.T) {
			tok := mustLoad(t, src)
			got := StripDarkVariant(EmitThemeCSS(tok))
			want := EmitThemeCSSOpts(tok, EmitOptions{DarkVariant: DarkVariantNone})
			if got != want {
				t.Errorf("StripDarkVariant(EmitThemeCSS) != EmitThemeCSSOpts(none)\n got: %.400q\nwant: %.400q", got, want)
			}
		})
	}
	// Idempotent, and a no-op on CSS that never had one.
	plain := EmitThemeCSSOpts(mustLoad(t, kitSource(t)), EmitOptions{DarkVariant: DarkVariantNone})
	if StripDarkVariant(plain) != plain {
		t.Error("StripDarkVariant is not a no-op on CSS with no custom variant")
	}
}

// TestThemeCSSMatchesTheCommittedArtifact — the embedded stylesheet must be the
// one `appkit gen` produces from the kit token set. Embedding a stale artifact
// would ship one theme to Go importers and another to `appkit css` consumers,
// with nothing failing.
func TestThemeCSSMatchesTheCommittedArtifact(t *testing.T) {
	if got, want := ThemeCSS(), EmitThemeCSS(mustLoad(t, kitSource(t))); got != want {
		t.Error("appkit/ui/tokens/theme.css is stale relative to cue/tokens/kit.cue — run `task gen`")
	}
}

// TestALightIslandIsExpressibleInsideADarkPage is the symmetry the emitter
// lacked. `[data-theme="dark"]` re-sets the tokens on any subtree, so a dark
// island inside a light page has always worked; `:root` carries the light
// values and matches ONE element, so there was no way back. A two-scheme
// comparison therefore only worked on a light page, and a consumer wanting to
// show both had to write the palette a second time — which is the drift the
// token set exists to prevent.
func TestALightIslandIsExpressibleInsideADarkPage(t *testing.T) {
	kit := EmitThemeCSSOpts(mustLoad(t, kitSource(t)), EmitOptions{})

	light := block(t, kit, `[data-theme="light"] {`)
	dark := block(t, kit, `[data-theme="dark"] {`)
	root := block(t, kit, ":root {")

	if light == "" {
		t.Fatal(`no [data-theme="light"] block: a light island inside a dark page is inexpressible, so a two-scheme comparison works in one direction only`)
	}

	// It carries the SAME values :root does. Two spellings of one palette is
	// the thing this generator exists to prevent, so the forced-light block has
	// to be the light branch and not a second opinion about it.
	for _, decl := range colorDecls(root) {
		if !strings.Contains(light, decl) {
			t.Errorf("the forced-light block is missing %q, which :root carries — the two are meant to be one palette", decl)
		}
	}
	// And it is not the dark branch wearing a light selector.
	for _, decl := range colorDecls(dark) {
		if strings.Contains(light, decl) {
			t.Errorf("the forced-light block carries the DARK value %q", decl)
		}
	}

	// Source order: the two selectors are mutually exclusive on any one
	// element, so order decides nothing today. It is asserted because it is
	// what a reader checks first when it ever does, and "the later block wins"
	// is the rule they will assume.
	if strings.Index(kit, `[data-theme="light"] {`) < strings.Index(kit, `[data-theme="dark"] {`) {
		t.Error(`the forced-light block precedes the forced-dark block; emit light last so the assumed rule and the real one agree`)
	}
}

// colorDecls pulls the `--ak-color-*: value;` lines out of one block.
func colorDecls(block string) []string {
	var out []string
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "--ak-color-") {
			out = append(out, line)
		}
	}
	return out
}
