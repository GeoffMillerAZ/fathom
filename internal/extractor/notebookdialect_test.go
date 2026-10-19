package extractor

// notebookdialect_test.go -- goldens for the jupyter dialect (LB-Q-jupyter, design
// 13.2 steps 10/12/14): every testdata/*.ipynb fixture's symbols against a
// hand-written golden, then the rules the goldens do not isolate.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type notebookGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type notebookGolden struct {
	Symbols []notebookGoldenSymbol `json:"symbols"`
}

func loadNotebookExtractor(t *testing.T) *NotebookExtractor {
	t.Helper()
	x, err := NewNotebookDialectExtractor()
	if err != nil {
		t.Fatalf("NewNotebookDialectExtractor: %v", err)
	}
	return x
}

func notebookSymbols(t *testing.T, x *NotebookExtractor, rel string, src []byte) []notebookGoldenSymbol {
	t.Helper()
	nodes, err := x.Extract("svc", rel, src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	out := []notebookGoldenSymbol{}
	for _, n := range nodes {
		if n.Service != "svc" || n.FilePath != rel {
			t.Fatalf("plumbing changed: service %q file %q", n.Service, n.FilePath)
		}
		out = append(out, notebookGoldenSymbol{
			Name: n.Name, Kind: string(n.Kind), Package: n.Package, Line: n.Line, EndLine: n.EndLine,
			Signature: n.Signature, Calls: n.Calls, Tier: tierName(n.Tier),
		})
	}
	return out
}

// TestNotebookDialectGoldens: each fixture is read as sitting at its own basename.
func TestNotebookDialectGoldens(t *testing.T) {
	x := loadNotebookExtractor(t)
	dir := filepath.Join("dialects", "jupyter", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []string
	for _, e := range ents {
		if filepath.Ext(e.Name()) == ".ipynb" {
			fixtures = append(fixtures, e.Name())
		}
	}
	sort.Strings(fixtures)
	if len(fixtures) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			got := notebookSymbols(t, x, name, src)
			raw, err := os.ReadFile(filepath.Join(dir, name+".golden.json"))
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want notebookGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatal(err)
			}
			if want.Symbols == nil {
				want.Symbols = []notebookGoldenSymbol{}
			}
			if !reflect.DeepEqual(got, want.Symbols) {
				t.Errorf("symbols\n got: %+v\nwant: %+v", got, want.Symbols)
			}
		})
	}
}

// TestNotebookLinesAreTheNotebookFilesLines: a definition's line is the line of the
// notebook file that holds the JSON string its source line begins in, whether the
// cell's source is a list (one line per element) or one string, and an element that
// holds several source lines gives all of them its own line.
func TestNotebookLinesAreTheNotebookFilesLines(t *testing.T) {
	src := "{\n \"cells\": [\n  {\n   \"cell_type\": \"code\",\n   \"source\": [\n" +
		"    \"x = 1\\n\",\n" + // 6
		"    \"def f():\\n\",\n" + // 7
		"    \"    return 1\\n\\n\\ndef g():\\n    return f()\"\n" + // 8: three source lines in one element
		"   ]\n  }\n ]\n}\n"
	x := loadNotebookExtractor(t)
	got := map[string][2]int{}
	for _, s := range notebookSymbols(t, x, "n.ipynb", []byte(src)) {
		got[s.Name] = [2]int{s.Line, s.EndLine}
	}
	want := map[string][2]int{"<module>": {6, 8}, "x": {6, 6}, "f": {7, 8}, "g": {8, 8}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("lines\n got: %v\nwant: %v", got, want)
	}
}

// TestNotebookReadsOnlyCodeCells: markdown and raw cells and a cell's outputs and
// metadata are never read, and a notebook in another language reads as empty.
func TestNotebookReadsOnlyCodeCells(t *testing.T) {
	x := loadNotebookExtractor(t)
	mk := func(lang, cells string) []byte {
		return []byte(`{"cells": [` + cells + `], "metadata": {` + lang + `}, "nbformat": 4}`)
	}
	code := `{"cell_type": "code", "source": "def f(): pass", "outputs": [{"text": ["def g(): pass"]}], "metadata": {"tags": ["def h(): pass"]}}`
	md := `{"cell_type": "markdown", "source": "def m(): pass"}, {"cell_type": "raw", "source": "def r(): pass"}`
	names := func(src []byte) string {
		var out []string
		for _, s := range notebookSymbols(t, x, "n.ipynb", src) {
			if s.Kind != "JupyterModule" {
				out = append(out, s.Name)
			}
		}
		return strings.Join(out, ",")
	}
	for _, c := range []struct {
		name string
		src  []byte
		want string
	}{
		{"code only", mk(``, md+`,`+code), "f"},
		{"python by language_info", mk(`"language_info": {"name": "Python"}`, code), "f"},
		{"python by kernelspec", mk(`"kernelspec": {"language": "python"}`, code), "f"},
		{"language_info wins over kernelspec", mk(`"kernelspec": {"language": "python"}, "language_info": {"name": "julia"}`, code), ""},
		{"julia", mk(`"language_info": {"name": "julia"}`, code), ""},
		{"not json", []byte("def f(): pass"), ""},
		{"not an object", []byte(`[1, 2]`), ""},
		{"a v3 notebook has no cells at the top", []byte(`{"worksheets": [{"cells": [{"cell_type": "code", "input": ["def f(): pass"]}]}], "nbformat": 3}`), ""},
	} {
		if got := names(c.src); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
