package catalog

import (
	"slices"
	"testing"
)

// The palette's originating consumer (Fathom) has not integrated it, so the
// catalog must not report it as an observed consumer; the inventory item
// candidate.command-palette records the same empty consumer list.
func TestCommandPaletteClaimsNoUnintegratedConsumer(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Get("interaction.command-palette")
	if !ok || e.Kind != "primitive" || e.Maturity != "experimental" || e.Preview != "/palette" ||
		!slices.Equal(e.Layers, []string{"L2", "L3b"}) || !slices.Equal(e.Profiles, []string{"go-web"}) {
		t.Fatalf("palette fit or promotion claim changed: %+v", e)
	}
	if len(e.Consumers) != 0 {
		t.Errorf("consumers %v: an originating consumer that has not integrated the palette is not an observed consumer", e.Consumers)
	}
}

// Common phrasings of the palette's defining task (a keyboard shortcut that opens
// a site-wide search) must reach it through declared task phrases (ADR-0062).
func TestCommandPaletteCommonPhrasingsFindIt(t *testing.T) {
	c, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		"command palette", "search dialog", "search results", "palette",
		"keyboard shortcut", "keyboard shortcut search", "cmd+k command palette", "ctrl+k", "mod+k",
		"search palette", "global search", "site search", "search modal", "quick open", "command menu",
	} {
		page, err := c.Search(Query{Text: text, Limit: 5})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range page.Items {
			found = found || item.ID == "interaction.command-palette"
		}
		if !found {
			t.Errorf("query %q does not find interaction.command-palette in its first five results", text)
		}
	}
}
