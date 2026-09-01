package tokens

import (
	"math"
	"testing"
)

func TestContrast_ReferencePoints(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"#ffffff", "#000000", 21.0}, // the two extremes
		{"#000", "#fff", 21.0},       // short hex, reversed order
		{"#767676", "#ffffff", 4.54}, // WCAG's canonical AA-on-white gray
		{"oklch(1 0 0)", "oklch(0 0 0)", 21.0},
		{"oklch(0.5 0 0)", "oklch(0.5 0 0)", 1.0}, // a color against itself
	}
	for _, c := range cases {
		got, err := Contrast(c.a, c.b)
		if err != nil {
			t.Fatalf("Contrast(%q,%q): %v", c.a, c.b, err)
		}
		if math.Abs(got-c.want) > 0.02 {
			t.Errorf("Contrast(%q,%q) = %.2f, want %.2f", c.a, c.b, got, c.want)
		}
	}
}

func TestContrast_Symmetric(t *testing.T) {
	ab, err := Contrast("oklch(0.21 0.015 260)", "oklch(0.985 0.005 260)")
	if err != nil {
		t.Fatal(err)
	}
	ba, err := Contrast("oklch(0.985 0.005 260)", "oklch(0.21 0.015 260)")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(ab-ba) > 1e-9 {
		t.Errorf("Contrast is not symmetric: %.4f vs %.4f", ab, ba)
	}
}

// TestContrast_RefusesWhatItCannotMeasure is the point of the package: an
// unmeasurable or uncomposited value must error, never return a number that
// reads like a pass.
func TestContrast_RefusesWhatItCannotMeasure(t *testing.T) {
	for _, v := range []string{
		"var(--ak-color-ink)",            // a reference, not a value
		"color-mix(in oklch, red, blue)", // needs a browser to resolve
		"rebeccapurple",                  // named color, not in the kit vocabulary
		"oklch(0.5 0.1)",                 // malformed
		"#12345",                         // malformed
		"oklch(0.6 0.1 260 / 0.4)",       // translucent — no contrast until composited
	} {
		if got, err := Contrast(v, "#ffffff"); err == nil {
			t.Errorf("Contrast(%q, white) = %.2f, want an error", v, got)
		}
	}
}

func TestContrastOver(t *testing.T) {
	const fg, bg = "oklch(0.50 0.19 262)", "oklch(0.985 0.005 260)" // kit light accent on surface

	full, err := ContrastOver(fg, 1, bg)
	if err != nil {
		t.Fatal(err)
	}
	opaque, err := Contrast(fg, bg)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(full-opaque) > 1e-9 {
		t.Errorf("ContrastOver at alpha 1 = %.4f, want the opaque ratio %.4f", full, opaque)
	}

	none, err := ContrastOver(fg, 0, bg)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(none-1) > 1e-9 {
		t.Errorf("ContrastOver at alpha 0 = %.4f, want 1.00 (invisible)", none)
	}

	if _, err := ContrastOver(fg, 1.5, bg); err == nil {
		t.Error("ContrastOver accepted an alpha outside 0..1")
	}
}

