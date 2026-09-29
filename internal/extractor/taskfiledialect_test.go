package extractor

// taskfiledialect_test.go — goldens for the taskfile doc-engine dialect
// (LB-Q-taskfile, no incumbent, design §13.2 steps 10/12/14). The same golden
// shape as the kustomize and msbuild dialects: symbols (name/kind/package/
// line/end_line/signature/calls/tier) and the cross-file references the engine
// records, whether or not fusion.ResolveDocXrefs can resolve them (a remote
// include, one written with a template). A fixture is read as if it sat in
// ops/Taskfile.yml, so a relative include resolves against a directory that
// has siblings and a parent.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type taskfileGoldenSymbol struct {
	Name      string   `json:"name"`
	Kind      string   `json:"kind"`
	Package   string   `json:"package"`
	Line      int      `json:"line"`
	EndLine   int      `json:"end_line"`
	Signature string   `json:"signature,omitempty"`
	Calls     []string `json:"calls,omitempty"`
	Tier      string   `json:"tier"`
}

type taskfileGoldenCross struct {
	From     string `json:"from"`
	Edge     string `json:"edge"`
	Raw      string `json:"raw"`
	Target   string `json:"target"`
	Strategy string `json:"strategy"`
	Line     int    `json:"line"`
}

type taskfileGolden struct {
	Symbols   []taskfileGoldenSymbol `json:"symbols"`
	CrossFile []taskfileGoldenCross  `json:"cross_file"`
}

// taskfileFixtureRel is where every fixture is read as sitting.
const taskfileFixtureRel = "ops/Taskfile.yml"

func loadTaskfileExtractor(t *testing.T) *DocExtractor {
	t.Helper()
	dialects, err := LoadDialects()
	if err != nil {
		t.Fatalf("LoadDialects: %v", err)
	}
	for _, d := range dialects {
		if d.ID != "taskfile" {
			continue
		}
		x, err := LoadDocExtractor(d)
		if err != nil {
			t.Fatalf("LoadDocExtractor(taskfile): %v", err)
		}
		return x
	}
	t.Fatal("dialect \"taskfile\" not found in LoadDialects()")
	return nil
}

// taskfileFixtures lists the testdata files the dialect owns: a golden sits
// beside each as <file>.golden.json.
func taskfileFixtures(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("dialects", "taskfile", "testdata")
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.json") {
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

// TestTaskfileDialectGoldens: every testdata fixture's Taskfile, tasks and
// references, and its include references, against a hand-written golden.
func TestTaskfileDialectGoldens(t *testing.T) {
	x := loadTaskfileExtractor(t)
	for _, fx := range taskfileFixtures(t) {
		t.Run(filepath.Base(fx), func(t *testing.T) {
			src, err := os.ReadFile(fx)
			if err != nil {
				t.Fatal(err)
			}
			res, err := x.ExtractDoc("svc", taskfileFixtureRel, src)
			if err != nil {
				t.Fatalf("ExtractDoc: %v", err)
			}
			var got taskfileGolden
			got.Symbols = []taskfileGoldenSymbol{}
			for _, n := range res.Nodes {
				if n.Service != "svc" || n.FilePath != taskfileFixtureRel {
					t.Fatalf("%s: plumbing changed: service %q file %q", fx, n.Service, n.FilePath)
				}
				got.Symbols = append(got.Symbols, taskfileGoldenSymbol{
					Name: n.Name, Kind: string(n.Kind), Package: n.Package,
					Line: n.Line, EndLine: n.EndLine, Signature: n.Signature,
					Calls: n.Calls, Tier: tierName(n.Tier),
				})
			}
			got.CrossFile = []taskfileGoldenCross{}
			for _, c := range res.CrossFile {
				got.CrossFile = append(got.CrossFile, taskfileGoldenCross{
					From: c.From, Edge: c.Edge, Raw: c.Raw, Target: c.Target, Strategy: c.Strategy, Line: c.Line,
				})
			}
			gp := fx + ".golden.json"
			raw, err := os.ReadFile(gp)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}
			var want taskfileGolden
			if err := json.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", gp, err)
			}
			if want.Symbols == nil {
				want.Symbols = []taskfileGoldenSymbol{}
			}
			if want.CrossFile == nil {
				want.CrossFile = []taskfileGoldenCross{}
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

// TestTaskfileDialectNamesTasksAsWritten: a task is named by its key exactly as
// the file writes it: a colon in the key is part of the name (`generate:mocks`,
// which is not a namespace), a wildcard task keeps its `*`, and a quoted key
// loses only its quotes. Included tasks are named by the file that declares
// them, never by the namespace the includer gives (`website:build` is `build` in
// website/Taskfile.yml).
func TestTaskfileDialectNamesTasksAsWritten(t *testing.T) {
	src := "version: '3'\ntasks:\n  generate:mocks:\n    cmds: [x]\n  'quoted key':\n    cmds: [x]\n  \"release:*\":\n    cmds: [x]\n  install:\n    deps: [generate:mocks]\n"
	x := loadTaskfileExtractor(t)
	res, err := x.ExtractDoc("svc", "Taskfile.yml", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	calls := map[string][]string{}
	for _, n := range res.Nodes {
		if n.Kind == "TaskfileTask" {
			names = append(names, n.Name)
			calls[n.Name] = n.Calls
		}
	}
	if want := []string{"generate:mocks", "quoted key", "release:*", "install"}; !reflect.DeepEqual(names, want) {
		t.Errorf("task names = %q, want %q", names, want)
	}
	if want := []string{"generate:mocks"}; !reflect.DeepEqual(calls["install"], want) {
		t.Errorf("install calls = %q, want %q", calls["install"], want)
	}
}

// TestTaskfileDialectRootIsTheFile: a Taskfile that declares no tasks (only
// includes, or nothing but a version) is still a file with one root entity,
// named by the file, so an include has an entity to resolve to; a file that is
// not a mapping has none.
func TestTaskfileDialectRootIsTheFile(t *testing.T) {
	x := loadTaskfileExtractor(t)
	for _, c := range []struct {
		rel, src string
		want     []string
	}{
		{"Taskfile.yml", "version: '3'\nincludes:\n  a: ./a\n", []string{"Taskfile.yml"}},
		{"deploy/Taskfile.dist.yaml", "version: '3'\n", []string{"Taskfile.dist.yaml"}},
		{"taskfile.yml", "- a\n- b\n", nil},
		{"Taskfile.yml", "just a string\n", nil},
	} {
		res, err := x.ExtractDoc("svc", c.rel, []byte(c.src))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(res.Roots, c.want) {
			t.Errorf("%s %q: roots = %q, want %q", c.rel, c.src, res.Roots, c.want)
		}
	}
}
