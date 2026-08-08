package extractor

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// TestCppExtractor_ScopeNormalization pins CT-1 (docs/plans/cpp-toolchain.md):
// one spelling per logical symbol. Out-of-class definitions arrive from
// tree-sitter as qualified_identifiers ("Widget::render") and collected
// callees keep whatever the call site spelled ("ns::util") — both must
// normalize "::" to "." so the call graph's byName/byPkgName maps join them
// with inline-method symbols, which are already dot-qualified.
func TestCppExtractor_ScopeNormalization(t *testing.T) {
	src := []byte(`
namespace ns {
  void util() { }
}
class Widget {
public:
  void render();
};
void Widget::render() {
  ns::util();
}
`)
	e := NewCppExtractor()
	nodes, err := e.Extract("svc", "widget.cpp", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("out-of-class definition is dot-qualified", func(t *testing.T) {
		if _, ok := byName["Widget.render"]; !ok {
			t.Errorf("expected symbol %q, have %v", "Widget.render", names(nodes))
		}
		if _, ok := byName["Widget::render"]; ok {
			t.Error("raw :: spelling must not survive extraction")
		}
	})

	t.Run("no symbol name contains ::", func(t *testing.T) {
		for _, n := range nodes {
			if strings.Contains(n.Name, "::") {
				t.Errorf("symbol %q contains ::", n.Name)
			}
		}
	})

	t.Run("collected callees are normalized", func(t *testing.T) {
		n, ok := byName["Widget.render"]
		if !ok {
			t.Fatal("Widget.render missing")
		}
		var found bool
		for _, c := range n.Calls {
			if strings.Contains(c, "::") {
				t.Errorf("callee %q contains ::", c)
			}
			if c == "ns.util" {
				found = true
			}
		}
		if !found {
			t.Errorf("Widget.render.Calls = %v, want to contain %q", n.Calls, "ns.util")
		}
	})
}

// TestCppExtractor_TemplatesAndOperators pins CT-2: template declarations
// (class and function), operator overloads, destructors, and C++17 nested
// namespace definitions all extract.
func TestCppExtractor_TemplatesAndOperators(t *testing.T) {
	src := []byte(`
template <typename T>
class Box {
public:
  T get() { return v; }
  T v;
};
template <typename T>
T tmax(T a, T b) { return a > b ? a : b; }
class Str {
public:
  Str operator+(const Str& o) { return o; }
  ~Str() { }
};
namespace a::b {
  void deep() { }
}
`)
	e := NewCppExtractor()
	nodes, err := e.Extract("svc", "tmpl.hpp", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	type key struct {
		name string
		kind core.SourceKind
	}
	seen := make(map[key]bool, len(nodes))
	for _, n := range nodes {
		seen[key{n.Name, n.Kind}] = true
	}

	want := []key{
		{"Box", core.KindCppClass},
		{"Box.get", core.KindCppFunc},
		{"tmax", core.KindCppFunc},
		{"Str.operator+", core.KindCppFunc},
		{"Str.~Str", core.KindCppFunc},
		{"a.b", core.KindCppNamespace},
		{"deep", core.KindCppFunc},
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("missing symbol name=%q kind=%q; have %v", w.name, w.kind, names(nodes))
		}
	}
}

// names is a test helper listing extracted symbol names for failure messages.
func names(nodes []core.SymbolNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}
