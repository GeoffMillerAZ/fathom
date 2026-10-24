package extractor

// checkovdialect_test.go — goldens for the checkov-policies tags+doc dialect (LB-Q-checkov-policies,
// design §13.2 steps 10/12/14). A Python check rides the tags engine on the python
// grammar; a YAML graph check is a document for the doc engine. Each fixture
// sits at the repository path it stands for under testdata/<case>/ (the manifest's
// globs name the checks/ layout), beside its golden; the fixtures are written for
// the dialect, not copied from a project.

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

type checkovGolden struct {
	Engine    string            `json:"engine"`
	Symbols   []goldenSymbol    `json:"symbols"`
	CrossFile []helmGoldenCross `json:"cross_file"`
}

func checkovFixtures(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("dialects", "checkov-policies", "testdata")
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && !strings.HasSuffix(p, ".golden.json") {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(out)
	if len(out) == 0 {
		t.Fatalf("%s: no fixtures", root)
	}
	return out
}

// checkovRel is the repository path a fixture stands for: its path under testdata/.
func checkovRel(fx string) string {
	rel, _ := filepath.Rel(filepath.Join("dialects", "checkov-policies", "testdata"), fx)
	return filepath.ToSlash(rel)
}

func checkovExtract(t *testing.T, x *TagsDocExtractor, rel string, src []byte) checkovGolden {
	t.Helper()
	g := checkovGolden{Engine: "tags", Symbols: []goldenSymbol{}, CrossFile: []helmGoldenCross{}}
	if x.IsDocFile(rel) {
		g.Engine = "doc"
		res, err := x.ExtractDoc("svc", rel, src)
		if err != nil {
			t.Fatalf("ExtractDoc %s: %v", rel, err)
		}
		for _, c := range res.CrossFile {
			g.CrossFile = append(g.CrossFile, helmGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line})
		}
	}
	nodes, err := x.Extract("svc", rel, src)
	if err != nil {
		t.Fatalf("Extract %s: %v", rel, err)
	}
	for _, n := range nodes {
		if n.Service != "svc" || n.FilePath != rel {
			t.Fatalf("%s: plumbing changed: service %q file %q", rel, n.Service, n.FilePath)
		}
		g.Symbols = append(g.Symbols, goldenSymbol{
			Name: n.Name, Kind: string(n.Kind), Package: n.Package, Line: n.Line,
			EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls, Tier: tierName(n.Tier),
		})
	}
	return g
}

func marshalCheckovGolden(g checkovGolden) []byte {
	b, _ := json.MarshalIndent(g, "", "  ")
	return append(b, '\n')
}

// TestCheckovDialectGoldens holds the dialect to its hand-written goldens.
func TestCheckovDialectGoldens(t *testing.T) {
	x, err := NewTagsDocExtractor("checkov-policies")
	if err != nil {
		t.Fatalf("NewTagsDocExtractor(checkov-policies): %v", err)
	}
	defer x.Close()
	for _, fx := range checkovFixtures(t) {
		rel := checkovRel(fx)
		t.Run(rel, func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			got := checkovExtract(t, x, rel, src)
			raw, err := os.ReadFile(fx + ".golden.json")
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want checkovGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s golden: %v", fx, err)
			}
			if want.Symbols == nil {
				want.Symbols = []goldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []helmGoldenCross{}
			}
			if !reflect.DeepEqual(marshalCheckovGolden(got), marshalCheckovGolden(want)) {
				t.Errorf("%s: golden mismatch\n--- got\n%s\n--- want\n%s", fx, marshalCheckovGolden(got), marshalCheckovGolden(want))
			}
		})
	}
}

// TestCheckovRouting: the doc spec's detect names the YAML graph checks; every
// other file the manifest claims is a Python check for the tags engine.
func TestCheckovRouting(t *testing.T) {
	x, err := NewTagsDocExtractor("checkov-policies")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	for rel, doc := range map[string]bool{
		"checkov/terraform/checks/graph_checks/aws/Check.yaml": true,
		"checks/graph_checks/Check.yml":                        true,
		"checkov/terraform/checks/resource/aws/Check.py":       false,
		"checkov/terraform/checks/data/aws/Check.py":           false,
		"checkov/terraform/checks/graph_checks/aws/Check.json": false,
	} {
		if got := x.IsDocFile(rel); got != doc {
			t.Errorf("IsDocFile(%q) = %v, want %v", rel, got, doc)
		}
	}
}

// TestCheckovSurvivesSyntaxError: a module whose later __init__ is unterminated
// never fails the file (design §13.2 step 10's required fixture), and the check
// before the error keeps its resource types.
func TestCheckovSurvivesSyntaxError(t *testing.T) {
	x, err := NewTagsDocExtractor("checkov-policies")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	src, err := os.ReadFile("dialects/checkov-policies/testdata/broken/checks/resource/broken.py")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := x.Extract("svc", "broken/checks/resource/broken.py", src)
	if err != nil {
		t.Fatalf("a recoverable syntax error must not error the file: %v", err)
	}
	var early []string
	for _, n := range nodes {
		if n.Name == "CKV_EARLY_1" {
			early = n.Calls
		}
	}
	if !reflect.DeepEqual(early, []string{"aws_early"}) {
		t.Fatalf("the check before the error must survive with its resource types; nodes %+v", nodes)
	}
}
