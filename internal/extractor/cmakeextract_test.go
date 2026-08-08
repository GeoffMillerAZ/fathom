package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cmakeFixture exercises every command CMakeExtractor recognizes, plus the
// hazards: comments containing command text, multi-line invocations,
// case-insensitive command names, link keywords, namespaced (::) targets,
// and variable-ref targets that cannot be attached.
const cmakeFixture = `
cmake_minimum_required(VERSION 3.20)
project(MyProj VERSION 1.0 LANGUAGES CXX)

option(MYPROJ_BUILD_TESTS "Build the tests" ON)

find_package(Threads REQUIRED)

add_library(core STATIC
  src/a.cc
  src/b.cc)

ADD_EXECUTABLE(mytool src/main.cc)

# a comment with add_library(bogus) inside must be ignored
add_custom_target(docs COMMAND doxygen)

function(myproj_add_warnings target)
  target_compile_options(${target} PRIVATE -Wall)
endfunction()

macro(myproj_setup)
endmacro()

target_link_libraries(mytool PRIVATE core Threads::Threads)
target_link_libraries(core PUBLIC fmt::fmt)
target_link_libraries(${unknown_var} PRIVATE whatever)
`

func TestCMakeExtractor_TargetsAndLinks(t *testing.T) {
	e := NewCMakeExtractor()
	nodes, err := e.Extract("svc", "CMakeLists.txt", []byte(cmakeFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("symbol kinds", func(t *testing.T) {
		want := map[string]core.SourceKind{
			"MyProj":              core.KindCMakeProject,
			"MYPROJ_BUILD_TESTS":  core.KindCMakeOption,
			"Threads":             core.KindCMakeDependency,
			"core":                core.KindCMakeTarget,
			"mytool":              core.KindCMakeTarget,
			"docs":                core.KindCMakeTarget,
			"myproj_add_warnings": core.KindCMakeFunction,
			"myproj_setup":        core.KindCMakeFunction,
		}
		for name, kind := range want {
			n, ok := byName[name]
			if !ok {
				t.Errorf("missing symbol %q; have %v", name, names(nodes))
				continue
			}
			if n.Kind != kind {
				t.Errorf("symbol %q: want kind %q, got %q", name, kind, n.Kind)
			}
		}
	})

	t.Run("commented commands are ignored", func(t *testing.T) {
		if _, ok := byName["bogus"]; ok {
			t.Error("add_library inside a # comment must not extract")
		}
	})

	t.Run("link edges attach to targets, keywords excluded, :: normalized", func(t *testing.T) {
		mytool := byName["mytool"]
		wantCalls := []string{"Threads.Threads", "core"}
		if len(mytool.Calls) != len(wantCalls) {
			t.Fatalf("mytool.Calls = %v, want %v", mytool.Calls, wantCalls)
		}
		for i, w := range wantCalls {
			if mytool.Calls[i] != w {
				t.Errorf("mytool.Calls[%d] = %q, want %q", i, mytool.Calls[i], w)
			}
		}
		coreTarget := byName["core"]
		if len(coreTarget.Calls) != 1 || coreTarget.Calls[0] != "fmt.fmt" {
			t.Errorf("core.Calls = %v, want [fmt.fmt]", coreTarget.Calls)
		}
	})

	t.Run("target types recorded in Signature", func(t *testing.T) {
		if sig := byName["core"].Signature; sig != "library STATIC" {
			t.Errorf("core.Signature = %q, want %q", sig, "library STATIC")
		}
		if sig := byName["mytool"].Signature; sig != "executable" {
			t.Errorf("mytool.Signature = %q, want %q", sig, "executable")
		}
		if sig := byName["docs"].Signature; sig != "custom" {
			t.Errorf("docs.Signature = %q, want %q", sig, "custom")
		}
	})

	t.Run("tier is pre-scan", func(t *testing.T) {
		for _, n := range nodes {
			if n.Tier != core.TierPreScan {
				t.Errorf("symbol %q: want TierPreScan (hand parser), got %q", n.Name, n.Tier)
			}
		}
	})

	t.Run("deterministic order", func(t *testing.T) {
		again, err := e.Extract("svc", "CMakeLists.txt", []byte(cmakeFixture))
		if err != nil {
			t.Fatalf("second Extract: %v", err)
		}
		if len(again) != len(nodes) {
			t.Fatalf("non-deterministic: %d vs %d symbols", len(nodes), len(again))
		}
		for i := range nodes {
			if nodes[i].Name != again[i].Name || nodes[i].Line != again[i].Line {
				t.Errorf("index %d differs: %q@%d vs %q@%d", i, nodes[i].Name, nodes[i].Line, again[i].Name, again[i].Line)
			}
		}
	})
}

// TestCMakeExtractor_ModuleFile pins .cmake module extraction: functions and
// macros defined in a module file extract with the file's package.
func TestCMakeExtractor_ModuleFile(t *testing.T) {
	src := []byte(`
# Helper module
function(myproj_enable_lto target)
endfunction()
`)
	e := NewCMakeExtractor()
	nodes, err := e.Extract("svc", "cmake/Lto.cmake", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	var found bool
	for _, n := range nodes {
		if n.Name == "myproj_enable_lto" && n.Kind == core.KindCMakeFunction {
			found = true
			if n.Line != 3 {
				t.Errorf("line = %d, want 3", n.Line)
			}
		}
	}
	if !found {
		t.Errorf("want myproj_enable_lto as KindCMakeFunction, have %v", names(nodes))
	}
}

func TestCMakeExtractor_EmptyAndCommentOnly(t *testing.T) {
	e := NewCMakeExtractor()
	for _, src := range []string{"", "# only a comment\n"} {
		nodes, err := e.Extract("svc", "CMakeLists.txt", []byte(src))
		if err != nil {
			t.Fatalf("Extract(%q): %v", src, err)
		}
		if len(nodes) != 0 {
			t.Errorf("Extract(%q): want 0 nodes, got %d", src, len(nodes))
		}
	}
}
