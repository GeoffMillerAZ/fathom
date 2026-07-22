package tsengine

import (
	"context"
	"strings"
	"testing"
)

func TestNewParser_UnknownGrammar(t *testing.T) {
	_, err := NewParser(context.Background(), "cobol", "tree_sitter_cobol")
	if err == nil {
		t.Fatal("expected error for unknown grammar, got nil")
	}
}

func TestNewParser_UnknownEntryPoint(t *testing.T) {
	_, err := NewParser(context.Background(), "python", "tree_sitter_nope")
	if err == nil {
		t.Fatal("expected error for unknown entry point, got nil")
	}
}

func TestParser_ParsePython(t *testing.T) {
	ctx := context.Background()
	p, err := NewParser(ctx, "python", "tree_sitter_python")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	defer p.Close()

	src := []byte("class Bar:\n    def baz(self):\n        return 1\n\ndef foo(x):\n    return x + 1\n")
	tree, err := p.Parse(src)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	defer tree.Close()

	root, err := tree.Root()
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	if root.StartByte != 0 {
		t.Errorf("root.StartByte = %d, want 0", root.StartByte)
	}
	if int(root.EndByte) != len(src) {
		t.Errorf("root.EndByte = %d, want %d", root.EndByte, len(src))
	}

	q, err := p.CompileQuery(`
(function_definition name: (identifier) @name) @definition.function
(class_definition name: (identifier) @name) @definition.class
`)
	if err != nil {
		t.Fatalf("CompileQuery: %v", err)
	}
	defer q.Close()

	matches, err := q.Run(tree)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(matches) != 3 {
		t.Fatalf("len(matches) = %d, want 3 (Bar class, baz method, foo function): %+v", len(matches), matches)
	}

	var gotNames []string
	for _, m := range matches {
		for _, c := range m.Captures {
			if c.Name == "name" {
				gotNames = append(gotNames, c.Node.Text(src))
			}
		}
	}
	want := map[string]bool{"Bar": true, "baz": true, "foo": true}
	if len(gotNames) != 3 {
		t.Fatalf("captured names = %v, want 3 entries from %v", gotNames, want)
	}
	for _, n := range gotNames {
		if !want[n] {
			t.Errorf("unexpected captured name %q", n)
		}
	}
}

func TestParser_CompileQuery_BadQuery(t *testing.T) {
	ctx := context.Background()
	p, err := NewParser(ctx, "python", "tree_sitter_python")
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	defer p.Close()

	_, err = p.CompileQuery("(this_is_not_a_real_node_kind)")
	if err == nil {
		t.Fatal("expected error for malformed query, got nil")
	}
	if !strings.Contains(err.Error(), "query compile failed") {
		t.Errorf("error = %v, want a %q message", err, "query compile failed")
	}
}

func TestParser_Determinism(t *testing.T) {
	ctx := context.Background()
	src := []byte("def a():\n    pass\n\ndef b():\n    pass\n\ndef c():\n    pass\n")

	run := func() []string {
		p, err := NewParser(ctx, "python", "tree_sitter_python")
		if err != nil {
			t.Fatalf("NewParser: %v", err)
		}
		defer p.Close()
		tree, err := p.Parse(src)
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		defer tree.Close()
		q, err := p.CompileQuery(`(function_definition name: (identifier) @name) @definition.function`)
		if err != nil {
			t.Fatalf("CompileQuery: %v", err)
		}
		defer q.Close()
		matches, err := q.Run(tree)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		var names []string
		for _, m := range matches {
			for _, c := range m.Captures {
				if c.Name == "name" {
					names = append(names, c.Node.Text(src))
				}
			}
		}
		return names
	}

	first := run()
	for i := 0; i < 5; i++ {
		got := run()
		if len(got) != len(first) {
			t.Fatalf("run %d: got %v, want %v", i, got, first)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d: got %v, want %v", i, got, first)
			}
		}
	}
}

func TestGrammars(t *testing.T) {
	names := Grammars()
	want := map[string]bool{"javascript": true, "typescript": true, "python": true, "ruby": true}
	if len(names) != len(want) {
		t.Fatalf("Grammars() = %v, want %d entries", names, len(want))
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected grammar %q", n)
		}
	}
}

func TestParser_TypeScriptBothEntryPoints(t *testing.T) {
	ctx := context.Background()
	for _, entry := range []string{"tree_sitter_typescript", "tree_sitter_tsx"} {
		p, err := NewParser(ctx, "typescript", entry)
		if err != nil {
			t.Fatalf("NewParser(%s): %v", entry, err)
		}
		tree, err := p.Parse([]byte("interface Foo { x: number }\n"))
		if err != nil {
			p.Close()
			t.Fatalf("Parse(%s): %v", entry, err)
		}
		tree.Close()
		p.Close()
	}
}
