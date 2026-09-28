package extractor

// k8sdialect_test.go — goldens for the kubernetes doc-engine dialect
// (LB-Q-kubernetes, port of k8sextract.go, design §13.2 steps 10/12). No
// generic "TestDialectGoldens" runner exists yet for doc-engine dialects
// (see packagejsondialect_test.go's own note), so this dialect gets its own
// runner, in the same golden shape package-json's uses
// (name/kind/package/line/end_line/signature/tier, plus "cross_file").

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type k8sGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type k8sGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type k8sGolden struct {
	Symbols   []k8sGoldenSymbol `json:"symbols"`
	CrossFile []k8sGoldenCross  `json:"cross_file"`
}

func loadK8sDialectExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "kubernetes" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(kubernetes): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"kubernetes\" not found in LoadDialects()")
	return nil
}

func k8sDialectFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "kubernetes", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
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

// TestK8sDialectGoldens (LB-Q-kubernetes, design §13.2 steps 10/12): every
// testdata/*.yaml fixture's entities and cross-file references, against a
// hand-written golden. relPath is simulated as the fixture's own basename
// (the fixtures live under dialects/kubernetes/testdata/, so they are not
// themselves picked up by classification/embedding, §2.1 — the dialect's
// match block claims nothing yet, so this is moot for now, but keeps the
// convention package-json's golden test set).
func TestK8sDialectGoldens(t *testing.T) {
	x := loadK8sDialectExtractor(t)
	for _, fx := range k8sDialectFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			relPath := filepath.Base(fx)
			res, err := x.ExtractDoc("svc", relPath, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got k8sGolden
			got.Symbols = []k8sGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != relPath {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, k8sGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []k8sGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, k8sGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want k8sGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []k8sGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []k8sGoldenCross{}
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
