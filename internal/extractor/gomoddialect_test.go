package extractor

// gomoddialect_test.go — goldens and contracts for the go-mod native
// dialect (LB-Q-go-mod, design §13.2 steps 10/12/15). The golden shape is the
// cargo dialect's: symbols (name/kind/package/line/end_line/signature/calls/
// tier) and the cross-file references the extractor records whether or not
// fusion.ResolveDocXrefs can resolve them, plus the file's root entities and
// the repo-relative path the fixture is extracted as (a target resolves
// against the referring file's directory, so the golden names it).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type goModGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type goModGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type goModGolden struct {
	Path      string              `json:"path"`
	Roots     []string            `json:"roots"`
	Symbols   []goModGoldenSymbol `json:"symbols"`
	CrossFile []goModGoldenCross  `json:"cross_file"`
}

func goModFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "go-mod", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		if ext := filepath.Ext(e.Name()); ext == ".mod" || ext == ".work" {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(out)
	if len(out) < 3 {
		t.Fatalf("%s: %d fixtures, want at least 3", dir, len(out))
	}
	return out
}

// TestGoModDialectGoldens: every testdata fixture's entities, roots and
// cross-file references against a hand-written golden.
func TestGoModDialectGoldens(t *testing.T) {
	x := NewGoModulesExtractor()
	for _, fx := range goModFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(fx + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want goModGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s.golden.json: %v", fx, err)
			}
			if want.Path == "" {
				t.Fatalf("%s.golden.json: no path", fx)
			}
			res, err := x.ExtractDoc("svc", want.Path, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			got := goModGolden{Path: want.Path, Roots: res.Roots}
			got.Symbols = []goModGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != want.Path {
					t.Fatalf("plumbing changed: service %q file %q", n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, goModGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: string(n.Tier),
				})
			}
			got.CrossFile = []goModGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, goModGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			if want.Symbols == nil {
				want.Symbols = []goModGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []goModGoldenCross{}
			}
			if !reflect.DeepEqual(got.Roots, want.Roots) {
				t.Errorf("roots: got %q, want %q", got.Roots, want.Roots)
			}
			if !reflect.DeepEqual(got.Symbols, want.Symbols) {
				t.Errorf("symbols\n got: %+v\nwant: %+v", got.Symbols, want.Symbols)
			}
			if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
				t.Errorf("cross_file\n got: %+v\nwant: %+v", got.CrossFile, want.CrossFile)
			}
			// Extract is ExtractDoc's nodes.
			nodes, err := x.Extract("svc", want.Path, src)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			if !reflect.DeepEqual(nodes, res.Nodes) {
				t.Errorf("Extract and ExtractDoc disagree on the nodes")
			}
		})
	}
}

// goModIncumbentFixture is gomodextract_test.go's fixture: the incumbent's
// contract (a ModuleDep per require, the version as Signature, tier
// pre-scan, deterministic) holds for the dialect too (§13.2 step 15).
const goModIncumbentFixture = `module github.com/example/myapp

go 1.21

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/sync v0.5.0 // indirect
)

require github.com/stretchr/testify v1.8.4
`

