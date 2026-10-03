package extractor

// cuedoc_test.go — goldens and contracts for the CUE dialect (LB-Q-cue): the
// entities, roots and cross-file references extractCUE gives each fixture of
// dialects/cue/testdata (a small module: a vendored package under cue.mod/pkg,
// a package with an import, files referring to each other) and a malformed
// file. The goldens were written from the extractor and checked against the
// cue binary's own answer for the module (module.oracle.json, `cue lsp` through
// calibration/oracles/cue; TestCUEDialectAgreesWithTheCueBinary holds that).

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type cueGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
}

type cueGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target,omitempty"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type cueGoldenFile struct {
	Roots     []string          `json:"roots"`
	Symbols   []cueGoldenSymbol `json:"symbols"`
	CrossFile []cueGoldenCross  `json:"cross_file"`
}

// cueFixtures maps each fixture's repo-relative path (as extracted) to its
// file on disk: the module's files relative to the module, broken.cue as is.
func cueFixtures(t *testing.T) map[string]string {
	t.Helper()
	dir := filepath.Join("dialects", "cue", "testdata")
	out := map[string]string{"broken.cue": filepath.Join(dir, "broken.cue")}
	mod := filepath.Join(dir, "module")
	err := filepath.WalkDir(mod, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".cue") {
			return err
		}
		rel, _ := filepath.Rel(mod, p)
		out[filepath.ToSlash(rel)] = p
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 6 {
		t.Fatalf("%d CUE fixtures, want at least 6", len(out))
	}
	return out
}

func cueGolden(rel string, res DocResult) cueGoldenFile {
	g := cueGoldenFile{Roots: res.Roots, Symbols: []cueGoldenSymbol{}, CrossFile: []cueGoldenCross{}}
	if g.Roots == nil {
		g.Roots = []string{}
	}
	for _, n := range res.Nodes {
		g.Symbols = append(g.Symbols, cueGoldenSymbol{Name: n.Name, Kind: string(n.Kind), Package: n.Package, Line: n.Line, EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls})
	}
	for _, c := range res.CrossFile {
		g.CrossFile = append(g.CrossFile, cueGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line})
	}
	return g
}

// TestCUEDialectGoldens: every fixture's entities (name, kind, package, range,
// signature, references), roots and cross-file references against the golden.
func TestCUEDialectGoldens(t *testing.T) {
	fx := cueFixtures(t)
	got := map[string]cueGoldenFile{}
	for rel, p := range fx {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		got[rel] = cueGolden(rel, extractCUE("svc", rel, src))
	}
	gp := filepath.Join("dialects", "cue", "testdata", "goldens.json")
	gb, err := json.MarshalIndent(got, "", " ")
	if err != nil {
		t.Fatal(err)
	}
	gb = append(gb, '\n')
	if *updateGoldens {
		if err := os.WriteFile(gp, gb, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(gp)
	if err != nil {
		t.Fatalf("read golden: %v (run with -update to create)", err)
	}
	if !bytes.Equal(gb, want) {
		var w map[string]cueGoldenFile
		_ = json.Unmarshal(want, &w)
		var paths []string
		for p := range got {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			a, _ := json.MarshalIndent(got[p], "", " ")
			b, _ := json.MarshalIndent(w[p], "", " ")
			if !bytes.Equal(a, b) {
				t.Errorf("%s: golden mismatch\n--- got\n%s\n--- want\n%s", p, a, b)
			}
		}
	}
}

// TestCUEDialectIsRegistered: the native engine's registry answers for the
// dialect with its DocResulter (indexbench lang-oracle measures it through
// this, and the doc-xref pass reads its cross-file references).
func TestCUEDialectIsRegistered(t *testing.T) {
	e, ok := NativeDialectExtractor("cue")
	if !ok {
		t.Fatal("NativeDialectExtractor(cue): not registered")
	}
	if _, isDoc := e.(DocResulter); !isDoc {
		t.Error("the CUE extractor must give the cross-file pass its DocResult")
	}
}

// TestCUEMalformedFileKeepsWhatParses: a mid-edit save (a malformed import, an
// unclosed struct) yields the entities the parser recovered, no empty-named
// import, and no panic.
func TestCUEMalformedFileKeepsWhatParses(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("dialects", "cue", "testdata", "broken.cue"))
	if err != nil {
		t.Fatal(err)
	}
	res := extractCUE("svc", "broken.cue", src)
	names := map[string]bool{}
	for _, n := range res.Nodes {
		if n.Name == "" {
			t.Errorf("an empty-named entity: %+v", n)
		}
		names[n.Name] = true
	}
	if !names["good"] {
		t.Errorf("the well-formed field was not read: %v", names)
	}
}

// TestCUEQualifiedImportNamesItsPackage: an import `path:pkg` binds the
// package's name, pkg, and a selector through it is a reference below that
// package (the cue-import strategy reads the qualifier, cuexref.go).
func TestCUEQualifiedImportNamesItsPackage(t *testing.T) {
	res := extractCUE("svc", "app/app.cue", []byte("package app\n\nimport \"example.com/m/deploy:k8s\"\n\nx: k8s.#D.spec\n"))
	var calls []string
	for _, n := range res.Nodes {
		if n.Name == "x" {
			calls = n.Calls
		}
	}
	if want := []string{"k8s.#D", "k8s.#D.spec"}; strings.Join(calls, ",") != strings.Join(want, ",") {
		t.Errorf("x.Calls = %v, want %v", calls, want)
	}
	var refs []string
	for _, c := range res.CrossFile {
		refs = append(refs, c.Strategy+" "+c.Raw+" "+c.Target)
	}
	want := []string{"cue-import example.com/m/deploy:k8s ", "cue-import #D example.com/m/deploy:k8s", "cue-import #D.spec example.com/m/deploy:k8s"}
	if strings.Join(refs, "|") != strings.Join(want, "|") {
		t.Errorf("cross-file references %q, want %q", refs, want)
	}
}
