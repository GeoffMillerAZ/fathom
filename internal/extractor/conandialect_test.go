package extractor

// conandialect_test.go — goldens for the conan tags+doc dialect (LB-Q-conan,
// design §13.2 steps 10/12/14). conanfile.py rides the tags engine on the python
// grammar; conanfile.txt (an INI decode), conan.lock (JSON, read as YAML) and
// conandata.yml are documents for the doc engine. Each fixture directory
// testdata/<case>/ holds one file named as a real one is and its golden; the
// fixtures are written for the dialect, not copied from a project.

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type conanGolden struct {
	Engine    string            `json:"engine"`
	Symbols   []goldenSymbol    `json:"symbols"`
	CrossFile []helmGoldenCross `json:"cross_file"`
}

func conanFixtures(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("dialects", "conan", "testdata")
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !strings.HasSuffix(p, ".golden.json") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", root)
	}
	return out
}

// conanRel is the repository path a fixture stands for: its path under testdata/.
func conanRel(fx string) string {
	rel, _ := filepath.Rel(filepath.Join("dialects", "conan", "testdata"), fx)
	return filepath.ToSlash(rel)
}

func conanExtract(t *testing.T, x *TagsDocExtractor, rel string, src []byte) conanGolden {
	t.Helper()
	g := conanGolden{Engine: "tags", Symbols: []goldenSymbol{}, CrossFile: []helmGoldenCross{}}
	if x.IsDocFile(rel) {
		g.Engine = "doc"
		res, err := x.ExtractDoc("svc", rel, src)
		if err != nil {
			t.Fatalf("ExtractDoc %s: %v", rel, err)
		}
		for _, c := range res.CrossFile {
			g.CrossFile = append(g.CrossFile, helmGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line})
		}
	}
	nodes, err := x.Extract("svc", rel, src)
	if err != nil {
		t.Fatalf("Extract %s: %v", rel, err)
	}
	for _, n := range nodes {
		if n.Service != "svc" || n.FilePath != rel {
			t.Fatalf("%s: plumbing changed: service %q file %q", rel, n.Service, n.FilePath)
		}
		g.Symbols = append(g.Symbols, goldenSymbol{
			Name: n.Name, Kind: string(n.Kind), Package: n.Package, Line: n.Line,
			EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls, Tier: tierName(n.Tier),
		})
	}
	return g
}

func marshalConanGolden(g conanGolden) []byte {
	b, _ := json.MarshalIndent(g, "", "  ")
	return append(b, '\n')
}

// TestConanDialectGoldens holds the dialect to its hand-written goldens.
func TestConanDialectGoldens(t *testing.T) {
	x, err := NewTagsDocExtractor("conan")
	if err != nil {
		t.Fatalf("NewTagsDocExtractor(conan): %v", err)
	}
	defer x.Close()
	for _, fx := range conanFixtures(t) {
		rel := conanRel(fx)
		t.Run(rel, func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			got := conanExtract(t, x, rel, src)
			raw, err := os.ReadFile(fx + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want conanGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s golden: %v", fx, err)
			}
			if want.Symbols == nil {
				want.Symbols = []goldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []helmGoldenCross{}
			}
			if !reflect.DeepEqual(marshalConanGolden(got), marshalConanGolden(want)) {
				t.Errorf("%s: golden mismatch\n--- got\n%s\n--- want\n%s", fx, marshalConanGolden(got), marshalConanGolden(want))
			}
		})
	}
}

// TestConanRouting: the doc spec's detect names the documents; conanfile.py is
// the one file the tags engine reads.
func TestConanRouting(t *testing.T) {
	x, err := NewTagsDocExtractor("conan")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	for rel, doc := range map[string]bool{
		"conanfile.txt":                              true,
		"recipes/zlib/all/conandata.yml":             true,
		"conan.lock":                                 true,
		"app/conan.lock":                             true,
		"recipes/zlib/all/conanfile.py":              false,
		"recipes/zlib/all/test_package/conanfile.py": false,
	} {
		if got := x.IsDocFile(rel); got != doc {
			t.Errorf("IsDocFile(%q) = %v, want %v", rel, got, doc)
		}
	}
}

// TestConanSurvivesSyntaxError: a recipe whose later class has an unterminated
// def never fails the file (design §13.2 step 10's required fixture), and
// the recipe before the error keeps what it requires.
func TestConanSurvivesSyntaxError(t *testing.T) {
	x, err := NewTagsDocExtractor("conan")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	src, err := os.ReadFile("dialects/conan/testdata/broken/conanfile.py")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := x.Extract("svc", "broken/conanfile.py", src)
	if err != nil {
		t.Fatalf("a recoverable syntax error must not error the file: %v", err)
	}
	var early []string
	for _, n := range nodes {
		if n.Name == "early" {
			early = n.Calls
		}
	}
	if !reflect.DeepEqual(early, []string{"zlib"}) {
		t.Fatalf("the recipe before the error must survive with its requires; nodes %+v", nodes)
	}
}