// TestGoModDialectKeepsIncumbentContracts: every SourceKind string the
// incumbent emitted survives (ModuleDep, name = module path, signature =
// version, tier pre-scan); the dialect adds symbols beside them and loses
// none.
func TestGoModDialectKeepsIncumbentContracts(t *testing.T) {
	x := NewGoModulesExtractor()
	nodes, err := x.Extract("svc", "go.mod", []byte(goModIncumbentFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	old, err := NewGoModExtractor().Extract("svc", "go.mod", []byte(goModIncumbentFixture))
	if err != nil {
		t.Fatalf("incumbent Extract: %v", err)
	}
	byName := map[string]core.SymbolNode{}
	for _, n := range nodes {
		if n.Kind == core.KindModuleDep {
			byName[n.Name] = n
		}
	}
	if len(byName) != len(old) {
		t.Errorf("ModuleDep symbols: got %d, incumbent %d", len(byName), len(old))
	}
	for _, o := range old {
		n, ok := byName[o.Name]
		if !ok {
			t.Errorf("incumbent's %q is missing", o.Name)
			continue
		}
		if n.Signature != o.Signature || n.Tier != o.Tier || n.Line != o.Line || n.Package != o.Package || n.FilePath != o.FilePath || n.Service != o.Service {
			t.Errorf("%q: got %+v, incumbent %+v", o.Name, n, o)
		}
	}
	if len(nodes) != len(old)+1 {
		t.Errorf("want the incumbent's %d ModuleDep symbols plus the module's own, got %d", len(old), len(nodes))
	}
	again, err := x.Extract("svc", "go.mod", []byte(goModIncumbentFixture))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(nodes, again) {
		t.Error("two extractions of one file differ")
	}
}

// TestGoModDialectNamesKeepKeysDistinct: core.SymbolKey carries no kind, so a
// module a go.mod requires and replaces are two entities only because the
// replace is named replace/<path>; likewise a go.work's use entry.
func TestGoModDialectNamesKeepKeysDistinct(t *testing.T) {
	src := "module example.com/m\n\nrequire example.com/dep v1.0.0\n\nreplace example.com/dep => ./dep\n"
	nodes, err := NewGoModulesExtractor().Extract("svc", "go.mod", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, n := range nodes {
		if prev, dup := seen[n.Key()]; dup {
			t.Errorf("key %s is both %s and %s", n.Key(), prev, n.Kind)
		}
		seen[n.Key()] = string(n.Kind)
	}
	if len(seen) != 3 {
		t.Errorf("want module, require and replace, got %v", seen)
	}
}

// TestGoModDialectReferencesAreTheRequires: only what the module declares it
// requires is a reference — not a replace, an exclude or a retract, and not
// the module itself.
func TestGoModDialectReferencesAreTheRequires(t *testing.T) {
	src := `module example.com/m

require (
	example.com/a v1.0.0
	example.com/b v1.0.0 // indirect
)

replace example.com/c => ./c
replace example.com/b => example.com/fork v1.2.0
exclude example.com/d v1.0.0
retract v0.9.0
`
	nodes, err := NewGoModulesExtractor().Extract("svc", "go.mod", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.Kind == core.KindGoModule {
			want := []string{"example.com/a", "example.com/b"}
			if !reflect.DeepEqual(n.Calls, want) {
				t.Errorf("module calls: got %q, want %q", n.Calls, want)
			}
			continue
		}
		if len(n.Calls) != 0 {
			t.Errorf("%s %s carries calls %q; only the module does", n.Kind, n.Name, n.Calls)
		}
	}
}

// TestGoModDialectSurvivesGarbage: no input panics or errors, and what a
// file's readable lines say is still read.
func TestGoModDialectSurvivesGarbage(t *testing.T) {
	x := NewGoModulesExtractor()
	for name, src := range map[string]string{
		"empty":          "",
		"comment only":   "// nothing here\n",
		"binary":         "\x00\x01\x02\xff\xfe module \x00",
		"unclosed block": "module example.com/m\n\nrequire (\n\texample.com/a v1.0.0\n",
		"stray paren":    "module example.com/m\n)\nrequire example.com/a v1.0.0\n",
		"no version":     "module example.com/m\nrequire example.com/a\n",
		"conflict":       "module example.com/m\n<<<<<<< HEAD\nrequire example.com/a v1.0.0\n=======\nrequire example.com/a v1.1.0\n>>>>>>> branch\n",
		"major mismatch": "module example.com/m\nrequire example.com/a/v3 v1.0.0\nrequire example.com/b v1.0.0\n",
	} {
		t.Run(name, func(t *testing.T) {
			for _, base := range []string{"go.mod", "go.work"} {
				nodes, err := x.Extract("svc", base, []byte(src))
				if err != nil {
					t.Fatalf("%s: Extract returned %v", base, err)
				}
				again, _ := x.Extract("svc", base, []byte(src))
				if !reflect.DeepEqual(nodes, again) {
					t.Errorf("%s: two extractions differ", base)
				}
			}
		})
	}
	nodes, _ := x.Extract("svc", "go.mod", []byte("module example.com/m\nrequire example.com/a/v3 v1.0.0\nrequire example.com/b v1.0.0\n"))
	var got []string
	for _, n := range nodes {
		got = append(got, n.Name)
	}
	if want := []string{"example.com/m", "example.com/b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("a line modfile rejects is dropped and the rest read: got %q, want %q", got, want)
	}
}

// TestGoModDialectClosesATruncatedBlock: a file cut off inside a block (a
// partial fetch, a merge left half-edited) still gives the lines it has.
func TestGoModDialectClosesATruncatedBlock(t *testing.T) {
	nodes, err := NewGoModulesExtractor().Extract("svc", "go.mod", []byte("module example.com/m\n\nrequire (\n\texample.com/a v1.0.0\n\texample.com/b v1.1.0\n"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range nodes {
		got = append(got, n.Name)
	}
	if want := []string{"example.com/m", "example.com/a", "example.com/b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestGoModDialectIsRegistered: the native engine's registry answers for the
// dialect (indexbench lang-oracle and lang-parity measure it through this).
func TestGoModDialectIsRegistered(t *testing.T) {
	e, ok := NativeDialectExtractor("go-mod")
	if !ok {
		t.Fatal("NativeDialectExtractor(go-mod): not registered")
	}
	if _, isDoc := e.(DocResulter); !isDoc {
		t.Error("the go-mod extractor must give the cross-file pass its DocResult")
	}
	if _, ok := NativeDialectExtractor("no-such-dialect"); ok {
		t.Error("an unknown id must not resolve")
	}
}
