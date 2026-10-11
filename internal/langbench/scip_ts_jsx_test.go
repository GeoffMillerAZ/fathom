package langbench

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// tsJSXFixture is one component file in a JSX variant (doc: app.tsx or app.jsx)
// and one TypeScript file (cast.ts), with scip-typescript's index of them: the
// producer references a component at its opening tag and again at its closing
// tag, an intrinsic tag at JSX.IntrinsicElements' property, a member tag at each
// segment, and writes nothing for a member of an untyped receiver.
func tsJSXFixture(t *testing.T, doc string) ([]byte, map[string]string, []core.SymbolNode) {
	t.Helper()
	const pre = "scip-typescript npm app 1.0.0 "
	foo := pre + "`foo.tsx`/"
	app := pre + "`" + doc + "`/"
	cast := pre + "`cast.ts`/"
	const intrinsic = "scip-typescript npm @types/react 18.3.3 `index.d.ts`/global/JSX/IntrinsicElements#div."
	src := "import { Foo, useThing, Untyped } from './foo'\n" + // 0
		"export function App(a: number) {\n" + // 1
		"  if (a < Foo) a++\n" + // 2: a comparison, never an element
		"  return (\n" + // 3
		"    <Foo.Bar title=\"x\">\n" + // 4: a member tag: Bar is the call, Foo a read
		"      <Foo />\n" + // 5: a self-closing element, counted once
		"      <div>{useThing<Props>()}</div>\n" + // 6: an intrinsic element; type arguments
		"      <>{a}</>\n" + // 7: a fragment names nothing
		"      <Untyped.Thing />\n" + // 8: no occurrence at Thing (an untyped receiver)
		"    </Foo.Bar>\n" + // 9: the closing tag is the same element
		"  )\n" + // 10
		"}\n" // 11
	castSrc := "export function cast(y: unknown) {\n" + // 0
		"  return <Foo>y\n" + // 1: a type assertion in .ts, no element
		"}\n" // 2
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript", Version: "0.4.0"}},
		Documents: []*scippb.Document{
			{RelativePath: doc, Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(app+"App().", []int32{1, 0, 11, 1}, 1, 16, 19),
				scipCallsiteRef(foo+"Foo.", 2, 10, 13),
				scipCallsiteRef(foo+"Foo.", 4, 5, 8),
				scipCallsiteRef(foo+"Bar0:", 4, 9, 12),
				scipCallsiteRef(foo+"Foo.", 5, 7, 10),
				scipCallsiteRef(intrinsic, 6, 7, 10),
				scipCallsiteRef(foo+"useThing().", 6, 12, 20),
				scipCallsiteRef(foo+"Props#", 6, 21, 26),
				scipCallsiteRef(intrinsic, 6, 32, 35),
				scipCallsiteRef(app+"App().(a)", 7, 9, 10),
				scipCallsiteRef(foo+"Untyped.", 8, 7, 14),
				scipCallsiteRef(foo+"Foo.", 9, 6, 9),
				scipCallsiteRef(foo+"Bar0:", 9, 10, 13),
			}},
			{RelativePath: "cast.ts", Occurrences: []*scippb.Occurrence{
				scipCallsiteDef(cast+"cast().", []int32{0, 0, 2, 1}, 0, 16, 20),
				scipCallsiteRef(foo+"Foo.", 1, 10, 13),
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "App", Kind: "JSComponent", FilePath: doc, Line: 2, EndLine: 12,
			Calls: []string{"Foo.Bar", "Foo", "useThing", "Untyped.Thing"}},
		{Service: "s", Name: "cast", Kind: "JSFunc", FilePath: "cast.ts", Line: 1, EndLine: 3},
	}
	return raw, map[string]string{doc: src, "cast.ts": castSrc}, nodes
}

