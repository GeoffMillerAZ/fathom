package main

import (
	"reflect"
	"testing"
)

func TestMatches(t *testing.T) {
	for _, tc := range []struct {
		rel, format string
		want        bool
	}{
		{".fathom/concerns.cue", "concerns", true},
		{"a/.fathom/concerns.cue", "concerns", true},
		{".fathom/guard.cue", "concerns", false},
		{".fathom/layers/ui/layer.cue", "layer-packs", true},
		{".fathom/rules/arch.cue", "layer-packs", true},
		{".fathom/rules/sub/arch.cue", "layer-packs", false},
		{".fathom/concerns.cue", "layer-packs", false},
	} {
		if got := matches(tc.rel, tc.format); got != tc.want {
			t.Errorf("matches(%q, %q) = %v, want %v", tc.rel, tc.format, got, tc.want)
		}
	}
}

func TestPlaceConcerns(t *testing.T) {
	f := ".fathom/concerns.cue"
	defs := []def{
		{f, "concerns", 5, 20, "field"},
		{f, "a", 6, 8, "field"},
		{f, "want", 7, 7, "field"}, // depth 2: never a concern, even when named like one
		{f, "", 10, 14, "dynamic"},
		{f, "want", 11, 11, "field"},
		{f, "concerns", 30, 32, "field"},
		{f, "b", 31, 31, "field"},
		{f, "c", 32, 32, "field"}, // a one-line concern: its own field shares its lines
		{f, "off", 32, 32, "field"},
	}
	got, missing := place(f, "concerns", []string{"a", "b", "c", "gen", "want"}, defs)
	want := []def{
		{f, "a", 6, 8, "rule"},
		{f, "b", 31, 31, "rule"},
		{f, "c", 32, 32, "rule"},
		{f, "gen", 10, 14, "rule"},
		{f, "want", 10, 14, "rule"},
	}
	if !reflect.DeepEqual(got, want) || len(missing) != 0 {
		t.Errorf("place = %+v, missing %v; want %+v", got, missing, want)
	}
}

func TestPlaceLayerAndRules(t *testing.T) {
	l := ".fathom/layers/x/layer.cue"
	defs := []def{{l, "name", 3, 3, "field"}, {l, "budgets", 5, 8, "field"}, {l, "name", 6, 6, "field"}}
	got, _ := place(l, "layer-packs", []string{"x"}, defs)
	if !reflect.DeepEqual(got, []def{{l, "x", 3, 3, "policy"}}) {
		t.Errorf("layer = %+v", got)
	}
	r := ".fathom/rules/arch.cue"
	got, _ = place(r, "layer-packs", []string{"arch"}, []def{{r, "rules", 4, 9, "field"}})
	if !reflect.DeepEqual(got, []def{{r, "arch", 4, 9, "rule"}}) {
		t.Errorf("rules = %+v", got)
	}
	if _, missing := place(r, "layer-packs", []string{"arch"}, nil); len(missing) != 1 {
		t.Errorf("a rule file with no rules symbol should be missing, got %v", missing)
	}
}
