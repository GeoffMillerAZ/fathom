package langbench

import (
	"reflect"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// tsCallSiteFixture is one TypeScript file and scip-typescript's index of it,
// in the producer's own vocabulary: a constructor is `<constructor>`, a getter
// `<get>size`, an exported arrow const a term, a parameter `run().(cb)`, and
// columns are UTF-16 code units (line 12 holds "é", two bytes, one unit).
func tsCallSiteFixture(t *testing.T) ([]byte, string, []core.SymbolNode) {
	t.Helper()
	const pre = "scip-typescript npm app 1.0.0 `m.ts`/"
	src := "class Foo {\n" + // 0
		"  constructor(x: number) { this.bar(); }\n" + // 1
		"  get size() { return helper(); }\n" + // 2
		"  bar() { return 1; }\n" + // 3
		"}\n" + // 4
		"export const helper = () => 1;\n" + // 5
		"function run(cb: () => void, list: number[]) {\n" + // 6
		"  const f = new Foo(1);\n" + // 7
		"  list.map(helper);\n" + // 8
		"  cb();\n" + // 9
		"  f.bar!();\n" + // 10
		"  f.size;\n" + // 11
		"  log(\"é\", go());\n" + // 12
		"}\n" // 13
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "m.ts", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"Foo#", []int32{0, 0, 4, 1}, 0, 6, 9),
			scipCallsiteDef(pre+"Foo#`<constructor>`().", []int32{1, 2, 1, 40}, 1, 2, 13),
			scipCallsiteDef(pre+"Foo#`<get>size`().", []int32{2, 2, 2, 33}, 2, 6, 10),
			scipCallsiteDef(pre+"Foo#bar().", []int32{3, 2, 3, 21}, 3, 2, 5),
			scipCallsiteDef(pre+"helper.", nil, 5, 13, 19),
			scipCallsiteDef(pre+"run().", []int32{6, 0, 13, 1}, 6, 9, 12),
			scipCallsiteDef(pre+"run().(cb)", nil, 6, 13, 15),
			scipCallsiteRef(pre+"Foo#bar().", 1, 32, 35),             // this.bar(): a call
			scipCallsiteRef(pre+"helper.", 2, 22, 28),                // an arrow const called: a term written as a call
			scipCallsiteRef(pre+"Foo#`<constructor>`().", 7, 16, 19), // new Foo(1): a call to Foo
			scipCallsiteRef("scip-typescript npm typescript 5.9.3 lib/`lib.es5.d.ts`/Array#map().", 8, 7, 10),
			scipCallsiteRef(pre+"helper.", 8, 11, 17),           // passed as a value: no call
			scipCallsiteRef(pre+"run().(cb)", 9, 2, 4),          // a parameter called
			scipCallsiteRef(pre+"Foo#bar().", 10, 4, 7),         // f.bar!(): a call
			scipCallsiteRef(pre+"Foo#`<get>size`().", 11, 4, 8), // a getter read: no call
			scipCallsiteRef(pre+"go().", 12, 11, 13),            // UTF-16 column 11 is byte 12
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "Foo", Kind: "JSClass", FilePath: "m.ts", Line: 1, EndLine: 5},
		{Service: "s", Name: "Foo.constructor", Kind: "JSMethod", FilePath: "m.ts", Line: 2, EndLine: 2, Calls: []string{"this.bar"}},
		{Service: "s", Name: "Foo.size", Kind: "JSMethod", FilePath: "m.ts", Line: 3, EndLine: 3, Calls: []string{"helper"}},
		{Service: "s", Name: "Foo.bar", Kind: "JSMethod", FilePath: "m.ts", Line: 4, EndLine: 4},
		{Service: "s", Name: "helper", Kind: "JSFunc", FilePath: "m.ts", Line: 6, EndLine: 6},
		{Service: "s", Name: "run", Kind: "JSFunc", FilePath: "m.ts", Line: 7, EndLine: 14,
			Calls: []string{"Foo", "list.map", "cb", "f.bar", "log", "go"}},
	}
	return raw, src, nodes
}

