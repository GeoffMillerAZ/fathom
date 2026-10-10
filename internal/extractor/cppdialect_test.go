package extractor

// cppdialect_test.go — LB-Q-cpp (design §13.2 steps 10, 12 and 15). The
// per-fixture goldens run in the generic TestDialectGoldens (dialects/cpp/
// testdata); this file holds a port's extra obligation: the CT-1/CT-2
// contracts the retired CGO cppextract.go's own tests pinned hold for the
// dialect, written out here since LB-F393 deleted the walker (its MERGE
// against the dialect is docs/bench/readings/lang/parity/cpp.json).

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cppDialect is the C++ tags dialect.
func cppDialect(t *testing.T) *TSExtractor {
	t.Helper()
	d, err := NewTSExtractor("cpp")
	if err != nil {
		t.Fatalf("NewTSExtractor(cpp): %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

// cppQ is a symbol's name as the dialect spells it: it makes a namespace a
// container, so a definition inside "namespace ns" carries "ns." (symbol keys
// stay unique when two namespaces of one file define the same name, and an
// inline member, a member defined in the namespace block and one defined as
// ns::C::m() share one key); the retired walker never pushed a namespace onto
// its scope stack, and kinds and every other name are as it wrote them.
func cppQ(ns, name string) string {
	if ns != "" {
		return ns + "." + name
	}
	return name
}

// names is a test helper listing extracted symbol names for failure messages.
func names(nodes []core.SymbolNode) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}

type cppKey struct {
	name string
	kind core.SourceKind
}

func cppSeen(t *testing.T, e Extractor, path, src string) (map[cppKey]core.SymbolNode, []core.SymbolNode) {
	t.Helper()
	nodes, err := e.Extract("svc", path, []byte(src))
	if err != nil {
		t.Fatalf("Extract %s: %v", path, err)
	}
	m := map[cppKey]core.SymbolNode{}
	for _, n := range nodes {
		m[cppKey{n.Name, n.Kind}] = n
	}
	return m, nodes
}

// TestCppDialectKeepsIncumbentContracts: names, kinds, tier, qualification
// and callee spelling the CGO walker's own tests pinned.
func TestCppDialectKeepsIncumbentContracts(t *testing.T) {
	const basic = `
namespace myns {
  class MyClass {
  public:
    void method() { }
    int *ptr_method(char *s) { return 0; }
  };
  struct MyStruct { int x; };
  enum MyEnum { A, B };
}
void free_func(int x) { }
`
	const scope = `
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
`
	const tmpl = `
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
`
	const calls = `
void callee(int x) { }
void helper(void) { }
void caller(void) {
    callee(1);
    helper();
}
class Widget {
public:
    void render() {
        callee(0);
    }
};
`
	e := cppDialect(t)
	seen, nodes := cppSeen(t, e, "src/foo.cpp", basic)
	for _, w := range []cppKey{
		{"myns", core.KindCppNamespace}, {cppQ("myns", "MyClass"), core.KindCppClass},
		{cppQ("myns", "MyClass.method"), core.KindCppFunc}, {cppQ("myns", "MyClass.ptr_method"), core.KindCppFunc},
		{cppQ("myns", "MyStruct"), core.KindCppStruct}, {cppQ("myns", "MyEnum"), core.KindCppEnum},
		{"free_func", core.KindCppFunc},
	} {
		n, ok := seen[w]
		if !ok {
			t.Errorf("basic: missing %q %q; have %v", w.name, w.kind, names(nodes))
			continue
		}
		if n.Tier != core.TierVerified || n.Service != "svc" || n.FilePath != "src/foo.cpp" || n.EndLine <= 0 {
			t.Errorf("basic: %q plumbing wrong: tier %q service %q file %q end %d", w.name, n.Tier, n.Service, n.FilePath, n.EndLine)
		}
	}
	for _, bad := range []string{"method", "ptr_method"} {
		for _, n := range nodes {
			if n.Name == bad {
				t.Errorf("basic: unqualified method %q emitted", bad)
			}
		}
	}

	// CT-1: one spelling per logical symbol.
	seen, nodes = cppSeen(t, e, "widget.cpp", scope)
	if _, ok := seen[cppKey{"Widget.render", core.KindCppFunc}]; !ok {
		t.Errorf("scope: missing Widget.render; have %v", names(nodes))
	}
	for _, n := range nodes {
		if strings.Contains(n.Name, "::") {
			t.Errorf("scope: symbol %q contains ::", n.Name)
		}
		for _, c := range n.Calls {
			if strings.Contains(c, "::") {
				t.Errorf("scope: callee %q of %q contains ::", c, n.Name)
			}
		}
	}
	if n := seen[cppKey{"Widget.render", core.KindCppFunc}]; !containsString(n.Calls, "ns.util") {
		t.Errorf("scope: Widget.render.Calls = %v, want ns.util", n.Calls)
	}

	// CT-2: templates, operators, destructors, nested namespaces.
	seen, nodes = cppSeen(t, e, "tmpl.hpp", tmpl)
	for _, w := range []cppKey{
		{"Box", core.KindCppClass}, {"Box.get", core.KindCppFunc}, {"tmax", core.KindCppFunc},
		{"Str.operator+", core.KindCppFunc}, {"Str.~Str", core.KindCppFunc},
		{"a.b", core.KindCppNamespace}, {cppQ("a.b", "deep"), core.KindCppFunc},
	} {
		if _, ok := seen[w]; !ok {
			t.Errorf("templates: missing %q %q; have %v", w.name, w.kind, names(nodes))
		}
	}

	// Call collection: sorted, exact.
	seen, _ = cppSeen(t, e, "calls.cpp", calls)
	if got := seen[cppKey{"caller", core.KindCppFunc}].Calls; strings.Join(got, ",") != "callee,helper" {
		t.Errorf("calls: caller.Calls = %v, want [callee helper]", got)
	}
	if got := seen[cppKey{"Widget.render", core.KindCppFunc}].Calls; strings.Join(got, ",") != "callee" {
		t.Errorf("calls: Widget.render.Calls = %v, want [callee]", got)
	}
	if got := seen[cppKey{"callee", core.KindCppFunc}].Calls; len(got) != 0 {
		t.Errorf("calls: callee.Calls = %v, want none", got)
	}

	// LB-F434: an empty file is still a module (K18), and nothing else.
	if nodes, err := e.Extract("svc", "empty.cpp", nil); err != nil || len(nodes) != 1 || !nodes[0].IsModuleSymbol() {
		t.Errorf("empty source: %d nodes (%v), err %v; want its module symbol alone", len(nodes), nodes, err)
	}
}

// TestCppDialectDeclaredBeyondIncumbent: what the dialect adds beyond the
// retired CGO walker (a port may add kinds, never drop one, §11 step 3) and
// where it is deliberately stricter.
func TestCppDialectDeclaredBeyondIncumbent(t *testing.T) {
	d, err := NewTSExtractor("cpp")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	const src = `
#define LIMIT 4
union Cell { int i; float f; };
typedef unsigned long Id;
using Ptr = int *;
class Fwd;
struct Pending;
class Shape { public: Shape() = default; int sides() const; };
template <typename T> class Box { public: T &get(); };
template <typename T> T &Box<T>::get() { return v_; }
int Shape::sides() const { return 4; }
`
	seen, nodes := cppSeen(t, d, "src/x.cpp", src)
	for _, w := range []cppKey{
		{"LIMIT", "CppMacro"}, {"Cell", "CppUnion"}, {"Id", "CppTypedef"}, {"Ptr", "CppTypedef"},
		{"Box", core.KindCppClass}, {"Box.get", core.KindCppFunc}, {"Shape.sides", core.KindCppFunc},
	} {
		if _, ok := seen[w]; !ok {
			t.Errorf("missing %q %q; have %v", w.name, w.kind, names(nodes))
		}
	}
	// Forward declarations, prototypes and `= default` are not definitions.
	for _, n := range nodes {
		switch n.Name {
		case "Fwd", "Pending", "Shape.Shape":
			t.Errorf("%q must not be a definition", n.Name)
		}
	}
	if n := seen[cppKey{"Box.get", core.KindCppFunc}]; n.Signature != "T &Box<T>::get()" {
		t.Errorf("Box.get signature = %q (the source spelling is kept, only the symbol key is normalized)", n.Signature)
	}
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
