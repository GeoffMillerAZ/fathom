package extractor

import (
	"testing"
	"testing/fstest"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestPackageStrategyFile (LB-F519, wave lf7 F1): package strategy "file"
// keys every definition of a file by the file's base name WITH its extension
// (X.psd1, X.psm1, Button.vue, +page.svelte, hero.component.html), so two
// files of one dialect sharing a name in one directory mint distinct module
// symbols; strategy "dir" keys both by the stem and collides (O915). A
// fixture manifest with the strategy and module_symbol loads (checkModuleSymbol
// admits "dir" or "file"); its rules, on python's grammar and tags query,
// extract two such files.
func TestPackageStrategyFile(t *testing.T) {
	const manifest = `{"id":"a","display":"A","kind":"language","engine":"tags","lang_tag":"a",
"match":{"extensions":[".a",".b"]},"grammar":{"blob":"python","entry_point":"tree_sitter_python"},"queries":{"tags":"tags.scm"},
"kinds":{"function":{"source_kind":"AF","canonical":"function","callable":true},"module":{"source_kind":"AModule","canonical":"module"}},
"required_kinds":["function"],"target_level":"L1","queue_id":"a","module_symbol":"module","package":{"strategy":"file"}}`
	fixture, err := loadDialects(fstest.MapFS{
		"dialects/a/dialect.json": &fstest.MapFile{Data: []byte(manifest)},
		"dialects/a/tags.scm":     &fstest.MapFile{Data: nil},
	})
	if err != nil {
		t.Fatalf("a module_symbol dialect with package strategy \"file\" must load: %v", err)
	}
	if len(fixture) != 1 || fixture[0].Package.Strategy != "file" {
		t.Fatalf("fixture = %+v, want one dialect with package strategy file", fixture)
	}

	extractWith := func(strategy string) *TSExtractor {
		t.Helper()
		ds, err := LoadDialects()
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range ds {
			if d.ID != "python" {
				continue
			}
			d.Package.Strategy = strategy
			rules, err := compileDialectRules(d)
			if err != nil {
				t.Fatal(err)
			}
			v := tsVariant{blob: d.Grammar.Blob, entryPoint: d.Grammar.EntryPoint, rules: rules}
			return newTSExtractorFromSpec(&tsLangSpec{name: "a", variantForExt: map[string]tsVariant{".a": v, ".b": v}, defaultVariant: v})
		}
		t.Fatal("no python dialect")
		return nil
	}
	src := []byte("def f():\n    pass\n\nf()\n")
	keys := func(e *TSExtractor, rel string) (pkgs map[string]bool, module string) {
		t.Helper()
		nodes, err := e.Extract("svc", rel, src)
		if err != nil {
			t.Fatal(err)
		}
		pkgs = map[string]bool{}
		for _, n := range nodes {
			pkgs[n.Package] = true
		}
		return pkgs, moduleOf(t, nodes).Key()
	}

	file := extractWith(fixture[0].Package.Strategy)
	defer file.Close()
	for rel, want := range map[string]struct{ pkg, key string }{
		"lib/X.a":     {"X.a", core.SymbolKey("svc", "lib/X.a", core.ModuleSymbolName)},
		"lib/X.b":     {"X.b", core.SymbolKey("svc", "lib/X.b", core.ModuleSymbolName)},
		"X.a":         {"X.a", core.SymbolKey("svc", "X.a", core.ModuleSymbolName)},
		"lib/+page.a": {"+page.a", core.SymbolKey("svc", "lib/+page.a", core.ModuleSymbolName)},
		"lib/x.y.b":   {"x.y.b", core.SymbolKey("svc", "lib/x.y.b", core.ModuleSymbolName)},
	} {
		pkgs, key := keys(file, rel)
		if len(pkgs) != 1 || !pkgs[want.pkg] {
			t.Errorf("%s: packages %v, want every definition's %q (the base name with its extension)", rel, pkgs, want.pkg)
		}
		if key != want.key {
			t.Errorf("%s: module key %q, want %q", rel, key, want.key)
		}
	}
	_, ka := keys(file, "lib/X.a")
	_, kb := keys(file, "lib/X.b")
	if ka == kb {
		t.Errorf("strategy file: X.a and X.b share module key %q", ka)
	}

	dir := extractWith("dir")
	defer dir.Close()
	pkgs, da := keys(dir, "lib/X.a")
	_, db := keys(dir, "lib/X.b")
	if !pkgs["X"] || da != db {
		t.Errorf("strategy dir: packages %v, keys %q and %q; want the stem X and one shared key (the collision F1 lets a dialect leave)", pkgs, da, db)
	}
}