// TestScoreSCIPTypeScriptCallSites (LB-F174, O440): scip-typescript's vocabulary
// read at the call site, as Python's and Go's are (LB-F65). A constructor
// reference is a call to the class written there (`new Foo(` calls Foo), a
// constructor or accessor definition takes the name the language gives it
// (as SCIPDefs already names them for L1), a term, a parameter or a local
// written as a call is one, a callable referenced without a call (a value, a
// getter read) is not, `!(` and type arguments still read as a call, and a
// column is a UTF-16 code unit, so a call after a non-ASCII character on its
// line is still read where it is written.
func TestScoreSCIPTypeScriptCallSites(t *testing.T) {
	raw, src, nodes := tsCallSiteFixture(t)
	read := func(string) []byte { return []byte(src) }
	for _, dialect := range []string{"typescript", "javascript"} {
		opts := SCIPOptionsFor(dialect, read)
		opts.OracleBlindSites = false
		res, err := ScoreSCIPWith(raw, "s", nodes, opts)
		if err != nil {
			t.Fatal(err)
		}
		want := map[CallPair]bool{
			{"m.ts", "constructor", "bar"}: true, {"m.ts", "size", "helper"}: true,
			{"m.ts", "run", "Foo"}: true, {"m.ts", "run", "map"}: true, {"m.ts", "run", "cb"}: true,
			{"m.ts", "run", "bar"}: true, {"m.ts", "run", "go"}: true,
		}
		if !reflect.DeepEqual(res.CallTruth, want) {
			t.Errorf("%s: call truth = %v, want %v", dialect, res.CallTruth, want)
		}
		if res.Counts[CntScipCallsUnwritten] != 1 {
			t.Errorf("%s: unwritten = %d, want 1 (the getter read)", dialect, res.Counts[CntScipCallsUnwritten])
		}
		if res.Counts[CntScipCalledNames] != 2 {
			t.Errorf("%s: called names = %d, want 2 (the arrow const, the parameter)", dialect, res.Counts[CntScipCalledNames])
		}
		if res.Counts[CntScipDefsMapped] != 6 {
			t.Errorf("%s: mapped definitions = %d, want 6 (the constructor and the getter by their names)", dialect, res.Counts[CntScipDefsMapped])
		}
		if !res.CallFound[CallPair{"m.ts", "run", "log"}] || res.Counts[CntCallsOracleBlind] != 0 {
			t.Errorf("%s: with OracleBlindSites off, (run, log) must stay found and nothing counted blind", dialect)
		}
	}
}

