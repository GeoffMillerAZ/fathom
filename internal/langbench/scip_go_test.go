package langbench

import (
	"reflect"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIPGoPackageRefsNameNoFile (LB-F298): scip-go defines a package
// symbol at the package clause of EVERY file of the package, so a reference
// to the package (an import spec, the `b` of `b.Foo`) names no one file; the
// index's first document of the package stands in for it, a guess. A
// reference to a symbol defined in more than one document is no file edge
// (design §1.2 amendment, as scip-typescript's ambiguous module file is no
// import truth), counted apart, while the reference to Foo still draws
// main.go -> b/foo.go.
func TestScoreSCIPGoPackageRefsNameNoFile(t *testing.T) {
	const m = "scip-go gomod example.com/m v0.0.0 "
	const pkgB = m + "`example.com/m/b`/"
	def := func(sym string, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-go", Version: "0.2.7"}},
		Documents: []*scippb.Document{
			{RelativePath: "b/a.go", Occurrences: []*scippb.Occurrence{def(pkgB, 0, 8, 9)}},
			{RelativePath: "b/foo.go", Occurrences: []*scippb.Occurrence{def(pkgB, 0, 8, 9), def(pkgB+"Foo().", 2, 5, 8)}},
			{RelativePath: "main.go", Occurrences: []*scippb.Occurrence{ref(pkgB, 5, 1, 18), ref(pkgB, 9, 13, 14), ref(pkgB+"Foo().", 9, 15, 18)}},
			{RelativePath: "only.go", Occurrences: []*scippb.Occurrence{ref(pkgB, 5, 1, 18)}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	res, err := ScoreSCIPWith(raw, "s", nil, SCIPOptionsFor("go", nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.FileEdges) != 1 || !res.FileEdges[[2]string{"main.go", "b/foo.go"}] {
		t.Errorf("file edges = %v, want only main.go -> b/foo.go (the Foo reference)", res.FileEdges)
	}
	// main.go -> b/a.go and only.go -> b/a.go are drawn by the package symbol alone.
	if got := res.Counts[CntScipFileEdgesMultiDef]; got != 2 {
		t.Errorf("%s = %d, want 2 (main.go -> b/a.go, only.go -> b/a.go)", CntScipFileEdgesMultiDef, got)
	}
}

// TestScoreSCIPGoDefClasses (LB-F299): scip-go's definitions that fathom has
// no node for are counted by descriptor class (package, field, interface
// method, ...), with the defined total beside, so the def_mapping misses are
// classified in the reading rather than left a bare ratio.
func TestScoreSCIPGoDefClasses(t *testing.T) {
	const m = "scip-go gomod example.com/m v0.0.0 "
	const pkg = m + "`example.com/m`/"
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-go", Version: "0.2.7"}},
		Documents: []*scippb.Document{{RelativePath: "a.go",
			Occurrences: []*scippb.Occurrence{
				def(pkg, nil, 0, 8, 9),
				def(pkg+"T#", []int32{2, 0, 4, 1}, 2, 5, 6),
				def(pkg+"T#field.", nil, 3, 1, 6),
				def(pkg+"I#Do.", nil, 6, 1, 3),
				def(pkg+"Run().", []int32{8, 0, 8, 12}, 8, 5, 8),
				def(pkg+"T#M().", []int32{9, 0, 9, 12}, 9, 9, 10),
				def(pkg+"Max.", nil, 11, 6, 9),
			},
			Symbols: []*scippb.SymbolInformation{
				{Symbol: pkg + "T#field.", Kind: scippb.SymbolInformation_Field},
				{Symbol: pkg + "I#Do.", Kind: scippb.SymbolInformation_Method},
				{Symbol: pkg + "Max.", Kind: scippb.SymbolInformation_Constant},
			}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	// Only T, Run and M are fathom's.
	nodes := []core.SymbolNode{
		{Name: "T", FilePath: "a.go", Line: 3, EndLine: 5, Kind: core.KindGoType},
		{Name: "Run", FilePath: "a.go", Line: 9, EndLine: 9, Kind: core.KindGoFunc},
		{Name: "T.M", FilePath: "a.go", Line: 10, EndLine: 10, Kind: core.KindGoMethod},
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("go", nil))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"scip_go_defs_package": 1, "scip_go_defs_unmapped_package": 1,
		"scip_go_defs_type": 1, "scip_go_defs_func": 1, "scip_go_defs_method": 1,
		"scip_go_defs_field": 1, "scip_go_defs_unmapped_field": 1,
		"scip_go_defs_interface-method": 1, "scip_go_defs_unmapped_interface-method": 1,
		"scip_go_defs_package-const": 1, "scip_go_defs_unmapped_package-const": 1,
	}
	got := map[string]int{}
	for k, v := range res.Counts {
		if len(k) > 11 && k[:11] == "scip_go_def" {
			got[k] = v
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("go def class counts = %v, want %v", got, want)
	}
}

// TestScoreSCIPGoDropsDocumentsOutsideTheSubject (LB-F299): `scip-go index
// ./...` indexes the testmain each test package's `go test` build generates
// into the Go build cache, a document whose path leaves the subject
// (`../../gocache/ab/...-d`) with an init() and a main() no extractor of the
// subject reads. Such a document is no subject file (design §1.4: the oracle
// is judged over the files the subject holds): left out of every count, with
// the documents counted apart.
func TestScoreSCIPGoDropsDocumentsOutsideTheSubject(t *testing.T) {
	const m = "scip-go gomod example.com/m v0.0.0 "
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-go", Version: "0.2.7"}},
		Documents: []*scippb.Document{
			{RelativePath: "a.go", Occurrences: []*scippb.Occurrence{def(m+"`example.com/m`/Run().", []int32{0, 0, 0, 12}, 0, 5, 8)}},
			{RelativePath: "../../../gocache/e1/abc-d", Occurrences: []*scippb.Occurrence{
				def(m+"`example.com/m.test`/main().", []int32{3, 0, 6, 1}, 3, 5, 9),
				ref(m+"`example.com/m`/Run().", 4, 1, 4),
			}},
		}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{{Name: "Run", FilePath: "a.go", Line: 1, EndLine: 1, Kind: core.KindGoFunc}}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptionsFor("go", nil))
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[CntScipDefs] != 1 || res.Counts[CntScipDefsMapped] != 1 {
		t.Errorf("scip_defs = %d, mapped = %d, want 1 and 1 (the testmain's main() is no subject definition)", res.Counts[CntScipDefs], res.Counts[CntScipDefsMapped])
	}
	if res.Counts[CntScipEdges] != 0 {
		t.Errorf("scip_edges = %d, want 0 (the testmain's reference is no subject edge)", res.Counts[CntScipEdges])
	}
	if res.Docs["../../../gocache/e1/abc-d"] || !res.Docs["a.go"] {
		t.Errorf("docs = %v, want only a.go", res.Docs)
	}
	if got := res.Counts[CntScipGoDocsOutside]; got != 1 {
		t.Errorf("%s = %d, want 1", CntScipGoDocsOutside, got)
	}
}
