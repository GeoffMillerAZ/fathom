package extractor

import (
	"os"
	"reflect"
	"testing"
)

// astro hosts its frontmatter, scripts and styles (F2's reviewed list,
// tsextract_host_test.go; appended here so each host's wave adds its own line).
func init() { injectionHostsWant = append(injectionHostsWant, "astro") }

// TestAstroHostsRangeImports (LB-F544, F2): an Astro component's imports are
// its frontmatter's and its JavaScript scripts', at host lines, as the file
// resolver reads them (ExtractFileSpecs) and as the harness pairs them
// (ExtractImportSpans); a JSON-LD or other non-JavaScript script is data and
// imports nothing.
func TestAstroHostsRangeImports(t *testing.T) {
	e, err := NewTSExtractor("astro")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	type imp struct {
		Module string
		Local  string
		Line   int
	}
	for fixture, want := range map[string][]imp{
		"component.astro": {
			{"./Card.astro", "Card", 2},
			{"../ui/index.ts", "UI", 3},
			{"../lib/format.js", "format", 4},
			{"~/components/Logo.astro", "Logo", 5},
			{"astro:content", "getCollection", 6},
		},
		"scripts.astro": {{"./client.ts", "start", 3}},
		"styles.astro":  {{"./go.js", "go", 10}},
		"broken.astro":  {{"./Shell.astro", "Shell", 2}},
	} {
		src, err := os.ReadFile("dialects/astro/testdata/" + fixture)
		if err != nil {
			t.Fatal(err)
		}
		rel := "src/pages/" + fixture
		specs, err := e.ExtractFileSpecs(rel, src)
		if err != nil {
			t.Fatalf("%s: ExtractFileSpecs: %v", fixture, err)
		}
		var got []imp
		for _, s := range specs.Imports {
			got = append(got, imp{s.Module, s.LocalName, s.Line})
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: file specs' imports = %+v, want %+v", fixture, got, want)
		}
		spans, err := e.ExtractImportSpans(rel, src)
		if err != nil {
			t.Fatalf("%s: ExtractImportSpans: %v", fixture, err)
		}
		got = nil
		for _, s := range spans {
			got = append(got, imp{s.Module, s.LocalName, s.Line})
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: import spans = %+v, want %+v", fixture, got, want)
		}
	}
}

// TestFileResolveLangsIncludesInjectionHost (LB-F545): an injection host is a
// file-resolve language by its import strategy alone, since its imports are its
// ranges' (F2): astro names no imports query of its own.
func TestFileResolveLangsIncludesInjectionHost(t *testing.T) {
	langs, err := FileResolveLangs()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range langs {
		if l.Lang == "astro" {
			if l.Strategy != "relative-path" || !reflect.DeepEqual(l.Extensions, []string{".astro"}) {
				t.Errorf("astro file-resolve language = %+v, want relative-path over .astro", l)
			}
			return
		}
	}
	t.Errorf("astro is not a file-resolve language: %+v", langs)
}
