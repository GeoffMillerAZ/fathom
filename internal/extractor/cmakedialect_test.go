package extractor

// cmakedialect_test.go — LB-Q-cmake (design §13.2 steps 10 and 12). The per-fixture goldens run in the
// generic TestDialectGoldens (dialects/cmake/testdata); this file holds the obligations a golden cannot
// express: the parser survives a syntax error, the manifest claims the files the queue entry names, the
// link attachment keeps the incumbent's kinds and call spellings where it reaches, and it stays linear.

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func cmakeDialectNodes(t *testing.T, path, src string) map[string]core.SymbolNode {
	t.Helper()
	e, err := NewTSExtractor("cmake")
	if err != nil {
		t.Fatalf("NewTSExtractor(cmake): %v", err)
	}
	defer e.Close()
	nodes, err := e.Extract("svc", path, []byte(src))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	by := map[string]core.SymbolNode{}
	for _, n := range nodes {
		by[n.Name] = n
	}
	return by
}

// TestCMakeDialect_SurvivesSyntaxError: a stray endif() and a nameless add_library() between two targets must
// not lose the commands around them (the required syntax-error fixture, §13.2 step 10).
func TestCMakeDialect_SurvivesSyntaxError(t *testing.T) {
	src, err := os.ReadFile("dialects/cmake/testdata/broken.cmake")
	if err != nil {
		t.Fatal(err)
	}
	by := cmakeDialectNodes(t, "CMakeLists.txt", string(src))
	for _, want := range []string{"Broken", "first", "second", "AFTER"} {
		if _, ok := by[want]; !ok {
			t.Errorf("symbol %q lost around the error; have %v", want, by)
		}
	}
}

// TestCMakeDialect_KeepsIncumbentSpellings: the incumbent cmakeextract.go's SourceKind strings and its
// CT-1 spelling of a namespaced target (`fmt::fmt` is `fmt.fmt`) hold for the commands both read.
func TestCMakeDialect_KeepsIncumbentSpellings(t *testing.T) {
	const src = `cmake_minimum_required(VERSION 3.20)
project(MyProj VERSION 1.0 LANGUAGES CXX)
option(MYPROJ_BUILD_TESTS "Build the tests" ON)
find_package(Threads REQUIRED)
add_library(core STATIC
  src/a.cc
  src/b.cc)
target_link_libraries(core PUBLIC fmt::fmt)
ADD_EXECUTABLE(mytool src/main.cc)
target_link_libraries(mytool PRIVATE core Threads::Threads)
# a comment with add_library(bogus) inside must be ignored
add_custom_target(docs COMMAND doxygen)
function(myproj_add_warnings target)
endfunction()
macro(myproj_setup)
endmacro()
`
	by := cmakeDialectNodes(t, "CMakeLists.txt", src)
	want := map[string]core.SourceKind{
		"MyProj": core.KindCMakeProject, "MYPROJ_BUILD_TESTS": core.KindCMakeOption, "Threads": core.KindCMakeDependency,
		"core": core.KindCMakeTarget, "mytool": core.KindCMakeTarget, "docs": core.KindCMakeTarget,
		"myproj_add_warnings": core.KindCMakeFunction, "myproj_setup": core.KindCMakeFunction,
	}
	for name, kind := range want {
		if n, ok := by[name]; !ok || n.Kind != kind {
			t.Errorf("%s: kind %q, want %q", name, n.Kind, kind)
		}
	}
	if _, ok := by["bogus"]; ok {
		t.Error("add_library inside a # comment must not extract")
	}
	if got := by["mytool"].Calls; !slices.Equal(got, []string{"Threads.Threads", "core"}) {
		t.Errorf("mytool.Calls = %v, want [Threads.Threads core]", got)
	}
	if got := by["core"].Calls; !slices.Equal(got, []string{"fmt.fmt"}) {
		t.Errorf("core.Calls = %v, want [fmt.fmt]", got)
	}
	if c := by["core"]; c.EndLine != c.Line+2 {
		t.Errorf("core spans %d-%d, want the three lines of the multi-line add_library", c.Line, c.EndLine)
	}
}

// TestCMakeDialect_LinkAttachmentBound: a link command attaches to its target across up to four commands and
// not further (the measured gap distribution, tags.scm); the bound is stated, not silent.
func TestCMakeDialect_LinkAttachmentBound(t *testing.T) {
	for gap := 0; gap <= 6; gap++ {
		src := "add_library(a STATIC a.cc)\n" + strings.Repeat("message(STATUS x)\n", gap) + "target_link_libraries(a PUBLIC b)\n"
		got := cmakeDialectNodes(t, "CMakeLists.txt", src)["a"].Calls
		want := []string{"b"}
		if gap > 4 {
			want = nil
		}
		if !slices.Equal(got, want) {
			t.Errorf("gap %d: a.Calls = %v, want %v", gap, got, want)
		}
	}
}

// TestCMakeDialect_LinearInTargets: pairing every target with every link command in a file is quadratic and
// the engine's scan budget fails the file; the bounded attachment must extract a flat file of hundreds of
// targets whole.
func TestCMakeDialect_LinearInTargets(t *testing.T) {
	const n = 600
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "add_library(lib%d STATIC l%d.cc)\ntarget_link_libraries(lib%d PUBLIC lib%d)\n", i, i, i, (i+1)%n)
	}
	by := cmakeDialectNodes(t, "CMakeLists.txt", b.String())
	if len(by) != n {
		t.Fatalf("%d symbols, want %d", len(by), n)
	}
	if got := by["lib599"].Calls; !slices.Equal(got, []string{"lib0"}) {
		t.Errorf("lib599.Calls = %v, want [lib0]", got)
	}
}

// TestCMakeDialect_ClaimsItsFiles: the queue entry names CMakeLists.txt and *.cmake.
func TestCMakeDialect_ClaimsItsFiles(t *testing.T) {
	ds, err := LoadDialects()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range ds {
		if d.ID != "cmake" {
			continue
		}
		if !slices.Contains(d.Match.Basenames, "CMakeLists.txt") || !slices.Contains(d.Match.Extensions, ".cmake") {
			t.Errorf("match = %+v, want basename CMakeLists.txt and extension .cmake", d.Match)
		}
		return
	}
	t.Fatal("cmake dialect not registered")
}
