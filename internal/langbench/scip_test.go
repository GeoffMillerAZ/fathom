package langbench

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	fscip "github.com/geoffmilleraz/fathom/internal/scip"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

func tsFixtureNodes() []core.SymbolNode {
	return []core.SymbolNode{
		{Service: "s", Name: "add", Kind: core.KindJSFunc, FilePath: "math.ts", Line: 1},
		{Service: "s", Name: "Greeter", Kind: core.KindJSClass, FilePath: "greeter.ts", Line: 3},
		{Service: "s", Name: "Greeter.greet", Kind: core.KindJSMethod, FilePath: "greeter.ts", Line: 4, Calls: []string{"add", "console.log"}},
		{Service: "s", Name: "run", Kind: core.KindJSFunc, FilePath: "main.ts", Line: 3, Calls: []string{"g.greet", "made.up"}},
	}
}

// TestScoreSCIP (§1.2 L2–L4 rows, §1.4 calls): call truth from callable
// references inside mapped definitions; definitions mapped by position and
// name; cross-file edges at file granularity.
func TestScoreSCIP(t *testing.T) {
	raw, err := proto.Marshal(fscip.BuildTSFixtureIndexNoDisplayNames())
	if err != nil {
		t.Fatal(err)
	}
	res, err := ScoreSCIP(raw, "s", tsFixtureNodes())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		CntScipDefs: 4, CntScipDefsMapped: 4,
		CntCallTruth: 3, CntCallTruthFound: 3, CntCallFathom: 4, CntCallFathomInTrue: 3,
	}
	for k, v := range want {
		if res.Counts[k] != v {
			t.Errorf("%s = %d, want %d (all %v)", k, res.Counts[k], v, res.Counts)
		}
	}
	if res.Counts[CntScipEdges] == 0 || res.Counts[CntScipEdgesTyped] > res.Counts[CntScipEdges] {
		t.Errorf("typed counts %v", res.Counts)
	}
	if !res.FileEdges[[2]string{"greeter.ts", "math.ts"}] || !res.FileEdges[[2]string{"main.ts", "greeter.ts"}] || len(res.FileEdges) != 2 {
		t.Errorf("file edges = %v", res.FileEdges)
	}
	if res.Tool != "scip-typescript" {
		t.Errorf("tool = %q", res.Tool)
	}

	fathomEdges := []core.Edge{
		{FromFile: "main.ts", ToSymbol: tsFixtureNodes()[1].Key()},
		{FromFile: "main.ts", ToSymbol: tsFixtureNodes()[0].Key()},
	}
	counts := map[string]int{}
	ScoreFileEdges(res.FileEdges, fathomEdges, tsFixtureNodes(), counts)
	if counts[CntFileEdgesTruth] != 2 || counts[CntFileEdgesFathom] != 2 || counts[CntFileEdgesAgree] != 1 {
		t.Errorf("file edge counts = %v", counts)
	}
}

