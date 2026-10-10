package langbench

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIPRustMacroGenerated (LB-F249): a definition rust-analyzer finds in a macro's
// expansion (`rgtest!(r16, ..)` writes no `fn r16`) is written by no source line, so it is
// counted apart (scip_defs_macro_generated) and is in neither def_mapping's denominator nor
// numerator; the references inside it are no typed edge fathom could attribute either
// (scip_edges_macro_generated, out of typed_recall's denominator). A field, an enum
// variant and a definition written with its keyword stay. Another producer keeps all.
func TestScoreSCIPRustMacroGenerated(t *testing.T) {
	const pre = "rust-analyzer cargo app 0.1.0 "
	src := "fn real() {}\n" + // 0
		"rgtest!(r16, |dir| {\n" + // 1: defines fn r16
		"    real();\n" + // 2
		"});\n" + // 3
		"struct S { f: u8 }\n" // 4
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	doc := &scippb.Document{RelativePath: "src/lib.rs",
		Occurrences: []*scippb.Occurrence{
			def(pre+"real().", []int32{0, 0, 0, 12}, 0, 3, 7),
			def(pre+"r16().", []int32{1, 0, 3, 3}, 1, 8, 11),
			ref(pre+"real().", 2, 4, 8),
			def(pre+"S#", []int32{4, 0, 4, 18}, 4, 7, 8),
			def(pre+"S#f.", []int32{4, 11, 4, 16}, 4, 11, 12),
		},
		Symbols: []*scippb.SymbolInformation{
			{Symbol: pre + "real().", Kind: scippb.SymbolInformation_Function},
			{Symbol: pre + "r16().", Kind: scippb.SymbolInformation_Function},
			{Symbol: pre + "S#", Kind: scippb.SymbolInformation_Struct},
			{Symbol: pre + "S#f.", Kind: scippb.SymbolInformation_Field},
		}}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "rust-analyzer"}}, Documents: []*scippb.Document{doc}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Package: "lib", Name: "real", Kind: "RustFn", FilePath: "src/lib.rs", Line: 1, EndLine: 1},
		{Service: "s", Package: "lib", Name: "S", Kind: "RustStruct", FilePath: "src/lib.rs", Line: 5, EndLine: 5},
		{Service: "s", Package: "lib", Name: "S.f", Kind: core.KindRustField, FilePath: "src/lib.rs", Line: 5, EndLine: 5},
	}
	read := func(string) []byte { return []byte(src) }

	plain, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptions{Source: read})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Counts[CntScipDefs] != 4 || plain.Counts[CntScipDefsMapped] != 3 || plain.Counts[CntScipDefsMacroGenerated] != 0 {
		t.Errorf("without the rule: defs %d mapped %d generated %d, want 4 3 0", plain.Counts[CntScipDefs], plain.Counts[CntScipDefsMapped], plain.Counts[CntScipDefsMacroGenerated])
	}
	opts := SCIPOptionsFor("rust", read)
	if !opts.RustMacroDefs {
		t.Fatal("SCIPOptionsFor(rust) with a source reader must turn RustMacroDefs on")
	}
	got, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got.Counts[CntScipDefs] != 3 || got.Counts[CntScipDefsMapped] != 3 || got.Counts[CntScipDefsMacroGenerated] != 1 {
		t.Errorf("rust: defs %d mapped %d generated %d, want 3 3 1", got.Counts[CntScipDefs], got.Counts[CntScipDefsMapped], got.Counts[CntScipDefsMacroGenerated])
	}
	// The one reference, `real()` inside r16, comes from an unmapped definition: in
	// typed_recall's denominator without the rule, out of it (and counted) with it.
	if plain.Counts[CntScipEdges] != 1 || plain.Counts[CntScipEdgesTyped] != 0 {
		t.Errorf("without the rule: edges %d typed %d, want 1 0", plain.Counts[CntScipEdges], plain.Counts[CntScipEdgesTyped])
	}
	if got.Counts[CntScipEdges] != 0 || got.Counts[CntScipEdgesMacroGenerated] != 1 {
		t.Errorf("rust: edges %d generated %d, want 0 1", got.Counts[CntScipEdges], got.Counts[CntScipEdgesMacroGenerated])
	}
	// L2 is untouched: the generated function is no mapped definition either way.
	if len(got.CallTruth) != len(plain.CallTruth) {
		t.Errorf("call truth changed: %v vs %v", got.CallTruth, plain.CallTruth)
	}
	// Without a source reader the rule cannot read a prefix and stays off.
	if SCIPOptionsFor("rust", nil).RustMacroDefs {
		t.Error("RustMacroDefs on without a source reader")
	}
}

