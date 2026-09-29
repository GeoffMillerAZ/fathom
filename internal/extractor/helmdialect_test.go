package extractor

// helmdialect_test.go — goldens for the helm tags+doc dialect (LB-Q-helm,
// design §13.2 steps 10/12). No generic runner exists for a tags+doc dialect
// (TestDialectGoldens reads tags dialects only), so this one walks
// dialects/helm/testdata as a tree of small charts: each file is extracted as
// the path it has under testdata/, so Chart.yaml and values.yaml reach the
// doc engine and templates/*.tpl, templates/*.yaml and NOTES.txt the tags
// engine exactly as classification would send them. Regenerate with the
// shared -update flag; a regenerated golden is reviewed as a diff, never
// accepted blind.

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

type helmGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type helmGolden struct {
	Engine    string            `json:"engine"`
	Symbols   []goldenSymbol    `json:"symbols"`
	CrossFile []helmGoldenCross `json:"cross_file"`
}

func helmFixtures(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("dialects", "helm", "testdata")
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

// helmRel is the repository path a fixture stands for: its path under
// testdata/.
func helmRel(fx string) string {
	rel, _ := filepath.Rel(filepath.Join("dialects", "helm", "testdata"), fx)
	return filepath.ToSlash(rel)
}

func helmExtractGolden(t *testing.T, x *TagsDocExtractor, fx string) helmGolden {
	t.Helper()
	src, err := os.ReadFile(fx)
	if err != nil {
		t.Fatal(err)
	}
	rel := helmRel(fx)
	g := helmGolden{Engine: "tags", Symbols: []goldenSymbol{}, CrossFile: []helmGoldenCross{}}
	if x.IsDocFile(rel) {
		g.Engine = "doc"
		res, err := x.ExtractDoc("svc", rel, src)
		if err != nil {
			t.Fatalf("ExtractDoc %s: %v", fx, err)
		}
		for _, c := range res.CrossFile {
			g.CrossFile = append(g.CrossFile, helmGoldenCross{From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line})
		}
	}
	nodes, err := x.Extract("svc", rel, src)
	if err != nil {
		t.Fatalf("Extract %s: %v", fx, err)
	}
	for _, n := range nodes {
		if n.Service != "svc" || n.FilePath != rel {
			t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
		}
		g.Symbols = append(g.Symbols, goldenSymbol{
			Name: n.Name, Kind: string(n.Kind), Package: n.Package, Line: n.Line,
			EndLine: n.EndLine, Signature: n.Signature, Calls: n.Calls, Tier: tierName(n.Tier),
		})
	}
	return g
}

func marshalHelmGolden(g helmGolden) []byte {
	b, _ := json.MarshalIndent(g, "", "  ")
	return append(b, '\n')
}

// TestDialectGoldens_Helm holds the helm dialect to its hand-checked goldens:
// chart, dependency and value-key documents, named templates, template calls,
// .Values accessors and the cross-file dependency references.
func TestDialectGoldens_Helm(t *testing.T) {
	x, err := NewTagsDocExtractor("helm")
	if err != nil {
		t.Fatalf("NewTagsDocExtractor(helm): %v", err)
	}
	defer x.Close()
	for _, fx := range helmFixtures(t) {
		t.Run(helmRel(fx), func(t *testing.T) {
			got := helmExtractGolden(t, x, fx)
			gp := fx + ".golden.json"
			if *updateGoldens {
				if err := os.WriteFile(gp, marshalHelmGolden(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v (run with -update to create)", err)
			}
			var want helmGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if !reflect.DeepEqual(marshalHelmGolden(got), marshalHelmGolden(want)) {
				t.Errorf("%s: golden mismatch\n--- got\n%s\n--- want\n%s", fx, marshalHelmGolden(got), marshalHelmGolden(want))
			}
		})
	}
}

// TestHelmRouting: the doc spec's detect names the documents, everything else
// the manifest matches is a template.
func TestHelmRouting(t *testing.T) {
	x, err := NewTagsDocExtractor("helm")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	for rel, doc := range map[string]bool{
		"Chart.yaml":                 true,
		"charts/redis/values.yaml":   true,
		"templates/deployment.yaml":  false,
		"a/templates/_helpers.tpl":   false,
		"a/templates/NOTES.txt":      false,
		"a/templates/tests/pod.yaml": false,
		"values-prod.yaml":           false,
	} {
		if got := x.IsDocFile(rel); got != doc {
			t.Errorf("IsDocFile(%q) = %v, want %v", rel, got, doc)
		}
	}
}

// TestHelmSurvivesSyntaxError: a template with an unterminated action and a
// chart whose YAML does not parse never fail the file (design §13.2 step 10's
// required fixture): the definitions before the error still come out.
func TestHelmSurvivesSyntaxError(t *testing.T) {
	x, err := NewTagsDocExtractor("helm")
	if err != nil {
		t.Fatal(err)
	}
	defer x.Close()
	src, err := os.ReadFile("dialects/helm/testdata/broken/templates/broken.tpl")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := x.Extract("svc", "broken/templates/broken.tpl", src)
	if err != nil {
		t.Fatalf("a recoverable syntax error must not error the file: %v", err)
	}
	var names []string
	for _, n := range nodes {
		if n.Kind == "HelmTemplate" {
			names = append(names, n.Name)
		}
	}
	if len(names) == 0 || names[0] != "broken.before" {
		t.Fatalf("the definition before the error must survive; got %v", names)
	}
	for _, rel := range []string{"broken/Chart.yaml", "broken/values.yaml"} {
		src, _ := os.ReadFile(filepath.Join("dialects", "helm", "testdata", rel))
		if _, err := x.Extract("svc", rel, src); err != nil {
			t.Fatalf("%s: malformed YAML must yield no nodes, not an error: %v", rel, err)
		}
	}
}
