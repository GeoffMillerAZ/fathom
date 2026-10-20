package extractor

// nixdialect_test.go — LB-Q-nix (design §13.2 steps 10 and 12). The per-fixture goldens run in the generic
// TestDialectGoldens (dialects/nix/testdata); this file holds what a golden cannot express: the parser survives a
// syntax error, and the root-expression rule (the file's expression peeled through a function, a let and a with;
// a key at depth one is a definition, a deeper key is not).

import (
	"os"
	"slices"
	"testing"
)

func nixSymbols(t *testing.T, src string) []string {
	t.Helper()
	e, err := NewTSExtractor("nix")
	if err != nil {
		t.Fatalf("NewTSExtractor(nix): %v", err)
	}
	defer e.Close()
	nodes, err := e.Extract("svc", "x.nix", []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var out []string
	for _, n := range nodes {
		out = append(out, string(n.Kind)+":"+n.Name)
	}
	return out
}

func TestNixDialect_SurvivesSyntaxError(t *testing.T) {
	src, err := os.ReadFile("dialects/nix/testdata/broken.nix")
	if err != nil {
		t.Fatal(err)
	}
	got := nixSymbols(t, string(src))
	for _, want := range []string{"NixFunction:before", "NixAttribute:kept", "NixAttribute:last"} {
		if !slices.Contains(got, want) {
			t.Errorf("symbols around the error = %v, missing %s", got, want)
		}
	}
}

// TestNixDialect_RootExpressionDepth: only the root expression's own names are definitions.
func TestNixDialect_RootExpressionDepth(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"attrset", "{ a = 1; b.c = 2; d = { e = 3; }; }", []string{"NixAttribute:a", "NixAttribute:b", "NixAttribute:d"}},
		{"rec", "rec { a = 1; f = x: x; }", []string{"NixAttribute:a", "NixFunction:f"}},
		{"function then let then set", "{ p }: let v = 1; in { s = v; }", []string{"NixParam:p", "NixVariable:v", "NixAttribute:s"}},
		{"with then let", "with import <x> { }; let v = 1; in { s = v; }", []string{"NixVariable:v", "NixAttribute:s"}},
		{"curried", "a: b: { s = 1; }", []string{"NixParam:a", "NixParam:b", "NixAttribute:s"}},
		{"an inner let and a nested set are not the root", "{ f = x: let y = 1; in { z = y; }; g = { h = 1; }; }", []string{"NixFunction:f", "NixAttribute:g"}},
		{"an application is no root set", "{ lib }: lib.mkIf true { a = 1; }", []string{"NixParam:lib"}},
		{"kinds", "{ options.x.y = 1; pkg = callPackage ./p.nix { }; o = lib.mkOption { }; n = null; }",
			[]string{"NixOption:options", "NixPackage:pkg", "NixOption:o", "NixAttribute:n"}},
	}
	for _, c := range cases {
		got := nixSymbols(t, c.src)
		slices.Sort(got)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: symbols = %v, want %v", c.name, got, want)
		}
	}
}
