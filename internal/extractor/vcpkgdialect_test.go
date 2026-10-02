package extractor

// vcpkgdialect_test.go — goldens for the vcpkg doc-engine dialect
// (LB-Q-vcpkg, design §13.2 steps 10/12/14). Each fixture is a directory
// testdata/<case>/ holding one manifest named as a real one is (an entity may
// be limited to a basename, so the base name matters) and its golden. The
// fixtures are written for the dialect, not copied from a project.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type vcpkgGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type vcpkgGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type vcpkgGolden struct {
	Symbols   []vcpkgGoldenSymbol `json:"symbols"`
	CrossFile []vcpkgGoldenCross  `json:"cross_file"`
}

func loadVcpkgExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "vcpkg" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(vcpkg): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"vcpkg\" not found in LoadDialects()")
	return nil
}

func vcpkgFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "vcpkg", "testdata")
	var out []string
	cases, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	for _, c := range cases {
		if !c.IsDir() {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(dir, c.Name(), "*.json"))
		for _, f := range files {
			if filepath.Ext(filepath.Base(f[:len(f)-len(".json")])) == ".golden" {
				continue
			}
			out = append(out, f)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

func vcpkgExtract(t *testing.T, x *DocExtractor, relPath string, src []byte) vcpkgGolden {
	t.Helper()
	res, err := x.ExtractDoc("svc", relPath, src)
	if err != nil {
		t.Fatalf("ExtractDoc: %v", err)
	}
	got := vcpkgGolden{Symbols: []vcpkgGoldenSymbol{}, CrossFile: []vcpkgGoldenCross{}}
	for _, n := range res.Nodes {
		if n.Service != "svc" || n.FilePath != relPath {
			t.Fatalf("%s: plumbing changed: service %q file %q", relPath, n.Service, n.FilePath)
		}
		got.Symbols = append(got.Symbols, vcpkgGoldenSymbol{
			Name: n.Name, Kind: string(n.Kind), Package: n.Package,
			Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
			Calls: n.Calls, Tier: tierName(n.Tier),
		})
	}
	for _, c := range res.CrossFile {
		got.CrossFile = append(got.CrossFile, vcpkgGoldenCross{
			From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
		})
	}
	return got
}

// TestVcpkgDialectGoldens: every fixture's entities and cross-file
// references against a hand-written golden. relPath is "<case>/<basename>".
func TestVcpkgDialectGoldens(t *testing.T) {
	x := loadVcpkgExtractor(t)
	for _, fx := range vcpkgFixtures(t) {
		rel := filepath.Base(filepath.Dir(fx)) + "/" + filepath.Base(fx)
		t.Run(rel, func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			got := vcpkgExtract(t, x, rel, src)
			raw, err := os.ReadFile(fx + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want vcpkgGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s golden: %v", fx, err)
			}
			if want.Symbols == nil {
				want.Symbols = []vcpkgGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []vcpkgGoldenCross{}
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

// TestVcpkgDialectFileScope: the two documents are told apart by basename. A
// vcpkg.json holds packages, features and dependencies and no registries; a
// vcpkg-configuration.json holds registries and overlay ports and no package,
// even when it happens to carry a "name" or a "dependencies" key.
func TestVcpkgDialectFileScope(t *testing.T) {
	x := loadVcpkgExtractor(t)
	cfg := `{"name":"not-a-package","dependencies":["zlib"],"registries":[{"kind":"git","repository":"r"}]}`
	got := vcpkgExtract(t, x, "vcpkg-configuration.json", []byte(cfg))
	if len(got.Symbols) != 1 || got.Symbols[0].Kind != "VcpkgRegistry" || len(got.CrossFile) != 0 {
		t.Errorf("vcpkg-configuration.json: %+v", got)
	}
	man := `{"name":"p","version":"1","registries":[{"name":"x"}],"overlay-ports":["./o"]}`
	got = vcpkgExtract(t, x, "vcpkg.json", []byte(man))
	if len(got.Symbols) != 1 || got.Symbols[0].Kind != "VcpkgPackage" || len(got.CrossFile) != 0 {
		t.Errorf("vcpkg.json: %+v", got)
	}
}

// TestVcpkgDialectUnnamed: a manifest with no name is no package (a
// manifest-mode project may omit it), and a document that is not JSON yields
// nothing.
func TestVcpkgDialectUnnamed(t *testing.T) {
	x := loadVcpkgExtractor(t)
	for name, src := range map[string]string{
		"no name":  `{"dependencies":["zlib"]}`,
		"not json": "name = \"x\"",
		"empty":    "",
	} {
		got := vcpkgExtract(t, x, "vcpkg.json", []byte(src))
		if len(got.Symbols) != 0 {
			t.Errorf("%s: symbols %+v", name, got.Symbols)
		}
	}
}
