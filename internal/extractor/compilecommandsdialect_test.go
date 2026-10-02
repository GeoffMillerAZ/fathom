package extractor

// compilecommandsdialect_test.go — goldens and obligations for the compile-commands doc-engine dialect
// (LB-Q-compile-commands, no incumbent, design §13.2 steps 10/12/14): a translation unit per database entry,
// its include directories, macros and output as the unit's references. The golden shape is the other doc
// dialects' (symbols: name/kind/package/line/end_line/signature/calls/tier); every fixture is read as if it
// sat at the repository root as compile_commands.json (the doc spec detects that basename).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
)

type compdbGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type compdbGolden struct {
	Symbols []compdbGoldenSymbol `json:"symbols"`
}

func compdbExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	x, err := NewDocDialectExtractor("compile-commands")
	if err != nil {
		t.Fatalf("NewDocDialectExtractor(compile-commands): %v", err)
	}
	return x
}

func compdbFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "compile-commands", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") || filepath.Ext(e.Name()) != ".json" {
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

// TestCompileCommandsDialectGoldens: every testdata fixture's units and flags against a hand-written golden.
func TestCompileCommandsDialectGoldens(t *testing.T) {
	x := compdbExtractor(t)
	for _, fx := range compdbFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", "compile_commands.json", src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			got := compdbGolden{Symbols: []compdbGoldenSymbol{}}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != "compile_commands.json" {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, compdbGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package, Line: n.Line, EndLine: n.EndLine,
					Signature: n.Signature, Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			if len(res.CrossFile) != 0 {
				t.Errorf("%s: cross_file %+v; the dialect declares none (L2)", fx, res.CrossFile)
			}
			raw, err := os.ReadFile(fx + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want compdbGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			if want.Symbols == nil {
				want.Symbols = []compdbGoldenSymbol{}
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("%s: symbols\n got: %+v\nwant: %+v", fx, got.Symbols, want.Symbols)
			}
		})
	}
}

// TestCompileCommandsFlagGrammarIsIncludeRoots: the include flags are read as fusion's includeRoots reads
// them (resolvestrategy.go): -I joined or separate, -iquote/-isystem/-idirafter separate only, a surrounding
// quote pair trimmed from each token, a trailing flag naming nothing. This test is the contract between the
// two readers; a change to includeRoots' grammar changes this table in the same commit.
func TestCompileCommandsFlagGrammarIsIncludeRoots(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		inc  []string
	}{
		{"joined -I", []string{"-Idir"}, []string{"-Idir"}},
		{"separate -I", []string{"-I", "dir"}, []string{"-Idir"}},
		{"separate -iquote -isystem -idirafter", []string{"-iquote", "a", "-isystem", "b", "-idirafter", "c"}, []string{"-iquotea", "-isystemb", "-idirafterc"}},
		{"joined -isystem is not read", []string{"-isystem/usr/include", "-iquote/q"}, nil},
		{"quotes trimmed", []string{`"-Idir"`, `'-I'`, `'other dir'`}, []string{"-Idir", "-Iother dir"}},
		{"trailing -I", []string{"-c", "x.c", "-I"}, nil},
		{"-I with an empty next argument", []string{"-I", `""`}, nil},
		{"repeats once", []string{"-Ia", "-Ia", "-I", "a"}, []string{"-Ia"}},
	} {
		inc, _, _ := compdbFlags(c.args)
		if !slices.Equal(inc, c.inc) {
			t.Errorf("%s: includes %v, want %v", c.name, inc, c.inc)
		}
	}
	_, defs, out := compdbFlags([]string{"-DA", "-D", "B=1", "-DC=\"x\"", "-o", "o.o", "-c", "f.c"})
	if want := []string{"-DA", "-DB=1", `-DC="x"`}; !slices.Equal(defs, want) {
		t.Errorf("defines %v, want %v", defs, want)
	}
	if out != "o.o" {
		t.Errorf("output %q, want o.o", out)
	}
}

// TestCompileCommandsClaimsItsFile: the queue entry names compile_commands.json.
func TestCompileCommandsClaimsItsFile(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.ID == "compile-commands" {
			if !slices.Equal(d.Match.Basenames, []string{"compile_commands.json"}) {
				t.Errorf("basenames = %v, want [compile_commands.json]", d.Match.Basenames)
			}
			return
		}
	}
	t.Fatal("compile-commands dialect not registered")
}
