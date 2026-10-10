package extractor

// githubactionsdialect_test.go — goldens for the github-actions dialect
// (LB-Q-github-actions, design §13.2 steps 10/12, port of ghaextract.go).
// Modeled on packagejsondialect_test.go: no generic TestDialectGoldens
// runner exists yet for doc-engine dialects, so this dialect gets its own,
// in the same golden shape plus "calls" (this dialect's job "needs" is an
// intra-file reference, unlike package-json's dialect).
//
// Two file shapes route through one dialect (design §4): a workflow
// (basenames/globs under .github/workflows/) and a composite/reusable
// action definition (action.yml/action.yaml). Fixtures simulate a relPath
// per shape so cross_file's module-source resolution (relative to the
// referring file's directory) is exercised realistically; they are not
// named "action.yml" on disk so they are never themselves picked up by
// classification when fathom ingests its own repository (§2.1, precedent:
// packagejsondialect_test.go's package.json note).

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

type ghaGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type ghaGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type ghaGolden struct {
	Symbols   []ghaGoldenSymbol `json:"symbols"`
	CrossFile []ghaGoldenCross  `json:"cross_file"`
}

// ghaFixtureRelPath is the relPath each fixture simulates: a workflow
// under .github/workflows/, an action definition under a nested
// .github/actions/<name>/ (so local "./" uses: values show the
// module-source strategy's known dir-relative-vs-repo-root-relative gap,
// PROVENANCE.md).
var ghaFixtureRelPath = map[string]string{
	"workflow-basic.yml":    ".github/workflows/ci.yml",
	"workflow-reusable.yml": ".github/workflows/deploy.yml",
	"workflow-no-name.yml":  ".github/workflows/smoke.yml",
	"action-composite.yml":  ".github/actions/my-helper/action.yml",
	"broken.yml":            ".github/workflows/broken.yml",
}

func loadGitHubActionsExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "github-actions" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(github-actions): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"github-actions\" not found in LoadDialects()")
	return nil
}

func githubActionsFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "github-actions", "testdata")
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