// TestScoreSCIPJSXElementCalls (owner ruling M2, O860, K19): a JSX element is a
// call. `<Foo/>` compiles to jsx(Foo, props) (React.createElement under the
// classic runtime) and the renderer invokes Foo, so a reference at an opening
// or self-closing element's tag that names a component (TypeScript's rule: an
// identifier tag not starting a to z, or any member tag) is a call pair to the
// name the tag is written with, its last segment for a member tag (`<Foo.Bar>`
// calls Bar, as the dialects record a member tag, LB-F377); counted and, in
// the oracle-blind rule, a written call site. A closing tag is the same element
// (never a second pair), an intrinsic tag (`<div>`) is a string, a fragment
// names nothing, type arguments and a comparison are no element, and a .ts
// document has no JSX (`<Foo>y` is a type assertion there).
func TestScoreSCIPJSXElementCalls(t *testing.T) {
	for dialect, doc := range map[string]string{"typescript": "app.tsx", "javascript": "app.jsx"} {
		raw, srcs, nodes := tsJSXFixture(t, doc)
		opts := SCIPOptionsFor(dialect, func(p string) []byte { return []byte(srcs[p]) })
		if !opts.JSXElementCalls {
			t.Fatalf("%s: SCIP options do not read a JSX element as a call", dialect)
		}
		res, err := ScoreSCIPWith(raw, "s", nodes, opts)
		if err != nil {
			t.Fatal(err)
		}
		want := map[CallPair]bool{
			{doc, "App", "Bar"}: true, {doc, "App", "Foo"}: true, {doc, "App", "useThing"}: true,
		}
		if !reflect.DeepEqual(res.CallTruth, want) {
			t.Errorf("%s: call truth = %v, want %v", dialect, res.CallTruth, want)
		}
		if n := res.Counts[CntScipJSXElementCalls]; n != 2 {
			t.Errorf("%s: %s = %d, want 2 (<Foo.Bar> and <Foo />; not </Foo.Bar>, <div>, <>, Props or a < Foo)", dialect, CntScipJSXElementCalls, n)
		}
		if res.CallFound[CallPair{doc, "App", "Thing"}] || res.Counts[CntCallsOracleBlind] != 1 {
			t.Errorf("%s: (App, Thing): its one element site has no occurrence, so it is oracle-blind (found %v, blind %d)",
				dialect, res.CallFound[CallPair{doc, "App", "Thing"}], res.Counts[CntCallsOracleBlind])
		}
		if res.Counts[CntCallFathom] != 3 || res.Counts[CntCallFathomInTrue] != 3 || res.Counts[CntCallTruthFound] != 3 {
			t.Errorf("%s: L2 counts = %v, want 3 of 3 both ways", dialect, res.Counts)
		}
		if note := tsJSXElementNote("subj", res.Counts); note == "" {
			t.Errorf("%s: no disclosure of the JSX element calls", dialect)
		}
	}
	if note := tsJSXElementNote("subj", map[string]int{}); note != "" {
		t.Errorf("a subject the rule moved nothing for is disclosed: %q", note)
	}
}

// TestTSJSXElementAt: the element's tag, read from what is written at the
// reference's range.
func TestTSJSXElementAt(t *testing.T) {
	for _, c := range []struct {
		line, at string
		want     string
	}{
		{"<Foo/>", "Foo", "Foo"},
		{"  <Foo />", "Foo", "Foo"},
		{"<Foo.Bar.Baz x={1}>", "Baz", "Baz"},
		{"<Foo.Bar>", "Foo", ""}, // a member tag's receiver is a read
		{"<foo.bar>", "bar", "bar"},
		{"</Foo>", "Foo", ""},
		{"<div>", "div", ""},
		{"<my-element>", "my", ""},
		{"<svg:rect>", "rect", ""},
		{"return <Foo", "Foo", "Foo"},
		{"x = <Select<Opt> value={v} />", "Select", "Select"},
		{"x = <Select<Opt> value={v} />", "Opt", ""},
		{"{cond && <Foo {...p}/>}", "Foo", "Foo"},
		{"Click <Link>here</Link>", "Link", "Link"},
		{"const [s] = useState<Foo>(x)", "Foo", ""},
		{"if (i < Max) i++", "Max", ""},
		{"for (; i < Max; i++)", "Max", ""},
		{"a<Foo>", "Foo", ""},
		{"x < Foo.length", "length", ""},
		{"let ok = i < max", "max", ""},
		{"  <Foo", "Foo", "Foo"},
		{"x = <Foo", "Foo", "Foo"},
		{"(a) < Foo", "Foo", ""},
	} {
		i := indexWord(c.line, c.at)
		if i < 0 {
			t.Fatalf("%q not in %q", c.at, c.line)
		}
		got, ok := tsJSXElementAt([][]byte{[]byte(c.line)}, [4]int32{0, int32(i), 0, int32(i + len(c.at))})
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%q at %q: got %q, %v; want %q", c.line, c.at, got, ok, c.want)
		}
	}
	for doc, want := range map[string]bool{"a.tsx": true, "a.jsx": true, "a.js": true, "a.mjs": true, "a.cjs": true, "a.ts": false, "a.d.ts": false, "a.mts": false, "a.cts": false} {
		if tsJSXVariant(doc) != want {
			t.Errorf("tsJSXVariant(%q) = %v, want %v", doc, !want, want)
		}
	}
}

// indexWord is the byte offset of the first whole-word occurrence of w in s.
func indexWord(s, w string) int {
	for i := 0; i+len(w) <= len(s); i++ {
		if s[i:i+len(w)] != w {
			continue
		}
		if (i > 0 && isTSIdentByte(s[i-1])) || (i+len(w) < len(s) && isTSIdentByte(s[i+len(w)])) {
			continue
		}
		return i
	}
	return -1
}
