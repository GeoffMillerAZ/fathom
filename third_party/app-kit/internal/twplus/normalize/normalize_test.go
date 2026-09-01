package normalize

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/twplus/parse"
)

func ptr(s string) *string { return &s }

func TestBuildDerivesFrameworkAndMode(t *testing.T) {
	c := parse.Component{
		Path: []string{"Application UI", "Elements", "Badges", "Flat"},
		Raw: parse.RawComponent{
			Name: "Flat",
			Snippets: []parse.RawSnippet{
				{Code: `<span class="rounded-md flex flex">b</span>`, Language: "html", Name: "html", Mode: ptr("light"), Version: 4, SupportsDarkMode: true},
				{Code: `<template><span/></template>`, Language: "html", Name: "vue", Mode: ptr("dark"), Version: 4, SupportsDarkMode: true},
				{Code: `const X=()=>null`, Language: "jsx", Name: "react", Mode: ptr("system"), Version: 3, SupportsDarkMode: true},
			},
		},
	}
	comp, rows, err := Build(c)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if comp.FullName != "Application UI.Elements.Badges.Flat" {
		t.Errorf("FullName = %q", comp.FullName)
	}
	if comp.Section != "Application UI" || comp.Group != "Elements" || comp.Category != "Badges" || comp.Name != "Flat" {
		t.Errorf("taxonomy split wrong: %+v", comp)
	}
	want := []string{"html", "vue", "react"}
	for i, v := range comp.Variants {
		if v.Framework != want[i] {
			t.Errorf("variant %d framework = %q, want %q", i, v.Framework, want[i])
		}
		if v.Name != "Flat" {
			t.Errorf("variant %d name = %q, want the component name 'Flat'", i, v.Name)
		}
	}
	// Classes ranked by frequency: "flex" (2x) before "rounded-md" (1x).
	if got := rows[0].Classes; !strings.HasPrefix(got, "flex ") || !strings.Contains(got, "rounded-md") {
		t.Errorf("classes = %q, want flex first and rounded-md present", got)
	}
	if !strings.Contains(rows[0].EmbedText, "Application UI > Elements > Badges > Flat") {
		t.Errorf("embed_text missing taxonomy line: %q", rows[0].EmbedText)
	}
}

func TestBuildNullModeBecomesNone(t *testing.T) {
	c := parse.Component{
		Path: []string{"Ecommerce", "Components", "Carts", "Slide-over"},
		Raw: parse.RawComponent{
			Name:     "Slide-over",
			Snippets: []parse.RawSnippet{{Code: "x", Language: "html", Name: "html", Mode: nil, Version: 4}},
		},
	}
	comp, rows, err := Build(c)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if comp.Variants[0].Mode != "none" {
		t.Errorf("nil mode → %q, want none", comp.Variants[0].Mode)
	}
	if rows[0].Mode != "none" {
		t.Errorf("row nil mode → %q, want none", rows[0].Mode)
	}
}

func TestBuildRejectsUnknownFramework(t *testing.T) {
	c := parse.Component{
		Path: []string{"Application UI", "Elements", "Badges", "Flat"},
		Raw:  parse.RawComponent{Name: "Flat", Snippets: []parse.RawSnippet{{Code: "x", Language: "svelte", Name: "svelte", Mode: ptr("light"), Version: 4}}},
	}
	if _, _, err := Build(c); err == nil {
		t.Fatal("expected error for unknown framework, got nil")
	}
}
