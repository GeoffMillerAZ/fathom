package extractor

// Direct top-level wire tests for the CT-1/CT-2 contracts. The behavioral
// suite reaches walkCpp/cppCanonicalName through CppExtractor.Extract —
// a method name every extractor type shares, which the reachability index
// resolves first-wins, so an unrelated ingest reorder can flip the winner
// and void these rows (it did, 2026-08-23, when the CUE wire tests landed
// in this package). The contracted symbols are called here directly.

import (
	"testing"

	ts "github.com/tree-sitter/go-tree-sitter"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/extractor/treesitter"
)

func TestCppCanonicalNameWiredDirect(t *testing.T) {
	if got := cppCanonicalName("Widget::render"); got != "Widget.render" {
		t.Fatalf("cppCanonicalName(Widget::render) = %q, want Widget.render", got)
	}
	if got := cppCanonicalName("a::b::c"); got != "a.b.c" {
		t.Fatalf("cppCanonicalName(a::b::c) = %q, want a.b.c", got)
	}
}

func TestWalkCppWiredDirect(t *testing.T) {
	src := []byte("class Widget {\npublic:\n  void render();\n};\nvoid Widget::render() { }\n")
	var nodes []core.SymbolNode
	err := treesitter.WithTree(cppLang, src, func(root *ts.Node) {
		walkCpp(root, src, "svc", "pkg", "widget.cpp", nil, &nodes)
	})
	if err != nil {
		t.Fatalf("WithTree: %v", err)
	}
	for _, n := range nodes {
		if n.Name == "Widget.render" {
			return
		}
	}
	names := make([]string, 0, len(nodes))
	for _, n := range nodes {
		names = append(names, n.Name)
	}
	t.Fatalf("walkCpp: want Widget.render among %v", names)
}
