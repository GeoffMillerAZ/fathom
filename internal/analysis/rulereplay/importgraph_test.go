package rulereplay

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

// writeTree writes files (repo-relative path -> content) under a fresh
// t.TempDir() and returns its root.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func deps(t *testing.T, idxDeps func(string) []string, pkg string) []string {
	t.Helper()
	got := idxDeps(pkg)
	sort.Strings(got)
	return got
}

func TestBuildImportGraphFromDir_SimpleImport(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod": "module tmpmod\n\ngo 1.24\n",
		"a/a.go": "package a\n\nimport \"tmpmod/b\"\n\nfunc F() { b.G() }\n",
		"b/b.go": "package b\n\nfunc G() {}\n",
	})
	idx, modPath, warnings, err := BuildImportGraphFromDir(dir)
	if err != nil {
		t.Fatalf("BuildImportGraphFromDir: %v", err)
	}
	if modPath != "tmpmod" {
		t.Errorf("modulePath = %q, want tmpmod", modPath)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none", warnings)
	}
	if got := deps(t, idx.PackageEdges.Deps, "a"); !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("Deps(a) = %v, want [b]", got)
	}
	if got := idx.PackageEdges.Deps("b"); len(got) != 0 {
		t.Errorf("Deps(b) = %v, want none", got)
	}
}

func TestBuildImportGraphFromDir_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, err := BuildImportGraphFromDir(dir); err == nil {
		t.Fatal("expected an error for a directory with no go.mod")
	}
}

func TestBuildImportGraphFromDir_SkipsTestFiles(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":          "module tmpmod\n\ngo 1.24\n",
		"a/a.go":          "package a\n\nfunc F() {}\n",
		"a/a_test.go":     "package a\n\nimport \"tmpmod/onlyintest\"\n\nfunc TestF() { _ = onlyintest.X }\n",
		"onlyintest/x.go": "package onlyintest\n\nvar X int\n",
	})
	idx, _, _, err := BuildImportGraphFromDir(dir)
	if err != nil {
		t.Fatalf("BuildImportGraphFromDir: %v", err)
	}
	if got := idx.PackageEdges.Deps("a"); len(got) != 0 {
		t.Errorf("Deps(a) = %v, want none (import lives only in _test.go)", got)
	}
}

func TestBuildImportGraphFromDir_SkipsTestdataAndNestedModule(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":               "module tmpmod\n\ngo 1.24\n",
		"a/a.go":               "package a\n\nimport \"tmpmod/b\"\n\nfunc F() { b.G() }\n",
		"a/testdata/fake.go":   "package fake\n\nimport \"tmpmod/shouldnotappear\"\n\nfunc _() { _ = shouldnotappear.X }\n",
		"shouldnotappear/x.go": "package shouldnotappear\n\nvar X int\n",
		"b/b.go":               "package b\n\nfunc G() {}\n",
		"nested/go.mod":        "module nested\n\ngo 1.24\n",
		"nested/x.go":          "package nested\n\nimport \"tmpmod/b\"\n\nfunc _() { b.G() }\n",
	})
	idx, _, _, err := BuildImportGraphFromDir(dir)
	if err != nil {
		t.Fatalf("BuildImportGraphFromDir: %v", err)
	}
	nodes := idx.PackageEdges.Nodes()
	for _, n := range nodes {
		if n == "a/testdata" || n == "nested" {
			t.Errorf("Nodes() = %v, should never include %q (testdata/nested-module boundary)", nodes, n)
		}
	}
	if got := deps(t, idx.PackageEdges.Deps, "a"); !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("Deps(a) = %v, want [b]", got)
	}
}

func TestBuildImportGraphFromDir_ModuleRootPackage(t *testing.T) {
	dir := writeTree(t, map[string]string{
		"go.mod":  "module tmpmod\n\ngo 1.24\n",
		"main.go": "package main\n\nimport \"tmpmod/a\"\n\nfunc main() { a.F() }\n",
		"a/a.go":  "package a\n\nfunc F() {}\n",
	})
	idx, _, _, err := BuildImportGraphFromDir(dir)
	if err != nil {
		t.Fatalf("BuildImportGraphFromDir: %v", err)
	}
	if got := deps(t, idx.PackageEdges.Deps, "."); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("Deps(.) = %v, want [a]", got)
	}
}
