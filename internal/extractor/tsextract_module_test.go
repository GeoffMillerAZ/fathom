package extractor

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// moduleOf returns the one module symbol among nodes, failing unless there is
// exactly one.
func moduleOf(t *testing.T, nodes []core.SymbolNode) core.SymbolNode {
	t.Helper()
	var out []core.SymbolNode
	for _, n := range nodes {
		if n.IsModuleSymbol() {
			out = append(out, n)
		}
	}
	if len(out) != 1 {
		t.Fatalf("module symbols = %+v, want exactly one", out)
	}
	return out[0]
}

// TestModuleSymbolPython (LB-F348, K18): with python's module_symbol on, each
// file has one module symbol — named core.ModuleSymbolName, kind PythonModule,
// line 1 to the file's last line — that owns the calls its top level makes and
// no definition encloses or claims: a statement's, an `if`'s, a `for`'s. A
// call inside a function, a decorator's, an assignment's (its variable's) and
// one in a class body stay where they were.
func TestModuleSymbolPython(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	fixture := filepath.Join("dialects", "python", "testdata", "module.py")
	src, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := e.Extract("svc", "app/module.py", src)
	if err != nil {
		t.Fatal(err)
	}
	m := moduleOf(t, nodes)
	lines := bytes.Count(src, []byte("\n"))
	if m.Kind != "PythonModule" || m.Line != 1 || m.EndLine != lines || m.Package != "module" || m.Signature != "" {
		t.Fatalf("module symbol = %+v, want PythonModule, lines 1..%d, package module, unsigned", m, lines)
	}
	if want := []string{"collect", "consume", "h", "main", "registry.update", "setup_logging"}; !reflect.DeepEqual(m.Calls, want) {
		t.Errorf("module calls = %q, want %q", m.Calls, want)
	}
	owner := map[string]string{}
	for _, n := range nodes {
		for _, c := range n.Calls {
			owner[c] = n.Name
		}
	}
	for call, want := range map[string]string{
		"load_config": "CONFIG", "os.path.join": "CONFIG", "register": "handler", "process": "handler",
		"compute": "size", "start": "Widget.ready",
	} {
		if owner[call] != want {
			t.Errorf("%s is %q's, want %q's", call, owner[call], want)
		}
	}
	if _, ok := owner["log"]; ok {
		t.Errorf("a call in a class body is the module's or another's (%q), want none's as before", owner["log"])
	}
	if m.Key() == core.SymbolKey("svc", "app/module", "Widget.__init__") || m.Key() != core.SymbolKey("svc", "app/module", core.ModuleSymbolName) {
		t.Errorf("module key = %q", m.Key())
	}

	// Its imports are the file's, unchanged: the file resolver makes them from
	// the module symbol (fusion.moduleImportEdges).
	imps, err := e.ExtractImports("app/module.py", src)
	if err != nil {
		t.Fatal(err)
	}
	var mods []string
	for _, s := range imps {
		mods = append(mods, s.Module)
	}
	if want := []string{"os.path", ".", "pkg.sub"}; !reflect.DeepEqual(mods, want) {
		t.Errorf("import modules = %q, want %q", mods, want)
	}

	// An empty file's top level is a module too (scip-python defines one per
	// document, empty __init__.py included).
	empty, err := e.Extract("svc", "app/__init__.py", nil)
	if err != nil {
		t.Fatal(err)
	}
	if m := moduleOf(t, empty); m.Line != 1 || m.EndLine != 1 || len(m.Calls) != 0 || len(empty) != 1 {
		t.Errorf("empty file = %+v, want its module symbol alone at line 1", empty)
	}
}

// withoutModuleSymbol is e's spec with every variant's module_symbol off.
func withoutModuleSymbol(t *testing.T, id string) *TSExtractor {
	t.Helper()
	reg, err := tsRegistry()
	if err != nil {
		t.Fatal(err)
	}
	spec := *reg[id]
	off := func(v tsVariant) tsVariant {
		r := *v.rules
		r.moduleKind = nil
		v.rules = &r
		return v
	}
	spec.defaultVariant = off(spec.defaultVariant)
	spec.variantForExt = map[string]tsVariant{}
	for ext, v := range reg[id].variantForExt {
		spec.variantForExt[ext] = off(v)
	}
	return newTSExtractorFromSpec(&spec)
}

// TestModuleSymbolIsAnAddition (LB-F348): python's goldens change by
// additions only — every fixture extracts exactly as with the switch off, plus
// its one module symbol at line 1.
func TestModuleSymbolIsAnAddition(t *testing.T) {
	on, err := NewTSExtractor("python")
	if err != nil {
		t.Fatal(err)
	}
	defer on.Close()
	off := withoutModuleSymbol(t, "python")
	defer off.Close()
	for _, fx := range dialectFixtures(t, "python") {
		src, err := os.ReadFile(fx)
		if err != nil {
			t.Fatal(err)
		}
		rel := "src/" + filepath.Base(fx)
		with, err := on.Extract("svc", rel, src)
		if err != nil {
			t.Fatal(err)
		}
		without, err := off.Extract("svc", rel, src)
		if err != nil {
			t.Fatal(err)
		}
		if m := moduleOf(t, with); m.Line != 1 {
			t.Errorf("%s: module symbol at line %d, want 1", fx, m.Line)
		}
		rest := slices.DeleteFunc(slices.Clone(with), func(n core.SymbolNode) bool { return n.IsModuleSymbol() })
		if !reflect.DeepEqual(rest, without) {
			t.Errorf("%s: the module symbol changed the other definitions\n--- with (less the module)\n%+v\n--- without\n%+v", fx, rest, without)
		}
		for _, n := range without {
			if n.IsModuleSymbol() {
				t.Errorf("%s: a module symbol with the switch off: %+v", fx, n)
			}
		}
	}
}