func TestRustGeneratedDef(t *testing.T) {
	lines := [][]byte{[]byte("pub(crate) fn declared() {}"), []byte("syntax!(literal1, \"a\");"), []byte("static ref USAGES: u8 = 1;"), []byte("    r#async,")}
	for _, c := range []struct {
		kind scippb.SymbolInformation_Kind
		r    [4]int32
		want bool
	}{
		{scippb.SymbolInformation_Function, [4]int32{0, 14, 0, 22}, false}, // fn declared
		{scippb.SymbolInformation_Function, [4]int32{1, 8, 1, 16}, true},   // syntax!(literal1
		{scippb.SymbolInformation_Struct, [4]int32{2, 11, 2, 17}, true},    // lazy_static's `static ref`
		{scippb.SymbolInformation_Field, [4]int32{3, 4, 3, 11}, false},     // a field has no keyword
		{scippb.SymbolInformation_EnumMember, [4]int32{3, 4, 3, 11}, false},
		{scippb.SymbolInformation_Module, [4]int32{1, 8, 1, 16}, false},  // a file module is the owner's question, never this rule's
		{scippb.SymbolInformation_Function, [4]int32{1, 8, 2, 3}, false}, // a multi-line range is no written name
	} {
		if got := rustGeneratedDef(c.kind, lines, c.r); got != c.want {
			t.Errorf("rustGeneratedDef(%v, %v) = %v, want %v", c.kind, c.r, got, c.want)
		}
	}
}