func TestScipBareName(t *testing.T) {
	for in, want := range map[string]string{
		"scip-typescript npm p 1 math/add().":      "add",
		"scip-typescript npm p 1 greeter/Greeter#": "Greeter",
		"scip-python python p 1 `a.b`/C#m().":      "m",
		"scip-java maven g a 1 pkg/Outer#Inner#f.": "f",
		"local 3": "",
		// scip-clang: a class template's constructor carries its arguments,
		// operators their own spelling.
		"cxx . . $ fmt/v12/detail/basic_fp#`basic_fp<F>`(9640a3c01162c49).":              "basic_fp",
		"cxx . . $ fmt/v12/basic_memory_buffer#append(edcd8b2679e9bd1a).":                "append",
		"cxx . . $ fmt/v12/detail/default_arg_formatter#`operator()`(818385b192fd64d1).": "operator()",
		"cxx . . $ fmt/v12/detail/`operator&`(30fe3d7215a1809e).":                        "operator&",
		"cxx . . $ n/D#`operator<=>`(1).":                                                "operator<=>",
		"cxx . . $ n/utf8_to_utf16#`operator basic_string_view`(2862b23f35b403d2).":      "basic_string_view",
		// rust-analyzer: an impl block's members sit under `impl#[Type]`.
		"rust-analyzer cargo grep-pcre2 0.1.8 matcher/impl#[RegexMatcherBuilder]build().": "build",
		"rust-analyzer cargo grep-pcre2 0.1.8 matcher/impl#[RegexMatcher][Matcher]Error#": "Error",
		"rust-analyzer cargo ripgrep 14.1.1 flags/Flag#is_switch().":                      "is_switch",
	} {
		if got, _ := scipBareName(in); got != want {
			t.Errorf("scipBareName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestScoreSCIPEnclosingFallback (owner decision D5): a producer that never
// sets occurrence.enclosing_range (scip-clang 0.4.0) still yields call truth
// — a definition mapped to a fathom definition takes that definition's
// line span as its enclosing range, and the result counts how many did.
func TestScoreSCIPEnclosingFallback(t *testing.T) {
	idx := fscip.BuildTSFixtureIndexNoDisplayNames()
	for _, d := range idx.Documents {
		for _, o := range d.Occurrences {
			o.EnclosingRange = nil
		}
	}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := tsFixtureNodes()
	for i, end := range []int{3, 8, 7, 6} {
		nodes[i].EndLine = end
	}
	res, err := ScoreSCIP(raw, "s", nodes)
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[CntCallTruth] != 3 || res.Counts[CntCallTruthFound] != 3 {
		t.Fatalf("call truth without enclosing ranges: %v", res.Counts)
	}
	if res.EnclosingFallback != 4 {
		t.Fatalf("EnclosingFallback = %d, want 4", res.EnclosingFallback)
	}
}

// TestSCIPDefs (owner decision D2): a SCIP index as a tier-A L1 oracle —
// every definition with its name, 1-based line, enclosing end line and a
// kind from SymbolInformation.kind, else the documentation's declaration
// keyword, else the descriptor suffix; scoped when its parent is a type.
func TestSCIPDefs(t *testing.T) {
	idx := fscip.BuildTSFixtureIndexNoDisplayNames()
	// greet documents itself the way scip-typescript's hover text does.
	idx.Documents[1].Symbols[1].Documentation = []string{"```ts\n(method) Greeter.greet(name: string): void\n```"}
	idx.Documents[1].Symbols[0].Kind = scippb.SymbolInformation_Class
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	defs, tool, _, err := SCIPDefs(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tool != "scip-typescript" {
		t.Errorf("tool %q", tool)
	}
	want := []OracleDef{
		{Path: "greeter.ts", Name: "Greeter", Line: 3, End: 8, Kind: "class"},
		{Path: "greeter.ts", Name: "greet", Line: 4, End: 7, Kind: "method", Scoped: true},
		{Path: "main.ts", Name: "run", Line: 3, End: 6, Kind: "function"},
		{Path: "math.ts", Name: "add", Line: 1, End: 3, Kind: "function"},
	}
	if !reflect.DeepEqual(defs, want) {
		t.Fatalf("SCIPDefs =\n%+v\nwant\n%+v", defs, want)
	}
	for in, want := range map[string]string{
		"```ts\nclass Foo\n```":                "class",
		"```ts\n(property) Foo.x: number\n```": "property",
		"```java\npublic interface I\n```":     "interface",
		"```ts\nconst x: 1\n```":               "const",
		"just prose, no declaration keyword":   "",
	} {
		if got := docKeyword(in); got != want {
			t.Errorf("docKeyword(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSCIPDefsFromSource: with the subject's sources, a local declaration
// (a function inside another definition) is named from its own occurrence
// range, a local variable is not a definition, an accessor loses its
// <get> marker, and a symbol with two definition occurrences (interface X
// and const X, merged) takes each one's kind from the keyword before it.
func TestSCIPDefsFromSource(t *testing.T) {
	const pre = "scip-typescript npm p 1 m/"
	def := func(sym string, line, c0, c1 int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: []int32{line, c0, c1}, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	doc := func(sym, d string) *scippb.SymbolInformation {
		return &scippb.SymbolInformation{Symbol: sym, Documentation: []string{"```ts\n" + d + "\n```"}}
	}
	src := "export interface X { a: 1 }\n" + // 0
		"export const X = mk();\n" + // 1
		"function outer() {\n" + // 2
		"  function inner() {}\n" + // 3
		"  const v = 1;\n" + // 4
		"}\n" + // 5
		"class C { get size() { return 1 } }\n" // 6
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-typescript"}},
		Documents: []*scippb.Document{{RelativePath: "m.ts",
			Occurrences: []*scippb.Occurrence{
				def(pre+"X#", 0, 17, 18), def(pre+"X#", 1, 13, 14),
				def(pre+"outer().", 2, 9, 14), def("local 1", 3, 11, 16), def("local 2", 4, 8, 9),
				def(pre+"C#", 6, 6, 7), def(pre+"C#`<get>size`().", 6, 14, 18),
			},
			Symbols: []*scippb.SymbolInformation{
				doc(pre+"X#", "interface X"), doc(pre+"outer().", "function outer(): void"),
				doc("local 1", "function inner(): void"), doc("local 2", "var v: 1"),
				doc(pre+"C#", "class C"), doc(pre+"C#`<get>size`().", "get size(): number"),
			}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	defs, _, _, err := SCIPDefs(raw, func(string) []byte { return []byte(src) })
	if err != nil {
		t.Fatal(err)
	}
	want := []OracleDef{
		{Path: "m.ts", Name: "X", Line: 1, Kind: "interface"},
		{Path: "m.ts", Name: "X", Line: 2, Kind: "const"},
		{Path: "m.ts", Name: "outer", Line: 3, Kind: "function"},
		{Path: "m.ts", Name: "inner", Line: 4, Kind: "function"},
		{Path: "m.ts", Name: "C", Line: 7, Kind: "class"},
		{Path: "m.ts", Name: "size", Line: 7, Kind: "get", Scoped: true},
	}
	if !reflect.DeepEqual(defs, want) {
		t.Fatalf("SCIPDefs =\n%+v\nwant\n%+v", defs, want)
	}
}

// TestSCIPDefsRustAnalyzerShapes (LB-Q-rust): rust-analyzer's whole-file
// `crate/` module occurrence (a multi-line range, no name anyone wrote) is not
// a definition, an impl block's member is scoped to a type (its parent
// descriptor is `impl#[Type]`), and SymbolInformation.kind names each
// definition's kind.
func TestSCIPDefsRustAnalyzerShapes(t *testing.T) {
	const pre = "rust-analyzer cargo app 0.1.0 "
	def := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(scippb.SymbolRole_Definition), EnclosingRange: []int32{rng[0], 0, rng[0] + 2, 1}}
	}
	info := func(sym string, k scippb.SymbolInformation_Kind) *scippb.SymbolInformation {
		return &scippb.SymbolInformation{Symbol: sym, Kind: k}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "rust-analyzer"}},
		Documents: []*scippb.Document{{RelativePath: "src/lib.rs",
			Occurrences: []*scippb.Occurrence{
				def(pre+"crate/", 0, 0, 9, 0),
				def(pre+"Foo#", 1, 11, 14),
				def(pre+"impl#[Foo]LIMIT.", 3, 10, 15),
				def(pre+"impl#[Foo]new().", 4, 11, 14),
				def(pre+"helper().", 8, 3, 9),
			},
			Symbols: []*scippb.SymbolInformation{
				info(pre+"crate/", scippb.SymbolInformation_Module),
				info(pre+"Foo#", scippb.SymbolInformation_Struct),
				info(pre+"impl#[Foo]LIMIT.", scippb.SymbolInformation_Constant),
				info(pre+"impl#[Foo]new().", scippb.SymbolInformation_StaticMethod),
				info(pre+"helper().", scippb.SymbolInformation_Function),
			}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	defs, tool, _, err := SCIPDefs(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if tool != "rust-analyzer" {
		t.Errorf("tool %q", tool)
	}
	want := []OracleDef{
		{Path: "src/lib.rs", Name: "Foo", Line: 2, End: 4, Kind: "struct"},
		{Path: "src/lib.rs", Name: "LIMIT", Line: 4, End: 6, Kind: "constant", Scoped: true},
		{Path: "src/lib.rs", Name: "new", Line: 5, End: 7, Kind: "staticmethod", Scoped: true},
		{Path: "src/lib.rs", Name: "helper", Line: 9, End: 11, Kind: "function"},
	}
	if !reflect.DeepEqual(defs, want) {
		t.Fatalf("SCIPDefs =\n%+v\nwant\n%+v", defs, want)
	}
}

// TestScoreSCIPWithRust (LB-Q-rust): rust-analyzer references a macro
// invocation by the macro's name and an operator by the operator's own
// range. Under the Rust options the macro is a call and the operator, which
// has no call written at its range, is not (and is counted).
func TestScoreSCIPWithRust(t *testing.T) {
	const pre = "rust-analyzer cargo app 0.1.0 "
	src := "fn run(x: bool) -> bool {\n" + // 0
		"    println!(\"hi\");\n" + // 1
		"    helper();\n" + // 2
		"    !x\n" + // 3
		"}\n" // 4
	occ := func(sym string, roles int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: roles}
	}
	run := occ(pre+"run().", int32(scippb.SymbolRole_Definition), 0, 3, 6)
	run.EnclosingRange = []int32{0, 0, 4, 1}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "rust-analyzer"}},
		Documents: []*scippb.Document{{RelativePath: "src/lib.rs", Occurrences: []*scippb.Occurrence{
			run,
			occ(pre+"macros/println!", 0, 1, 4, 11),
			occ(pre+"helper().", 0, 2, 4, 10),
			occ("rust-analyzer cargo core 1.0.0 ops/Not#not().", 0, 3, 4, 5),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{{Service: "s", Package: "lib", Name: "run", Kind: "RustFn", FilePath: "src/lib.rs", Line: 1, EndLine: 5}}
	src2 := func(string) []byte { return []byte(src) }

	plain, err := ScoreSCIP(raw, "s", nodes)
	if err != nil {
		t.Fatal(err)
	}
	if plain.Counts[CntCallTruth] != 2 { // helper and the operator's `not`; the macro is not callable
		t.Errorf("default options: call truth = %d, want 2 (%v)", plain.Counts[CntCallTruth], plain.CallTruth)
	}
	rust, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("rust", src2))
	if err != nil {
		t.Fatal(err)
	}
	want := map[CallPair]bool{{"src/lib.rs", "run", "println"}: true, {"src/lib.rs", "run", "helper"}: true}
	if !reflect.DeepEqual(rust.CallTruth, want) {
		t.Errorf("rust options: call truth = %v, want %v", rust.CallTruth, want)
	}
	if rust.Counts[CntScipCallsUnwritten] != 1 {
		t.Errorf("unwritten operator calls = %d, want 1", rust.Counts[CntScipCallsUnwritten])
	}
}

// TestSCIPDefsRustMacroAndFileModules (LB-Q-rust): a definition rust-analyzer
// finds in a macro's expansion has no declaration written at its position
// (`syntax!(literal1, ..)` writes no `fn literal1`), so it is marked
// Generated and ScoreL1 leaves it out and counts it; a file module's one
// definition is its whole file, so its declaration is recovered from the
// reference written after `mod`.
func TestSCIPDefsRustMacroAndFileModules(t *testing.T) {
	const pre = "rust-analyzer cargo app 0.1.0 "
	src := map[string]string{
		"src/lib.rs": "mod util;\n" + // 0
			"fn real() {}\n" + // 1
			"syntax!(literal1, \"a\");\n" + // 2
			"use crate::real;\n", // 3
		"src/util.rs": "pub fn helper() {}\n",
	}
	def := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(scippb.SymbolRole_Definition), EnclosingRange: []int32{rng[0], 0, rng[0], 30}}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	info := func(sym string, k scippb.SymbolInformation_Kind) *scippb.SymbolInformation {
		return &scippb.SymbolInformation{Symbol: sym, Kind: k}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "rust-analyzer"}}, Documents: []*scippb.Document{
		{RelativePath: "src/lib.rs", Occurrences: []*scippb.Occurrence{
			def(pre+"crate/", 0, 0, 3, 0),
			ref(pre+"util/", 0, 4, 8),
			def(pre+"real().", 1, 3, 7),
			def(pre+"literal1().", 2, 8, 16),
			ref(pre+"crate/", 3, 4, 9),
		}, Symbols: []*scippb.SymbolInformation{
			info(pre+"crate/", scippb.SymbolInformation_Module), info(pre+"real().", scippb.SymbolInformation_Function),
			info(pre+"literal1().", scippb.SymbolInformation_Function),
		}},
		{RelativePath: "src/util.rs", Occurrences: []*scippb.Occurrence{
			def(pre+"util/", 0, 0, 0, 0), // a whole-file module definition: multi-line only when the file is
			def(pre+"util/helper().", 0, 7, 13),
		}, Symbols: []*scippb.SymbolInformation{
			info(pre+"util/", scippb.SymbolInformation_Module), info(pre+"util/helper().", scippb.SymbolInformation_Function),
		}},
	}}
	// util.rs is a single line, so its module range must span lines to be a
	// whole-file occurrence.
	idx.Documents[1].Occurrences[0].Range = []int32{0, 0, 1, 0}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	defs, _, _, err := SCIPDefs(raw, func(p string) []byte { return []byte(src[p]) })
	if err != nil {
		t.Fatal(err)
	}
	want := []OracleDef{
		{Path: "src/lib.rs", Name: "util", Line: 1, End: 1, Kind: "module"},
		{Path: "src/lib.rs", Name: "real", Line: 2, End: 2, Kind: "function"},
		{Path: "src/lib.rs", Name: "literal1", Line: 3, End: 3, Kind: "function", Generated: true},
		{Path: "src/util.rs", Name: "helper", Line: 1, End: 1, Kind: "function"},
	}
	if !reflect.DeepEqual(defs, want) {
		t.Fatalf("SCIPDefs =\n%+v\nwant\n%+v", defs, want)
	}
	m := Manifest{ID: "rust", OracleKinds: map[string]map[string]string{"scip": {"function": "function", "module": "module"}},
		Kinds: map[string]KindSpec{"function": {SourceKind: "RustFn", Canonical: "function", Callable: true}, "mod": {SourceKind: "RustMod", Canonical: "module"}}}
	nodes := []core.SymbolNode{
		{Name: "real", Kind: "RustFn", FilePath: "src/lib.rs", Line: 2, EndLine: 2},
		{Name: "util", Kind: "RustMod", FilePath: "src/lib.rs", Line: 1, EndLine: 1},
		{Name: "helper", Kind: "RustFn", FilePath: "src/util.rs", Line: 1, EndLine: 1},
	}
	l1 := ScoreL1(m, "scip", nodes, defs)
	if l1.Counts[CntMatched] != 3 || l1.Counts[CntOracleDefs] != 3 || l1.Excluded["scip:macro-generated"] != 1 {
		t.Errorf("ScoreL1 counts %v excluded %v, want 3 matched of 3 oracle defs and one macro-generated exclusion", l1.Counts, l1.Excluded)
	}
}
