package extractor

import (
	"testing"

	"github.com/geoffmilleraz/sextant/internal/core"
)

// rustFixture is a deterministic Rust source fragment exercising all symbol
// types the RustExtractor emits: free function, struct, enum, trait, module,
// and an impl block containing a method.
const rustFixture = `fn hello() -> u32 { 42 }
struct Foo { x: i32 }
enum Bar { A, B }
trait MyTrait { fn method(&self); }
mod mymod { }
impl Foo { fn new() -> Foo { Foo { x: 0 } } }
`

func TestRustExtractor_BasicSymbols(t *testing.T) {
	e := NewRustExtractor()
	nodes, err := e.Extract("svc", "src/lib.rs", []byte(rustFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	// Build a (name, kind) set — Foo appears twice (struct + impl), so we
	// can't use a plain name→node map.
	type key struct {
		name string
		kind core.SourceKind
	}
	seen := make(map[key]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		seen[key{n.Name, n.Kind}] = n
	}

	want := []key{
		{"hello", core.KindRustFn},
		{"Foo", core.KindRustStruct},
		{"Bar", core.KindRustEnum},
		{"MyTrait", core.KindRustTrait},
		{"mymod", core.KindRustMod},
		{"Foo", core.KindRustImpl},   // impl block emits the type name
		{"Foo.new", core.KindRustFn}, // method inside impl is qualified
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
		if n.FilePath != "src/lib.rs" {
			t.Errorf("symbol %q: want FilePath %q, got %q", w.name, "src/lib.rs", n.FilePath)
		}
	}
}

func TestRustExtractor_ImplMethodQualification(t *testing.T) {
	src := []byte(`impl MyType { fn do_work(&self) {} fn helper() {} }`)
	e := NewRustExtractor()
	nodes, err := e.Extract("svc", "src/foo.rs", src)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}
	if _, ok := byName["MyType.do_work"]; !ok {
		t.Error("expected qualified symbol MyType.do_work")
	}
	if _, ok := byName["MyType.helper"]; !ok {
		t.Error("expected qualified symbol MyType.helper")
	}
	if _, ok := byName["do_work"]; ok {
		t.Error("unqualified do_work should not appear — only qualified form expected")
	}
}

func TestRustExtractor_TierVerified(t *testing.T) {
	e := NewRustExtractor()
	nodes, err := e.Extract("svc", "main.rs", []byte(rustFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, n := range nodes {
		if n.Tier != core.TierVerified {
			t.Errorf("symbol %q: expected TierVerified, got %q", n.Name, n.Tier)
		}
	}
}

func TestRustExtractor_Determinism(t *testing.T) {
	e := NewRustExtractor()
	a, err := e.Extract("svc", "lib.rs", []byte(rustFixture))
	if err != nil {
		t.Fatalf("first Extract: %v", err)
	}
	b, err := e.Extract("svc", "lib.rs", []byte(rustFixture))
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

func TestRustExtractor_EmptySource(t *testing.T) {
	e := NewRustExtractor()
	nodes, err := e.Extract("svc", "empty.rs", []byte{})
	if err != nil {
		t.Fatalf("Extract empty: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes for empty source, got %d", len(nodes))
	}
}
