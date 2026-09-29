package extractor

// makefiledialect_test.go — LB-Q-makefile (design §13.2 steps 10 and 12). The per-fixture
// goldens run in the generic TestDialectGoldens (dialects/makefile/testdata); this file holds the
// obligations a golden cannot express: the parser must survive a syntax error, and the manifest
// must claim exactly the files the queue entry names.

import (
	"os"
	"slices"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestMakefileExtractor_SurvivesSyntaxError: a stray `)))` between two rules must not lose the
// rules and variables around it (the required syntax-error fixture, §13.2 step 10).
func TestMakefileExtractor_SurvivesSyntaxError(t *testing.T) {
	e, err := NewTSExtractor("makefile")
	if err != nil {
		t.Fatalf("NewTSExtractor(makefile): %v", err)
	}
	defer e.Close()
	src, err := os.ReadFile("dialects/makefile/testdata/broken.mk")
	if err != nil {
		t.Fatal(err)
	}
	nodes, err := e.Extract("svc", "Makefile", src)
	if err != nil {
		t.Fatalf("Extract must not error on a recoverable syntax error: %v", err)
	}
	var names []string
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	if want := []string{"GOOD", "before", "after", "LATE"}; !slices.Equal(names, want) {
		t.Fatalf("symbols around the error = %v, want %v", names, want)
	}
}

// TestMakefileDialect_Definitions pins the four kinds and what is deliberately not a definition:
// special targets declare attributes, `+=` appends, and a pattern rule is not a target.
func TestMakefileDialect_Definitions(t *testing.T) {
	e, err := NewTSExtractor("makefile")
	if err != nil {
		t.Fatalf("NewTSExtractor(makefile): %v", err)
	}
	defer e.Close()
	src := "CC = gcc\nLIBS += -lm\n.PHONY: all\nall: prog\n%.o: %.c\n\tcc -c $<\ndefine greet\nhi\nendef\n"
	nodes, err := e.Extract("svc", "Makefile", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.SourceKind{}
	for _, n := range nodes {
		got[n.Name] = n.Kind
	}
	want := map[string]core.SourceKind{"CC": "MakeVariable", "all": "MakeTarget", "%.o": "MakePatternRule", "greet": "MakeDefine"}
	if len(got) != len(want) {
		t.Fatalf("symbols = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: kind %q, want %q", k, got[k], v)
		}
	}
	for _, n := range nodes {
		if n.Name == "all" && !slices.Equal(n.Calls, []string{"prog"}) {
			t.Errorf("all.Calls = %v, want [prog]", n.Calls)
		}
	}
}

// TestMakefileDialect_ClaimsItsFiles: the queue entry names Makefile, *.mk and GNUmakefile.
func TestMakefileDialect_ClaimsItsFiles(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.ID != "makefile" {
			continue
		}
		for _, b := range []string{"Makefile", "makefile", "GNUmakefile"} {
			if !slices.Contains(d.Match.Basenames, b) {
				t.Errorf("basename %s not claimed", b)
			}
		}
		for _, x := range []string{".mk", ".make"} {
			if !slices.Contains(d.Match.Extensions, x) {
				t.Errorf("extension %s not claimed", x)
			}
		}
		return
	}
	t.Fatal("makefile dialect not registered")
}