// TestScoreSCIPRustNamespaceL4 (K18's Rust half, LB-F389): a Rust module is a namespace,
// file granularity, never a fathom symbol. rust-analyzer defines one per file module (its
// whole file), per crate root (`crate/`) and per inline `mod x { }`; those definitions
// leave def_mapping (scip_defs_namespace), a reference to one leaves typed_recall
// (scip_edges_namespace_target) and so does a reference made at a module's top level, a
// `use` line or an impl header, whose only enclosing definition is the module
// (scip_edges_namespace_from): the S1 shape of LB-F239 and Go's LB-F344, counted, never
// silent. Without the rule (any other producer) each stays an unmapped definition or
// endpoint.
func TestScoreSCIPRustNamespaceL4(t *testing.T) {
	const pre = "rust-analyzer cargo app 0.1.0 "
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	lib := "mod util;\n" + // 0
		"use crate::util::Helper;\n" + // 1
		"\n" + // 2
		"fn top() {\n" + // 3
		"    util::f();\n" + // 4
		"}\n" // 5
	util := "pub struct Helper;\npub fn f() {}\n"
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "rust-analyzer"}},
		Documents: []*scippb.Document{
			{RelativePath: "src/lib.rs", Occurrences: []*scippb.Occurrence{
				def(pre+"crate/", []int32{0, 0, 6, 0}, 0, 0, 6, 0),
				ref(pre+"util/", 0, 4, 8),
				ref(pre+"crate/", 1, 4, 9),
				ref(pre+"util/", 1, 11, 15),
				ref(pre+"util/Helper#", 1, 17, 23),
				def(pre+"top().", []int32{3, 0, 5, 1}, 3, 3, 6),
				ref(pre+"util/", 4, 4, 8),
				ref(pre+"util/f().", 4, 10, 11),
			}, Symbols: []*scippb.SymbolInformation{
				{Symbol: pre + "crate/", Kind: scippb.SymbolInformation_Module},
				{Symbol: pre + "top().", Kind: scippb.SymbolInformation_Function},
			}},
			{RelativePath: "src/util.rs", Occurrences: []*scippb.Occurrence{
				def(pre+"util/", []int32{0, 0, 2, 0}, 0, 0, 2, 0),
				def(pre+"util/Helper#", []int32{0, 0, 0, 18}, 0, 11, 17),
				def(pre+"util/f().", []int32{1, 0, 1, 13}, 1, 7, 8),
			}, Symbols: []*scippb.SymbolInformation{
				{Symbol: pre + "util/", Kind: scippb.SymbolInformation_Module},
				{Symbol: pre + "util/Helper#", Kind: scippb.SymbolInformation_Struct},
				{Symbol: pre + "util/f().", Kind: scippb.SymbolInformation_Function},
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Package: "src/lib", Name: "util", Kind: "RustMod", FilePath: "src/lib.rs", Line: 1, EndLine: 1},
		{Service: "s", Package: "src/lib", Name: "top", Kind: "RustFn", FilePath: "src/lib.rs", Line: 4, EndLine: 6},
		{Service: "s", Package: "src/util", Name: "Helper", Kind: "RustStruct", FilePath: "src/util.rs", Line: 1, EndLine: 1},
		{Service: "s", Package: "src/util", Name: "f", Kind: "RustFn", FilePath: "src/util.rs", Line: 2, EndLine: 2},
	}
	src := map[string]string{"src/lib.rs": lib, "src/util.rs": util}
	read := func(p string) []byte { return []byte(src[p]) }

	plain, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptions{Source: read})
	if err != nil {
		t.Fatal(err)
	}
	if plain.Counts[CntScipDefs] != 5 || plain.Counts[CntScipDefsNamespace] != 0 {
		t.Errorf("without the rule: defs %d namespace %d, want 5 0", plain.Counts[CntScipDefs], plain.Counts[CntScipDefsNamespace])
	}
	// Without the rule internal/scip joins a whole-file module definition (line 0) to
	// whatever its file defines on line 1 by position (O527's shape): every reference to a
	// module, or made at a module's top level, reads as a typed edge from or to that
	// definition.
	if plain.Counts[CntScipEdges] != 6 || plain.Counts[CntScipEdgesTyped] != 6 {
		t.Errorf("without the rule: edges %d typed %d, want 6 6", plain.Counts[CntScipEdges], plain.Counts[CntScipEdgesTyped])
	}
	opts := SCIPOptionsFor("rust", read)
	got, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	if got.Counts[CntScipDefs] != 3 || got.Counts[CntScipDefsMapped] != 3 || got.Counts[CntScipDefsNamespace] != 2 {
		t.Errorf("rust: defs %d mapped %d namespace %d, want 3 3 2", got.Counts[CntScipDefs], got.Counts[CntScipDefsMapped], got.Counts[CntScipDefsNamespace])
	}
	// util/ three times and crate/ once are namespace targets; the reference to Helper on
	// the `use` line is made from the crate's top level; f() inside top is the typed edge.
	if got.Counts[CntScipEdgesNamespaceTarget] != 4 || got.Counts[CntScipEdgesNamespaceFrom] != 1 {
		t.Errorf("rust: namespace target %d from %d, want 4 1", got.Counts[CntScipEdgesNamespaceTarget], got.Counts[CntScipEdgesNamespaceFrom])
	}
	if got.Counts[CntScipEdges] != 1 || got.Counts[CntScipEdgesTyped] != 1 {
		t.Errorf("rust: edges %d typed %d, want 1 1", got.Counts[CntScipEdges], got.Counts[CntScipEdgesTyped])
	}
	notes := SCIPRuleNotes("app", "rust-analyzer", got.Counts)
	if len(notes) != 1 || !strings.Contains(notes[0], "app: a Rust module is a namespace") || !strings.Contains(notes[0], "2 module definitions") {
		t.Errorf("notes %q", notes)
	}
	if n := SCIPRuleNotes("app", "rust-analyzer", plain.Counts); len(n) != 0 {
		t.Errorf("without the rule no note: %q", n)
	}
	// L3 is untouched: a module reference is still a file edge of the truth.
	if !got.FileEdges[[2]string{"src/lib.rs", "src/util.rs"}] || len(got.FileEdges) != len(plain.FileEdges) {
		t.Errorf("file edges changed: %v vs %v", got.FileEdges, plain.FileEdges)
	}
}
