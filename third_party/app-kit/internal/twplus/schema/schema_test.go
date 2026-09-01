package schema_test

import (
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/twplus/normalize"
	"github.com/geoffmilleraz/app-kit/internal/twplus/schema"
)

func newValidator(t *testing.T) *schema.Validator {
	t.Helper()
	v, err := schema.NewValidator()
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func comp(section, mode, framework string, tw int) normalize.Component {
	return normalize.Component{
		Section: section, Group: "Elements", Category: "Badges", Name: "Flat",
		FullName: section + ".Elements.Badges.Flat",
		Variants: []normalize.Variant{{
			FullName: section + ".Elements.Badges.Flat", Name: "Flat", Code: "x",
			Language: "html", Framework: framework, TailwindVersion: tw, Mode: mode, SupportsDarkMode: true,
		}},
	}
}

func TestValidateAccepts(t *testing.T) {
	v := newValidator(t)
	for _, c := range []normalize.Component{
		comp("Application UI", "light", "react", 4),
		comp("Marketing", "dark", "vue", 3),
		comp("Ecommerce", "none", "html", 4),
	} {
		if err := v.ValidateComponent(c); err != nil {
			t.Errorf("ValidateComponent(%s/%s) unexpected error: %v", c.Section, c.Variants[0].Mode, err)
		}
	}
}

func TestValidateRejects(t *testing.T) {
	v := newValidator(t)
	cases := map[string]normalize.Component{
		"ecommerce must be none":       comp("Ecommerce", "light", "html", 4),
		"non-ecommerce cannot be none": comp("Application UI", "none", "html", 4),
		"unknown framework":            comp("Application UI", "light", "svelte", 4),
		"bad tailwind version":         comp("Application UI", "light", "html", 5),
		"unknown section":              comp("Widgets", "light", "html", 4),
	}
	for name, c := range cases {
		if err := v.ValidateComponent(c); err == nil {
			t.Errorf("%s: expected validation error, got nil", name)
		}
	}
}
