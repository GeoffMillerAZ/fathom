package extractor

// packagejsondialect_test.go — goldens for the package-json dialect
// (LB-Q-package-json, design §13.2 step 10/12). No generic
// "TestDialectGoldens" runner exists yet for doc-engine dialects (only
// TestTagsDialectParity, and only for the five pre-manifest tags
// languages) so this dialect gets its own, in the same golden shape
// (name/kind/package/line/end_line/signature/tier) plus a "cross_file"
// section doc dialects have and tags dialects don't (there is no
// "imports" concept here — cross-file references resolve by strategy,
// design §4).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type pkgJSONGoldenSymbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Package   string `json:"package"`
	Line      int    `json:"line"`
	EndLine   int    `json:"end_line"`
	Signature string `json:"signature,omitempty"`
	Tier      string `json:"tier"`
}

type pkgJSONGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type pkgJSONGolden struct {
	Symbols   []pkgJSONGoldenSymbol `json:"symbols"`
	CrossFile []pkgJSONGoldenCross  `json:"cross_file"`
}

func loadPackageJSONExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "package-json" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(package-json): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"package-json\" not found in LoadDialects()")
	return nil
}

func packageJSONFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "package-json", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
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

// TestPackageJSONDialectGoldens (LB-Q-package-json, design §13.2 steps
// 10/12): every testdata/*.json fixture's entities and cross-file
// references, against a hand-written golden. relPath is simulated as
// "package.json" (the fixtures live at other basenames on disk so they
// are not themselves picked up by classification/embedding, §2.1).
func TestPackageJSONDialectGoldens(t *testing.T) {
	x := loadPackageJSONExtractor(t)
	for _, fx := range packageJSONFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", "package.json", src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got pkgJSONGolden
			got.Symbols = []pkgJSONGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != "package.json" {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, pkgJSONGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature, Tier: tierName(n.Tier),
				})
				if len(n.Calls) != 0 {
					t.Errorf("%s: %s: unexpected Calls %v (this dialect declares no intra-file references)", fx, n.Name, n.Calls)
				}
			}
			got.CrossFile = []pkgJSONGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, pkgJSONGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
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
