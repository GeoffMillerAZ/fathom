package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/contextbench/publicloc"
)

// copyFixture copies testdata/module into a temp dir: go/packages runs the
// go command, and a module living under a directory named testdata is
// exactly the shape `./...` refuses to walk.
func copyFixture(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "module")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, 0o644)
	})
	if err != nil {
		t.Fatalf("copying fixture: %v", err)
	}
	return dst
}

func TestCallersTruth_FixtureModule(t *testing.T) {
	root := copyFixture(t)
	got, err := CallersTruth(root)
	if err != nil {
		t.Fatalf("CallersTruth: %v", err)
	}
	if got.Module != "example.com/loctruthfixture" {
		t.Errorf("module = %q", got.Module)
	}
	if got.Packages != 3 || got.PackagesWithErrors != 0 {
		t.Errorf("packages=%d errors=%d, want 3/0", got.Packages, got.PackagesWithErrors)
	}
	want := []publicloc.CallEdge{
		{CalleeFile: "a/a.go", Callee: "Bar", CallerFile: "a/a.go", Caller: "Foo", Line: 4},
		{CalleeFile: "a/a.go", Callee: "Foo", CallerFile: "a/a.go", Caller: "T.M", Line: 13},
		{CalleeFile: "a/a.go", Callee: "Foo", CallerFile: "a/a.go", Caller: "init", Line: 27},
		{CalleeFile: "a/a.go", Callee: "Foo", CallerFile: "b/b.go", Caller: "Run", Line: 8},
		{CalleeFile: "a/a.go", Callee: "Foo", CallerFile: "b/b.go", Caller: "Twice", Line: 13},
		{CalleeFile: "a/a.go", Callee: "Foo", CallerFile: "b/b.go", Caller: "Twice", Line: 14},
		{CalleeFile: "a/a.go", Callee: "Gen", CallerFile: "a/a.go", Caller: "UseGen", Line: 25},
		{CalleeFile: "a/a.go", Callee: "T.M", CallerFile: "b/b.go", Caller: "Run", Line: 8},
		{CalleeFile: "b/b.go", Callee: "Run", CallerFile: "main.go", Caller: "main", Line: 6},
	}
	if !reflect.DeepEqual(got.Edges, want) {
		gj, _ := json.MarshalIndent(got.Edges, "", "  ")
		wj, _ := json.MarshalIndent(want, "", "  ")
		t.Errorf("edges mismatch\n got: %s\nwant: %s", gj, wj)
	}
	// The interface call in Via must not surface as a static caller of T.M,
	// and the test file's call to Foo must not surface at all.
	for _, e := range got.Edges {
		if e.Caller == "Via" {
			t.Errorf("dynamic dispatch counted as a static call: %+v", e)
		}
		if strings.HasSuffix(e.CallerFile, "_test.go") {
			t.Errorf("test file counted as a caller: %+v", e)
		}
	}
}

func TestCallersTruth_IsDeterministic(t *testing.T) {
	root := copyFixture(t)
	a, err := CallersTruth(root)
	if err != nil {
		t.Fatal(err)
	}
	b, err := CallersTruth(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("two runs over the same tree differ")
	}
	callers := a.CallersOf(publicloc.FuncRef{File: "a/a.go", Func: "Foo"})
	wantCallers := []publicloc.FuncRef{
		{File: "b/b.go", Func: "Run"},
		{File: "a/a.go", Func: "T.M"},
		{File: "b/b.go", Func: "Twice"},
		{File: "a/a.go", Func: "init"},
	}
	if !reflect.DeepEqual(callers, wantCallers) {
		t.Errorf("CallersOf(Foo) = %+v, want %+v", callers, wantCallers)
	}
}

func TestRun_FilterAndOutFile(t *testing.T) {
	root := copyFixture(t)
	out := filepath.Join(t.TempDir(), "callers.json")
	var stdout, stderr bytes.Buffer
	if rc := run([]string{"-root", root, "-out", out, "-func", "b/b.go:Run"}, &stdout, &stderr); rc != 0 {
		t.Fatalf("run exited %d: %s", rc, stderr.String())
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var cf publicloc.CallersFile
	if err := json.Unmarshal(raw, &cf); err != nil {
		t.Fatalf("output is not a CallersFile: %v", err)
	}
	if len(cf.Edges) != 1 || cf.Edges[0].Caller != "main" {
		t.Errorf("filtered edges = %+v, want the single main→Run edge", cf.Edges)
	}
	if !strings.Contains(stderr.String(), "1 edges") {
		t.Errorf("stderr summary missing: %q", stderr.String())
	}
}

func TestRun_UsageErrors(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if rc := run(nil, &stdout, &stderr); rc != 2 {
		t.Errorf("no -root: rc=%d, want 2", rc)
	}
	if rc := run([]string{"-root", filepath.Join(t.TempDir(), "missing")}, &stdout, &stderr); rc != 1 {
		t.Errorf("missing root: rc=%d, want 1", rc)
	}
}
