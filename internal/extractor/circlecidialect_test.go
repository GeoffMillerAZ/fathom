package extractor

// circlecidialect_test.go — goldens for the circleci doc-engine dialect
// (LB-Q-circleci, design §13.2 steps 10/12/14). Same golden shape as the
// docker-compose, kubernetes and github-actions dialects: symbols
// (name/kind/package/line/end_line/signature/calls/tier) and the cross-file
// references the engine records, whether or not fusion.ResolveDocXrefs can
// resolve them (see PROVENANCE.md: include:local and trigger:include name a
// file, and a pipeline file has no root entity to resolve to).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type circleCIGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type circleCIGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type circleCIGolden struct {
	Symbols   []circleCIGoldenSymbol `json:"symbols"`
	CrossFile []circleCIGoldenCross  `json:"cross_file"`
}

func loadCircleCIExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "circleci" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(circleci): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"circleci\" not found in LoadDialects()")
	return nil
}

func circleCIFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "circleci", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yml" {
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

// TestCircleCIDialectGoldens: every testdata/*.yml fixture's jobs, hidden
// templates and cross-file references against a hand-written golden. relPath
// is the fixture's own basename, so a repo-path target resolves against the
// repository root.
func TestCircleCIDialectGoldens(t *testing.T) {
	x := loadCircleCIExtractor(t)
	for _, fx := range circleCIFixtures(t) {
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
			var got circleCIGolden
			got.Symbols = []circleCIGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != relPath {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, circleCIGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []circleCIGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, circleCIGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want circleCIGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []circleCIGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []circleCIGoldenCross{}
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

// TestCircleCIDialectTopLevelSections: only the four sections that declare
// something are read (jobs, workflows, commands, executors), a workflows
// "version" scalar is not a workflow, and an entry that is not a map is not
// an entity.
func TestCircleCIDialectTopLevelSections(t *testing.T) {
	src := `version: 2.1
orbs:
  node: circleci/node@5.1.0
parameters:
  p: {type: string}
jobs:
  a:
    docker: [{image: x}]
  bad: just-a-scalar
workflows:
  version: 2
  w:
    jobs: [a]
commands:
  c:
    steps: []
executors:
  e:
    docker: [{image: y}]
`
	x := loadCircleCIExtractor(t)
	res, err := x.ExtractDoc("svc", "config.yml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, n := range res.Nodes {
		got[n.Name] = string(n.Kind)
	}
	want := map[string]string{
		"a": "CircleCIJob", "w": "CircleCIWorkflow", "c": "CircleCICommand", "e": "CircleCIExecutor",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entities\n got: %v\nwant: %v", got, want)
	}
}

// TestCircleCIDialectEdgeForms: a workflow runs a job written as a bare
// name or as a one-key map, requires names the jobs that must finish first
// (a block or flow list, once per target), a step invokes a command as a
// bare name or as a one-key map at any depth (when and unless nest steps), a
// job names its executor as a string or under name:, and a name that no
// section declares (an orb job, an approval, a built-in step) is never an
// edge.
func TestCircleCIDialectEdgeForms(t *testing.T) {
	src := `version: 2.1
executors:
  e1: {docker: [{image: a}]}
  e2: {docker: [{image: b}]}
commands:
  c1:
    steps: [checkout]
  c2:
    steps:
      - c1
      - run: x
      - when:
          condition: true
          steps:
            - unless:
                condition: false
                steps: [c1]
jobs:
  j1:
    executor: e1
    steps: [checkout, c1, {c2: {p: 1}}, run: z]
  j2:
    executor: {name: e2}
    steps:
      - c2
      - orb/step
  j3:
    docker: [{image: c}]
    steps: [ghost]
workflows:
  w:
    jobs:
      - j1
      - j2:
          requires: [j1, j1]
      - gate:
          type: approval
          requires:
            - j2
      - j3:
          requires: [gate, orb/job]
      - orb/job
`
	x := loadCircleCIExtractor(t)
	res, err := x.ExtractDoc("svc", "config.yml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, n := range res.Nodes {
		got[n.Name] = n.Calls
	}
	want := map[string][]string{
		"e1": nil, "e2": nil,
		"c1": nil,
		"c2": {"c1"},
		"j1": {"c1", "c2", "e1"},
		"j2": {"c2", "e2"},
		"j3": nil,
		"w":  {"j1", "j2", "j3"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls\n got: %v\nwant: %v", got, want)
	}
}
