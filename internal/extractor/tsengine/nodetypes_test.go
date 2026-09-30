package tsengine

import (
	"context"
	"testing"
)

// TestQueryCaptureNodeTypes (LB-F46): a query asked for node types names
// each capture's grammar type — what a Zed outline.scm's @item kind is —
// and one not asked leaves Type empty, so extraction's hot path pays no
// extra guest call.
func TestQueryCaptureNodeTypes(t *testing.T) {
	p, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	src := []byte("class Foo:\n    def bar(self):\n        pass\n")
	tree, err := p.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	defer tree.Close()
	const query = "(class_definition name: (identifier) @name) @item\n(function_definition name: (identifier) @name) @item\n"

	for _, typed := range []bool{false, true} {
		q, err := p.CompileQuery(query)
		if err != nil {
			t.Fatal(err)
		}
		if typed {
			q.CaptureNodeTypes()
		}
		matches, err := q.Run(tree)
		q.Close()
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]string{}
		for _, m := range matches {
			for _, c := range m.Captures {
				got[c.Node.Text(src)[:3]+"@"+c.Name] = c.Type
			}
		}
		want := map[string]string{"cla@item": "class_definition", "Foo@name": "identifier", "def@item": "function_definition", "bar@name": "identifier"}
		if len(got) != len(want) {
			t.Fatalf("captures %v, want %d", got, len(want))
		}
		for k, w := range want {
			if !typed {
				w = ""
			}
			if got[k] != w {
				t.Errorf("typed=%v: %s type %q, want %q (%v)", typed, k, got[k], w, got)
			}
		}
	}
}
