package extractor

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// testTSExtractor builds a TSExtractor over one ad-hoc dialect: grammar
// blob/entry, a query, and a capture-kind table (engine tests that need a
// query shape no embedded dialect has yet).
func testTSExtractor(blob, entry, query string, kinds map[string]tsDefKind, mutate func(*tsDialectRules)) *TSExtractor {
	rules := &tsDialectRules{id: "test", kinds: kinds, methodSep: ".", containerSep: ".", packageStrategy: "dir", sigMax: 200, queryFile: "test.scm", query: query}
	if mutate != nil {
		mutate(rules)
	}
	v := tsVariant{blob: blob, entryPoint: entry, rules: rules}
	return newTSExtractorFromSpec(&tsLangSpec{name: "test", variantForExt: map[string]tsVariant{}, defaultVariant: v})
}

// TestTSReferenceNameCapture (LB-F4, design §3.4): a @name inside
// @reference.call is the callee; without one the call expression's text
// stands (today's behaviour); @reference.class/implementation/type are
// counted, not stored.
func TestTSReferenceNameCapture(t *testing.T) {
	query := `
(function_definition name: (identifier) @name body: (_) @body) @definition.function
(call function: (identifier) @name) @reference.call
(call function: (attribute attribute: (identifier) @name)) @reference.call
(call function: (subscript)) @reference.call
(class_definition superclasses: (argument_list (identifier) @name @reference.class))
(type (identifier) @name) @reference.type
`
	e := testTSExtractor("python", "tree_sitter_python", query,
		map[string]tsDefKind{"function": {Kind: core.KindPyFunc, Callable: true}}, nil)
	defer e.Close()
	src := `def f(a: int):
    foo()
    obj.method(1)
    handlers[0](a)

class A(Base, Mixin):
    pass
`
	nodes, err := e.Extract("svc", "m.py", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Name != "f" {
		t.Fatalf("nodes = %v", symStrings(nodes))
	}
	if got, want := strings.Join(nodes[0].Calls, ","), "foo,handlers[0](a),method"; got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
	if got := e.TypeRefs(); got != 3 {
		t.Errorf("TypeRefs() = %d, want 3 (Base, Mixin, int)", got)
	}
}
