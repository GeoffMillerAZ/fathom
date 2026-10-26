package extractor

import (
	"context"
	"os"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

func TestGLSLGrammarMacroPrecision(t *testing.T) {
	src, err := os.ReadFile("dialects/glsl/testdata/macro_precision.frag")
	if err != nil {
		t.Fatal(err)
	}
	p, err := tsengine.NewParser(context.Background(), "glsl", "tree_sitter_glsl")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	result, err := ProbeParse(p, ProbeTarget{}, src)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Tree.Close()
	if result.WrittenErrors != 0 {
		t.Fatalf("macro precision has %d ERROR/MISSING nodes", result.WrittenErrors)
	}
}

func TestGLSLArrayConstructorTrees(t *testing.T) {
	p, err := tsengine.NewParser(context.Background(), "glsl", "tree_sitter_glsl")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	q, err := p.CompileQuery("(call_expression function: (_) @callee)")
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	q.CaptureNodeTypes()
	for _, tc := range []struct {
		callee string
		expr   string
	}{
		{"float[3]", "float[3](1.0, 2.0, 3.0)"},
		{"vec4[]", "vec4[](vec4(1.0))"},
		{"vec4[2]", "vec4[2](vec4(1.0), vec4(2.0))"},
		{"vec3[3]", "vec3[3](vec3(1.0), vec3(2.0), vec3(3.0))"},
		{"mat4[2]", "mat4[2](mat4(1.0), mat4(2.0))"},
		{"Sample[2]", "Sample[2](Sample(1.0), Sample(2.0))"},
		{"Sample[]", "Sample[](Sample(1.0), Sample(2.0))"},
		{"vec4[1][1]", "vec4[1][1](vec4[1](vec4(1.0)))"},
	} {
		t.Run(tc.callee, func(t *testing.T) {
			src := []byte("struct Sample { float value; }; void main() { " + tc.expr + "; }")
			result, err := ProbeParse(p, ProbeTarget{}, src)
			if err != nil {
				t.Fatal(err)
			}
			defer result.Tree.Close()
			if result.WrittenErrors != 0 {
				t.Fatalf("%s has %d ERROR/MISSING nodes", tc.callee, result.WrittenErrors)
			}
			matches, err := q.Run(result.Tree)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, match := range matches {
				for _, capture := range match.Captures {
					if capture.Node.Text(src) == tc.callee {
						found = true
						if capture.Type != "array_type_specifier" {
							t.Errorf("callee %s has type %s, want array_type_specifier", tc.callee, capture.Type)
						}
					}
				}
			}
			if !found {
				t.Fatalf("missing callee %s", tc.callee)
			}
		})
	}
}

func TestGLSLArrayIndexingTrees(t *testing.T) {
	p, err := tsengine.NewParser(context.Background(), "glsl", "tree_sitter_glsl")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	src := []byte("void main() { vec4 a[2]; vec4 x = a[0]; float b[2][2]; float y = b[0][1]; }")
	result, err := ProbeParse(p, ProbeTarget{}, src)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Tree.Close()
	if result.WrittenErrors != 0 {
		t.Fatalf("array indexing has %d ERROR/MISSING nodes", result.WrittenErrors)
	}
	q, err := p.CompileQuery("(subscript_expression) @index")
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	matches, err := q.Run(result.Tree)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"a[0]": false, "b[0]": false, "b[0][1]": false}
	for _, match := range matches {
		for _, capture := range match.Captures {
			if _, ok := want[capture.Node.Text(src)]; ok {
				want[capture.Node.Text(src)] = true
			}
		}
	}
	for text, found := range want {
		if !found {
			t.Errorf("index %s lost its subscript_expression", text)
		}
	}
}
