package designadapt

import (
	"strings"
	"testing"
)

func TestHeroEnhancedArrowTrackIsScopedToCSS(t *testing.T) {
	cssBytes, err := heroImplementation.ReadFile("hero_styles.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(cssBytes)
	selector := `[data-appkit-hero] a:has(> span[aria-hidden="true"])`
	override := selector + ` {
  display: inline-grid;
  grid-template-columns: minmax(0, 1fr) auto;
}`
	if strings.Count(css, selector) != 1 || !strings.Contains(css, override) {
		t.Fatal("enhanced arrow link is missing its bounded text and icon tracks")
	}
	base := `[data-appkit-hero] a {
  display: inline-flex;`
	if strings.Count(css, base) != 1 || strings.Index(css, override) < strings.Index(css, base) {
		t.Fatal("arrow override does not preserve the base enhanced link treatment")
	}
	scrollMargin := `  white-space: normal;
  scroll-margin-block: 0.5rem;
}`
	if strings.Count(css, "scroll-margin-block: 0.5rem;") != 1 || !strings.Contains(css, scrollMargin) {
		t.Fatal("enhanced links do not retain a scoped focus-scroll margin")
	}
	if strings.Count(css, "display: inline-grid;") != 1 || strings.Count(css, "grid-template-columns: minmax(0, 1fr) auto;") != 1 {
		t.Fatal("arrow grid repair escaped its single scoped rule")
	}

	// This repair belongs only to optional companion CSS. Neither generated
	// template structure nor the runtime model should encode its layout rule.
	for _, name := range []string{"hero.go", "hero_model.go.txt"} {
		body, err := heroImplementation.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, marker := range []string{selector, "display: inline-grid", "grid-template-columns", "scroll-margin-block"} {
			if strings.Contains(string(body), marker) {
				t.Fatalf("%s contains CSS-only arrow repair %q", name, marker)
			}
		}
	}
}
