package scip

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// goFieldFixture is a scip-go-shaped index of
//
//	0  package m
//	1  type U struct{}
//	2  type T struct {
//	3  	f U
//	4  }
//	5  func Run(t T) { _ = t.f }
//
// where fathom's extractor has a node for U, T and Run, and none for the
// package symbol or the field f.
func goFieldFixture() (*scippb.Index, []core.SymbolNode) {
	const (
		pkg  = "scip-go gomod example.com/m v0.0.0 `example.com/m`/"
		symU = pkg + "U#"
		symT = pkg + "T#"
		symF = pkg + "T#f."
		symR = pkg + "Run()."
	)
	def := func(sym string, r, enc []int32) *scippb.Occurrence {
		return &scippb.Occurrence{Range: r, EnclosingRange: enc, Symbol: sym, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-go", Version: "0.2.7"}},
		Documents: []*scippb.Document{{RelativePath: "a.go", Occurrences: []*scippb.Occurrence{
			def(pkg, []int32{0, 8, 0, 9}, nil),
			def(symU, []int32{1, 5, 1, 6}, []int32{1, 0, 1, 15}),
			def(symT, []int32{2, 5, 2, 6}, []int32{2, 0, 4, 1}),
			def(symF, []int32{3, 1, 3, 2}, []int32{3, 1, 3, 4}),
			{Range: []int32{3, 3, 3, 4}, Symbol: symU}, // the field's type, inside f's range
			def(symR, []int32{5, 5, 5, 8}, []int32{5, 0, 5, 26}),
			{Range: []int32{5, 20, 5, 21}, Symbol: symT}, // t's type, in Run
			{Range: []int32{5, 22, 5, 23}, Symbol: symF}, // t.f
			{Range: []int32{5, 12, 5, 13}, Symbol: pkg},  // a reference to the package
		}}}}
	nodes := []core.SymbolNode{
		{Service: "svc", Package: "m", Name: "U", FilePath: "a.go", Line: 2, EndLine: 2, Kind: core.KindGoType},
		{Service: "svc", Package: "m", Name: "T", FilePath: "a.go", Line: 3, EndLine: 5, Kind: core.KindGoType},
		{Service: "svc", Package: "m", Name: "Run", FilePath: "a.go", Line: 6, EndLine: 6, Kind: core.KindGoFunc},
	}
	return idx, nodes
}

// TestImportIndexGoSkippedByClass (LB-F299): an occurrence skipped as an
// unmapped endpoint under scip-go is also counted by the descriptor class of
// the symbol on the unmapped side, so typed_recall's misses are classified in
// the reading: here a reference to a field (the target unmapped) and, before
// the climb, a reference inside a field. The reference to the package symbol
// is no unmapped endpoint since K18 (TestImportIndexGoNamespaceTarget).
func TestImportIndexGoSkippedByClass(t *testing.T) {
	idx, nodes := goFieldFixture()
	imp := importOf(t, idx, nodes)
	got := imp.Report.SkippedByClass
	want := map[string]int{"from:field": 1, "to:field": 1, SkipNamespaceTarget: 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SkippedByClass = %v, want %v", got, want)
	}
}

// TestImportIndexGoSkippedFromTestFile (O673's first lever measured): an
// unmapped endpoint in a _test.go document is also counted under
// SkipTestFilePrefix and its class, a subset of the class count, so the
// reading says how much of typed_recall's field miss a member pass storing
// no field reference from a test file would leave. A non-test document
// counts none.
func TestImportIndexGoSkippedFromTestFile(t *testing.T) {
	idx, nodes := goFieldFixture()
	idx.Documents[0].RelativePath = "a_test.go"
	for i := range nodes {
		nodes[i].FilePath = "a_test.go"
	}
	got := importOf(t, idx, nodes).Report.SkippedByClass
	want := map[string]int{"from:field": 1, "to:field": 1, SkipNamespaceTarget: 1,
		SkipTestFilePrefix + "from:field": 1, SkipTestFilePrefix + "to:field": 1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SkippedByClass = %v, want %v", got, want)
	}
}

// TestImportIndexGoNamespaceTarget (K18, wave lf2, LB-F344): a Go package is
// a namespace, never a fathom symbol (file granularity, graded at L3 by file
// edges), so a reference to the package symbol scip-go defines at every
// file's package clause is no typed edge missed: it leaves the unmapped
// endpoints and is counted apart (Skipped["namespace-target"], the S1 shape
// of LB-F239's parameter targets), for scip-go alone.
func TestImportIndexGoNamespaceTarget(t *testing.T) {
	idx, nodes := goFieldFixture()
	rep := importOf(t, idx, nodes).Report
	if rep.Skipped[SkipNamespaceTarget] != 1 || rep.Skipped["unmapped-endpoint"] != 2 {
		t.Fatalf("scip-go Skipped = %v, want namespace-target 1 and unmapped-endpoint 2 (the field's two)", rep.Skipped)
	}
	idx.Metadata.ToolInfo.Name = "scip-other"
	rep = importOf(t, idx, nodes).Report
	if rep.Skipped[SkipNamespaceTarget] != 0 || rep.Skipped["unmapped-endpoint"] != 3 {
		t.Fatalf("another producer's Skipped = %v, want unmapped-endpoint 3 and no namespace rule", rep.Skipped)
	}
}

// TestGoScopedTermIsAParameter (LB-F299, S1): scip-go writes most parameters
// and locals as `local N`, which the local rule (LB-F55) already keeps out of
// typed_recall, but a parameter of a function literal in a package-level
// initializer, and a local of a function scope it names, as a term of a
// meta descriptor (`Options:c.`, `PrepareCmd:cmd.`), a shape neither the
// descriptor rule nor the local rule reached. It is a parameter definition;
// a field, a package variable and a method are not.
func TestGoScopedTermIsAParameter(t *testing.T) {
	const pre = "scip-go gomod github.com/cli/cli/v2 v2.69.0 `github.com/cli/cli/v2/internal/config`/"
	for sym, want := range map[string]bool{
		pre + "Options:c.":                 true,
		pre + "PrepareCmd:cmd.":            true,
		pre + "T#f.":                       false,
		pre + "Version.":                   false,
		pre + "T#M().":                     false,
		pre + "Run().":                     false,
		pre + "T#":                         false,
		"scip-typescript npm x 1 `a:b`/c.": false,
	} {
		if got := isParameterDefinition(sym, scippb.SymbolInformation_UnspecifiedKind); got != want {
			t.Errorf("isParameterDefinition(%q) = %v, want %v", sym, got, want)
		}
	}
}
