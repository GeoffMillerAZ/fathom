package langbench

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/geoffmilleraz/fathom/internal/core"
	scippb "github.com/geoffmilleraz/fathom/third_party/scip"
)

// TestSCIPOptionsForCSharpOracleBlind (O464, LB-F175): C# is scored with the
// oracle-blind exclusion on when the source can be read, off when it cannot.
func TestSCIPOptionsForCSharpOracleBlind(t *testing.T) {
	if opts := SCIPOptionsFor("csharp", func(string) []byte { return nil }); !opts.OracleBlindSites || opts.Source == nil {
		t.Errorf("csharp with a source: OracleBlindSites %v, Source set %v, want both", opts.OracleBlindSites, opts.Source != nil)
	}
	if opts := SCIPOptionsFor("csharp", nil); opts.OracleBlindSites {
		t.Error("csharp without a source: OracleBlindSites must stay off (no written site can be read)")
	}
}

// TestScoreSCIPCSharpOracleBlindSites (O464, O271): scip-dotnet writes no
// occurrence for the name of a generic method called with explicit type
// arguments (`Workloads.FuncAsync<int>()`, `Make<List<int>>(2)`), so a fathom
// pair whose every written site is `name<...>(` with nothing of the index
// there is one the oracle cannot judge: left out of both L2 counts and
// counted. A site the index did write keeps its pair scored, and a pair with
// no written site found stays. scip-dotnet carries no enclosing_range: the
// mapped fathom definition's lines stand in (D5).
func TestScoreSCIPCSharpOracleBlindSites(t *testing.T) {
	const pre = "scip-dotnet nuget . . App/"
	src := "class C {\n" + // 0
		"  void Run(Registry reg) {\n" + // 1
		"    reg.Add(1);\n" + // 2
		"    Workloads.FuncAsync<int>();\n" + // 3
		"    Make<List<int>>(2);\n" + // 4
		"    reg.Touch<int>();\n" + // 5
		"  }\n" + // 6
		"}\n" // 7
	idx := &scippb.Index{Metadata: &scippb.Metadata{ToolInfo: &scippb.ToolInfo{Name: "scip-dotnet", Version: "0.2.14"}},
		Documents: []*scippb.Document{{RelativePath: "C.cs", Occurrences: []*scippb.Occurrence{
			scipCallsiteDef(pre+"C#", nil, 0, 6, 7),
			scipCallsiteDef(pre+"C#Run().", nil, 1, 7, 10),
			scipCallsiteRef(pre+"Registry#", 1, 11, 19),
			scipCallsiteRef(pre+"Registry#Add().", 2, 8, 11), // reg.Add(1): judged, a truth pair
			scipCallsiteRef(pre+"Workloads#", 3, 4, 13),      // the receiver's type, never the method
			scipCallsiteRef("scip-dotnet nuget System . System/Int32#", 3, 24, 27),
			scipCallsiteRef("scip-dotnet nuget System . System/Collections/Generic/List`1#", 4, 9, 13),
			scipCallsiteRef(pre+"Registry#Touch().", 5, 8, 13), // the index did write this generic call
		}}}}
	raw, err := proto.Marshal(idx)
	if err != nil {
		t.Fatal(err)
	}
	nodes := []core.SymbolNode{
		{Service: "s", Name: "C", Kind: "CSharpClass", FilePath: "C.cs", Line: 1, EndLine: 8},
		{Service: "s", Name: "C.Run", Kind: "CSharpMethod", FilePath: "C.cs", Line: 2, EndLine: 7,
			Calls: []string{"reg.Add", "Workloads.FuncAsync", "Make", "reg.Touch", "Unwritten"}},
	}
	opts := SCIPOptionsFor("csharp", func(string) []byte { return []byte(src) })
	res, err := ScoreSCIPWith(raw, "s", nodes, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, blind := range []string{"FuncAsync", "Make"} {
		if res.CallFound[CallPair{"C.cs", "Run", blind}] {
			t.Errorf("(Run, %s): written only as `%s<...>(` with no occurrence there, yet still scored", blind, blind)
		}
	}
	if p := (CallPair{"C.cs", "Run", "Add"}); !res.CallTruth[p] || !res.CallFound[p] {
		t.Errorf("(Run, Add): want a found truth pair (truth %v, found %v)", res.CallTruth[p], res.CallFound[p])
	}
	if p := (CallPair{"C.cs", "Run", "Touch"}); !res.CallTruth[p] || !res.CallFound[p] {
		t.Errorf("(Run, Touch): the index wrote `Touch<int>(`, want a found truth pair (truth %v, found %v)", res.CallTruth[p], res.CallFound[p])
	}
	if !res.CallFound[CallPair{"C.cs", "Run", "Unwritten"}] {
		t.Error("(Run, Unwritten): no written site was found, so it must stay scored")
	}
	if res.Counts[CntCallsOracleBlind] != 2 {
		t.Errorf("oracle-blind = %d, want 2 (FuncAsync, Make)", res.Counts[CntCallsOracleBlind])
	}
	if res.Counts[CntCallFathom] != len(res.CallFound) {
		t.Errorf("call_fathom %d != scored pairs %d", res.Counts[CntCallFathom], len(res.CallFound))
	}
}

// TestCSharpGenericWrittenSites (O464): the written-site match reads a C#
// generic method's explicit type arguments between its name and `(`, nested
// ones included, and a comparison or a lambda arrow is no call.
func TestCSharpGenericWrittenSites(t *testing.T) {
	cases := []struct {
		line   string
		callee string
		want   int
	}{
		{"x.Handle<InvalidOperationException>();", "Handle", 1},
		{"JsonConvert.DeserializeObject<IList<decimal>>(s);", "DeserializeObject", 1},
		{"Pair<System.Exception, string>(5);", "Pair", 1},
		{"Make<(int, string)[]>(x);", "Make", 1},
		{"if (Count < Max) return;", "Count", 0},
		{"Func<int, int> f = n => Sq(n);", "Func", 0},
	}
	for _, tc := range cases {
		lines := [][]byte{[]byte(tc.line)}
		if got := len(tsWrittenCallSites(lines, 1, 1, tc.callee)); got != tc.want {
			t.Errorf("%q: %d written sites of %s, want %d", tc.line, got, tc.callee, tc.want)
		}
	}
}
