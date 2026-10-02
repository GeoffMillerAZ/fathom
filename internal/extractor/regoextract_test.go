package extractor

// regoextract_test.go — the Rego native dialect (LB-Q-rego, LB-F131): each
// testdata fixture against its golden, written by hand from reading the
// fixture (OPA 1.18.2's own parse of the three valid fixtures agrees on
// every definition, line, end line and call), plus the reader's contracts.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type regoGoldenImport struct {
	Local  string `json:"local"`
	Module string `json:"module"`
	Line   int    `json:"line"`
}

type regoGolden struct {
	Path    string             `json:"path"`
	Symbols []confGoldenSymbol `json:"symbols"`
	Imports []regoGoldenImport `json:"imports"`
	Refs    []RefSpec          `json:"refs"`
	Exports []ExportSpec       `json:"exports"`
}

// TestRegoDialectGoldens: every fixture against its golden, extracted as the
// golden's repository path, through the extractor ProxyDialectExtractor
// registers for the dialect (what fusion dispatches .rego to).
func TestRegoDialectGoldens(t *testing.T) {
	x, ok := ProxyDialectExtractor("rego")
	if !ok {
		t.Fatal(`ProxyDialectExtractor("rego"): not registered`)
	}
	fx, ok := x.(FileSpecExtractor)
	if !ok {
		t.Fatal("the rego extractor is not a FileSpecExtractor")
	}
	dir := filepath.Join("dialects", "rego", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
			continue
		}
		n++
		file := filepath.Join(dir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if lines := strings.Count(string(src), "\n"); lines > 200 {
				t.Fatalf("fixture has %d lines, the recipe's cap is 200", lines)
			}
			raw, err := os.ReadFile(file + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want regoGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("golden: %v", err)
			}
			nodes, err := x.Extract("svc", want.Path, src)
			if err != nil {
				t.Fatalf("Extract: %v", err)
			}
			specs, err := fx.ExtractFileSpecs(want.Path, src)
			if err != nil {
				t.Fatalf("ExtractFileSpecs: %v", err)
			}
			got := regoGolden{Path: want.Path, Symbols: []confGoldenSymbol{}, Imports: []regoGoldenImport{}, Refs: []RefSpec{}, Exports: []ExportSpec{}}
			pkg := ""
			for _, nd := range nodes {
				if nd.Kind == kindRegoPackage {
					pkg = nd.Name
				}
			}
			for _, nd := range nodes {
				if nd.Service != "svc" || nd.FilePath != want.Path || nd.Package != pkg || nd.Tier != core.TierPreScan {
					t.Fatalf("plumbing changed: %+v", nd)
				}
				got.Symbols = append(got.Symbols, confGoldenSymbol{Name: nd.Name, Kind: string(nd.Kind), Line: nd.Line, EndLine: nd.EndLine, Signature: nd.Signature, Calls: nd.Calls})
			}
			for _, s := range specs.Imports {
				if s.Kind != ImportWhole {
					t.Fatalf("import %+v: a Rego import binds its package whole", s)
				}
				got.Imports = append(got.Imports, regoGoldenImport{Local: s.LocalName, Module: s.Module, Line: s.Line})
			}
			got.Refs = append(got.Refs, specs.Refs...)
			got.Exports = append(got.Exports, specs.Exports...)
			if !reflect.DeepEqual(got, want) {
				gb, _ := json.MarshalIndent(got, "", "  ")
				t.Fatalf("mismatch; got:\n%s", gb)
			}
			spans, err := fx.ExtractImportSpans(want.Path, src)
			if err != nil || len(spans) != len(specs.Imports) {
				t.Fatalf("ExtractImportSpans: %d spans, %v; %d imports", len(spans), err, len(specs.Imports))
			}
			for i, sp := range spans {
				if sp.ImportSpec != specs.Imports[i] || sp.StartLine != sp.Line || sp.EndLine != sp.Line {
					t.Fatalf("span %d: %+v", i, sp)
				}
			}
		})
	}
	if n < 3 {
		t.Errorf("%d fixtures, want at least 3", n)
	}
}

// TestRegoRepairs: a file read as written needs no repair; an unclosed body,
// an unterminated string and a stray closer each count, and what follows an
// unclosed body is still read.
func TestRegoRepairs(t *testing.T) {
	for _, c := range []struct {
		src  string
		want int
	}{
		{"package p\n\nallow if {\n\tinput.x\n}\n", 0},
		{"package p\n\nallow if {\n\tinput.x\n\nafter := 1\n", 1},
		{"package p\n\nmsg := \"open\n", 1},
		{"package p\n\nx := 1 }\n", 1},
	} {
		if got := RegoRepairs([]byte(c.src)); got != c.want {
			t.Errorf("RegoRepairs(%q) = %d, want %d", c.src, got, c.want)
		}
	}
	nodes, _ := NewRegoExtractor().Extract("s", "p.rego", []byte("package p\n\nallow if {\n\tinput.x\n\nafter := 1\n"))
	if len(nodes) != 3 || nodes[2].Name != "after" || nodes[2].Line != 6 {
		t.Fatalf("the rule after an unclosed body: %+v", nodes)
	}
}

// TestRegoDeclarations: the package path and the distinct rule names, what
// the rego-package strategy indexes.
func TestRegoDeclarations(t *testing.T) {
	src := "package data.a.b\n\nimport rego.v1\n\nf(x) := x\nf(x) := 1 if x == 0\np.q.r := 1\nallow if true\n"
	pkg, rules := RegoDeclarations([]byte(src))
	if pkg != "a.b" || !reflect.DeepEqual(rules, []string{"allow", "f", "p.q.r"}) {
		t.Fatalf("RegoDeclarations = %q %q", pkg, rules)
	}
}

// TestRegoIsNative: NativeDialectExtractor answers for rego (the harness
// measures it through it) and NewFileSpecExtractor gives the native reader,
// not a tags extractor.
func TestRegoIsNative(t *testing.T) {
	if _, ok := NativeDialectExtractor("rego"); !ok {
		t.Fatal(`NativeDialectExtractor("rego"): not registered`)
	}
	e, err := NewFileSpecExtractor("rego")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := e.(*RegoExtractor); !ok || e.Name() != "rego" {
		t.Fatalf("NewFileSpecExtractor(rego) = %T %q", e, e.Name())
	}
	langs, err := FileResolveLangs()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, l := range langs {
		if l.Lang == "rego" {
			found = l.Strategy == "rego-package" && reflect.DeepEqual(l.Extensions, []string{".rego"})
		}
	}
	if !found {
		t.Fatalf("FileResolveLangs has no rego entry with the rego-package strategy: %+v", langs)
	}
}
