package extractor

// appframeworkdialect_test.go — goldens for the Azure and AWS app-framework
// doc-engine dialects (wave iac: LB-Q-arm-templates, LB-Q-serverless,
// LB-Q-pulumi-yaml, LB-Q-aws-cdk; design §13.2 steps 10/12/14): each
// fixture's entities, the references they make to the document's own
// entities (Calls), and their cross-file references. A fixture is read as
// if it sat in infra/<its path under testdata>, so a relative path resolves
// against a directory; a fixture may sit in a subdirectory when the dialect
// claims it by basename (serverless.yml, Pulumi.yaml, cdk.json).

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

type appGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type appGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type appGolden struct {
	Symbols   []appGoldenSymbol `json:"symbols"`
	CrossFile []appGoldenCross  `json:"cross_file"`
}

// appFrameworkDialects are the dialects these goldens cover.
var appFrameworkDialects = []string{"arm-templates", "serverless", "pulumi-yaml", "aws-cdk"}

// appFixtures lists a dialect's fixtures, testdata-relative, sorted.
func appFixtures(t *testing.T, id string) []string {
	t.Helper()
	dir := filepath.Join("dialects", id, "testdata")
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasSuffix(p, ".golden.json") {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", dir)
	}
	return out
}

func TestAppFrameworkDialectGoldens(t *testing.T) {
	for _, id := range appFrameworkDialects {
		t.Run(id, func(t *testing.T) {
			x, err := NewDocDialectExtractor(id)
			if err != nil {
				t.Fatalf("NewDocDialectExtractor(%s): %v", id, err)
			}
			for _, fx := range appFixtures(t, id) {
				t.Run(fx, func(t *testing.T) {
					path := filepath.Join("dialects", id, "testdata", filepath.FromSlash(fx))
					rel := "infra/" + fx
					src, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					res, err := x.ExtractDoc("svc", rel, src)
					if err != nil {
						t.Fatalf("ExtractDoc: %v", err)
					}
					got := appGolden{Symbols: []appGoldenSymbol{}, CrossFile: []appGoldenCross{}}
					for _, n := range res.Nodes {
						if n.Service != "svc" || n.FilePath != rel {
							t.Fatalf("plumbing changed: service %q file %q", n.Service, n.FilePath)
						}
						got.Symbols = append(got.Symbols, appGoldenSymbol{
							Name: n.Name, Kind: string(n.Kind), Package: n.Package,
							Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
							Calls: n.Calls, Tier: tierName(n.Tier),
						})
					}
					for _, c := range res.CrossFile {
						got.CrossFile = append(got.CrossFile, appGoldenCross{
							From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
						})
					}
					gp := path + ".golden.json"
					if *updateGoldens {
						b, _ := json.MarshalIndent(got, "", "  ")
						if err := os.WriteFile(gp, append(b, '\n'), 0o644); err != nil {
							t.Fatal(err)
						}
						return
					}
					raw, err := os.ReadFile(gp)
					if err != nil {
						t.Fatalf("read golden: %v (run with -update to create)", err)
					}
					var want appGolden
					if err := json.Unmarshal(raw, &want); err != nil {
						t.Fatalf("%s: %v", gp, err)
					}
					if want.Symbols == nil {
						want.Symbols = []appGoldenSymbol{}
					}
					if want.CrossFile == nil {
						want.CrossFile = []appGoldenCross{}
					}
					if !reflect.DeepEqual(got.Symbols, want.Symbols) {
						t.Errorf("symbols\n got: %+v\nwant: %+v", got.Symbols, want.Symbols)
					}
					if !reflect.DeepEqual(got.CrossFile, want.CrossFile) {
						t.Errorf("cross_file\n got: %+v\nwant: %+v", got.CrossFile, want.CrossFile)
					}
				})
			}
		})
	}
}
