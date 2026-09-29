package extractor

// rustdialect_test.go — LB-Q-rust (design §13.2 steps 10, 12 and 15). The
// per-fixture goldens run in the generic TestDialectGoldens (dialects/rust/
// testdata); this file holds a port's extra obligation: the incumbent's own
// contracts (rustextract_test.go) must hold for the dialect too, run here
// through both extractors so a divergence names which side moved.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func rustBothExtractors(t *testing.T) map[string]Extractor {
	t.Helper()
	d, err := NewTSExtractor("rust")
	if err != nil {
		t.Fatalf("NewTSExtractor(rust): %v", err)
	}
	t.Cleanup(d.Close)
	return map[string]Extractor{"incumbent": NewRustExtractor(), "dialect": d}
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

// TestRustDialectKeepsIncumbentContracts: names, kinds, tier, qualification
// and callee spelling the CGO walker's own tests pin (KindRust* strings are
// byte-identical, §11 step 3).
func TestRustDialectKeepsIncumbentContracts(t *testing.T) {
	for who, e := range rustBothExtractors(t) {
		t.Run(who+"/symbols", func(t *testing.T) {
			seen, nodes := rustExtract(t, e, "src/lib.rs", rustFixture)
			for _, w := range []rustKey{
				{"hello", core.KindRustFn}, {"Foo", core.KindRustStruct}, {"Bar", core.KindRustEnum},
				{"MyTrait", core.KindRustTrait}, {"mymod", core.KindRustMod},
				{"Foo", core.KindRustImpl}, {"Foo.new", core.KindRustFn},
			} {
				n, ok := seen[w]
				if !ok {
					t.Errorf("missing %+v", w)
					continue
				}
				if n.Tier != core.TierVerified || n.Service != "svc" || n.FilePath != "src/lib.rs" || n.EndLine <= 0 {
					t.Errorf("%+v: tier %q service %q file %q end %d", w, n.Tier, n.Service, n.FilePath, n.EndLine)
				}
			}
			for i := 1; i < len(nodes); i++ {
				if nodes[i].Line < nodes[i-1].Line {
					t.Errorf("not sorted by line at %d: %d after %d", i, nodes[i].Line, nodes[i-1].Line)
				}
			}
		})
		t.Run(who+"/impl method qualification", func(t *testing.T) {
			seen, _ := rustExtract(t, e, "src/foo.rs", `impl MyType { fn do_work(&self) {} fn helper() {} }`)
			for _, n := range []string{"MyType.do_work", "MyType.helper"} {
				if _, ok := seen[rustKey{n, core.KindRustFn}]; !ok {
					t.Errorf("missing qualified %s", n)
				}
			}
			for _, n := range []string{"do_work", "helper"} {
				if _, ok := seen[rustKey{n, core.KindRustFn}]; ok {
					t.Errorf("unqualified %s appeared", n)
				}
			}
		})
		t.Run(who+"/empty and determinism", func(t *testing.T) {
			if _, nodes := rustExtract(t, e, "empty.rs", ""); len(nodes) != 0 {
				t.Errorf("empty source: %d nodes", len(nodes))
			}
			_, a := rustExtract(t, e, "lib.rs", rustFixture)
			_, b := rustExtract(t, e, "lib.rs", rustFixture)
			if len(a) != len(b) {
				t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
			}
			for i := range a {
				if a[i].Name != b[i].Name || a[i].Kind != b[i].Kind || a[i].Line != b[i].Line {
					t.Errorf("index %d differs: %+v vs %+v", i, a[i], b[i])
				}
			}
		})
		t.Run(who+"/call edges", func(t *testing.T) {
			seen, _ := rustExtract(t, e, "src/calls.rs", rustCallFixture)
			if g := seen[rustKey{"greet", core.KindRustFn}]; strings.Join(g.Calls, ",") != "helper,println" {
				t.Errorf("greet.Calls = %v, want [helper println]", g.Calls)
			}
			if h := seen[rustKey{"helper", core.KindRustFn}]; len(h.Calls) != 0 {
				t.Errorf("helper.Calls = %v, want none", h.Calls)
			}
		})
	}
}

// TestRustDialectBeyondIncumbent: what a port may add (§11 step 3) — an
// impl named by its head type rather than the type field's source text,
// trait methods, unions, module-level aliases, consts, statics and macros,
// a Signature on every callable, and no call for a constructor.
func TestRustDialectBeyondIncumbent(t *testing.T) {
	e, err := NewTSExtractor("rust")
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
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
