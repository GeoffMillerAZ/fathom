package parse

import (
	"strings"
	"testing"
)

// fixture mirrors the real dump's nesting and the per-snippet `preview` field
// that Walk must drop (RawSnippet has no Preview field).
const fixture = `{
  "component_count": 1,
  "tailwindplus": {
    "Application UI": {
      "Elements": {
        "Badges": {
          "Flat": {
            "name": "Flat",
            "snippets": [
              {"code":"<span class=\"rounded-md\">x</span>","language":"html","name":"html","mode":"light","version":4,"supportsDarkMode":true,"preview":"<!doctype html><html>BIG PREVIEW</html>"},
              {"code":"const X = () => null","language":"jsx","name":"react","mode":"dark","version":3,"supportsDarkMode":true,"preview":"<!doctype html>"}
            ]
          }
        }
      }
    }
  }
}`

func TestWalk(t *testing.T) {
	var got []Component
	if err := Walk(strings.NewReader(fixture), func(c Component) error {
		got = append(got, c)
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d components, want 1", len(got))
	}
	c := got[0]
	if want := "Application UI.Elements.Badges.Flat"; c.FullName() != want {
		t.Errorf("FullName = %q, want %q", c.FullName(), want)
	}
	if len(c.Raw.Snippets) != 2 {
		t.Fatalf("got %d snippets, want 2", len(c.Raw.Snippets))
	}
	if c.Raw.Snippets[0].Name != "html" || c.Raw.Snippets[1].Name != "react" {
		t.Errorf("snippet frameworks = %q,%q; want html,react", c.Raw.Snippets[0].Name, c.Raw.Snippets[1].Name)
	}
	if m := c.Raw.Snippets[0].Mode; m == nil || *m != "light" {
		t.Errorf("snippet[0].Mode = %v, want light", m)
	}
	if c.Raw.Snippets[1].Version != 3 {
		t.Errorf("snippet[1].Version = %d, want 3", c.Raw.Snippets[1].Version)
	}
}

func TestWalkNullMode(t *testing.T) {
	// Ecommerce arrives with a null mode; it must decode to a nil pointer.
	const eco = `{"tailwindplus":{"Ecommerce":{"Components":{"Carts":{"Slide-over":{"name":"Slide-over",
		"snippets":[{"code":"x","language":"html","name":"html","mode":null,"version":4,"supportsDarkMode":false}]}}}}}}`
	var got Component
	if err := Walk(strings.NewReader(eco), func(c Component) error { got = c; return nil }); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if got.Raw.Snippets[0].Mode != nil {
		t.Errorf("null mode decoded to %v, want nil", got.Raw.Snippets[0].Mode)
	}
}
