package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// cFixture exercises the four C symbol types the CExtractor emits:
//
//   - function_definition → KindCFunc (name via cDeclName through the declarator chain)
//   - struct_specifier with body → KindCStruct
//   - enum_specifier with body → KindCEnum
//   - type_definition → KindCTypedef (alias name via declarator field)
//
// A bare function prototype (declaration without body) is included to verify
// it is NOT emitted.
const cFixture = `
void do_work(int x) { }
int *ptr_func(char *s) { return 0; }
struct Point { int x; int y; };
enum Direction { NORTH, SOUTH, EAST, WEST };
typedef struct Point Point_t;
typedef int MyInt;
int add(int a, int b);
`

func TestCExtractor_BasicSymbols(t *testing.T) {
	e := NewCExtractor()
	nodes, err := e.Extract("svc", "src/foo.c", []byte(cFixture))
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
		{"do_work", core.KindCFunc},
		{"ptr_func", core.KindCFunc},
		{"Point", core.KindCStruct},
		{"Direction", core.KindCEnum},
		{"Point_t", core.KindCTypedef},
		{"MyInt", core.KindCTypedef},
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
		if n.FilePath != "src/foo.c" {
			t.Errorf("symbol %q: want FilePath %q, got %q", w.name, "src/foo.c", n.FilePath)
		}
	}

	// Bare prototype "int add(int a, int b);" must NOT be emitted.
	if _, ok := seen[key{"add", core.KindCFunc}]; ok {
		t.Error("prototype-only declaration 'add' should not be emitted")
	}
}

func TestCExtractor_DeclaratorNesting(t *testing.T) {
	// Verify that cDeclName correctly resolves names through declarator
	// chains: pointer-return function, plain function, typedef alias.
	src := []byte(`
void plain(void) { }
int *ptr_ret(char *s) { return 0; }
typedef void (*Callback)(int);
`)
	e := NewCExtractor()
	nodes, err := e.Extract("svc", "test.c", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	for _, wantName := range []string{"plain", "ptr_ret", "Callback"} {
		if _, ok := byName[wantName]; !ok {
			t.Errorf("expected symbol %q not found in output", wantName)
		}
	}
}

func TestCExtractor_TierVerified(t *testing.T) {
	e := NewCExtractor()
	nodes, err := e.Extract("svc", "main.c", []byte(cFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: expected TierVerified, got %q", n.Name, n.Tier)
		}
	}
}

func TestCExtractor_Determinism(t *testing.T) {
	e := NewCExtractor()
	a, err := e.Extract("svc", "lib.c", []byte(cFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	b, err := e.Extract("svc", "lib.c", []byte(cFixture))
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

func TestCExtractor_EmptySource(t *testing.T) {
	e := NewCExtractor()
	nodes, err := e.Extract("svc", "empty.c", []byte{})
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty source, got %d", len(nodes))
	}
}

// cCallFixture has caller() calling callee() and helper(), while callee() and
// helper() themselves make no calls.
const cCallFixture = `
void callee(int x) { }
void helper(void) { }
void caller(void) {
    callee(1);
    helper();
}
`

func TestCExtractor_CallEdges(t *testing.T) {
	e := NewCExtractor()
	nodes, err := e.Extract("svc", "calls.c", []byte(cCallFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("caller calls callee and helper", func(t *testing.T) {
		n, ok := byName["caller"]
		if !ok {
			t.Fatal("expected symbol caller")
		}
		wantCalls := map[string]bool{"callee": true, "helper": true}
		for _, c := range n.Calls {
			if !wantCalls[c] {
				t.Errorf("unexpected call %q in caller.Calls", c)
			}
			delete(wantCalls, c)
		}
		for missing := range wantCalls {
			t.Errorf("caller.Calls missing %q", missing)
		}
	})

	t.Run("caller Calls is sorted", func(t *testing.T) {
		n := byName["caller"]
		for i := 1; i < len(n.Calls); i++ {
			if n.Calls[i] < n.Calls[i-1] {
				t.Errorf("Calls not sorted at [%d]: %q > %q", i, n.Calls[i-1], n.Calls[i])
			}
		}
	})

	t.Run("callee has no calls", func(t *testing.T) {
		n := byName["callee"]
		if len(n.Calls) != 0 {
			t.Errorf("callee.Calls = %v, want empty", n.Calls)
		}
	})

	t.Run("helper has no calls", func(t *testing.T) {
		n := byName["helper"]
		if len(n.Calls) != 0 {
			t.Errorf("helper.Calls = %v, want empty", n.Calls)
		}
	})
}
