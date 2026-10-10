package main

// tests_select_srcread_test.go — TQ-24 (docs/plans/test-intelligence.md,
// O783): a test that reads another package's source through a literal path
// (filepath.Glob + go/parser) never imports that package, so an edit there
// selected nothing under --safe. The changed file a literal read names now
// seeds the enclosing function.

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func srcReadFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/srcfix\n\ngo 1.22\n")
	write("a/x.go", "package a\n\nfunc X() int { return 1 }\n")
	write("a/notes.md", "doc\n")
	write("b/b.go", "package b\n\nfunc B() int { return 2 }\n")
	write("b/b_test.go", `package b

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func scanA(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "a", "*.go"))
	for _, f := range files {
		_, _ = parser.ParseFile(token.NewFileSet(), f, nil, 0)
	}
}

func dyn(p string) { _, _ = os.ReadFile(p) }

func TestReadsA(t *testing.T) { scanA(t) }

func TestDynamic(t *testing.T) { dyn("x") }

func TestDirectGlob(t *testing.T) { fs, _ := filepath.Glob("../a/*.go"); _ = fs }

func TestPlain(t *testing.T) { _ = B() }
`)
	return root
}

func TestTestsSelectSafeSelectsTestReadingChangedSource(t *testing.T) {
	root := srcReadFixture(t)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "src-svc", "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	old := testsSelectRoot
	testsSelectRoot = root
	t.Cleanup(func() { testsSelectRoot = old })
	setEconomicsLedger(t, filepath.Join(t.TempDir(), "econ.jsonl"))

	selectFor := func(changed ...string) string {
		t.Helper()
		var out, errBuf bytes.Buffer
		args := []string{"--since", "HEAD~1", "--service", "src-svc", "--store", storePath, "--safe"}
		if code := runTestsSelect(args, fakeGitRunner(changed), &out, &errBuf); code != 0 {
			t.Fatalf("exit %d: %s", code, errBuf.String())
		}
		return out.String()
	}
	text := selectFor("a/x.go")
	if !strings.Contains(text, "TestReadsA") {
		t.Fatalf("a change to a/x.go must select the test that globs ../a/*.go:\n%s", text)
	}
	if !strings.Contains(text, "TestDirectGlob") {
		t.Fatalf("a Test function that makes the read itself must be selected:\n%s", text)
	}
	if strings.Contains(text, "TestPlain") {
		t.Errorf("an unrelated test was selected:\n%s", text)
	}
	if text := selectFor("a/notes.md"); strings.Contains(text, "TestReadsA") {
		t.Errorf("a file the glob does not match selected the reader:\n%s", text)
	}
}

func TestSrcReadChangedSymbols(t *testing.T) {
	root := srcReadFixture(t)
	storePath := filepath.Join(t.TempDir(), "s.sqlite")
	var ing bytes.Buffer
	if code := run([]string{"ingest", "--service", "src-svc", "--path", root, "--store", storePath}, &ing, &ing); code != 0 {
		t.Fatalf("ingest failed (%d): %s", code, ing.String())
	}
	nodes := listNodesForTest(t, storePath, "src-svc")
	got, st := srcReadChangedSymbols(root, nodes, []string{"a/x.go"})
	if joined := strings.Join(got, " "); !strings.Contains(joined, ".scanA") || !strings.Contains(joined, ".TestDirectGlob") || strings.Contains(joined, ".dyn") {
		t.Errorf("seeds = %v, want scanA only", got)
	}
	if st.Reads != 4 || st.Literal != 2 || st.Dynamic != 2 {
		t.Errorf("stats = %+v, want 4 reads (two glob literals, ParseFile and ReadFile dynamic)", st)
	}
	if got, _ := srcReadChangedSymbols(root, nodes, []string{"b/b.go"}); len(got) != 0 {
		t.Errorf("no read names b/b.go, got %v", got)
	}
}
