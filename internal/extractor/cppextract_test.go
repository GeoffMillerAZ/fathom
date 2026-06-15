package extractor

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// cppFixture exercises all the C++ symbol types CppExtractor emits:
//
//   - namespace_definition → KindCppNamespace
//   - class_specifier → KindCppClass
//   - struct_specifier with body → KindCppStruct
//   - enum_specifier with body → KindCppEnum
//   - function_definition (inline method in class) → KindCppFunc, qualified as "Class.method"
//   - function_definition (free function) → KindCppFunc, unqualified
const cppFixture = `
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

func TestCppExtractor_BasicSymbols(t *testing.T) {
	e := NewCppExtractor()
	nodes, err := e.Extract("svc", "src/foo.cpp", []byte(cppFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	type key struct {
		name string
		kind core.SourceKind
	}
	seen := make(map[key]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		seen[key{n.Name, n.Kind}] = n
	}

	want := []key{
		{"myns", core.KindCppNamespace},
		{"MyClass", core.KindCppClass},
		{"MyClass.method", core.KindCppFunc},
		{"MyClass.ptr_method", core.KindCppFunc},
		{"MyStruct", core.KindCppStruct},
		{"MyEnum", core.KindCppEnum},
		{"free_func", core.KindCppFunc},
	}
	for _, w := range want {
		n, ok := seen[w]
		if !ok {
			t.Errorf("missing symbol name=%q kind=%q", w.name, w.kind)
			continue
		}
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: want TierVerified, got %q", w.name, n.Tier)
		}
		if n.Service != "svc" {
			t.Errorf("symbol %q: want Service %q, got %q", w.name, "svc", n.Service)
		}
		if n.FilePath != "src/foo.cpp" {
			t.Errorf("symbol %q: want FilePath %q, got %q", w.name, "src/foo.cpp", n.FilePath)
		}
	}
}

func TestCppExtractor_MethodQualification(t *testing.T) {
	src := []byte(`
class Widget {
public:
    void render() { }
    int compute(int x) { return x; }
};
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
	for _, wantName := range []string{"Widget.render", "Widget.compute"} {
		if _, ok := byName[wantName]; !ok {
			t.Errorf("expected qualified symbol %q not found", wantName)
		}
	}
	// Unqualified form must not appear.
	for _, bad := range []string{"render", "compute"} {
		if _, ok := byName[bad]; ok {
			t.Errorf("unqualified symbol %q should not appear", bad)
		}
	}
}

func TestCppExtractor_TierVerified(t *testing.T) {
	e := NewCppExtractor()
	nodes, err := e.Extract("svc", "main.cpp", []byte(cppFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: expected TierVerified, got %q", n.Name, n.Tier)
		}
	}
}

func TestCppExtractor_Determinism(t *testing.T) {
	e := NewCppExtractor()
	a, err := e.Extract("svc", "lib.cpp", []byte(cppFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	b, err := e.Extract("svc", "lib.cpp", []byte(cppFixture))
	if err != nil {
		t.Fatalf("second Extract: %v", err)
	}
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: first run %d symbols, second %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Line != b[i].Line {
			t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
}

func TestCppExtractor_EmptySource(t *testing.T) {
	e := NewCppExtractor()
	nodes, err := e.Extract("svc", "empty.cpp", []byte{})
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty source, got %d", len(nodes))
	}
}
