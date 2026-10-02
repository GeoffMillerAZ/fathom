package tsengine

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

const guardQuery = `
(class_definition
  name: (identifier) @name
  (#eq? @name "Special")
  (#requires-text! "class[ \t]+Special")) @definition.class

(function_definition name: (identifier) @name) @definition.function

(call
  function: (identifier) @name
  (#eq? @name "require")
  (#requires-text! "require")
  (#requires-text! "\\(")) @reference.call
`

// TestQueryRequiresTextGuardsPrunePatterns (LB-F180, LB-M5): a pattern with a
// (#requires-text! "re") directive runs only on a source its regexp matches.
// Over a source that cannot satisfy it the guest executes the query without
// that pattern (a variant compiled once and reused), so the guest's per-
// pattern cost is not paid; the matches, with their original pattern
// indexes, equal the unguarded query's whenever the guard is a necessary
// condition.
func TestQueryRequiresTextGuardsPrunePatterns(t *testing.T) {
	p, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	q, err := p.CompileQuery(guardQuery)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if q.PatternCount() != 3 {
		t.Fatalf("PatternCount = %d, want 3", q.PatternCount())
	}
	run := func(src string) (got, want []Match, patterns int) {
		t.Helper()
		tree, err := p.Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		defer tree.Close()
		if got, err = q.Run(tree); err != nil {
			t.Fatal(err)
		}
		patterns = q.LastRunPatterns()
		if want, err = q.RunUnguarded(tree); err != nil {
			t.Fatal(err)
		}
		return got, want, patterns
	}

	// No guard holds: only the unguarded function pattern runs.
	got, want, n := run("def foo():\n    pass\n\nclass Plain:\n    x = 1\n")
	if n != 1 {
		t.Errorf("patterns run over a source no guard admits = %d, want 1", n)
	}
	if len(got) != 1 || !reflect.DeepEqual(got, want) {
		t.Errorf("pruned matches %+v differ from unguarded %+v", got, want)
	} else if got[0].PatternIndex != 1 || got[0].Captures[0].Name != "definition.function" {
		// The variant holds one pattern, whose capture ids differ from the
		// full query's: the match keeps its original index and capture names.
		t.Errorf("pruned match = pattern %d capture %q, want pattern 1 definition.function", got[0].PatternIndex, got[0].Captures[0].Name)
	}

	// One guard holds.
	got, want, n = run("def foo():\n    pass\n\nclass Special:\n    x = 1\n")
	if n != 2 {
		t.Errorf("patterns run with the class guard satisfied = %d, want 2", n)
	}
	if len(got) != 2 || !reflect.DeepEqual(got, want) {
		t.Errorf("matches %+v differ from unguarded %+v", got, want)
	}
	if got[1].PatternIndex != 0 || got[1].Captures[0].Name != "definition.class" {
		t.Errorf("the class match reports pattern %d capture %q, want 0 definition.class", got[1].PatternIndex, got[1].Captures[0].Name)
	}

	// Every guard holds (the third pattern's two directives both need to).
	got, want, n = run("class Special:\n    pass\n\nrequire(1)\n")
	if n != 3 {
		t.Errorf("patterns run with every guard satisfied = %d, want 3", n)
	}
	if !reflect.DeepEqual(got, want) || len(got) != 2 {
		t.Errorf("matches %+v differ from unguarded %+v", got, want)
	}
	if v := VariantsForTest(q); v != 2 {
		t.Errorf("variants compiled = %d, want 2 (function only; function and class; the full query is the query itself)", v)
	}

	// A second source of the first shape reuses the variant.
	run("def bar():\n    pass\n")
	if v := VariantsForTest(q); v != 2 {
		t.Errorf("variants after reuse = %d, want 2", v)
	}
}

// TestQueryRequiresTextDirectiveErrors: a malformed guard fails compilation,
// naming it, never degrades to an unguarded pattern.
func TestQueryRequiresTextDirectiveErrors(t *testing.T) {
	p, err := NewParser(context.Background(), "python", "tree_sitter_python")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, src := range []string{
		`(function_definition name: (identifier) @name (#requires-text! @name)) @d`,
		`(function_definition name: (identifier) @name (#requires-text! "a" "b")) @d`,
		`(function_definition name: (identifier) @name (#requires-text! "(")) @d`,
	} {
		_, err := p.CompileQuery(src)
		if err == nil || !strings.Contains(err.Error(), "requires-text!") {
			t.Errorf("CompileQuery(%s) = %v, want an error naming #requires-text!", src, err)
		}
	}
}