// TestGitHubActionsDialectGoldens (LB-Q-github-actions, design §13.2 steps
// 10/12): every testdata/*.yml fixture's entities and cross-file
// references, against a hand-written golden.
func TestGitHubActionsDialectGoldens(t *testing.T) {
	x := loadGitHubActionsExtractor(t)
	for _, fx := range githubActionsFixtures(t) {
		base := filepath.Base(fx)
		relPath, ok := ghaFixtureRelPath[base]
		if !ok {
			t.Fatalf("%s: no simulated relPath registered in ghaFixtureRelPath", base)
		}
		t.Run(base, func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", relPath, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got ghaGolden
			got.Symbols = []ghaGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != relPath {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, ghaGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []ghaGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, ghaGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want ghaGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []ghaGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []ghaGoldenCross{}
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

// TestGitHubActionsDialectIncumbentParity (design §13.2 step 15, port of
// ghaextract.go): the essential facts ghaextract_test.go's own fixture
// established — a named workflow, jobs with runs-on and needs edges,
// uses: steps captured, a run: step not captured as an action — hold for
// the new dialect too. Not a byte-for-byte reuse of ghaextract_test.go:
// the file-basename name fallback is the doc engine's `$file` alternative
// (`.name | $file cut:.`, TestGitHubActionsDialectUnnamedWorkflow;
// PROVENANCE.md), which cuts at the first dot where the incumbent cut at the
// last.
func TestGitHubActionsDialectIncumbentParity(t *testing.T) {
	// The same shape as ghaextract_test.go's own ghaFixture (package
	// extractor_test, not visible here): a named workflow, two jobs where
	// "deploy" needs "build", a uses: step and a run: step.
	const parityFixture = `name: CI Pipeline
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      - name: Run tests
        run: go test ./...
  deploy:
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v4
`
	x := loadGitHubActionsExtractor(t)
	res, err := x.ExtractDoc("svc", ".github/workflows/ci.yml", []byte(parityFixture))
	if err != nil {
		t.Fatalf("ExtractDoc: %v", err)
	}
	byKindName := make(map[string]bool, len(res.Nodes))
	var workflowName, buildSig, deploySig string
	var deployCalls []string
	for _, n := range res.Nodes {
		byKindName[string(n.Kind)+":"+n.Name] = true
		switch {
		case string(n.Kind) == "GHAWorkflow":
			workflowName = n.Name
		case string(n.Kind) == "GHAJob" && n.Name == "build":
			buildSig = n.Signature
		case string(n.Kind) == "GHAJob" && n.Name == "deploy":
			deploySig = n.Signature
			deployCalls = n.Calls
		}
	}
	if workflowName != "CI Pipeline" {
		t.Errorf("workflow name: got %q, want %q", workflowName, "CI Pipeline")
	}
	if buildSig != "ubuntu-latest" || deploySig != "ubuntu-latest" {
		t.Errorf("runs-on signatures: build=%q deploy=%q, want ubuntu-latest", buildSig, deploySig)
	}
	if len(deployCalls) != 1 || deployCalls[0] != "build" {
		t.Errorf("deploy needs: got %v, want [build]", deployCalls)
	}
	if !byKindName["GHAAction:actions/checkout@v4"] {
		t.Error("expected a GHAAction node for actions/checkout@v4")
	}
	if byKindName["GHAAction:go test ./..."] {
		t.Error("a run: step must not produce a GHAAction node")
	}
}

// TestGitHubActionsDialectUnnamedWorkflow (LB-F306): a workflow with no name:
// is named from its file, as the incumbent named it, by data (`$file`, cut at
// the first dot); a named one keeps its name:.
func TestGitHubActionsDialectUnnamedWorkflow(t *testing.T) {
	x := loadGitHubActionsExtractor(t)
	for _, tc := range []struct{ rel, src, want string }{
		{".github/workflows/release.yml", "on: push\njobs:\n  a:\n    runs-on: x\n", "release"},
		{".github/workflows/ci.test.yaml", "on: push\njobs:\n  a:\n    runs-on: x\n", "ci"},
		{".github/workflows/release.yml", "name: Ship it\non: push\njobs:\n  a:\n    runs-on: x\n", "Ship it"},
	} {
		res, err := x.ExtractDoc("svc", tc.rel, []byte(tc.src))
		if err != nil {
			t.Fatal(err)
		}
		got := ""
		for _, n := range res.Nodes {
			if string(n.Kind) == "GHAWorkflow" {
				got = n.Name
			}
		}
		if got != tc.want {
			t.Errorf("%s: workflow named %q, want %q", tc.rel, got, tc.want)
		}
	}
}

// TestGHAPinKind (LB-F311): a remote action's ref is a commit SHA (40 hex) or
// a tag or branch; a local path, a docker:// image and an unversioned value
// are no pin.
func TestGHAPinKind(t *testing.T) {
	sha := "actions/checkout@" + strings.Repeat("ab12", 10)
	for _, tc := range []struct {
		uses string
		want core.SourceKind
		ok   bool
	}{
		{sha, GHAPinSHA, true},
		{"actions/checkout@v4", GHAPinRef, true},
		{"github/codeql-action/analyze@main", GHAPinRef, true},
		{"o/r@" + strings.Repeat("AB12", 10), GHAPinRef, true},
		{"o/r@" + strings.Repeat("ab12", 9), GHAPinRef, true},
		{"./.github/actions/setup", "", false},
		{"docker://alpine:3.19", "", false},
		{"actions/checkout", "", false},
		{"o/r@${{ inputs.ref }}", "", false},
	} {
		got, ok := GHAPinKind(tc.uses)
		if got != tc.want || ok != tc.ok {
			t.Errorf("GHAPinKind(%q) = %q, %v; want %q, %v", tc.uses, got, ok, tc.want, tc.ok)
		}
	}
}

func TestGHADialectExtractorPins(t *testing.T) {
	x, err := NewDocDialectExtractor("github-actions")
	if err != nil {
		t.Fatal(err)
	}
	g := NewGHADialectExtractor(x)
	sha := strings.Repeat("0123456789", 4)
	src := "on: push\njobs:\n  a:\n    runs-on: x\n    steps:\n      - uses: actions/checkout@v4\n      - uses: o/r/sub@" + sha + "\n      - uses: ./local\n      - uses: actions/checkout@v4\n"
	nodes, err := g.Extract("svc", ".github/workflows/ci.yml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, n := range nodes {
		if strings.HasPrefix(n.Name, "pin:") {
			got[n.Name] = string(n.Kind) + " " + n.Signature
		}
	}
	want := map[string]string{
		"pin:actions/checkout@v4": "GHAPinRef actions/checkout",
		"pin:o/r/sub@" + sha:      "GHAPinSHA o/r/sub",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("pins %v, want %v", got, want)
	}
}
