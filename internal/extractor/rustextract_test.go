package extractor

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
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
		if n.EndLine <= 0 {
			t.Errorf("symbol %q: EndLine not set (got %d)", n.Name, n.EndLine)
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

// rustCallFixture is a minimal Rust file for verifying call-edge extraction.
// greet() calls helper() (call_expression) and println! (macro_invocation).
// helper() makes no calls.
const rustCallFixture = `fn greet() {
    helper();
    println!("hi");
}
fn helper() {}
`

func TestRustExtractor_CallEdges(t *testing.T) {
	e := NewRustExtractor()
	nodes, err := e.Extract("svc", "src/calls.rs", []byte(rustCallFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	byName := make(map[string]core.SymbolNode, len(nodes))
	for _, n := range nodes {
		byName[n.Name] = n
	}

	t.Run("greet calls helper and println", func(t *testing.T) {
		n, ok := byName["greet"]
		if !ok {
			t.Fatal("expected symbol greet")
		}
		wantCalls := map[string]bool{"helper": true, "println": true}
		for _, c := range n.Calls {
			if !wantCalls[c] {
				t.Errorf("unexpected call %q in greet.Calls", c)
			}
			delete(wantCalls, c)
		}
		for missing := range wantCalls {
			t.Errorf("greet.Calls missing %q", missing)
		}
	})

	t.Run("greet Calls is sorted", func(t *testing.T) {
		n := byName["greet"]
		for i := 1; i < len(n.Calls); i++ {
			if n.Calls[i] < n.Calls[i-1] {
				t.Errorf("Calls not sorted at [%d]: %q > %q", i, n.Calls[i-1], n.Calls[i])
			}
		}
	})

	t.Run("helper has no calls", func(t *testing.T) {
		n := byName["helper"]
		if len(n.Calls) != 0 {
			t.Errorf("helper.Calls = %v, want empty", n.Calls)
		}
	})
}

// rustMembersFixture exercises every definition kind LB-F82 adds: named and
// tuple struct fields, union fields, enum variants (unit, tuple, struct-like)
// with the fields of a struct-like variant, and associated consts and types
// of an impl and a trait, plus a trait's required method.
const rustMembersFixture = `pub struct Config {
    pub name: String,
    verbose: bool,
}
struct Pair(u32, String);
union Bits { a: u32, b: f32 }
enum Shape {
    Empty,
    Circle(f64),
    Rect { w: u32, h: u32 },
}
trait Source {
    const LIMIT: usize;
    type Item;
    fn next(&mut self) -> Option<Self::Item>;
}
impl Source for Config {
    const LIMIT: usize = 4;
    type Item = u8;
    fn next(&mut self) -> Option<u8> { None }
}
impl Config {
    const DEFAULT: u32 = 1;
    fn new() -> Config { Config { name: String::new(), verbose: false } }
}
`

// TestRustExtractor_FieldsVariantsAssocItems (LB-F82): the CGO walker ingest
// dispatches for .rs emits fields, enum variants and associated items, each
// qualified Owner.name the way methods are.
func TestRustExtractor_FieldsVariantsAssocItems(t *testing.T) {
	nodes, err := NewRustExtractor().Extract("svc", "src/lib.rs", []byte(rustMembersFixture))
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	type key struct {
		name string
		kind core.SourceKind
	}
	seen := map[key]core.SymbolNode{}
	for _, n := range nodes {
		seen[key{n.Name, n.Kind}] = n
	}
	for _, w := range []struct {
		name string
		kind core.SourceKind
		line int
	}{
		{"Config.name", core.KindRustField, 2},
		{"Config.verbose", core.KindRustField, 3},
		{"Bits.a", core.KindRustField, 6},
		{"Bits.b", core.KindRustField, 6},
		{"Shape.Empty", core.KindRustVariant, 8},
		{"Shape.Circle", core.KindRustVariant, 9},
		{"Shape.Rect", core.KindRustVariant, 10},
		{"Rect.w", core.KindRustField, 10},
		{"Rect.h", core.KindRustField, 10},
		{"Source.LIMIT", core.KindRustAssocConst, 13},
		{"Source.Item", core.KindRustAssocType, 14},
		{"Source.next", core.KindRustFn, 15},
		{"Config.LIMIT", core.KindRustAssocConst, 18},
		{"Config.Item", core.KindRustAssocType, 19},
		{"Config.DEFAULT", core.KindRustAssocConst, 23},
	} {
		n, ok := seen[key{w.name, w.kind}]
		if !ok {
			t.Errorf("missing %s (%s)", w.name, w.kind)
			continue
		}
		if n.Line != w.line || n.Tier != core.TierVerified || n.EndLine < n.Line {
			t.Errorf("%s: line %d tier %q end %d, want line %d", w.name, n.Line, n.Tier, n.EndLine, w.line)
		}
	}
	// A tuple field has no name and rust-analyzer defines none: not emitted.
	for k := range seen {
		if k.name == "Pair.0" || k.name == "Pair.1" || k.name == "Circle.0" {
			t.Errorf("tuple field %q emitted", k.name)
		}
	}
	// A struct, enum and union stay one symbol each, and no unqualified member
	// name leaks.
	for _, bad := range []string{"name", "verbose", "Empty", "LIMIT", "Item"} {
		for k := range seen {
			if k.name == bad {
				t.Errorf("unqualified member %q emitted", bad)
			}
		}
	}
	// A struct literal's field initialisers are not definitions.
	count := 0
	for _, n := range nodes {
		if n.Name == "Config.name" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Config.name emitted %d times, want 1", count)
	}
}

// TestRustExtractor_MacroRules (O471 option B): a macro_rules! item is a RustMacro named
// by its identifier, wherever it is written (module level, inside an inline mod), so a
// `#[macro_use] mod macros;` re-export has a top-level name to carry to a `use` of it.
// The dialect emits the same kind and name.
func TestRustExtractor_MacroRules(t *testing.T) {
	const src = "#[macro_export]\nmacro_rules! arg {\n    ($x:expr) => { $x };\n}\nmod inner {\n    macro_rules! twice { ($x:expr) => { $x * 2 }; }\n}\nfn f() { arg!(1); }\n"
	d, err := NewTSExtractor("rust")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for who, e := range map[string]Extractor{"incumbent": NewRustExtractor(), "dialect": d} {
		nodes, err := e.Extract("svc", "src/macros.rs", []byte(src))
		if err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		got := map[string]int{}
		for _, n := range nodes {
			if string(n.Kind) == "RustMacro" {
				got[n.Name] = n.Line
			}
		}
		if len(got) != 2 || got["arg"] != 2 || got["twice"] != 6 {
			t.Errorf("%s: RustMacro = %v, want arg@2 twice@6", who, got)
		}
	}
}