// TestContrastOver_OpacityRampFallsBelowFloor pins the fact behind the rule in
// docs/theming-hazards.md §H5: an opacity-encoded mark drawn in a role that
// clears the floor at full strength does NOT clear it once composited. The kit
// already ships such a ramp (viz.Heatmap: fill-opacity 0.12 + 0.85*norm), so
// this is measured behavior, not a hypothetical.
func TestContrastOver_OpacityRampFallsBelowFloor(t *testing.T) {
	tok, err := Load(kitSource(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	accent, surface := tok.Colors["accent"].Light, tok.Colors["surface"].Light

	low, err := ContrastOver(accent, 0.12, surface) // the ramp's floor cell
	if err != nil {
		t.Fatal(err)
	}
	if low >= MeaningFloor {
		t.Errorf("the ramp's lowest cell now measures %.2f:1 — it used to fail the %.1f:1 floor; "+
			"update docs/theming-hazards.md §H5 if the palette changed", low, MeaningFloor)
	}

	// Report where the ramp actually becomes readable, so the number in the doc
	// can be re-derived rather than trusted.
	for n := 0.0; n <= 1.0001; n += 0.01 {
		op := 0.12 + 0.85*n
		r, err := ContrastOver(accent, op, surface)
		if err != nil {
			t.Fatal(err)
		}
		if r >= MeaningFloor {
			t.Logf("viz.Heatmap ramp (light): clears %.1f:1 at norm %.2f (fill-opacity %.2f); "+
				"floor cell = %.2f:1", MeaningFloor, n, op, low)
			return
		}
	}
	t.Logf("viz.Heatmap ramp (light): never clears %.1f:1; floor cell = %.2f:1", MeaningFloor, low)
}

// meaningRoles are the roles the kit uses for marks that carry meaning — a
// chart mark, a status dot, a graded fill, a focus ring, body/secondary text.
// Every one of them is drawn on a surface, so every one of them has a number.
var meaningRoles = []string{
	"ink", "ink-muted", "ink-faint",
	"accent", "accent-hover", "focus",
	"positive", "caution", "danger", "info",
}

// knownBelowFloor records the kit palette's measured meaning-floor failures.
// It is a ratchet, not an excuse list: a new failure fails this test, and
// fixing a listed one ALSO fails this test (with a note to delete the entry),
// so the list can only shrink. Each entry is documented in
// docs/theming-hazards.md §"Known open in this kit".
//
//	caution/light — oklch(0.70 0.14 80) on surface measures 2.60:1. Amber at a
//	lightness that reads as "warning" on a near-white surface cannot clear 3:1;
//	the fix is a darker light-scheme caution (or pairing the mark with a label),
//	and it is a token change with visual consequences, so it is recorded here
//	rather than made silently.
var knownBelowFloor = map[string]bool{
	"caution on surface (light)": true,
}

// TestKitPaletteMeaningFloor measures every meaning-bearing role against the
// surface it is drawn on, in both schemes. Run it with -v to print the kit's
// contrast table — that table is the answer to "did you measure it?".
//
//	go test ./appkit/ui/tokens -run TestKitPaletteMeaningFloor -v
func TestKitPaletteMeaningFloor(t *testing.T) {
	tok, err := Load(kitSource(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, scheme := range []string{"light", "dark"} {
		for _, role := range meaningRoles {
			got, err := ContrastRoles(tok, role, "surface", scheme)
			if err != nil {
				t.Fatalf("ContrastRoles(%q, surface, %q): %v", role, scheme, err)
			}
			key := role + " on surface (" + scheme + ")"
			t.Logf("%-34s %5.2f:1", key, got)
			switch {
			case got < MeaningFloor && !knownBelowFloor[key]:
				t.Errorf("%s = %.2f:1, below the %.1f:1 meaning floor — fix the token, or "+
					"stop using this role for marks that carry meaning", key, got, MeaningFloor)
			case got >= MeaningFloor && knownBelowFloor[key]:
				t.Errorf("%s now measures %.2f:1 and clears the floor — delete it from "+
					"knownBelowFloor and from docs/theming-hazards.md", key, got)
			}
		}
	}
}

// TestEveryKitRoleIsMeasurable guards the instrument itself: if a token set
// introduces a color form Contrast cannot parse, the measurement silently stops
// happening — which is exactly the failure this package exists to prevent.
func TestEveryKitRoleIsMeasurable(t *testing.T) {
	tok, err := Load(kitSource(t))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for _, role := range ColorNames {
		for _, scheme := range []string{"light", "dark"} {
			if _, err := ContrastRoles(tok, role, "surface", scheme); err != nil {
				t.Errorf("role %q (%s) is not measurable: %v", role, scheme, err)
			}
		}
	}
}
