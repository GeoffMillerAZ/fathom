package langbench

import (
	"testing"

	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
	"google.golang.org/protobuf/proto"
)

// TestScoreSCIPDartLocalSymbolsAreDocumentScoped (LB-F245): scip-dart numbers
// its `local N` symbols per document (SCIP: a local symbol is valid in its own
// document only), so `local 0` in b.dart, a local variable, is not the private
// method `local 0` that a.dart defines. A reference to a local symbol is never a
// cross-file edge; a reference to a public symbol still is.
func TestScoreSCIPDartLocalSymbolsAreDocumentScoped(t *testing.T) {
	const pre = "scip-dart pub app . lib/`a.dart`/"
	srcA := "class Foo {\n  void _helper() {}\n}\n"
	srcB := "void run() {\n  var x = 1;\n  print(x);\n}\n"
	srcC := "void go() {\n  Foo();\n}\n"
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(scippb.SymbolRole_Definition), EnclosingRange: enc}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dart"}},
		Documents: []*scippb.Document{
			{RelativePath: "lib/a.dart",
				Occurrences: []*scippb.Occurrence{
					def(pre+"Foo#", []int32{0, 0, 2, 1}, 0, 6, 9),
					def("local 0", []int32{1, 2, 1, 19}, 1, 7, 14),
				},
				Symbols: []*scippb.SymbolInformation{{Symbol: "local 0", Kind: scippb.SymbolInformation_Method}}},
			{RelativePath: "lib/b.dart",
				Occurrences: []*scippb.Occurrence{
					def("local 0", []int32{1, 2, 1, 12}, 1, 6, 7),
					ref("local 0", 2, 8, 9),
				},
				Symbols: []*scippb.SymbolInformation{{Symbol: "local 0"}}},
			{RelativePath: "lib/c.dart", Occurrences: []*scippb.Occurrence{ref(pre+"Foo#", 1, 2, 5)}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	src := func(p string) []byte {
		switch p {
		case "lib/a.dart":
			return []byte(srcA)
		case "lib/b.dart":
			return []byte(srcB)
		}
		return []byte(srcC)
	}
	res, err := ScoreSCIPWith(raw, "s", nil, SCIPOptionsFor("dart", src))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.FileEdges) != 1 || !res.FileEdges[[2]string{"lib/c.dart", "lib/a.dart"}] {
		t.Errorf("file edges = %v, want only lib/c.dart -> lib/a.dart (the public Foo)", res.FileEdges)
	}
}
