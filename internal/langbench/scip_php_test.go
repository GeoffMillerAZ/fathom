package langbench

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestSCIPOptionsForPHPOracleBlind (O464, LB-F175): PHP is scored with the
// oracle-blind exclusion on when the source can be read, off when it cannot.
func TestSCIPOptionsForPHPOracleBlind(t *testing.T) {
	if opts := SCIPOptionsFor("php", func(string) []byte { return nil }); !opts.OracleBlindSites || opts.Source == nil {
		t.Errorf("php with a source: OracleBlindSites %v, Source set %v, want both", opts.OracleBlindSites, opts.Source != nil)
	}
	if opts := SCIPOptionsFor("php", nil); opts.OracleBlindSites {
		t.Error("php without a source: OracleBlindSites must stay off (no written site can be read)")
	}
}

// TestScoreSCIPPHPOracleBlindSites (O464): scip-php writes no occurrence for
// a method called on a receiver it cannot type (`$r->getUri()` on an untyped
// parameter), so a fathom pair whose every written site has nothing of the
// index at the callee is left out of both L2 counts and counted; a typed
// call (`$this->transfer(`) stays a judged truth pair, and a pair with no
// written site found stays scored. A variable's own name (`$getUri`) is no
// written site of `getUri`.
func TestScoreSCIPPHPOracleBlindSites(t *testing.T) {
	const pre = "scip-php composer acme/app 1.0.0 Acme/"
	src := "<?php\n" + // 0
		"class Client {\n" + // 1
		"    public function send($r) {\n" + // 2
		"        $this->transfer($r);\n" + // 3
		"        $r->getUri();\n" + // 4
		"        $getUri = 1;\n" + // 5
		"    }\n" + // 6
		"    public function transfer($r) {}\n" + // 7
		"}\n" // 8
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-php", Version: "71a5b117"}},
		Documents: []*scippb.Document{{RelativePath: "src/Client.php", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"Client#", nil, 1, 6, 12),
			scipCallsiteDef(pre+"Client#send().", nil, 2, 20, 24),
			scipCallsiteDef(pre+"Client#transfer().", nil, 7, 20, 28),
			scipCallsiteRef(pre+"Client#transfer().", 3, 15, 23),
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "Client", Kind: "PhpClass", FilePath: "src/Client.php", Line: 2, EndLine: 9},
		{Service: "s", Name: "Client.send", Kind: "PhpMethod", FilePath: "src/Client.php", Line: 3, EndLine: 7,
			Calls: []string{"this.transfer", "r.getUri", "unwritten"}},
		{Service: "s", Name: "Client.transfer", Kind: "PhpMethod", FilePath: "src/Client.php", Line: 8, EndLine: 8},
	}
	opts := SCIPOptionsFor("php", func(string) []byte { return []byte(src) })
	res, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	if res.CallFound[CallPair{"src/Client.php", "send", "getUri"}] {
		t.Error("(send, getUri): its one written site has no occurrence, yet it is still scored")
	}
	if p := (CallPair{"src/Client.php", "send", "transfer"}); !res.CallTruth[p] || !res.CallFound[p] {
		t.Errorf("(send, transfer): want a found truth pair (truth %v, found %v)", res.CallTruth[p], res.CallFound[p])
	}
	if !res.CallFound[CallPair{"src/Client.php", "send", "unwritten"}] {
		t.Error("(send, unwritten): no written site was found, so it must stay scored")
	}
	if res.Counts[CntCallsOracleBlind] != 1 {
		t.Errorf("oracle-blind = %d, want 1 (getUri)", res.Counts[CntCallsOracleBlind])
	}
}
