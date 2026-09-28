package extractor

// pyprojectdialect_test.go — goldens for the python-packaging dialect
// (LB-Q-python-packaging, design §13.2 step 10/12): pyproject.toml's
// [project] table (PEP 621) plus a [tool.poetry] fallback, scripts/
// gui-scripts/entry-points as route entities, [tool.*] sections as
// generic inventory, and PEP 735 dependency-groups (with an
// include-group intra-file reference — the one place this format has
// an entity-to-entity edge the doc engine's selectors can express, the
// gap LB-Q-package-json-rest documented for that dialect). Same golden
// shape as packagejsondialect_test.go, plus a "calls" field for that
// reference.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type pyProjectGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Tier      string   `json:"tier"`
	Calls     []string `json:"calls,omitempty"`
}

type pyProjectGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type pyProjectGolden struct {
	Symbols   []pyProjectGoldenSymbol `json:"symbols"`
	CrossFile []pyProjectGoldenCross  `json:"cross_file"`
}

func loadPythonPackagingExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "python-packaging" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(python-packaging): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"python-packaging\" not found in LoadDialects()")
	return nil
}

func pythonPackagingFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "python-packaging", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".toml" {
			continue
		}
		if len(e.Name()) > len(".golden.json") && e.Name()[len(e.Name())-len(".golden.json"):] == ".golden.json" {
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

// TestPythonPackagingDialectGoldens (LB-Q-python-packaging, design §13.2
// steps 10/12): every testdata/*.toml fixture's entities, intra-file
// references (Calls) and cross-file references, against a hand-written
// golden. relPath is simulated as "pyproject.toml" (the fixtures live at
// other basenames on disk so they are not themselves picked up by
// classification/embedding, §2.1).
func TestPythonPackagingDialectGoldens(t *testing.T) {
	x := loadPythonPackagingExtractor(t)
	for _, fx := range pythonPackagingFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", "pyproject.toml", src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got pyProjectGolden
			got.Symbols = []pyProjectGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != "pyproject.toml" {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, pyProjectGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature, Tier: tierName(n.Tier),
					Calls: n.Calls,
				})
			}
			got.CrossFile = []pyProjectGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, pyProjectGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want pyProjectGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []pyProjectGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []pyProjectGoldenCross{}
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
