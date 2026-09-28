package tsengine

import (
	"context"
	"strings"
	"testing"
)

// TestTreeSExpr (LB-F20): the probe's tree rendering names every named
// node with its field, 1-based line and a leaf's text, and honours the cap.
func TestTreeSExpr(t *testing.T) {
	p, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	src := []byte("class Foo:\n    def bar(self, x):\n        return baz(x)\n")
	tree, err := p.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()

	lines, truncated, err := tree.SExpr(0)
	if err != nil {
		t.Fatal(err)
	}
	if truncated != 0 {
		t.Fatalf("uncapped render truncated %d", truncated)
	}
	out := strings.Join(lines, "\n")
	for _, want := range []string{
		"(module L1",
		"  (class_definition L1",
		"name: (identifier L1 \"Foo\")",
		"(function_definition L2",
		"name: (identifier L2 \"bar\")",
		"function: (identifier L3 \"baz\")",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("SExpr lacks %q:\n%s", want, out)
		}
	}

	capped, truncated, err := tree.SExpr(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped) != 3 || truncated == 0 {
		t.Fatalf("SExpr(3) = %d lines, %d truncated", len(capped), truncated)
	}
}

// TestPeakModulePages: a parse records its module's memory for the
// language-breadth reading's peak_module_pages.
func TestPeakModulePages(t *testing.T) {
	ResetPeakModulePages()
	p, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	tree, err := p.Parse([]byte("x = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	tree.Close()
	if got := PeakModulePages(); got == 0 || got > MemoryLimitPages {
		t.Fatalf("PeakModulePages = %d, want in (0, %d]", got, MemoryLimitPages)
	}
}
