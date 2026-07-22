package extractor

import (
	"sort"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func symStrings(nodes []core.SymbolNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = string(n.Kind) + ":" + n.Name
	}
	sort.Strings(out)
	return out
}

func mustContainAll(t *testing.T, got []string, want ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, g := range got {
		set[g] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing %q in %v", w, got)
		}
	}
}

func TestNewTSExtractor_UnknownLanguage(t *testing.T) {
	if _, err := NewTSExtractor("cobol"); err == nil {
		t.Fatal("expected error for unknown language")
	}
}

func TestTSExtractor_Python(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`X = 1

class Bar:
    y = 2

    def baz(self):
        return helper()

    def nested_owner(self):
        def inner():
            return 1
        return inner()

def foo(x):
    return x + 1
`)
	nodes, err := e.Extract("svc", "pkg/mod.py", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindPyVar)+":X",
		string(core.KindPyClass)+":Bar",
		string(core.KindPyFunc)+":Bar.baz",
		string(core.KindPyFunc)+":Bar.nested_owner",
		string(core.KindPyFunc)+":foo",
	)
	// y = 2 (inside the class body, not module level) must NOT appear as a
	// KindPyVar, and "inner" must NOT be qualified as "Bar.inner" — it's
	// nested inside a method, not directly inside the class.
	for _, g := range got {
		if g == string(core.KindPyVar)+":y" {
			t.Errorf("class-body assignment leaked as a module var: %v", got)
		}
		if g == string(core.KindPyFunc)+":Bar.inner" {
			t.Errorf("function nested in a method wrongly inherited the class scope: %v", got)
		}
	}
	// "inner" itself should still be found, just unqualified.
	mustContainAll(t, got, string(core.KindPyFunc)+":inner")

	// Calls: baz calls helper(); foo calls nothing (x+1 has no call).
	for _, n := range nodes {
		if n.Name == "Bar.baz" {
			if len(n.Calls) != 1 || n.Calls[0] != "helper" {
				t.Errorf("Bar.baz calls = %v, want [helper]", n.Calls)
			}
		}
	}

	// Package/service/file/tier plumbing.
	for _, n := range nodes {
		if n.Service != "svc" {
			t.Errorf("Service = %q, want svc", n.Service)
		}
		if n.FilePath != "pkg/mod.py" {
			t.Errorf("FilePath = %q, want pkg/mod.py", n.FilePath)
		}
		if n.Tier != core.TierVerified {
			t.Errorf("Tier = %v, want TierVerified", n.Tier)
		}
	}
}

func TestTSExtractor_Python_Determinism(t *testing.T) {
	e, err := NewTSExtractor("python")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte("def a():\n    pass\n\nclass B:\n    def c(self):\n        pass\n")
	var first []string
	for i := 0; i < 5; i++ {
		nodes, err := e.Extract("svc", "f.py", src)
		if err != nil {
			t.Fatalf("Extract: %v", err)
		}
		got := symStrings(nodes)
		if first == nil {
			first = got
			continue
		}
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

func TestTSExtractor_JavaScript(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`function foo(x) {
  return x + 1;
}

class Bar {
  baz() {
    return helper();
  }
}

const qux = (a) => a * 2;
`)
	nodes, err := e.Extract("svc", "src/app.js", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindJSFunc)+":foo",
		string(core.KindJSClass)+":Bar",
		string(core.KindJSMethod)+":Bar.baz",
		string(core.KindJSFunc)+":qux",
	)
	for _, n := range nodes {
		if n.Name == "Bar.baz" {
			if len(n.Calls) != 1 || n.Calls[0] != "helper" {
				t.Errorf("Bar.baz calls = %v, want [helper]", n.Calls)
			}
		}
	}
}

func TestTSExtractor_TypeScript(t *testing.T) {
	e, err := NewTSExtractor("javascript") // "javascript" spans .ts/.tsx too
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`interface Point {
  x: number;
}

type ID = string;

enum Color { Red, Green }

class Shape {
  area(): number {
    return 0;
  }
}
`)
	nodes, err := e.Extract("svc", "src/shapes.ts", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindTSInterface)+":Point",
		string(core.KindTSType)+":ID",
		string(core.KindTSEnum)+":Color",
		string(core.KindJSClass)+":Shape",
		string(core.KindJSMethod)+":Shape.area",
	)
}

func TestTSExtractor_TSX(t *testing.T) {
	e, err := NewTSExtractor("javascript")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`interface Props {
  name: string;
}

class Widget {
  render(): number {
    return 1;
  }
}
`)
	nodes, err := e.Extract("svc", "src/widget.tsx", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindTSInterface)+":Props",
		string(core.KindJSClass)+":Widget",
		string(core.KindJSMethod)+":Widget.render",
	)
}

func TestTSExtractor_Ruby(t *testing.T) {
	e, err := NewTSExtractor("ruby")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`module Greeter
  def self.hello(name)
    format(name)
  end
end

class Person
  def initialize(name)
    @name = name
  end

  def greet
    Greeter.hello(@name)
  end
end
`)
	nodes, err := e.Extract("svc", "lib/person.rb", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindRubyModule)+":Greeter",
		string(core.KindRubyMethod)+":Greeter.hello",
		string(core.KindRubyClass)+":Person",
		string(core.KindRubyMethod)+":Person.initialize",
		string(core.KindRubyMethod)+":Person.greet",
	)
	for _, n := range nodes {
		if n.Name == "Person.greet" {
			if len(n.Calls) != 1 || n.Calls[0] != "hello" {
				t.Errorf("Person.greet calls = %v, want [hello]", n.Calls)
			}
		}
	}
}

func TestTSExtractor_Ruby_NestedModuleChaining(t *testing.T) {
	e, err := NewTSExtractor("ruby")
	if err != nil {
		t.Fatalf("NewTSExtractor: %v", err)
	}
	src := []byte(`module Outer
  module Inner
    class Widget
      def run
      end
    end
  end
end
`)
	nodes, err := e.Extract("svc", "lib/nested.rb", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	got := symStrings(nodes)
	mustContainAll(
		t, got,
		string(core.KindRubyModule)+":Outer",
		string(core.KindRubyModule)+":Outer::Inner",
		string(core.KindRubyClass)+":Outer::Inner::Widget",
		string(core.KindRubyMethod)+":Outer::Inner::Widget.run",
	)
}

func TestTSExtractor_EmptyFile(t *testing.T) {
	for _, lang := range []string{"javascript", "python", "ruby"} {
		e, err := NewTSExtractor(lang)
		if err != nil {
			t.Fatalf("NewTSExtractor(%s): %v", lang, err)
		}
		nodes, err := e.Extract("svc", "empty.txt", nil)
		if err != nil {
			t.Fatalf("Extract(%s, empty): %v", lang, err)
		}
		if len(nodes) != 0 {
			t.Errorf("Extract(%s, empty) = %v, want none", lang, nodes)
		}
	}
}

func TestEngineProvenance(t *testing.T) {
	got := EngineProvenance()
	for _, ext := range []string{".py", ".js", ".jsx", ".ts", ".tsx", ".rb"} {
		if got[ext] != "tree-sitter-wasm" {
			t.Errorf("EngineProvenance()[%q] = %q, want tree-sitter-wasm", ext, got[ext])
		}
	}
}
