package main

import (
	"fmt"
	"reflect"
	"testing"
)

func roles(src string) map[string][]identRole {
	out := map[string][]identRole{}
	for _, id := range scan([]byte(src)).Idents {
		out[id.Text] = append(out[id.Text], id.Role)
	}
	return out
}

// TestScanRoles: labels, let, alias and loop variables are declarations; a
// value's identifiers, a selector's and an interpolation's are candidate
// references; comments and plain string text are not scanned.
func TestScanRoles(t *testing.T) {
	src := `package kit

import (
	"strings"
	b "example.com/x/base"
)

// ref in a comment: commented
#Kit: {
	b.#Base
	name?: string
	X=other: ref1
	let L = {z: 1}
	use: L.z
	for k, v in src {
		"\(k)": v
	}
	text: "plain ${notref} \(interp)"
	raw: #"""
		\#(rawref) and \(notinterp)
		"""#
}
`
	r := roles(src)
	check := func(name string, want ...identRole) {
		t.Helper()
		if got := r[name]; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: roles %v, want %v", name, got, want)
		}
	}
	check("kit", rolePackage)
	check("b", roleImportAlias, roleRef)
	check("#Base", roleRef)
	check("name", roleLabel)
	check("X", roleDecl)
	check("other", roleLabel)
	check("ref1", roleRef)
	check("L", roleDecl, roleRef)
	check("z", roleLabel, roleRef)
	check("k", roleDecl, roleRef)
	check("v", roleDecl, roleRef)
	check("src", roleRef)
	check("interp", roleRef)
	check("rawref", roleRef)
	for _, absent := range []string{"commented", "notref", "notinterp", "plain"} {
		if _, ok := r[absent]; ok {
			t.Errorf("%s: scanned, but it is comment or string text", absent)
		}
	}
}

// TestScanImportLineDoesNotLeak: after `import "path"`, an identifier
// followed by a string (an interpolation's last name before the next string)
// is a reference, not an import alias.
func TestScanImportLineDoesNotLeak(t *testing.T) {
	src := "package p\n\nimport \"list\"\n\na: #\"\"\"\n\t\\#(f1)\n\t\"\"\"#\nb: #\"\"\"\n\t\\#(f2)\n\t\"\"\"#\n"
	r := roles(src)
	if !reflect.DeepEqual(r["f1"], []identRole{roleRef}) || !reflect.DeepEqual(r["f2"], []identRole{roleRef}) {
		t.Fatalf("roles %v", r)
	}
	res := scan([]byte(src))
	if len(res.Imports) != 1 || res.Imports[0].Path != "list" || res.Imports[0].Line != 3 {
		t.Fatalf("imports %+v", res.Imports)
	}
}

func TestScanImports(t *testing.T) {
	res := scan([]byte("package p\nimport (\n\t\"strings\"\n\ts \"example.com/m/schema\"\n)\nimport t \"example.com/m/t:tpkg\"\n"))
	var got []string
	for _, im := range res.Imports {
		got = append(got, im.Alias+"="+im.Path)
	}
	if want := []string{"=strings", "s=example.com/m/schema", "t=example.com/m/t:tpkg"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("imports %v, want %v", got, want)
	}
	if res.Imports[1].AliasAt.Line != 4 || res.Imports[1].AliasAt.Col != 1 {
		t.Errorf("alias position %+v", res.Imports[1].AliasAt)
	}
}

func TestNormalizeLabelAndBareName(t *testing.T) {
	for name, want := range map[string][3]string{
		"#Kit":       {"#Kit", "definition", "true"},
		"_#Hidden":   {"_#Hidden", "definition", "true"},
		"X=aliased":  {"aliased", "field", "true"},
		`"quoted-a"`: {"quoted-a", "field", "true"},
		"[string]":   {"[string]", "pattern", "false"},
		`"gen_\(k)"`: {`"gen_\(k)"`, "dynamic", "false"},
		"(k)":        {"(k)", "dynamic", "false"},
		"plain":      {"plain", "field", "true"},
	} {
		l, k, s := normalizeLabel(name)
		if got := [3]string{l, k, map[bool]string{true: "true", false: "false"}[s]}; got != want {
			t.Errorf("%s: %v, want %v", name, got, want)
		}
	}
	for full, want := range map[string]string{"#Kit": "Kit", "#Kit.ports.port": "port", "a.b c": "c", "web": "web"} {
		if got := bareName(full); got != want {
			t.Errorf("bareName(%q) = %q, want %q", full, got, want)
		}
	}
}

func TestModulePathDropsMajorVersion(t *testing.T) {
	if got := modulePath([]byte("module: \"example.com/demo@v0\"\nlanguage: version: \"v0.16.0\"\n")); got != "example.com/demo" {
		t.Fatalf("modulePath = %q", got)
	}
}

func TestInstanceImportPaths(t *testing.T) {
	m := &module{root: "/s/mod", rel: "mod", path: "example.com/m"}
	for rel, want := range map[string][2]string{
		"mod/schema/a.cue":                       {"example.com/m/schema", "schema"},
		"mod/schema/b.cue":                       {"example.com/m/schema:other", "other"},
		"mod/root.cue":                           {"example.com/m:root", "root"},
		"mod/cue.mod/pkg/vendor.example/x/x.cue": {"vendor.example/x", "x"},
	} {
		f := &cueFile{rel: rel, module: m, scan: scanResult{Package: want[1]}}
		if arg, dir := instance(f, "/s"); arg != want[0] || dir != "/s/mod" {
			t.Errorf("%s: %s in %s, want %s", rel, arg, dir, want[0])
		}
	}
}

func TestModuleDefs(t *testing.T) {
	src := "module: \"example.com/m@v0\"\nlanguage: {\n\tversion: \"v0.16.0\"\n}\nsource: kind: \"self\"\ndeps: {\n\t\"example.com/dep@v0\": {\n\t\tv: \"v0.1.0\"\n\t}\n}\n"
	f := &cueFile{rel: "cue.mod/module.cue", src: []byte(src)}
	exported := map[string]any{"module": "example.com/m@v0", "language": map[string]any{"version": "v0.16.0"},
		"source": map[string]any{"kind": "self"}, "deps": map[string]any{"example.com/dep@v0": map[string]any{"v": "v0.1.0"}}}
	got := map[string][2]int{}
	for _, d := range moduleDefs(f, exported) {
		got[d.Name+"@"+fmt.Sprint(d.Line)] = [2]int{d.Line, d.End}
	}
	want := map[string][2]int{"module@1": {1, 1}, "language@2": {2, 4}, "version@3": {3, 3}, "source@5": {5, 5}, "kind@5": {5, 5},
		"deps@6": {6, 10}, "dep@v0@7": {7, 9}, "v@8": {8, 8}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("moduleDefs %v, want %v", got, want)
	}
}