// TestScoreSCIPOracleBlindSites (LB-F175, O440, design §1.4 amendment
// 2026-10-02): a fathom pair the truth lacks whose every written call site has
// no occurrence of the index at its callee's range is a call the oracle
// cannot judge (scip-typescript writes nothing where it resolves no type:
// `super(...)`, a member of an untyped receiver): out of both L2 counts and
// counted. A pair with one judged site, or with no site found, stays.
func TestScoreSCIPOracleBlindSites(t *testing.T) {
	raw, src, nodes := tsCallSiteFixture(t)
	nodes[5].Calls = append(nodes[5].Calls, "missing")
	// (Foo, bar): fathom attributes constructor's this.bar() to the class too;
	// the index encloses it in the constructor, so the truth lacks the pair, but
	// the index wrote an occurrence at its site: judged, so it stays.
	nodes[0].Calls = []string{"this.bar"}
	opts := SCIPOptionsFor("typescript", func(string) []byte { return []byte(src) })
	if !opts.OracleBlindSites {
		t.Fatal("typescript's SCIP options do not leave oracle-blind sites out")
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.CallFound[CallPair{"m.ts", "run", "log"}] {
		t.Error("(run, log): its one site has no occurrence, yet it is still scored")
	}
	if !res.CallFound[CallPair{"m.ts", "run", "missing"}] {
		t.Error("(run, missing): no written site was found, so it must stay scored")
	}
	if !res.CallFound[CallPair{"m.ts", "run", "go"}] {
		t.Error("(run, go): a truth pair was dropped")
	}
	if judged := (CallPair{"m.ts", "Foo", "bar"}); res.CallTruth[judged] || !res.CallFound[judged] {
		t.Errorf("(Foo, bar): want a fathom-only pair kept, its site judged by an occurrence (truth %v, found %v)",
			res.CallTruth[judged], res.CallFound[judged])
	}
	if res.Counts[CntCallsOracleBlind] != 1 {
		t.Errorf("oracle-blind = %d, want 1", res.Counts[CntCallsOracleBlind])
	}
	if res.Counts[CntCallFathom] != len(res.CallFound) {
		t.Errorf("call_fathom %d != scored pairs %d", res.Counts[CntCallFathom], len(res.CallFound))
	}
}

// TestScoreSCIPTypeScriptObjectLiteralProperties (LB-F413, owner ruling D2,
// K19, O747): scip-typescript defines every object literal's property, in any
// expression, as a global symbol of its file whose last descriptor is the meta
// `<name><n>:` (a counter per name and file). That is no definition an
// extractor names, so def_mapping leaves it out of its denominator, counted
// (scip_defs_object_literal) and disclosed, as LB-F239 leaves parameters;
// typed_recall's references to it, and every other count, are unchanged.
func TestScoreSCIPTypeScriptObjectLiteralProperties(t *testing.T) {
	const pre = "scip-typescript npm app 1.0.0 src/`a.ts`/"
	src := "export function make(data: number) {\n" + // 0
		"  return { success: true, data };\n" + // 1
		"}\n" + // 2
		"export const cfg = { mode: \"x\", success: false };\n" + // 3
		"export function use() {\n" + // 4
		"  return make(1).success && cfg.mode;\n" + // 5
		"}\n" // 6
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript", Version: "0.4.0"}},
		Documents: []*scippb.Document{{RelativePath: "src/a.ts", Occurrences: []*scippb.Occurrence{
			def(pre+"make().", []int32{0, 0, 2, 1}, 0, 16, 20),
			def(pre+"make().(data)", nil, 0, 21, 25),
			def(pre+"success0:", nil, 1, 11, 18),
			def(pre+"data0:", nil, 1, 25, 29),
			def(pre+"cfg.", nil, 3, 13, 16),
			def(pre+"mode0:", nil, 3, 21, 25),
			def(pre+"success1:", nil, 3, 32, 39),
			def(pre+"use().", []int32{4, 0, 6, 1}, 4, 16, 19),
			ref(pre+"make().", 5, 9, 13),
			ref(pre+"success0:", 5, 17, 24),
			ref(pre+"cfg.", 5, 28, 31),
			ref(pre+"mode0:", 5, 32, 36),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Package: "src/a", Name: "make", Kind: "JSFunc", FilePath: "src/a.ts", Line: 1, EndLine: 3},
		{Service: "s", Package: "src/a", Name: "cfg", Kind: "JSConst", FilePath: "src/a.ts", Line: 4, EndLine: 4},
		{Service: "s", Package: "src/a", Name: "use", Kind: "JSFunc", FilePath: "src/a.ts", Line: 5, EndLine: 7, Calls: []string{core.SymbolKey("s", "src/a", "make")}},
	}
	read := func(string) []byte { return []byte(src) }

	plain, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptions{Source: read})
	if err != nil {
		t.Fatal(err)
	}
	ts, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("typescript", read))
	if err != nil {
		t.Fatal(err)
	}
	if plain.Counts[CntScipDefs] != 7 || plain.Counts[CntScipDefsObjectLiteral] != 0 {
		t.Errorf("without the rule: defs %d object-literal %d, want 7 0", plain.Counts[CntScipDefs], plain.Counts[CntScipDefsObjectLiteral])
	}
	if ts.Counts[CntScipDefs] != 3 || ts.Counts[CntScipDefsObjectLiteral] != 4 || ts.Counts[CntScipDefsMapped] != 3 {
		t.Errorf("def_mapping: defs %d object-literal %d mapped %d, want 3 4 3", ts.Counts[CntScipDefs], ts.Counts[CntScipDefsObjectLiteral], ts.Counts[CntScipDefsMapped])
	}
	for _, k := range []string{CntScipEdges, CntScipEdgesTyped} {
		if plain.Counts[k] != ts.Counts[k] {
			t.Errorf("%s moved with the rule: %d -> %d; only def_mapping's denominator may", k, plain.Counts[k], ts.Counts[k])
		}
	}
	notes := SCIPRuleNotes("app", "scip-typescript", ts.Counts)
	found := false
	for _, n := range notes {
		if strings.Contains(n, "4 object-literal properties") && strings.Contains(n, CntScipDefsObjectLiteral) {
			found = true
		}
	}
	if !found {
		t.Errorf("SCIPRuleNotes = %q, want the object-literal disclosure with its count", notes)
	}
	if n := SCIPRuleNotes("app", "scip-typescript", plain.Counts); len(n) != 0 {
		t.Errorf("a reading the rule moved nothing for discloses %q", n)
	}
}
