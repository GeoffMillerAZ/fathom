package extractor

// gitlabcidialect_test.go — goldens for the gitlab-ci doc-engine dialect
// (LB-Q-gitlab-ci, design §13.2 steps 10/12/14). Same golden shape as the
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

type gitlabCIGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type gitlabCIGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type gitlabCIGolden struct {
	Symbols   []gitlabCIGoldenSymbol `json:"symbols"`
	CrossFile []gitlabCIGoldenCross  `json:"cross_file"`
}

func loadGitLabCIExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "gitlab-ci" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(gitlab-ci): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"gitlab-ci\" not found in LoadDialects()")
	return nil
}

func gitlabCIFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "gitlab-ci", "testdata")
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

// TestGitLabCIDialectGoldens: every testdata/*.yml fixture's jobs, hidden
// templates and cross-file references against a hand-written golden. relPath
// is the fixture's own basename, so a repo-path target resolves against the
// repository root.
func TestGitLabCIDialectGoldens(t *testing.T) {
	x := loadGitLabCIExtractor(t)
	for _, fx := range gitlabCIFixtures(t) {
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
			var got gitlabCIGolden
			got.Symbols = []gitlabCIGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != relPath {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, gitlabCIGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []gitlabCIGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, gitlabCIGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want gitlabCIGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []gitlabCIGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []gitlabCIGoldenCross{}
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

// TestGitLabCIDialectTopLevelKeys: the top-level keys GitLab reserves are
// never jobs, a hidden key (leading ".") is a template whatever its name, a
// job is a map (a scalar or list under an unreserved key is not one), and a
// job name may hold spaces and punctuation, compared whole.
func TestGitLabCIDialectTopLevelKeys(t *testing.T) {
	src := `default:
  image: alpine
workflow:
  rules:
    - if: $CI_COMMIT_BRANCH
include: other.yml
stages: [test]
variables:
  A: "1"
cache:
  key: k
image: alpine
services: [redis]
before_script: [echo]
after_script: [echo]
spec:
  inputs: {}
types: [test]
setup: just-a-scalar
scripts-list:
  - not-a-job
.pre-hidden:
  script: [echo]
"my job: fast":
  script: [echo]
pages:
  script: [echo]
`
	x := loadGitLabCIExtractor(t)
	res, err := x.ExtractDoc("svc", ".gitlab-ci.yml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, n := range res.Nodes {
		got[n.Name] = string(n.Kind)
	}
	want := map[string]string{
		".pre-hidden":  "GitLabCITemplate",
		"my job: fast": "GitLabCIJob",
		"pages":        "GitLabCIJob",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("entities\n got: %v\nwant: %v", got, want)
	}
}

// TestGitLabCIDialectEdgeForms: needs and dependencies read in every form
// GitLab writes them (a bare name, a {job:} map, a flow list, an empty
// list), extends as one name or a list, and all three edges land in Calls
// once per target.
func TestGitLabCIDialectEdgeForms(t *testing.T) {
	src := `a:
  script: [x]
b:
  script: [x]
c:
  extends: .t
  needs: a
  dependencies: [b, b]
  script: [x]
d:
  extends: [.t, .u, .t]
  needs:
    - a
    - job: b
      optional: true
    - {job: c, artifacts: false}
  dependencies: []
  script: [x]
.t:
  script: [x]
.u:
  extends: .t
`
	x := loadGitLabCIExtractor(t)
	res, err := x.ExtractDoc("svc", ".gitlab-ci.yml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, n := range res.Nodes {
		got[n.Name] = n.Calls
	}
	want := map[string][]string{
		"a":  nil,
		"b":  nil,
		"c":  {".t", "a", "b"},
		"d":  {".t", ".u", "a", "b", "c"},
		".t": nil,
		".u": {".t"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("calls\n got: %v\nwant: %v", got, want)
	}
}