// TestModuleSymbolOnlyWhereTheManifestNamesIt (LB-F347): the module pass is
// a tags-engine capability a manifest turns on; every dialect that does not
// (conan included, on python's grammar) compiles without it, mints none, and
// extracts its fixtures byte-identically to their committed goldens.
func TestModuleSymbolOnlyWhereTheManifestNamesIt(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	var on []string
	sawConan := false
	for _, d := range ds {
		if d.Engine != "tags" && d.Engine != "tags+doc" {
			continue
		}
		r, err := compileDialectRules(d)
		if err != nil {
			t.Fatalf("%s: %v", d.ID, err)
		}
		if (r.moduleKind != nil) != (d.ModuleSymbol != "") {
			t.Errorf("%s: compiled module kind %v for module_symbol %q", d.ID, r.moduleKind, d.ModuleSymbol)
		}
		if d.ModuleSymbol != "" {
			on = append(on, d.ID)
			continue
		}
		defs := []*tsDef{{name: "f", start: 0, end: 3}}
		if got := mintTSModule(defs, []tsCallSite{{text: "g", start: 4, end: 7}}, nil, []byte("f()\ng()\n"), r, true); len(got) != 1 || &got[0] != &defs[0] {
			t.Errorf("%s: the module pass changed the definitions without module_symbol", d.ID)
		}
		if d.ID == "conan" {
			sawConan = true
			checkConanUnchanged(t)
			continue
		}
		if d.Engine != "tags" {
			continue
		}
		if _, err := os.Stat(filepath.Join("dialects", d.ID, "testdata")); err != nil {
			continue
		}
		e, err := NewTSExtractor(d.ID)
		if err != nil {
			t.Fatalf("NewTSExtractor(%s): %v", d.ID, err)
		}
		for _, fx := range dialectFixtures(t, d.ID) {
			got := extractGolden(t, e, fx)
			for _, s := range got.Symbols {
				if s.Name == core.ModuleSymbolName {
					t.Errorf("%s: %s minted a module symbol", d.ID, fx)
				}
			}
			raw, err := os.ReadFile(goldenPathFor(fx))
			if err != nil {
				t.Fatalf("%s: %v", fx, err)
			}
			if gb := marshalGolden(got); !bytes.Equal(gb, raw) {
				t.Errorf("%s: %s is no longer byte-identical to its golden", d.ID, fx)
			}
		}
		e.Close()
	}
	slices.Sort(on)
	if !reflect.DeepEqual(on, []string{"cpp", "css", "javascript", "python", "typescript"}) {
		t.Errorf("dialects with module_symbol = %q, want [cpp css javascript python typescript] (lf2 python, lf3 JS/TS, lf4 C++ for its #include and file-level references, LB-F434, lf5 CSS for its @import and unowned references, LB-F459; Ruby, Lua and shell turn it on in their own waves)", on)
	}
	if !sawConan {
		t.Error("conan (python's grammar, its own manifest) was not checked")
	}
}

// checkConanUnchanged: conan's conanfile.py rides the tags engine on python's
// grammar under its own manifest, which names no module_symbol; the fixtures
// the tags engine reads extract byte-identically to their goldens and mint no
// module symbol (its documents never reach the tags engine).
func checkConanUnchanged(t *testing.T) {
	t.Helper()
	x, err := NewTagsDocExtractor("conan")
	if err != nil {
		t.Fatalf("NewTagsDocExtractor(conan): %v", err)
	}
	defer x.Close()
	tagsFiles := 0
	for _, fx := range conanFixtures(t) {
		if x.IsDocFile(conanRel(fx)) {
			continue
		}
		tagsFiles++
		src, err := os.ReadFile(fx)
		if err != nil {
			t.Fatal(err)
		}
		got := conanExtract(t, x, conanRel(fx), src)
		for _, s := range got.Symbols {
			if s.Name == core.ModuleSymbolName {
				t.Errorf("conan: %s minted a module symbol", fx)
			}
		}
		raw, err := os.ReadFile(fx + ".golden.json")
		if err != nil {
			t.Fatalf("%s: %v", fx, err)
		}
		if gb := marshalConanGolden(got); !bytes.Equal(gb, raw) {
			t.Errorf("conan: %s is no longer byte-identical to its golden", fx)
		}
	}
	if tagsFiles == 0 {
		t.Error("conan: no fixture the tags engine reads")
	}
}
