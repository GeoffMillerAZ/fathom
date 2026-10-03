package langbench

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestRubySCIPOptions (LB-F257): scip-ruby writes no occurrence where Sorbet
// resolves no method (a call on a receiver it types T.untyped, `super`), so
// Ruby's SCIP options leave a fathom pair the truth lacks out of both counts
// when no written call site of it carries an occurrence (OracleBlindSites,
// under LB-F203's 10% cap); a pair with an occurrence at its site, or with no
// site written as a call, stays scored.
func TestRubySCIPOptions(t *testing.T) {
	const pre = "scip-ruby gem app 1.0 "
	src := "class Runner\n" + // 0
		"  def run(server)\n" + // 1
		"    helper(1)\n" + // 2
		"    server.pop(2)\n" + // 3
		"    stop\n" + // 4
		"    finish(3)\n" + // 5
		"  end\n" + // 6
		"  def helper(x) = x\n" + // 7
		"  def finish(x) = x\n" + // 8
		"end\n" // 9
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-ruby", Version: "0.5.0"}},
		Documents: []*scippb.Document{{RelativePath: "r.rb", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"Runner#", []int32{0, 0, 9, 3}, 0, 6, 12),
			scipCallsiteDef(pre+"Runner#run().", []int32{1, 2, 6, 5}, 1, 6, 9),
			scipCallsiteDef(pre+"Runner#helper().", []int32{7, 2, 7, 20}, 7, 6, 12),
			scipCallsiteDef(pre+"Runner#finish().", []int32{8, 2, 8, 20}, 8, 6, 12),
			scipCallsiteRef(pre+"Runner#helper().", 2, 4, 10),                    // resolved: truth
			scipCallsiteRef("scip-ruby gem other 1.0 Other#finish().", 5, 4, 10), // resolved elsewhere: judged
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "Runner", Kind: "RubyClass", FilePath: "r.rb", Line: 1, EndLine: 10},
		{Service: "s", Name: "Runner.run", Kind: "RubyMethod", FilePath: "r.rb", Line: 2, EndLine: 7,
			Calls: []string{"helper", "pop", "stop", "finish", "missing"}},
		{Service: "s", Name: "Runner.helper", Kind: "RubyMethod", FilePath: "r.rb", Line: 8, EndLine: 8},
		{Service: "s", Name: "Runner.finish", Kind: "RubyMethod", FilePath: "r.rb", Line: 9, EndLine: 9},
	}
	opts := SCIPOptionsFor("ruby", func(string) []byte { return []byte(src) })
	if !opts.OracleBlindSites {
		t.Fatal("ruby's SCIP options do not leave oracle-blind sites out")
	}
	if SCIPOptionsFor("ruby", nil).OracleBlindSites {
		t.Error("with no source to read, no site can be judged blind")
	}
	res, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.CallFound[CallPair{"r.rb", "run", "pop"}] {
		t.Error("(run, pop): `server.pop(` carries no occurrence, yet it is still scored")
	}
	for _, kept := range []string{"helper", "finish", "stop", "missing"} {
		if !res.CallFound[CallPair{"r.rb", "run", kept}] {
			t.Errorf("(run, %s): dropped; only a pair whose every written site has no occurrence is blind", kept)
		}
	}
	if res.Counts[CntCallsOracleBlind] != 1 {
		t.Errorf("oracle-blind = %d, want 1", res.Counts[CntCallsOracleBlind])
	}
}
