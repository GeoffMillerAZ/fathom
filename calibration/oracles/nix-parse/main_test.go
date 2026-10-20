package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

func siteNames(src string) []string {
	var out []string
	info := readRoot(lex([]byte(src)))
	for _, s := range info.sites {
		out = append(out, s.scope+":"+s.name)
	}
	for _, p := range info.params {
		out = append(out, "param:"+p.name)
	}
	sort.Strings(out)
	return out
}

// TestReadRoot: only the root expression's own names are definitions, peeled through at most three wrappers.
func TestReadRoot(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"set", "{ a = 1; b.c = 2; d = { e = 3; }; }", []string{"attr:a", "attr:b", "attr:d"}},
		{"rec set and inherit", "rec { a = 1; inherit x; f = y: y; }", []string{"attr:a", "attr:f"}},
		{"fn let set", "{ p, q ? 1 }: let v = 1; in { s = v; }", []string{"attr:s", "let:v", "param:p", "param:q"}},
		{"alias", "args@{ p, ... }: { s = 1; }", []string{"attr:s", "param:args", "param:p"}},
		{"with let", "with import <x> { }; let v = 1; in { s = v; }", []string{"attr:s", "let:v"}},
		{"set merged with another is no root", "{ a = 1; } // { b = 2; }", nil},
		{"application is no root", "{ lib }: lib.mkIf true { a = 1; }", []string{"param:lib"}},
		{"four wrappers", "a: b: c: d: { s = 1; }", []string{"param:a", "param:b", "param:c"}},
		{"strings and comments", "{ # a = 1;\n \"q\" = 2; /* b = 3; */ c = \"x; y = 4;\"; }", []string{"attr:c"}},
		{"let with a nested let and with in a value", "let a = let b = 1; in b; c = with x; y; in { s = 1; }", []string{"attr:s", "let:a", "let:c"}},
	}
	for _, c := range cases {
		got := siteNames(c.src)
		if !reflect.DeepEqual(got, c.want) && !(len(got) == 0 && len(c.want) == 0) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

func TestReadRootLines(t *testing.T) {
	info := readRoot(lex([]byte("{ a,\n b }:\n{\n  x =\n    1;\n  y = 2;\n}\n")))
	var got []string
	for _, s := range info.sites {
		got = append(got, s.name+":"+strconv.Itoa(s.line)+"-"+strconv.Itoa(s.end))
	}
	for _, p := range info.params {
		got = append(got, p.name+":"+strconv.Itoa(p.line))
	}
	want := []string{"x:4-5", "y:6-6", "a:1", "b:2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestLexPathsAndStrings(t *testing.T) {
	toks := lex([]byte("[ ./a.nix ../b/c ./${x}.nix \"./not\" ''./not'' <nixpkgs> a/b ~/h /abs ]"))
	var got []string
	for _, tk := range toks {
		if tk.kind == tPath {
			text := tk.text
			if tk.interp {
				text += " (interpolated)"
			}
			got = append(got, text)
		}
	}
	want := []string{"./a.nix", "../b/c", "./${x}.nix (interpolated)", "<nixpkgs>", "a/b", "~/h", "/abs"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("paths %v, want %v", got, want)
	}
}

func TestParseAST(t *testing.T) {
	printed := `({ lib, pkgs ? (import /r/x.nix), ... } @ args: (let h = (x: (y: (x + y))); in { a = (h 1 2); b = ((pkgs).callPackage /r/p.nix { }); c = (lib).types.bool; d = [ (1) ("x") ((h 1)) ]; e = (if a then b else (__sub 0 1)); inherit (lib) mkOption; }))`
	n, err := parseAST(printed)
	if err != nil {
		t.Fatal(err)
	}
	if n.kind != nLambda || !reflect.DeepEqual(n.formals, []string{"lib", "pkgs"}) || n.arg != "args" {
		t.Fatalf("lambda %+v", n)
	}
	lets, sets, params := rootScopes(n)
	if len(lets) != 1 || lets["h"].kind != nLambda || !params["lib"] || !params["args"] {
		t.Fatalf("lets %v params %v", lets, params)
	}
	if got := classify(sets["a"], "attr"); got != "attribute" {
		t.Errorf("a: %s", got)
	}
	if got := classify(sets["b"], "attr"); got != "package" {
		t.Errorf("b: %s", got)
	}
	cs := map[string]bool{}
	for _, k := range sets["e"].kids {
		callees(k, cs)
	}
	if len(cs) != 0 {
		t.Errorf("a subtraction is no call: %v", cs)
	}
	callees(sets["d"], cs)
	if !cs["h"] {
		t.Errorf("callees of d = %v", cs)
	}
	if sets["mkOption"] == nil || sets["mkOption"].kind != nSelect {
		t.Errorf("inherit (lib) mkOption should read as lib.mkOption: %+v", sets["mkOption"])
	}
}

func TestXref(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, src string) {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("u.nix", "{ f = x: x; g = 1; }")
	write("lib/default.nix", "{ h = 1; }")
	printed := "(let u = (import " + dir + "/u.nix); l = (import " + dir + "/lib); shadow = (import " + dir + "/u.nix); in { a = (u.f 1); b = ((import " + dir + "/u.nix).f 2); c = (l.h 3); d = ((y: (u.f y)) 1); e = ((u: (u.f 1)) 2); k = (shadow.zz 1); })"
	n, err := parseAST(printed)
	if err != nil {
		t.Fatal(err)
	}
	x := &xr{subject: dir}
	x.root(n, scope{})
	var got []string
	for _, c := range x.cands {
		got = append(got, c.from+">"+c.to+"."+c.attr)
	}
	sort.Strings(got)
	want := []string{"a>u.nix.f", "b>u.nix.f", "c>lib/default.nix.h", "d>u.nix.f", "k>u.nix.zz"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("candidates %v, want %v (e is shadowed by its own argument)", got, want)
	}
}

// TestAgainstNixInstantiate runs the real reference parser when the host has it.
func TestAgainstNixInstantiate(t *testing.T) {
	bin, err := exec.LookPath("nix-instantiate")
	if err != nil {
		t.Skip("nix-instantiate not on PATH")
	}
	dir := t.TempDir()
	src := "{ config, lib, pkgs, ... }:\nwith lib;\nlet\n  cfg = config.programs.foo;\n  helper = name: mkIf cfg.enable { ${name} = 1; };\nin\n{\n  imports = [ ./other.nix ];\n  options.programs.foo.enable = mkEnableOption \"foo\";\n  config = helper \"x\";\n  size = 2 * 3;\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "m.nix"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "other.nix"), []byte("{ }"), 0o644); err != nil {
		t.Fatal(err)
	}
	printed, why := parseFile(bin, filepath.Join(dir, "m.nix"))
	if why != "" {
		t.Skipf("parser refused: %s", why)
	}
	res, err := extract(dir, "m.nix", []byte(src), printed)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range res.defs {
		got = append(got, d.Kind+":"+d.Name)
	}
	sort.Strings(got)
	want := []string{"attribute:config", "attribute:imports", "attribute:size", "function:helper", "option:options", "param:config", "param:lib", "param:pkgs", "variable:cfg"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("defs %v, want %v", got, want)
	}
	if len(res.imports) != 1 || res.imports[0].ToPath != "other.nix" || res.imports[0].Line != 8 {
		t.Errorf("imports %+v", res.imports)
	}
	var calls []string
	for _, c := range res.calls {
		calls = append(calls, c.From+">"+c.To)
	}
	sort.Strings(calls)
	if want := []string{"config>helper", "helper>mkIf", "options>mkEnableOption"}; !reflect.DeepEqual(calls, want) {
		t.Errorf("calls %v, want %v", calls, want)
	}
}

// TestStmtEndWithInsideLet: a `with e;` in a nested let's binding owns its own `;`, not the statement's.
func TestStmtEndWithInsideLet(t *testing.T) {
	got := siteNames("{ a = let b = with x; y; in b; c = assert d; e; f = 1; }")
	want := []string{"attr:a", "attr:c", "attr:f"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
}
