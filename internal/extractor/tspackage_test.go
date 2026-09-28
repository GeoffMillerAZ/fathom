package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestTSPackageCapture (LB-F5, design §3.5): with package.strategy
// "capture", @scope.package sets SymbolNode.Package — file-wide when its
// match has no @scope range (a Kotlin/Java package header), else for the
// definitions inside that range, nested ranges joined with "."; a file
// without a capture keeps pkgFromPath. Other strategies ignore it.
func TestTSPackageCapture(t *testing.T) {
	kotlinQuery := `
(package_header (identifier) @scope.package)
(class_declaration (type_identifier) @name) @definition.class
(function_declaration (simple_identifier) @name) @definition.function
`
	kinds := map[string]tsDefKind{
		"class":    {Kind: core.KindKotlinClass, Container: true},
		"function": {Kind: core.KindKotlinFunction, Callable: true},
	}
	capture := func(r *tsDialectRules) { r.packageStrategy = "capture"; r.chainContainers = true }
	pkgs := func(t *testing.T, e *TSExtractor, path, src string) map[string]string {
		t.Helper()
		nodes, err := e.Extract("svc", path, []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]string{}
		for _, n := range nodes {
			out[n.Name] = n.Package
		}
		return out
	}

	t.Run("file-wide header", func(t *testing.T) {
		e := testTSExtractor("kotlin", "tree_sitter_kotlin", kotlinQuery, kinds, capture)
		defer e.Close()
		got := pkgs(t, e, "src/main/Shapes.kt", "package com.example.shapes\n\nclass C {\n  fun f() {}\n}\nfun top() {}\n")
		for _, name := range []string{"C", "C.f", "top"} {
			if got[name] != "com.example.shapes" {
				t.Errorf("%s package = %q, want com.example.shapes (all: %v)", name, got[name], got)
			}
		}
	})
	t.Run("no capture keeps pkgFromPath", func(t *testing.T) {
		e := testTSExtractor("kotlin", "tree_sitter_kotlin", kotlinQuery, kinds, capture)
		defer e.Close()
		if got := pkgs(t, e, "src/main/Default.kt", "fun top() {}\n"); got["top"] != "Default" {
			t.Errorf("package = %q, want pkgFromPath's Default", got["top"])
		}
	})
	t.Run("dir strategy ignores the capture", func(t *testing.T) {
		e := testTSExtractor("kotlin", "tree_sitter_kotlin", kotlinQuery, kinds, nil)
		defer e.Close()
		if got := pkgs(t, e, "src/main/Shapes.kt", "package com.example.shapes\nfun top() {}\n"); got["top"] != "Shapes" {
			t.Errorf("package = %q, want pkgFromPath's Shapes", got["top"])
		}
	})
	t.Run("scoped ranges nest", func(t *testing.T) {
		// Python classes stand in for block namespaces (C#'s shape): the
		// @scope capture bounds where the package name applies.
		q := `
(class_definition name: (identifier) @scope.package) @scope
(function_definition name: (identifier) @name) @definition.function
`
		e := testTSExtractor("python", "tree_sitter_python", q,
			map[string]tsDefKind{"function": {Kind: core.KindPyFunc, Callable: true}},
			func(r *tsDialectRules) { r.packageStrategy = "capture" })
		defer e.Close()
		got := pkgs(t, e, "lib/m.py", "class Outer:\n    def a(self): pass\n    class Inner:\n        def b(self): pass\n\ndef c(): pass\n")
		want := map[string]string{"a": "Outer", "b": "Outer.Inner", "c": "m"}
		for name, w := range want {
			if got[name] != w {
				t.Errorf("%s package = %q, want %q (all: %v)", name, got[name], w, got)
			}
		}
	})
}
