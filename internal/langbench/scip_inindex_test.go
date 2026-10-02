package langbench

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestScoreSCIPCallInIndex (language-breadth §1.6): a truth pair whose callee
// symbol the index itself defines is marked CallInIndex; a call to a
// dependency's symbol, defined in no document of the index, is truth but not
// in it, so Go's L2 can grade the calls a service can serve.
func TestScoreSCIPCallInIndex(t *testing.T) {
	const m = "scip-go gomod example.com/m v0.0.0 "
	const dep = "scip-go gomod example.com/dep v1.0.0 "
	def := func(sym string, enc []int32, rng ...int32) *scippb.Occurrence {
		return &scippb.Occurrence{Symbol: sym, Range: rng, EnclosingRange: enc, SymbolRoles: int32(scippb.SymbolRole_Definition)}
	}
	ref := func(sym string, rng ...int32) *scippb.Occurrence { return &scippb.Occurrence{Symbol: sym, Range: rng} }
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-go", Version: "0.2.7"}},
		Documents: []*scippb.Document{{RelativePath: "a.go", Occurrences: []*scippb.Occurrence{
			def(m+"`example.com/m`/run().", []int32{0, 0, 3, 1}, 0, 5, 8),
			ref(m+"`example.com/m`/helper().", 1, 1, 7),
			ref(dep+"`example.com/dep`/Fetch().", 2, 1, 6),
			def(m+"`example.com/m`/helper().", []int32{4, 0, 4, 20}, 4, 5, 11),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "run", Kind: core.KindGoFunc, FilePath: "a.go", Line: 1, EndLine: 4},
		{Service: "s", Name: "helper", Kind: core.KindGoFunc, FilePath: "a.go", Line: 5, EndLine: 5},
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, SCIPOptions{})
	if err != nil {
		t.Fatal(err)
	}
	helper, fetch := CallPair{"a.go", "run", "helper"}, CallPair{"a.go", "run", "Fetch"}
	if !res.CallTruth[helper] || !res.CallTruth[fetch] {
		t.Fatalf("truth = %v, want both calls", res.CallTruth)
	}
	if !res.CallInIndex[helper] || res.CallInIndex[fetch] {
		t.Errorf("CallInIndex = %v, want helper (defined in the index) and not Fetch (a dependency's)", res.CallInIndex)
	}
}
