package extractor

// composerdialect_test.go — goldens for the composer dialect (LB-Q-composer),
// in the package-json dialect's shape: every testdata/*.json fixture's
// entities and cross-file references against a golden, relPath simulated as
// "composer.json" (the fixtures live at other basenames on disk). Set
// COMPOSER_GOLDEN_UPDATE=1 to rewrite the goldens, then read them.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func loadComposerExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	x, err := NewDocDialectExtractor("composer")
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(composer): %v", err)
	}
	return x
}

func composerFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "composer", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		out = append(out, filepath.Join(dir, e.Name()))
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

func TestComposerDialectGoldens(t *testing.T) {
	x := loadComposerExtractor(t)
	update := os.Getenv("COMPOSER_GOLDEN_UPDATE") != ""
	for _, fx := range composerFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", "composer.json", src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got pkgJSONGolden
			got.Symbols = []pkgJSONGoldenSymbol{}
			for _, n := range res.Nodes {
				got.Symbols = append(got.Symbols, pkgJSONGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []pkgJSONGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, pkgJSONGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			if update {
				b, _ := json.MarshalIndent(got, "", "  ")
				if err := os.WriteFile(gp, append(b, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want pkgJSONGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []pkgJSONGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []pkgJSONGoldenCross{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("%s: cross_file\n got: %+v\nwant: %+v", fx, got.CrossFile, want.CrossFile)
			}
		})
	}
}

// TestComposerScriptRefs: an `@name` command is a reference to the script of
// that name, and only to one the document declares; @php, @composer and @putenv
// commands, a name nothing declares, and a command that merely mentions a
// script are none. Composer runs a referenced script's own listeners.
func TestComposerScriptRefs(t *testing.T) {
	x := loadComposerExtractor(t)
	src := `{"name":"a/b","scripts":{
 "ci": ["@lint", "@test --verbose", "@missing", "@php -v", "@putenv X=1", "@composer install", "run @lint later"],
 "lint": "phpcs",
 "test": ["@php artisan test"]
}}`
	res, err := x.ExtractDoc("svc", "composer.json", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, n := range res.Nodes {
		if string(n.Kind) == "ComposerScript" {
			got[n.Name] = n.Calls
		}
	}
	want := map[string][]string{"ci": {"lint", "test"}, "lint": nil, "test": nil}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("script refs %v, want %v", got, want)
	}
}
