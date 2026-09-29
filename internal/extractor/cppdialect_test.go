package extractor

// cppdialect_test.go — LB-Q-cpp (design §13.2 steps 10, 12 and 15). The
// per-fixture goldens run in the generic TestDialectGoldens (dialects/cpp/
// testdata); this file holds a port's extra obligation: the incumbent's own
// CT-1/CT-2 contracts (cppextract_test.go, cppextract_scope_test.go) must
// hold for the dialect too, run here through both extractors so a divergence
// names which side moved.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cppBothExtractors is the incumbent CGO walker and the tags dialect.
func cppBothExtractors(t *testing.T) map[string]Extractor {
	t.Helper()
	d, err := NewTSExtractor("cpp")
	if err != nil {
		t.Fatalf("NewTSExtractor(cpp): %v", err)
	}
	t.Cleanup(d.Close)
	return map[string]Extractor{"incumbent": NewCppExtractor(), "dialect": d}
}

// cppQ is a symbol's name as each side spells it: the dialect makes a
// namespace a container, so a definition inside "namespace ns" carries "ns."
// (symbol keys stay unique when two namespaces of one file define the same
// name, and an inline member, a member defined in the namespace block and one
// defined as ns::C::m() share one key); the incumbent walker never pushed a
// namespace onto its scope stack. Kinds and every other name are identical.
func cppQ(who, ns, name string) string {
	if who == "dialect" && ns != "" {
		return ns + "." + name
	}
	return name
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
// and callee spelling the CGO walker's own tests pin.
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
	for who, e := range cppBothExtractors(t) {
		t.Run(who, func(t *testing.T) {
			seen, nodes := cppSeen(t, e, "src/foo.cpp", basic)
			for _, w := range []cppKey{
				{"myns", core.KindCppNamespace}, {cppQ(who, "myns", "MyClass"), core.KindCppClass},
				{cppQ(who, "myns", "MyClass.method"), core.KindCppFunc}, {cppQ(who, "myns", "MyClass.ptr_method"), core.KindCppFunc},
				{cppQ(who, "myns", "MyStruct"), core.KindCppStruct}, {cppQ(who, "myns", "MyEnum"), core.KindCppEnum},
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
				{"a.b", core.KindCppNamespace}, {cppQ(who, "a.b", "deep"), core.KindCppFunc},
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

			if nodes, err := e.Extract("svc", "empty.cpp", nil); err != nil || len(nodes) != 0 {
				t.Errorf("empty source: %d nodes, err %v", len(nodes), err)
			}
		})
	}
}

// TestCppDialectDeclaredBeyondIncumbent: what the dialect adds beyond the
// CGO walker (a port may add kinds, never drop one, §11 step 3) and where it
// is deliberately stricter.
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
