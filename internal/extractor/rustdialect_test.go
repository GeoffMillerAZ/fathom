package extractor

// rustdialect_test.go — LB-Q-rust (design §13.2 steps 10, 12 and 15). The
// per-fixture goldens run in the generic TestDialectGoldens (dialects/rust/
// testdata); since the CGO rustextract.go is retired (LB-F390) its own contracts
// (names, kinds, tier, an impl's methods qualified by its type, callee spelling,
// an empty file) are a golden there too (contracts.rs, empty.rs). This file
// holds what a golden reads less clearly: what the dialect adds beyond the
// walker, its members and an inline module's items.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func rustDialect(t *testing.T) Extractor {
	t.Helper()
	d, err := NewTSExtractor("rust")
	if err != nil {
		t.Fatalf("NewTSExtractor(rust): %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

type rustKey struct {
	name string
	kind core.SourceKind
}

func rustExtract(t *testing.T, e Extractor, path, src string) (map[rustKey]core.SymbolNode, []core.SymbolNode) {
	t.Helper()
	nodes, err := e.Extract("svc", path, []byte(src))
	if err != nil {
		t.Fatalf("Extract %s: %v", path, err)
	}
	m := map[rustKey]core.SymbolNode{}
	for _, n := range nodes {
		m[rustKey{n.Name, n.Kind}] = n
	}
	return m, nodes
}

// rustMembersFixture exercises every definition kind LB-F82 added: named and
// tuple struct fields, a union's fields, every variant shape, and a trait's and an
// impl's associated consts, types and functions.
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

// TestRustDialectBeyondIncumbent: what the port added over the retired walker
// (§11 step 3) — an impl named by its head type rather than the type field's source text,
// trait methods, unions, module-level aliases, consts, statics and macros,
// a Signature on every callable, and no call for a constructor.
func TestRustDialectBeyondIncumbent(t *testing.T) {
	e := rustDialect(t)
	const src = `pub const LIMIT: u32 = 4;
static NAME: &str = "n";
type Alias = Vec<u8>;
macro_rules! twice { ($x:expr) => { $x * 2 }; }
union Bits { a: u32, b: f32 }

pub trait Draw {
    fn draw(&self) -> u32;
    fn twice(&self) -> u32 { helper(self.draw()) }
}

impl<T: Clone> Draw for Wrapper<T> {
    fn draw(&self) -> u32 { Some(1).map(|x| x + 1).unwrap() }
}

fn helper(x: u32) -> u32 {
    fn nested() -> u32 { 0 }
    let v = Vec::<u8>::new();
    let _ = (Ok::<u32, ()>(1), Wrapper(x), Shape::Circle(1.0), std::mem::take(&mut x));
    nested() + core::cmp::max(x, LIMIT) + v.len() as u32
}
`
	seen, nodes := rustExtract(t, e, "src/lib.rs", src)
	for _, w := range []rustKey{
		{"LIMIT", "RustConst"}, {"NAME", "RustStatic"}, {"Alias", "RustTypeAlias"},
		{"twice", "RustMacro"}, {"Bits", "RustUnion"}, {"Draw", core.KindRustTrait},
		{"Draw.draw", core.KindRustFn}, {"Draw.twice", core.KindRustFn},
		{"Wrapper", core.KindRustImpl}, {"Wrapper.draw", core.KindRustFn}, {"nested", core.KindRustFn},
	} {
		if _, ok := seen[w]; !ok {
			t.Errorf("missing %+v", w)
		}
	}
	if _, ok := seen[rustKey{"Wrapper<T>", core.KindRustImpl}]; ok {
		t.Error("an impl is named by its head type, not the source text of its type")
	}
	for _, n := range nodes {
		if n.Kind == core.KindRustFn && n.Signature == "" {
			t.Errorf("%s: no Signature", n.Name)
		}
	}
	if c := seen[rustKey{"Draw.twice", core.KindRustFn}]; strings.Join(c.Calls, ",") != "draw,helper" {
		t.Errorf("Draw.twice.Calls = %v, want [draw helper]", c.Calls)
	}
	if c := seen[rustKey{"Wrapper.draw", core.KindRustFn}]; strings.Join(c.Calls, ",") != "map,unwrap" {
		t.Errorf("Wrapper.draw.Calls = %v, want [map unwrap] (Some(1) is a constructor)", c.Calls)
	}
	h := seen[rustKey{"helper", core.KindRustFn}]
	if got := strings.Join(h.Calls, ","); got != "core.cmp.max,len,nested,new,std.mem.take" {
		t.Errorf("helper.Calls = %v, want a path spelled with the graph's separator, a turbofish path by its last name, and no constructor", h.Calls)
	}
}

// TestRustDialectMembers (LB-F82): the dialect emits a struct's and a union's
// fields, an enum's variants and the associated items, each qualified Owner.name;
// a struct-like variant's field is bare (a variant is not a container), where the
// retired walker wrote Variant.field.
func TestRustDialectMembers(t *testing.T) {
	e := rustDialect(t)
	seen, _ := rustExtract(t, e, "src/lib.rs", rustMembersFixture)
	for _, w := range []rustKey{
		{"Config.name", core.KindRustField}, {"Config.verbose", core.KindRustField},
		{"Bits.a", core.KindRustField}, {"Bits.b", core.KindRustField},
		{"Shape.Empty", core.KindRustVariant}, {"Shape.Circle", core.KindRustVariant}, {"Shape.Rect", core.KindRustVariant},
		{"w", core.KindRustField}, {"h", core.KindRustField},
		{"Source.LIMIT", core.KindRustAssocConst}, {"Source.Item", core.KindRustAssocType}, {"Source.next", core.KindRustFn},
		{"Config.LIMIT", core.KindRustAssocConst}, {"Config.Item", core.KindRustAssocType}, {"Config.DEFAULT", core.KindRustAssocConst},
	} {
		if _, ok := seen[w]; !ok {
			t.Errorf("missing %+v", w)
		}
	}
}

// TestRustDialectModItemsOnce (O456): a const or type alias inside an inline
// `mod { }` is one RustConst or RustTypeAlias, never also an associated member:
// a mod body and an impl or trait body are both a declaration_list in the grammar.
func TestRustDialectModItemsOnce(t *testing.T) {
	e := rustDialect(t)
	_, nodes := rustExtract(t, e, "src/lib.rs", "mod tests {\n    const A: i32 = 1;\n    type R = i32;\n}\n")
	count := map[string]int{}
	for _, n := range nodes {
		switch n.Kind {
		case "RustConst", "RustTypeAlias", "RustAssocConst", "RustAssocType":
			count[string(n.Kind)]++
		}
	}
	want := map[string]int{"RustConst": 1, "RustTypeAlias": 1}
	if len(count) != len(want) || count["RustConst"] != 1 || count["RustTypeAlias"] != 1 {
		t.Errorf("kinds = %v, want exactly %v", count, want)
	}
}
